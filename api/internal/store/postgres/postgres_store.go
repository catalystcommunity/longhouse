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
	if err := db.WithContext(ctx).Where("role_id = ?", roleID).First(&role).Error; err != nil {
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
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("name ASC").Limit(limit).Offset(offset).Find(&roles).Error; err != nil {
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
		Where("mr.member_id = ?", memberID).
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
	if err := db.WithContext(ctx).Where("skill_id = ?", skillID).First(&skill).Error; err != nil {
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
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("name ASC").Limit(limit).Offset(offset).Find(&skills).Error; err != nil {
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
		Where("ms.member_id = ?", memberID).
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
	if err := db.WithContext(ctx).Where("group_id = ?", groupID).First(&g).Error; err != nil {
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
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("name ASC").Limit(limit).Offset(offset).Find(&groups).Error; err != nil {
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
	if err := db.WithContext(ctx).Where("project_id = ?", projectID).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (s *PostgresStore) UpdateProject(ctx context.Context, project *models.Project) error {
	return db.WithContext(ctx).Save(project).Error
}

func (s *PostgresStore) DeleteProject(ctx context.Context, projectID string) error {
	return db.WithContext(ctx).Where("project_id = ?", projectID).Delete(&models.Project{}).Error
}

func (s *PostgresStore) ListProjectsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Project, error) {
	var projects []models.Project
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&projects).Error; err != nil {
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
	if err := db.WithContext(ctx).Where("event_id = ?", eventID).First(&event).Error; err != nil {
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
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("starts_at ASC NULLS LAST").Limit(limit).Offset(offset).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// Task operations

func (s *PostgresStore) CreateTask(ctx context.Context, task *models.Task) error {
	return db.WithContext(ctx).Create(task).Error
}

func (s *PostgresStore) GetTaskByID(ctx context.Context, taskID string) (*models.Task, error) {
	var task models.Task
	if err := db.WithContext(ctx).Where("task_id = ?", taskID).First(&task).Error; err != nil {
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
	if err := db.WithContext(ctx).Where("comment_id = ?", commentID).First(&comment).Error; err != nil {
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

func (s *PostgresStore) ListCommentsByTarget(ctx context.Context, targetType, targetID string, limit, offset int) ([]models.Comment, error) {
	var comments []models.Comment
	if err := db.WithContext(ctx).Where("target_type = ? AND target_id = ?", targetType, targetID).Order("created_at ASC").Limit(limit).Offset(offset).Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
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
