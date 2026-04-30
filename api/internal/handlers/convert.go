package handlers

import (
	"encoding/json"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// Conversion helpers between the GORM-tagged DB models and the CSIL-driven
// wire types. The DB layer carries time.Time + plain strings; CSIL carries
// RFC3339 strings + ID type aliases. Optional CSIL fields use pointers.

func ts(t time.Time) csil.Timestamp {
	return csil.Timestamp(t.UTC().Format(time.RFC3339))
}

func tsPtr(t *time.Time) *csil.Timestamp {
	if t == nil {
		return nil
	}
	v := ts(*t)
	return &v
}

func strPtrCopy(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

func roleToCSIL(r *models.Role) csil.Role {
	return csil.Role{
		RoleId:      csil.RoleID(r.RoleID),
		HouseId:     csil.HouseID(r.HouseID),
		Name:        r.Name,
		Description: strPtrCopy(r.Description),
		CreatedAt:   ts(r.CreatedAt),
		UpdatedAt:   ts(r.UpdatedAt),
	}
}

func groupToCSIL(g *models.Group) csil.Group {
	return csil.Group{
		GroupId:     csil.GroupID(g.GroupID),
		HouseId:     csil.HouseID(g.HouseID),
		Name:        g.Name,
		Description: strPtrCopy(g.Description),
		CreatedAt:   ts(g.CreatedAt),
		UpdatedAt:   ts(g.UpdatedAt),
	}
}

func groupsToCSIL(rs []models.Group) []csil.Group {
	out := make([]csil.Group, 0, len(rs))
	for i := range rs {
		out = append(out, groupToCSIL(&rs[i]))
	}
	return out
}

func skillToCSIL(s *models.Skill) csil.Skill {
	return csil.Skill{
		SkillId:     csil.SkillID(s.SkillID),
		HouseId:     csil.HouseID(s.HouseID),
		Name:        s.Name,
		Description: strPtrCopy(s.Description),
		CreatedAt:   ts(s.CreatedAt),
		UpdatedAt:   ts(s.UpdatedAt),
	}
}

func projectToCSIL(p *models.Project) csil.Project {
	out := csil.Project{
		ProjectId:   csil.ProjectID(p.ProjectID),
		HouseId:     csil.HouseID(p.HouseID),
		Name:        p.Name,
		Description: strPtrCopy(p.Description),
		CreatedAt:   ts(p.CreatedAt),
		UpdatedAt:   ts(p.UpdatedAt),
	}
	// CSIL string-enums generate as `interface{}` in Go; assign the
	// underlying string and take its address.
	if p.Status != "" {
		var s csil.ProjectStatus = p.Status
		out.Status = &s
	}
	return out
}

func eventToCSIL(e *models.Event) csil.Event {
	return csil.Event{
		EventId:       csil.EventID(e.EventID),
		HouseId:       csil.HouseID(e.HouseID),
		OwnerMemberId: csil.MemberID(e.OwnerMemberID),
		Title:         e.Title,
		Description:   strPtrCopy(e.Description),
		Location:      strPtrCopy(e.Location),
		StartsAt:      tsPtr(e.StartsAt),
		EndsAt:        tsPtr(e.EndsAt),
		AllDay:        boolPtr(e.AllDay),
		CreatedAt:     ts(e.CreatedAt),
		UpdatedAt:     ts(e.UpdatedAt),
	}
}

func eventsToCSIL(rs []models.Event) []csil.Event {
	out := make([]csil.Event, 0, len(rs))
	for i := range rs {
		out = append(out, eventToCSIL(&rs[i]))
	}
	return out
}

func boolPtr(b bool) *bool { v := b; return &v }

func memberToCSIL(m *models.Member) csil.Member {
	out := csil.Member{
		MemberId:       csil.MemberID(m.MemberID),
		HouseId:        csil.HouseID(m.HouseID),
		LinkkeysDomain: m.LinkkeysDomain,
		LinkkeysUserId: m.LinkkeysUserID,
		DisplayName:    strPtrCopy(m.DisplayName),
		CreatedAt:      ts(m.CreatedAt),
		UpdatedAt:      ts(m.UpdatedAt),
		LastSeenAt:     tsPtr(m.LastSeenAt),
	}
	if len(m.CachedPubKey) > 0 {
		key := m.CachedPubKey
		out.CachedPublicKey = &key
	}
	return out
}

func taskToCSIL(t *models.Task) csil.Task {
	out := csil.Task{
		TaskId:        csil.TaskID(t.TaskID),
		HouseId:       csil.HouseID(t.HouseID),
		OwnerMemberId: csil.MemberID(t.OwnerMemberID),
		Title:         t.Title,
		Description:   strPtrCopy(t.Description),
		DueAt:         tsPtr(t.DueAt),
		DeletedAt:     tsPtr(t.DeletedAt),
		CreatedAt:     ts(t.CreatedAt),
		UpdatedAt:     ts(t.UpdatedAt),
	}
	if t.AssignedToMemberID != nil {
		v := csil.MemberID(*t.AssignedToMemberID)
		out.AssignedToMemberId = &v
	}
	if t.AssignedToSkillID != nil {
		v := csil.SkillID(*t.AssignedToSkillID)
		out.AssignedToSkillId = &v
	}
	if t.ParentTaskID != nil {
		v := csil.TaskID(*t.ParentTaskID)
		out.ParentTaskId = &v
	}
	if t.RecurrenceRootTaskID != nil {
		v := csil.TaskID(*t.RecurrenceRootTaskID)
		out.RecurrenceRootTaskId = &v
	}
	if t.RecurrenceFreq != nil {
		var v csil.RecurrenceFreq = *t.RecurrenceFreq
		out.RecurrenceFreq = &v
	}
	if t.RecurrenceInterval > 0 {
		ri := int64(t.RecurrenceInterval)
		out.RecurrenceInterval = &ri
	}
	if len(t.RecurrenceByWeekday) > 0 {
		w := make([]int64, len(t.RecurrenceByWeekday))
		for i, d := range t.RecurrenceByWeekday {
			w[i] = int64(d)
		}
		out.RecurrenceByWeekday = w
	}
	out.NextRecurrenceAt = tsPtr(t.NextRecurrenceAt)
	if t.Status != "" {
		var s csil.TaskStatus = t.Status
		out.Status = &s
	}
	return out
}

func rolesToCSIL(rs []models.Role) []csil.Role {
	out := make([]csil.Role, 0, len(rs))
	for i := range rs {
		out = append(out, roleToCSIL(&rs[i]))
	}
	return out
}

func skillsToCSIL(rs []models.Skill) []csil.Skill {
	out := make([]csil.Skill, 0, len(rs))
	for i := range rs {
		out = append(out, skillToCSIL(&rs[i]))
	}
	return out
}

func projectsToCSIL(rs []models.Project) []csil.Project {
	out := make([]csil.Project, 0, len(rs))
	for i := range rs {
		out = append(out, projectToCSIL(&rs[i]))
	}
	return out
}

func membersToCSIL(rs []models.Member) []csil.Member {
	out := make([]csil.Member, 0, len(rs))
	for i := range rs {
		out = append(out, memberToCSIL(&rs[i]))
	}
	return out
}

func tasksToCSIL(rs []models.Task) []csil.Task {
	out := make([]csil.Task, 0, len(rs))
	for i := range rs {
		out = append(out, taskToCSIL(&rs[i]))
	}
	return out
}

func memberAuditToCSIL(a *models.MemberAudit) csil.MemberAudit {
	out := csil.MemberAudit{
		AuditId:         csil.MemberAuditID(a.AuditID),
		HouseId:         csil.HouseID(a.HouseID),
		SubjectMemberId: csil.MemberID(a.SubjectMemberID),
		Action:          a.Action,
		CreatedAt:       ts(a.CreatedAt),
	}
	if a.ActorMemberID != nil {
		v := csil.MemberID(*a.ActorMemberID)
		out.ActorMemberId = &v
	}
	if a.TargetType != nil {
		out.TargetType = a.TargetType
	}
	if a.TargetID != nil {
		out.TargetId = a.TargetID
	}
	// Detail is jsonb in the DB; in CSIL we modeled it as `text`. Render
	// to a JSON string for transport.
	if a.Detail != nil {
		if b, err := json.Marshal(a.Detail); err == nil {
			s := string(b)
			out.Detail = &s
		}
	}
	return out
}

func auditsToCSIL(rs []models.MemberAudit) []csil.MemberAudit {
	out := make([]csil.MemberAudit, 0, len(rs))
	for i := range rs {
		out = append(out, memberAuditToCSIL(&rs[i]))
	}
	return out
}

func shareToCSIL(s *models.Share) csil.Share {
	out := csil.Share{
		ShareId:        csil.ShareID(s.ShareID),
		HouseId:        csil.HouseID(s.HouseID),
		SharedBy:       csil.MemberID(s.SharedBy),
		LinkkeysDomain: s.LinkkeysDomain,
		LinkkeysUserId: s.LinkkeysUserID,
		ResourceType:   csil.ResourceType(s.ResourceType),
		ResourceId:     s.ResourceID,
		CreatedAt:      ts(s.CreatedAt),
		ExpiresAt:      tsPtr(s.ExpiresAt),
	}
	if s.AccessLevel != "" {
		var v csil.AccessLevel = s.AccessLevel
		out.AccessLevel = &v
	}
	return out
}

func sharesToCSIL(rs []models.Share) []csil.Share {
	out := make([]csil.Share, 0, len(rs))
	for i := range rs {
		out = append(out, shareToCSIL(&rs[i]))
	}
	return out
}
