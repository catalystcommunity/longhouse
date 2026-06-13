package postgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"github.com/jackc/pgx/v4/pgxpool"
	logrus "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

type PostgresStore struct{}

func (s *PostgresStore) Initialize() (func(), error) {
	uri := config.DbUri
	maxRetries := 30
	retryInterval := 2 * time.Second

	pgxpoolConfig, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return nil, fmt.Errorf("parsing database config: %w", err)
	}

	var pool *pgxpool.Pool
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err = pgxpool.ConnectConfig(context.Background(), pgxpoolConfig)
		if err == nil {
			break
		}
		if attempt == maxRetries {
			return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
		}
		logrus.WithError(err).Warnf("Database connection attempt %d/%d failed, retrying in %v", attempt, maxRetries, retryInterval)
		time.Sleep(retryInterval)
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Error,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err = gorm.Open(postgres.Open(uri), &gorm.Config{
		Logger: gormLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("opening gorm connection: %w", err)
	}

	return func() { pool.Close() }, nil
}

// House operations

func (s *PostgresStore) CreateHouse(ctx context.Context, house *models.House) error {
	return db.WithContext(ctx).Create(house).Error
}

func (s *PostgresStore) GetHouseByID(ctx context.Context, houseID string) (*models.House, error) {
	var house models.House
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).First(&house).Error; err != nil {
		return nil, err
	}
	return &house, nil
}

func (s *PostgresStore) UpdateHouse(ctx context.Context, house *models.House) error {
	return db.WithContext(ctx).Save(house).Error
}

func (s *PostgresStore) DeleteHouse(ctx context.Context, houseID string) error {
	return db.WithContext(ctx).Where("house_id = ?", houseID).Delete(&models.House{}).Error
}

func (s *PostgresStore) ListHouses(ctx context.Context, limit, offset int) ([]models.House, error) {
	var houses []models.House
	if err := db.WithContext(ctx).Order("created_at DESC").Limit(limit).Offset(offset).Find(&houses).Error; err != nil {
		return nil, err
	}
	return houses, nil
}

// Member operations

func (s *PostgresStore) CreateMember(ctx context.Context, member *models.Member) error {
	return db.WithContext(ctx).Create(member).Error
}

func (s *PostgresStore) GetMemberByID(ctx context.Context, memberID string) (*models.Member, error) {
	var member models.Member
	if err := db.WithContext(ctx).Where("member_id = ?", memberID).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (s *PostgresStore) GetMemberByIdentity(ctx context.Context, houseID, domain, userID string) (*models.Member, error) {
	var member models.Member
	if err := db.WithContext(ctx).Where("house_id = ? AND linkkeys_domain = ? AND linkkeys_user_id = ?", houseID, domain, userID).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (s *PostgresStore) FindMembersByLinkkeysIdentity(ctx context.Context, domain, userID string) ([]models.Member, error) {
	var members []models.Member
	if err := db.WithContext(ctx).Where("linkkeys_domain = ? AND linkkeys_user_id = ?", domain, userID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (s *PostgresStore) UpdateMember(ctx context.Context, member *models.Member) error {
	return db.WithContext(ctx).Save(member).Error
}

func (s *PostgresStore) DeleteMember(ctx context.Context, memberID string) error {
	return db.WithContext(ctx).Where("member_id = ?", memberID).Delete(&models.Member{}).Error
}

func (s *PostgresStore) ListMembersByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Member, error) {
	var members []models.Member
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("display_name ASC").Limit(limit).Offset(offset).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

// Role operations

func (s *PostgresStore) CreateRole(ctx context.Context, role *models.Role) error {
	return db.WithContext(ctx).Create(role).Error
}

func (s *PostgresStore) GetRoleByID(ctx context.Context, roleID string) (*models.Role, error) {
	var role models.Role
	if err := db.WithContext(ctx).Where("role_id = ? AND deleted_at IS NULL", roleID).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *PostgresStore) GetRoleByName(ctx context.Context, houseID, name string) (*models.Role, error) {
	var role models.Role
	if err := db.WithContext(ctx).Where("house_id = ? AND name = ?", houseID, name).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *PostgresStore) UpdateRole(ctx context.Context, role *models.Role) error {
	return db.WithContext(ctx).Save(role).Error
}

func (s *PostgresStore) DeleteRole(ctx context.Context, roleID string) error {
	return db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&models.Role{}).Error
}

func (s *PostgresStore) ListRolesByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Role, error) {
	var roles []models.Role
	if err := db.WithContext(ctx).Where("house_id = ? AND deleted_at IS NULL", houseID).Order("name ASC").Limit(limit).Offset(offset).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (s *PostgresStore) AssignRole(ctx context.Context, memberID, roleID string) error {
	mr := &models.MemberRole{MemberID: memberID, RoleID: roleID}
	return db.WithContext(ctx).Create(mr).Error
}

func (s *PostgresStore) RevokeRole(ctx context.Context, memberID, roleID string) error {
	return db.WithContext(ctx).Where("member_id = ? AND role_id = ?", memberID, roleID).Delete(&models.MemberRole{}).Error
}

func (s *PostgresStore) ListRolesForMember(ctx context.Context, memberID string) ([]models.Role, error) {
	var roles []models.Role
	err := db.WithContext(ctx).
		Joins("JOIN member_roles mr ON mr.role_id = roles.role_id").
		Where("mr.member_id = ? AND roles.deleted_at IS NULL", memberID).
		Order("roles.name ASC").
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	return roles, nil
}

// Skill operations

func (s *PostgresStore) CreateSkill(ctx context.Context, skill *models.Skill) error {
	return db.WithContext(ctx).Create(skill).Error
}

func (s *PostgresStore) GetSkillByID(ctx context.Context, skillID string) (*models.Skill, error) {
	var skill models.Skill
	if err := db.WithContext(ctx).Where("skill_id = ? AND deleted_at IS NULL", skillID).First(&skill).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

func (s *PostgresStore) UpdateSkill(ctx context.Context, skill *models.Skill) error {
	return db.WithContext(ctx).Save(skill).Error
}

func (s *PostgresStore) DeleteSkill(ctx context.Context, skillID string) error {
	return db.WithContext(ctx).Where("skill_id = ?", skillID).Delete(&models.Skill{}).Error
}

func (s *PostgresStore) ListSkillsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Skill, error) {
	var skills []models.Skill
	if err := db.WithContext(ctx).Where("house_id = ? AND deleted_at IS NULL", houseID).Order("name ASC").Limit(limit).Offset(offset).Find(&skills).Error; err != nil {
		return nil, err
	}
	return skills, nil
}

func (s *PostgresStore) AssignSkill(ctx context.Context, memberID, skillID string) error {
	ms := &models.MemberSkill{MemberID: memberID, SkillID: skillID}
	return db.WithContext(ctx).Create(ms).Error
}

func (s *PostgresStore) UnassignSkill(ctx context.Context, memberID, skillID string) error {
	return db.WithContext(ctx).Where("member_id = ? AND skill_id = ?", memberID, skillID).Delete(&models.MemberSkill{}).Error
}

func (s *PostgresStore) ListSkillsForMember(ctx context.Context, memberID string) ([]models.Skill, error) {
	var skills []models.Skill
	err := db.WithContext(ctx).
		Joins("JOIN member_skills ms ON ms.skill_id = skills.skill_id").
		Where("ms.member_id = ? AND skills.deleted_at IS NULL", memberID).
		Order("skills.name ASC").
		Find(&skills).Error
	if err != nil {
		return nil, err
	}
	return skills, nil
}

// Group-skill: a skill the group as a whole holds. Independent from
// per-member skills.

func (s *PostgresStore) AssignGroupSkill(ctx context.Context, groupID, skillID string) error {
	gs := &models.GroupSkill{GroupID: groupID, SkillID: skillID}
	return db.WithContext(ctx).Create(gs).Error
}

func (s *PostgresStore) UnassignGroupSkill(ctx context.Context, groupID, skillID string) error {
	return db.WithContext(ctx).
		Where("group_id = ? AND skill_id = ?", groupID, skillID).
		Delete(&models.GroupSkill{}).Error
}

func (s *PostgresStore) ListSkillsForGroup(ctx context.Context, groupID string) ([]models.Skill, error) {
	var skills []models.Skill
	err := db.WithContext(ctx).
		Joins("JOIN group_skills gs ON gs.skill_id = skills.skill_id").
		Where("gs.group_id = ? AND skills.deleted_at IS NULL", groupID).
		Order("skills.name ASC").
		Find(&skills).Error
	if err != nil {
		return nil, err
	}
	return skills, nil
}

// Group operations

func (s *PostgresStore) CreateGroup(ctx context.Context, group *models.Group) error {
	return db.WithContext(ctx).Create(group).Error
}

func (s *PostgresStore) GetGroupByID(ctx context.Context, groupID string) (*models.Group, error) {
	var g models.Group
	if err := db.WithContext(ctx).Where("group_id = ? AND deleted_at IS NULL", groupID).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *PostgresStore) UpdateGroup(ctx context.Context, group *models.Group) error {
	return db.WithContext(ctx).Save(group).Error
}

func (s *PostgresStore) DeleteGroup(ctx context.Context, groupID string) error {
	return db.WithContext(ctx).Where("group_id = ?", groupID).Delete(&models.Group{}).Error
}

func (s *PostgresStore) ListGroupsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Group, error) {
	var groups []models.Group
	if err := db.WithContext(ctx).Where("house_id = ? AND deleted_at IS NULL", houseID).Order("name ASC").Limit(limit).Offset(offset).Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *PostgresStore) AddGroupMember(ctx context.Context, groupID, memberID string) error {
	gm := &models.GroupMember{GroupID: groupID, MemberID: memberID}
	return db.WithContext(ctx).Create(gm).Error
}

func (s *PostgresStore) RemoveGroupMember(ctx context.Context, groupID, memberID string) error {
	return db.WithContext(ctx).Where("group_id = ? AND member_id = ?", groupID, memberID).Delete(&models.GroupMember{}).Error
}

func (s *PostgresStore) ListGroupMembers(ctx context.Context, groupID string) ([]models.Member, error) {
	var members []models.Member
	err := db.WithContext(ctx).
		Joins("JOIN group_members gm ON gm.member_id = members.member_id").
		Where("gm.group_id = ?", groupID).
		Order("members.display_name ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// Recurrence helpers

func (s *PostgresStore) ListDueRecurringTasks(ctx context.Context, before time.Time, limit int) ([]models.Task, error) {
	if limit <= 0 {
		limit = 100
	}
	var tasks []models.Task
	err := db.WithContext(ctx).
		Where("recurrence_freq IS NOT NULL AND next_recurrence_at IS NOT NULL AND next_recurrence_at <= ? AND deleted_at IS NULL", before).
		Order("next_recurrence_at ASC").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *PostgresStore) LatestRecurrenceChildOf(ctx context.Context, rootTaskID string) (*models.Task, error) {
	var task models.Task
	err := db.WithContext(ctx).
		Where("recurrence_root_task_id = ? AND deleted_at IS NULL", rootTaskID).
		Order("created_at DESC").
		First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// Project operations

func (s *PostgresStore) CreateProject(ctx context.Context, project *models.Project) error {
	return db.WithContext(ctx).Create(project).Error
}

func (s *PostgresStore) GetProjectByID(ctx context.Context, projectID string) (*models.Project, error) {
	var project models.Project
	if err := db.WithContext(ctx).Where("project_id = ? AND deleted_at IS NULL", projectID).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (s *PostgresStore) UpdateProject(ctx context.Context, project *models.Project) error {
	return db.WithContext(ctx).Save(project).Error
}

func (s *PostgresStore) DeleteProject(ctx context.Context, projectID string) error {
	// Clear any dependency edges touching this project before the hard delete
	// (the dependencies table has no FK back to projects). Best-effort: a
	// failure here shouldn't block the delete, but surface it if it occurs.
	if err := s.RemoveDependenciesForNode(ctx, models.DependencyProject, projectID); err != nil {
		return err
	}
	return db.WithContext(ctx).Where("project_id = ?", projectID).Delete(&models.Project{}).Error
}

// NewID returns a fresh ULID (as text) using the same DB generator the schema
// defaults use. Handlers call this once per logical delete to mint a
// deleted_op_id shared across every row the delete touches, so restore can
// revert the whole batch.
func (s *PostgresStore) NewID(ctx context.Context) (string, error) {
	var id string
	if err := db.WithContext(ctx).Raw("SELECT generate_ulid()::text").Scan(&id).Error; err != nil {
		return "", err
	}
	return id, nil
}

func softDeleteFields(byMemberID, opID string) map[string]any {
	return map[string]any{
		"deleted_at":           time.Now().UTC(),
		"deleted_by_member_id": byMemberID,
		"deleted_op_id":        opID,
	}
}

var restoreFields = map[string]any{
	"deleted_at":           nil,
	"deleted_by_member_id": nil,
	"deleted_op_id":        nil,
}

func (s *PostgresStore) SoftDeleteProject(ctx context.Context, projectID, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Project{}).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

func (s *PostgresStore) RestoreProjectsByOp(ctx context.Context, opID string) error {
	return db.WithContext(ctx).Model(&models.Project{}).
		Where("deleted_op_id = ?", opID).Updates(restoreFields).Error
}

// PurgeProjectsDeletedBefore permanently removes projects soft-deleted before
// cutoff, clearing their (FK-less) dependency edges first. Returns the count.
func (s *PostgresStore) PurgeProjectsDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var ids []string
	if err := db.WithContext(ctx).Model(&models.Project{}).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Pluck("project_id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	for _, id := range ids {
		if err := s.RemoveDependenciesForNode(ctx, models.DependencyProject, id); err != nil {
			return 0, err
		}
	}
	res := db.WithContext(ctx).Where("project_id IN ?", ids).Delete(&models.Project{})
	return res.RowsAffected, res.Error
}

func (s *PostgresStore) ListProjectsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Project, error) {
	var projects []models.Project
	if err := db.WithContext(ctx).Where("house_id = ? AND deleted_at IS NULL", houseID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

func (s *PostgresStore) AddProjectTask(ctx context.Context, projectID, taskID string, position int) error {
	pt := &models.ProjectTask{ProjectID: projectID, TaskID: taskID, Position: position}
	return db.WithContext(ctx).Create(pt).Error
}

func (s *PostgresStore) RemoveProjectTask(ctx context.Context, projectID, taskID string) error {
	return db.WithContext(ctx).Where("project_id = ? AND task_id = ?", projectID, taskID).Delete(&models.ProjectTask{}).Error
}

func (s *PostgresStore) ListProjectTasks(ctx context.Context, projectID string, limit, offset int) ([]models.Task, error) {
	var tasks []models.Task
	err := db.WithContext(ctx).
		Joins("JOIN project_tasks pt ON pt.task_id = tasks.task_id").
		Where("pt.project_id = ? AND tasks.deleted_at IS NULL", projectID).
		Order("pt.position ASC").
		Limit(limit).Offset(offset).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// ---- Project members / owners (facade over project_grants) ------------
//
// Since migration 000012 there are no project_members / project_owners
// tables. These methods preserve the old store surface by mapping onto
// project_grants: owners are member-grantees at 'full', members are member-
// grantees at 'edit' (owners ⊆ members). See docs/rbac.md.

func (s *PostgresStore) projectHouseID(ctx context.Context, projectID string) (string, error) {
	var p models.Project
	if err := db.WithContext(ctx).Select("house_id").
		Where("project_id = ?", projectID).First(&p).Error; err != nil {
		return "", err
	}
	return p.HouseID, nil
}

// AddProjectMember grants a member 'edit' without downgrading a higher
// existing grant (an owner stays full): ON CONFLICT DO NOTHING.
func (s *PostgresStore) AddProjectMember(ctx context.Context, projectID, memberID string) error {
	houseID, err := s.projectHouseID(ctx, projectID)
	if err != nil {
		return err
	}
	row := &models.ProjectGrant{
		ProjectID: projectID, HouseID: houseID,
		GranteeType: models.GranteeMember, GranteeID: memberID,
		AccessLevel: models.AccessEdit,
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error
}

// RemoveProjectMember revokes a member's project grant entirely.
func (s *PostgresStore) RemoveProjectMember(ctx context.Context, projectID, memberID string) error {
	return db.WithContext(ctx).
		Where("project_id = ? AND grantee_type = ? AND grantee_id = ?", projectID, models.GranteeMember, memberID).
		Delete(&models.ProjectGrant{}).Error
}

func (s *PostgresStore) ListProjectMembers(ctx context.Context, projectID string) ([]models.Member, error) {
	var members []models.Member
	err := db.WithContext(ctx).
		Joins("JOIN project_grants pg ON pg.grantee_id = members.member_id").
		Where("pg.project_id = ? AND pg.grantee_type = ? AND pg.access_level IN ?",
			projectID, models.GranteeMember, []string{models.AccessEdit, models.AccessFull}).
		Order("members.display_name ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// AddProjectOwner grants (or upgrades to) 'full'.
func (s *PostgresStore) AddProjectOwner(ctx context.Context, projectID, memberID string) error {
	houseID, err := s.projectHouseID(ctx, projectID)
	if err != nil {
		return err
	}
	row := &models.ProjectGrant{
		ProjectID: projectID, HouseID: houseID,
		GranteeType: models.GranteeMember, GranteeID: memberID,
		AccessLevel: models.AccessFull,
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "grantee_type"}, {Name: "grantee_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"access_level": models.AccessFull}),
	}).Create(row).Error
}

// RemoveProjectOwner demotes an owner back to a plain member ('edit'),
// preserving the old behavior where un-owning kept them on the project.
func (s *PostgresStore) RemoveProjectOwner(ctx context.Context, projectID, memberID string) error {
	return db.WithContext(ctx).Model(&models.ProjectGrant{}).
		Where("project_id = ? AND grantee_type = ? AND grantee_id = ? AND access_level = ?",
			projectID, models.GranteeMember, memberID, models.AccessFull).
		Update("access_level", models.AccessEdit).Error
}

func (s *PostgresStore) ListProjectOwners(ctx context.Context, projectID string) ([]models.Member, error) {
	var members []models.Member
	err := db.WithContext(ctx).
		Joins("JOIN project_grants pg ON pg.grantee_id = members.member_id").
		Where("pg.project_id = ? AND pg.grantee_type = ? AND pg.access_level = ?",
			projectID, models.GranteeMember, models.AccessFull).
		Order("members.display_name ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// ---- Resource grants (RBAC) -------------------------------------------

func (s *PostgresStore) ListTaskGrants(ctx context.Context, taskID string) ([]models.TaskGrant, error) {
	var grants []models.TaskGrant
	if err := db.WithContext(ctx).Where("task_id = ?", taskID).Find(&grants).Error; err != nil {
		return nil, err
	}
	return grants, nil
}

func (s *PostgresStore) PutTaskGrant(ctx context.Context, grant *models.TaskGrant) error {
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}, {Name: "grantee_type"}, {Name: "grantee_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"access_level"}),
	}).Create(grant).Error
}

func (s *PostgresStore) DeleteTaskGrant(ctx context.Context, taskID, granteeType, granteeID string) error {
	return db.WithContext(ctx).
		Where("task_id = ? AND grantee_type = ? AND grantee_id = ?", taskID, granteeType, granteeID).
		Delete(&models.TaskGrant{}).Error
}

func (s *PostgresStore) ListProjectGrants(ctx context.Context, projectID string) ([]models.ProjectGrant, error) {
	var grants []models.ProjectGrant
	if err := db.WithContext(ctx).Where("project_id = ?", projectID).Find(&grants).Error; err != nil {
		return nil, err
	}
	return grants, nil
}

func (s *PostgresStore) PutProjectGrant(ctx context.Context, grant *models.ProjectGrant) error {
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "grantee_type"}, {Name: "grantee_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"access_level"}),
	}).Create(grant).Error
}

func (s *PostgresStore) DeleteProjectGrant(ctx context.Context, projectID, granteeType, granteeID string) error {
	return db.WithContext(ctx).
		Where("project_id = ? AND grantee_type = ? AND grantee_id = ?", projectID, granteeType, granteeID).
		Delete(&models.ProjectGrant{}).Error
}

// ---- Resolver helpers --------------------------------------------------

// ListProjectsForTask returns the projects directly containing the task.
func (s *PostgresStore) ListProjectsForTask(ctx context.Context, taskID string) ([]models.Project, error) {
	var projects []models.Project
	if err := db.WithContext(ctx).
		Joins("JOIN project_tasks pt ON pt.project_id = projects.project_id").
		Where("pt.task_id = ? AND projects.deleted_at IS NULL", taskID).Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

// GetTaskAncestors walks parent_task_id upward via a recursive CTE,
// nearest-first, excluding the task itself. A depth guard bounds the walk in
// case of an (impossible) cyclic parent chain.
func (s *PostgresStore) GetTaskAncestors(ctx context.Context, taskID string) ([]models.Task, error) {
	var tasks []models.Task
	const q = `
WITH RECURSIVE chain AS (
    SELECT t.*, 1 AS depth
    FROM tasks t
    JOIN tasks child ON child.parent_task_id = t.task_id
    WHERE child.task_id = ?
  UNION ALL
    SELECT p.*, c.depth + 1
    FROM tasks p
    JOIN chain c ON c.parent_task_id = p.task_id
    WHERE c.depth < 100
)
SELECT * FROM chain ORDER BY depth ASC`
	if err := db.WithContext(ctx).Raw(q, taskID).Scan(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// ListGroupIDsForMember returns the ids of every group the member belongs to.
func (s *PostgresStore) ListGroupIDsForMember(ctx context.Context, memberID string) ([]string, error) {
	var ids []string
	if err := db.WithContext(ctx).Model(&models.GroupMember{}).
		Where("member_id = ?", memberID).Pluck("group_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ---- Dependencies ------------------------------------------------------

// AddDependency inserts an edge, ignoring a duplicate (the PK covers the
// full edge). The caller is responsible for self-loop and cycle checks.
func (s *PostgresStore) AddDependency(ctx context.Context, dep *models.Dependency) error {
	return db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(dep).Error
}

func (s *PostgresStore) RemoveDependency(ctx context.Context, dependentType, dependentID, dependencyType, dependencyID string) error {
	return db.WithContext(ctx).
		Where("dependent_type = ? AND dependent_id = ? AND dependency_type = ? AND dependency_id = ?",
			dependentType, dependentID, dependencyType, dependencyID).
		Delete(&models.Dependency{}).Error
}

// ListDependencies returns the edges where the node is the dependent — i.e.
// the things it depends on.
func (s *PostgresStore) ListDependencies(ctx context.Context, nodeType, nodeID string) ([]models.Dependency, error) {
	var rows []models.Dependency
	if err := db.WithContext(ctx).
		Where("dependent_type = ? AND dependent_id = ?", nodeType, nodeID).
		Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListDependents returns the edges where the node is the dependency — i.e.
// the things that depend on it (the computed reverse view).
func (s *PostgresStore) ListDependents(ctx context.Context, nodeType, nodeID string) ([]models.Dependency, error) {
	var rows []models.Dependency
	if err := db.WithContext(ctx).
		Where("dependency_type = ? AND dependency_id = ?", nodeType, nodeID).
		Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DependencyPathExists walks forward from (fromType,fromID) along
// dependent->dependency edges and reports whether (toType,toID) is reachable.
// Used before inserting an edge X->Y: if a path already runs from Y back to
// X, adding X->Y would close a cycle. UNION (not UNION ALL) dedupes visited
// nodes so the walk terminates even on pre-existing cyclic data. The whole
// check runs in the database.
func (s *PostgresStore) DependencyPathExists(ctx context.Context, fromType, fromID, toType, toID string) (bool, error) {
	const q = `
WITH RECURSIVE reach AS (
        SELECT ?::dependency_node_type AS t, ?::uuid AS id
    UNION
        SELECT d.dependency_type, d.dependency_id
        FROM dependencies d
        JOIN reach r ON d.dependent_type = r.t AND d.dependent_id = r.id
)
SELECT EXISTS (
    SELECT 1 FROM reach WHERE t = ?::dependency_node_type AND id = ?::uuid
)`
	var exists bool
	if err := db.WithContext(ctx).Raw(q, fromType, fromID, toType, toID).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

// RemoveDependenciesForNode deletes every edge touching the node in either
// position. Called when a node is hard-deleted (projects) so no edges dangle.
func (s *PostgresStore) RemoveDependenciesForNode(ctx context.Context, nodeType, nodeID string) error {
	return db.WithContext(ctx).
		Where("(dependent_type = ? AND dependent_id = ?) OR (dependency_type = ? AND dependency_id = ?)",
			nodeType, nodeID, nodeType, nodeID).
		Delete(&models.Dependency{}).Error
}

// ---- Milestones --------------------------------------------------------

func (s *PostgresStore) CreateMilestone(ctx context.Context, m *models.Milestone) error {
	return db.WithContext(ctx).Create(m).Error
}

func (s *PostgresStore) UpdateMilestone(ctx context.Context, m *models.Milestone) error {
	return db.WithContext(ctx).Save(m).Error
}

func (s *PostgresStore) DeleteMilestone(ctx context.Context, milestoneID string) error {
	return db.WithContext(ctx).
		Where("milestone_id = ?", milestoneID).
		Delete(&models.Milestone{}).Error
}

func (s *PostgresStore) GetMilestoneByID(ctx context.Context, milestoneID string) (*models.Milestone, error) {
	var m models.Milestone
	err := db.WithContext(ctx).Where("milestone_id = ? AND deleted_at IS NULL", milestoneID).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *PostgresStore) ListMilestonesByProject(ctx context.Context, projectID string) ([]models.Milestone, error) {
	var out []models.Milestone
	err := db.WithContext(ctx).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("position ASC").
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ---- Task assignees ----------------------------------------------------

func (s *PostgresStore) AddTaskAssignee(ctx context.Context, taskID, memberID string) error {
	ta := &models.TaskAssignee{TaskID: taskID, MemberID: memberID}
	return db.WithContext(ctx).Create(ta).Error
}

func (s *PostgresStore) RemoveTaskAssignee(ctx context.Context, taskID, memberID string) error {
	return db.WithContext(ctx).
		Where("task_id = ? AND member_id = ?", taskID, memberID).
		Delete(&models.TaskAssignee{}).Error
}

func (s *PostgresStore) ListTaskAssignees(ctx context.Context, taskID string) ([]models.Member, error) {
	var members []models.Member
	err := db.WithContext(ctx).
		Joins("JOIN task_assignees ta ON ta.member_id = members.member_id").
		Where("ta.task_id = ?", taskID).
		Order("members.display_name ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// Member audit log

func (s *PostgresStore) RecordMemberAudit(ctx context.Context, audit *models.MemberAudit) error {
	return db.WithContext(ctx).Create(audit).Error
}

func (s *PostgresStore) ListAuditsForMember(ctx context.Context, memberID string, limit, offset int) ([]models.MemberAudit, error) {
	var audits []models.MemberAudit
	if err := db.WithContext(ctx).
		Where("subject_member_id = ?", memberID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&audits).Error; err != nil {
		return nil, err
	}
	return audits, nil
}

// Trusted domain operations

func (s *PostgresStore) CreateTrustedDomain(ctx context.Context, td *models.TrustedDomain) error {
	return db.WithContext(ctx).Create(td).Error
}

func (s *PostgresStore) DeleteTrustedDomain(ctx context.Context, tdID string) error {
	return db.WithContext(ctx).Where("trusted_domain_id = ?", tdID).Delete(&models.TrustedDomain{}).Error
}

func (s *PostgresStore) ListTrustedDomains(ctx context.Context, houseID string) ([]models.TrustedDomain, error) {
	var domains []models.TrustedDomain
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("domain ASC").Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

func (s *PostgresStore) IsDomainTrusted(ctx context.Context, houseID, domain string) (bool, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&models.TrustedDomain{}).Where("house_id = ? AND domain = ?", houseID, domain).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *PostgresStore) HousesTrustingDomain(ctx context.Context, domain string) ([]models.House, error) {
	var houses []models.House
	err := db.WithContext(ctx).
		Joins("JOIN trusted_domains td ON td.house_id = houses.house_id").
		Where("td.domain = ?", domain).
		Order("houses.name ASC").
		Find(&houses).Error
	if err != nil {
		return nil, err
	}
	return houses, nil
}

// Event operations

func (s *PostgresStore) CreateEvent(ctx context.Context, event *models.Event) error {
	return db.WithContext(ctx).Create(event).Error
}

func (s *PostgresStore) GetEventByID(ctx context.Context, eventID string) (*models.Event, error) {
	var event models.Event
	if err := db.WithContext(ctx).Where("event_id = ? AND deleted_at IS NULL", eventID).First(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *PostgresStore) UpdateEvent(ctx context.Context, event *models.Event) error {
	return db.WithContext(ctx).Save(event).Error
}

func (s *PostgresStore) DeleteEvent(ctx context.Context, eventID string) error {
	return db.WithContext(ctx).Where("event_id = ?", eventID).Delete(&models.Event{}).Error
}

func (s *PostgresStore) ListEventsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Event, error) {
	var events []models.Event
	if err := db.WithContext(ctx).Where("house_id = ? AND deleted_at IS NULL", houseID).Order("starts_at ASC NULLS LAST").Limit(limit).Offset(offset).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// Event-recurrence spawning. Roots: rows that have recurrence_freq set AND
// a next_recurrence_at <= now. Children: rows with recurrence_root_event_id
// set. The worker fetches roots, asks for the latest child to know where
// it left off, then inserts the next occurrence and bumps the root's
// next_recurrence_at forward.

func (s *PostgresStore) ListDueRecurringEvents(ctx context.Context, before time.Time, limit int) ([]models.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	var events []models.Event
	err := db.WithContext(ctx).
		Where("recurrence_freq IS NOT NULL AND next_recurrence_at IS NOT NULL AND next_recurrence_at <= ? AND deleted_at IS NULL", before).
		Order("next_recurrence_at ASC").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (s *PostgresStore) LatestRecurrenceChildOfEvent(ctx context.Context, rootEventID string) (*models.Event, error) {
	var event models.Event
	err := db.WithContext(ctx).
		Where("recurrence_root_event_id = ?", rootEventID).
		Order("starts_at DESC NULLS LAST").
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// DeleteEventsAfter hard-deletes any child of `rootEventID` whose
// `starts_at` is >= `fromInclusive`. Used by the "delete this and future"
// flow; the root itself is not touched here — callers typically clear
// the root's recurrence_freq + next_recurrence_at separately so the
// spawner stops respawning.
func (s *PostgresStore) DeleteEventsAfter(ctx context.Context, rootEventID string, fromInclusive time.Time) error {
	return db.WithContext(ctx).
		Where("recurrence_root_event_id = ? AND starts_at >= ?", rootEventID, fromInclusive).
		Delete(&models.Event{}).Error
}

// SoftDeleteEvent soft-deletes a single event (or, when called on a recurrence
// root, the caller is expected to also soft-delete its children via
// SoftDeleteEventsAfter under the same opID).
func (s *PostgresStore) SoftDeleteEvent(ctx context.Context, eventID, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Event{}).
		Where("event_id = ? AND deleted_at IS NULL", eventID).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

// SoftDeleteEventsAfter soft-deletes the children of rootEventID at/after
// fromInclusive (the "this & future" sweep), stamping the shared opID.
func (s *PostgresStore) SoftDeleteEventsAfter(ctx context.Context, rootEventID string, fromInclusive time.Time, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Event{}).
		Where("recurrence_root_event_id = ? AND starts_at >= ? AND deleted_at IS NULL", rootEventID, fromInclusive).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

// SoftDeleteEventSeries soft-deletes a recurrence root and every child that
// points at it, under one opID (the "delete the whole series" case). The
// recurrence worker skips soft-deleted roots (ListDueRecurringEvents filters
// deleted_at IS NULL), so muting the root's recurrence_freq is unnecessary —
// restore is a clean un-delete and spawning resumes.
func (s *PostgresStore) SoftDeleteEventSeries(ctx context.Context, rootEventID, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Event{}).
		Where("(event_id = ? OR recurrence_root_event_id = ?) AND deleted_at IS NULL", rootEventID, rootEventID).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

func (s *PostgresStore) RestoreEventsByOp(ctx context.Context, opID string) error {
	return db.WithContext(ctx).Model(&models.Event{}).
		Where("deleted_op_id = ?", opID).Updates(restoreFields).Error
}

func (s *PostgresStore) PurgeEventsDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := db.WithContext(ctx).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Delete(&models.Event{})
	return res.RowsAffected, res.Error
}

// Task operations

func (s *PostgresStore) CreateTask(ctx context.Context, task *models.Task) error {
	return db.WithContext(ctx).Create(task).Error
}

func (s *PostgresStore) GetTaskByID(ctx context.Context, taskID string) (*models.Task, error) {
	var task models.Task
	if err := db.WithContext(ctx).Where("task_id = ? AND deleted_at IS NULL", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *PostgresStore) UpdateTask(ctx context.Context, task *models.Task) error {
	return db.WithContext(ctx).Save(task).Error
}

func (s *PostgresStore) DeleteTask(ctx context.Context, taskID string) error {
	return db.WithContext(ctx).Where("task_id = ?", taskID).Delete(&models.Task{}).Error
}

func (s *PostgresStore) ListTasksByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Task, error) {
	var tasks []models.Task
	if err := db.WithContext(ctx).
		Where("house_id = ? AND deleted_at IS NULL", houseID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// Comment operations

func (s *PostgresStore) CreateComment(ctx context.Context, comment *models.Comment) error {
	return db.WithContext(ctx).Create(comment).Error
}

func (s *PostgresStore) GetCommentByID(ctx context.Context, commentID string) (*models.Comment, error) {
	var comment models.Comment
	if err := db.WithContext(ctx).Where("comment_id = ? AND deleted_at IS NULL", commentID).First(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (s *PostgresStore) UpdateComment(ctx context.Context, comment *models.Comment) error {
	return db.WithContext(ctx).Save(comment).Error
}

func (s *PostgresStore) DeleteComment(ctx context.Context, commentID string) error {
	return db.WithContext(ctx).Where("comment_id = ?", commentID).Delete(&models.Comment{}).Error
}

func (s *PostgresStore) SoftDeleteComment(ctx context.Context, commentID, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Comment{}).
		Where("comment_id = ? AND deleted_at IS NULL", commentID).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

func (s *PostgresStore) RestoreCommentsByOp(ctx context.Context, opID string) error {
	return db.WithContext(ctx).Model(&models.Comment{}).
		Where("deleted_op_id = ?", opID).Updates(restoreFields).Error
}

func (s *PostgresStore) PurgeCommentsDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := db.WithContext(ctx).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Delete(&models.Comment{})
	return res.RowsAffected, res.Error
}

func (s *PostgresStore) ListCommentsByTarget(ctx context.Context, targetType, targetID string, limit, offset int) ([]models.Comment, error) {
	var comments []models.Comment
	if err := db.WithContext(ctx).Where("target_type = ? AND target_id = ? AND deleted_at IS NULL", targetType, targetID).Order("created_at ASC").Limit(limit).Offset(offset).Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (s *PostgresStore) CreateCommentWithNotifications(ctx context.Context, comment *models.Comment, event *models.NotificationEvent, recipientMemberIDs []string) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}
		if event == nil || len(recipientMemberIDs) == 0 {
			return nil
		}
		if err := tx.Create(event).Error; err != nil {
			return err
		}
		rows := make([]models.Notification, 0, len(recipientMemberIDs))
		for _, mid := range recipientMemberIDs {
			rows = append(rows, models.Notification{
				NotificationEventID: event.NotificationEventID,
				HouseID:             event.HouseID,
				MemberID:            mid,
			})
		}
		return tx.Create(&rows).Error
	})
}

// Notification feed operations

const notificationFeedSelect = `n.notification_id, n.notification_event_id, n.house_id, n.member_id, n.read_at, n.created_at,
	e.kind, e.actor_member_id, e.actor_name, e.target_type, e.target_id, e.target_title, e.body`

func (s *PostgresStore) ListNotificationsByMember(ctx context.Context, houseID, memberID string, unreadOnly bool, limit, offset int) ([]models.NotificationFeedItem, error) {
	var items []models.NotificationFeedItem
	q := db.WithContext(ctx).
		Table("notifications n").
		Select(notificationFeedSelect).
		Joins("JOIN notification_events e ON e.notification_event_id = n.notification_event_id").
		Where("n.house_id = ? AND n.member_id = ?", houseID, memberID)
	if unreadOnly {
		q = q.Where("n.read_at IS NULL")
	}
	if err := q.Order("n.created_at DESC").Limit(limit).Offset(offset).Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) GetNotificationFeedItem(ctx context.Context, notificationID string) (*models.NotificationFeedItem, error) {
	var item models.NotificationFeedItem
	err := db.WithContext(ctx).
		Table("notifications n").
		Select(notificationFeedSelect).
		Joins("JOIN notification_events e ON e.notification_event_id = n.notification_event_id").
		Where("n.notification_id = ?", notificationID).
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *PostgresStore) CountUnreadNotifications(ctx context.Context, houseID, memberID string) (int64, error) {
	var count int64
	err := db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("house_id = ? AND member_id = ? AND read_at IS NULL", houseID, memberID).
		Count(&count).Error
	return count, err
}

func (s *PostgresStore) MarkNotificationRead(ctx context.Context, notificationID string, readAt time.Time) error {
	return db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("notification_id = ? AND read_at IS NULL", notificationID).
		Update("read_at", readAt).Error
}

func (s *PostgresStore) MarkAllNotificationsRead(ctx context.Context, houseID, memberID string, readAt time.Time) error {
	return db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("house_id = ? AND member_id = ? AND read_at IS NULL", houseID, memberID).
		Update("read_at", readAt).Error
}

func (s *PostgresStore) CullNotificationEventsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&models.NotificationEvent{})
	return res.RowsAffected, res.Error
}

// Share operations

func (s *PostgresStore) CreateShare(ctx context.Context, share *models.Share) error {
	return db.WithContext(ctx).Create(share).Error
}

func (s *PostgresStore) GetShareByID(ctx context.Context, shareID string) (*models.Share, error) {
	var share models.Share
	if err := db.WithContext(ctx).Where("share_id = ?", shareID).First(&share).Error; err != nil {
		return nil, err
	}
	return &share, nil
}

func (s *PostgresStore) DeleteShare(ctx context.Context, shareID string) error {
	return db.WithContext(ctx).Where("share_id = ?", shareID).Delete(&models.Share{}).Error
}

func (s *PostgresStore) ListSharesByResource(ctx context.Context, resourceType, resourceID string) ([]models.Share, error) {
	var shares []models.Share
	if err := db.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).Order("created_at DESC").Find(&shares).Error; err != nil {
		return nil, err
	}
	return shares, nil
}

func (s *PostgresStore) ListSharesByHouse(ctx context.Context, houseID string) ([]models.Share, error) {
	var shares []models.Share
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("created_at DESC").Find(&shares).Error; err != nil {
		return nil, err
	}
	return shares, nil
}

func (s *PostgresStore) GetShareAccess(ctx context.Context, domain, userID, resourceType, resourceID string) (*models.Share, error) {
	var share models.Share
	if err := db.WithContext(ctx).Where(
		"linkkeys_domain = ? AND linkkeys_user_id = ? AND resource_type = ? AND resource_id = ? AND (expires_at IS NULL OR expires_at > NOW())",
		domain, userID, resourceType, resourceID,
	).First(&share).Error; err != nil {
		return nil, err
	}
	return &share, nil
}

// ---- Settings ---------------------------------------------------------

func (s *PostgresStore) GetHouseSettings(ctx context.Context, houseID string) ([]models.HouseSetting, error) {
	var rows []models.HouseSetting
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpsertHouseSetting writes (or replaces) one (house_id, key) row.
// `value` is raw JSON; the service layer is responsible for shaping it.
// `updated_at` is bumped automatically; `updated_by` is whatever the caller
// supplies (nil for system writes).
func (s *PostgresStore) UpsertHouseSetting(ctx context.Context, setting *models.HouseSetting) error {
	setting.UpdatedAt = time.Now().UTC()
	return db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "house_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at", "updated_by"}),
		}).
		Create(setting).Error
}

// ListMembersWithRoleName returns every member of houseID who holds the
// named role, ordered by created_at so callers that want "first admin"
// get a deterministic pick.
func (s *PostgresStore) ListMembersWithRoleName(ctx context.Context, houseID, roleName string) ([]models.Member, error) {
	var members []models.Member
	err := db.WithContext(ctx).
		Joins("JOIN member_roles mr ON mr.member_id = members.member_id").
		Joins("JOIN roles r ON r.role_id = mr.role_id").
		Where("members.house_id = ? AND r.house_id = ? AND r.name = ?", houseID, houseID, roleName).
		Order("members.created_at ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}
