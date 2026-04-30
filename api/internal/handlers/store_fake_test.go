package handlers

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

// memStore is an in-memory fake of the full Store interface used by the
// entity handler tests. It implements only the methods the handlers
// actually call; everything else inherits a panicking nil interface, which
// surfaces accidental uses immediately.
type memStore struct {
	store.Store

	mu sync.Mutex

	houses             []models.House
	members            []models.Member
	roles              []models.Role
	memberRoles        []models.MemberRole
	skills             []models.Skill
	memberSkills       []models.MemberSkill
	groups             []models.Group
	groupMembers       []models.GroupMember
	projects           []models.Project
	projectTasks       []models.ProjectTask
	tasks              []models.Task
	audits             []models.MemberAudit
	trustedDomainsList []models.TrustedDomain
	events             []models.Event
	comments           []models.Comment
	shares             []models.Share

	idSeq int
}

func newMemStore() *memStore { return &memStore{} }

func (m *memStore) nextID(prefix string) string {
	m.idSeq++
	return prefix + strconv.Itoa(m.idSeq)
}

// ----- house seeding helpers (test-only) -----

func (m *memStore) seedHouse(id, name string) {
	m.houses = append(m.houses, models.House{HouseID: id, Name: name})
}

func (m *memStore) seedMember(houseID, memberID, domain, userID string, roles ...string) {
	m.members = append(m.members, models.Member{
		MemberID:       memberID,
		HouseID:        houseID,
		LinkkeysDomain: domain,
		LinkkeysUserID: userID,
	})
	for _, name := range roles {
		var rid string
		for _, r := range m.roles {
			if r.HouseID == houseID && r.Name == name {
				rid = r.RoleID
				break
			}
		}
		if rid == "" {
			rid = m.nextID("role-")
			m.roles = append(m.roles, models.Role{RoleID: rid, HouseID: houseID, Name: name})
		}
		m.memberRoles = append(m.memberRoles, models.MemberRole{MemberID: memberID, RoleID: rid})
	}
}

// ----- House -----

func (m *memStore) GetHouseByID(_ context.Context, id string) (*models.House, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.houses {
		if m.houses[i].HouseID == id {
			return &m.houses[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// ----- Members -----

func (m *memStore) ListMembersByHouse(_ context.Context, houseID string, _, _ int) ([]models.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Member{}
	for _, x := range m.members {
		if x.HouseID == houseID {
			out = append(out, x)
		}
	}
	return out, nil
}

func (m *memStore) GetMemberByID(_ context.Context, memberID string) (*models.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.members {
		if m.members[i].MemberID == memberID {
			return &m.members[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memStore) UpdateMember(_ context.Context, in *models.Member) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.members {
		if m.members[i].MemberID == in.MemberID {
			m.members[i] = *in
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *memStore) FindMembersByLinkkeysIdentity(_ context.Context, domain, userID string) ([]models.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Member{}
	for _, x := range m.members {
		if x.LinkkeysDomain == domain && x.LinkkeysUserID == userID {
			out = append(out, x)
		}
	}
	return out, nil
}

// ----- Roles -----

func (m *memStore) CreateRole(_ context.Context, r *models.Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.RoleID = m.nextID("role-")
	m.roles = append(m.roles, *r)
	return nil
}

func (m *memStore) GetRoleByID(_ context.Context, id string) (*models.Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.roles {
		if m.roles[i].RoleID == id {
			return &m.roles[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memStore) UpdateRole(_ context.Context, in *models.Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.roles {
		if m.roles[i].RoleID == in.RoleID {
			m.roles[i] = *in
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *memStore) DeleteRole(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.roles[:0]
	for _, r := range m.roles {
		if r.RoleID != id {
			out = append(out, r)
		}
	}
	m.roles = out
	return nil
}

func (m *memStore) ListRolesByHouse(_ context.Context, houseID string, _, _ int) ([]models.Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Role{}
	for _, r := range m.roles {
		if r.HouseID == houseID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memStore) AssignRole(_ context.Context, memberID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memberRoles = append(m.memberRoles, models.MemberRole{MemberID: memberID, RoleID: roleID})
	return nil
}

func (m *memStore) RevokeRole(_ context.Context, memberID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.memberRoles[:0]
	for _, mr := range m.memberRoles {
		if mr.MemberID != memberID || mr.RoleID != roleID {
			out = append(out, mr)
		}
	}
	m.memberRoles = out
	return nil
}

func (m *memStore) ListRolesForMember(_ context.Context, memberID string) ([]models.Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Role{}
	for _, mr := range m.memberRoles {
		if mr.MemberID == memberID {
			for _, r := range m.roles {
				if r.RoleID == mr.RoleID {
					out = append(out, r)
				}
			}
		}
	}
	return out, nil
}

// ----- Skills -----

func (m *memStore) CreateSkill(_ context.Context, s *models.Skill) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s.SkillID = m.nextID("skill-")
	m.skills = append(m.skills, *s)
	return nil
}

func (m *memStore) GetSkillByID(_ context.Context, id string) (*models.Skill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.skills {
		if m.skills[i].SkillID == id {
			return &m.skills[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memStore) UpdateSkill(_ context.Context, in *models.Skill) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.skills {
		if m.skills[i].SkillID == in.SkillID {
			m.skills[i] = *in
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *memStore) DeleteSkill(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.skills[:0]
	for _, s := range m.skills {
		if s.SkillID != id {
			out = append(out, s)
		}
	}
	m.skills = out
	return nil
}

func (m *memStore) ListSkillsByHouse(_ context.Context, houseID string, _, _ int) ([]models.Skill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Skill{}
	for _, s := range m.skills {
		if s.HouseID == houseID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *memStore) AssignSkill(_ context.Context, memberID, skillID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memberSkills = append(m.memberSkills, models.MemberSkill{MemberID: memberID, SkillID: skillID})
	return nil
}

func (m *memStore) UnassignSkill(_ context.Context, memberID, skillID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.memberSkills[:0]
	for _, ms := range m.memberSkills {
		if ms.MemberID != memberID || ms.SkillID != skillID {
			out = append(out, ms)
		}
	}
	m.memberSkills = out
	return nil
}

func (m *memStore) ListSkillsForMember(_ context.Context, memberID string) ([]models.Skill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Skill{}
	for _, ms := range m.memberSkills {
		if ms.MemberID == memberID {
			for _, s := range m.skills {
				if s.SkillID == ms.SkillID {
					out = append(out, s)
				}
			}
		}
	}
	return out, nil
}

// ----- Projects -----

func (m *memStore) CreateProject(_ context.Context, p *models.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p.ProjectID = m.nextID("project-")
	m.projects = append(m.projects, *p)
	return nil
}

func (m *memStore) GetProjectByID(_ context.Context, id string) (*models.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.projects {
		if m.projects[i].ProjectID == id {
			return &m.projects[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memStore) UpdateProject(_ context.Context, in *models.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.projects {
		if m.projects[i].ProjectID == in.ProjectID {
			m.projects[i] = *in
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *memStore) DeleteProject(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.projects[:0]
	for _, p := range m.projects {
		if p.ProjectID != id {
			out = append(out, p)
		}
	}
	m.projects = out
	return nil
}

func (m *memStore) ListProjectsByHouse(_ context.Context, houseID string, _, _ int) ([]models.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Project{}
	for _, p := range m.projects {
		if p.HouseID == houseID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *memStore) AddProjectTask(_ context.Context, projectID, taskID string, position int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projectTasks = append(m.projectTasks, models.ProjectTask{ProjectID: projectID, TaskID: taskID, Position: position})
	return nil
}

func (m *memStore) RemoveProjectTask(_ context.Context, projectID, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.projectTasks[:0]
	for _, pt := range m.projectTasks {
		if pt.ProjectID != projectID || pt.TaskID != taskID {
			out = append(out, pt)
		}
	}
	m.projectTasks = out
	return nil
}

func (m *memStore) ListProjectTasks(_ context.Context, projectID string, _, _ int) ([]models.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Task{}
	for _, pt := range m.projectTasks {
		if pt.ProjectID == projectID {
			for _, t := range m.tasks {
				if t.TaskID == pt.TaskID && t.DeletedAt == nil {
					out = append(out, t)
				}
			}
		}
	}
	return out, nil
}

// ----- Tasks -----

func (m *memStore) CreateTask(_ context.Context, t *models.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.TaskID = m.nextID("task-")
	m.tasks = append(m.tasks, *t)
	return nil
}

func (m *memStore) GetTaskByID(_ context.Context, id string) (*models.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.tasks {
		if m.tasks[i].TaskID == id {
			return &m.tasks[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memStore) UpdateTask(_ context.Context, in *models.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.tasks {
		if m.tasks[i].TaskID == in.TaskID {
			m.tasks[i] = *in
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *memStore) DeleteTask(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.tasks[:0]
	for _, t := range m.tasks {
		if t.TaskID != id {
			out = append(out, t)
		}
	}
	m.tasks = out
	return nil
}

func (m *memStore) ListTasksByHouse(_ context.Context, houseID string, _, _ int) ([]models.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Task{}
	for _, t := range m.tasks {
		if t.HouseID == houseID && t.DeletedAt == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

// ----- Groups -----

func (m *memStore) CreateGroup(_ context.Context, g *models.Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g.GroupID = m.nextID("group-")
	m.groups = append(m.groups, *g)
	return nil
}

func (m *memStore) GetGroupByID(_ context.Context, id string) (*models.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.groups {
		if m.groups[i].GroupID == id {
			return &m.groups[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memStore) UpdateGroup(_ context.Context, in *models.Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.groups {
		if m.groups[i].GroupID == in.GroupID {
			m.groups[i] = *in
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *memStore) DeleteGroup(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.groups[:0]
	for _, g := range m.groups {
		if g.GroupID != id {
			out = append(out, g)
		}
	}
	m.groups = out
	return nil
}

func (m *memStore) ListGroupsByHouse(_ context.Context, houseID string, _, _ int) ([]models.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Group{}
	for _, g := range m.groups {
		if g.HouseID == houseID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (m *memStore) AddGroupMember(_ context.Context, groupID, memberID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupMembers = append(m.groupMembers, models.GroupMember{GroupID: groupID, MemberID: memberID})
	return nil
}

func (m *memStore) RemoveGroupMember(_ context.Context, groupID, memberID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.groupMembers[:0]
	for _, gm := range m.groupMembers {
		if gm.GroupID != groupID || gm.MemberID != memberID {
			out = append(out, gm)
		}
	}
	m.groupMembers = out
	return nil
}

func (m *memStore) ListGroupMembers(_ context.Context, groupID string) ([]models.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Member{}
	for _, gm := range m.groupMembers {
		if gm.GroupID == groupID {
			for _, mem := range m.members {
				if mem.MemberID == gm.MemberID {
					out = append(out, mem)
				}
			}
		}
	}
	return out, nil
}

// ----- Trusted domains -----

func (m *memStore) CreateTrustedDomain(_ context.Context, td *models.TrustedDomain) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.trustedDomainsList {
		if existing.HouseID == td.HouseID && existing.Domain == td.Domain {
			return errors.New("trusted_domain already exists")
		}
	}
	td.TrustedDomainID = m.nextID("td-")
	m.trustedDomainsList = append(m.trustedDomainsList, *td)
	return nil
}

func (m *memStore) DeleteTrustedDomain(_ context.Context, tdID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.trustedDomainsList[:0]
	for _, td := range m.trustedDomainsList {
		if td.TrustedDomainID != tdID {
			out = append(out, td)
		}
	}
	m.trustedDomainsList = out
	return nil
}

func (m *memStore) ListTrustedDomains(_ context.Context, houseID string) ([]models.TrustedDomain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.TrustedDomain{}
	for _, td := range m.trustedDomainsList {
		if td.HouseID == houseID {
			out = append(out, td)
		}
	}
	return out, nil
}

func (m *memStore) IsDomainTrusted(_ context.Context, houseID, domain string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, td := range m.trustedDomainsList {
		if td.HouseID == houseID && td.Domain == domain {
			return true, nil
		}
	}
	return false, nil
}

func (m *memStore) HousesTrustingDomain(_ context.Context, domain string) ([]models.House, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.House{}
	seen := map[string]bool{}
	for _, td := range m.trustedDomainsList {
		if td.Domain == domain && !seen[td.HouseID] {
			seen[td.HouseID] = true
			for _, h := range m.houses {
				if h.HouseID == td.HouseID {
					out = append(out, h)
				}
			}
		}
	}
	return out, nil
}

// ----- Events -----

func (m *memStore) seedEvent(e models.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.EventID == "" {
		e.EventID = m.nextID("event-")
	}
	m.events = append(m.events, e)
}

func (m *memStore) CreateEvent(_ context.Context, e *models.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.EventID = m.nextID("event-")
	m.events = append(m.events, *e)
	return nil
}

func (m *memStore) GetEventByID(_ context.Context, id string) (*models.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.events {
		if m.events[i].EventID == id {
			return &m.events[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memStore) UpdateEvent(_ context.Context, in *models.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.events {
		if m.events[i].EventID == in.EventID {
			m.events[i] = *in
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *memStore) DeleteEvent(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.events[:0]
	for _, e := range m.events {
		if e.EventID != id {
			out = append(out, e)
		}
	}
	m.events = out
	return nil
}

func (m *memStore) ListEventsByHouse(_ context.Context, houseID string, _, _ int) ([]models.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Event{}
	for _, e := range m.events {
		if e.HouseID == houseID {
			out = append(out, e)
		}
	}
	return out, nil
}

// ----- Comments -----

func (m *memStore) CreateComment(_ context.Context, c *models.Comment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.CommentID = m.nextID("comment-")
	m.comments = append(m.comments, *c)
	return nil
}

func (m *memStore) GetCommentByID(_ context.Context, id string) (*models.Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.comments {
		if m.comments[i].CommentID == id {
			return &m.comments[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memStore) DeleteComment(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.comments[:0]
	for _, c := range m.comments {
		if c.CommentID != id {
			out = append(out, c)
		}
	}
	m.comments = out
	return nil
}

func (m *memStore) ListCommentsByTarget(_ context.Context, targetType, targetID string, _, _ int) ([]models.Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Comment{}
	for _, c := range m.comments {
		if c.TargetType == targetType && c.TargetID == targetID {
			out = append(out, c)
		}
	}
	return out, nil
}

// ----- Audits -----

func (m *memStore) RecordMemberAudit(_ context.Context, a *models.MemberAudit) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a.AuditID = m.nextID("audit-")
	m.audits = append(m.audits, *a)
	return nil
}

func (m *memStore) ListAuditsForMember(_ context.Context, memberID string, _, _ int) ([]models.MemberAudit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.MemberAudit{}
	for _, a := range m.audits {
		if a.SubjectMemberID == memberID {
			out = append(out, a)
		}
	}
	return out, nil
}

// ----- Shares -----

func (m *memStore) seedShare(s models.Share) {
	if s.ShareID == "" {
		s.ShareID = m.nextID("share-")
	}
	m.shares = append(m.shares, s)
}

func (m *memStore) CreateShare(_ context.Context, s *models.Share) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.ShareID == "" {
		s.ShareID = m.nextID("share-")
	}
	m.shares = append(m.shares, *s)
	return nil
}

func (m *memStore) GetShareByID(_ context.Context, id string) (*models.Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.shares {
		if m.shares[i].ShareID == id {
			cp := m.shares[i]
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memStore) DeleteShare(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.shares {
		if m.shares[i].ShareID == id {
			m.shares = append(m.shares[:i], m.shares[i+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *memStore) ListSharesByResource(_ context.Context, rt, rid string) ([]models.Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Share{}
	for _, s := range m.shares {
		if s.ResourceType == rt && s.ResourceID == rid {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *memStore) ListSharesByHouse(_ context.Context, houseID string) ([]models.Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []models.Share{}
	for _, s := range m.shares {
		if s.HouseID == houseID {
			out = append(out, s)
		}
	}
	return out, nil
}

// silence "declared but not used" errors for the unused error import
var _ = errors.New
