package store

import (
	"context"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

var AppStore Store

type Store interface {
	Initialize() (cleanup func(), err error)

	// House operations
	CreateHouse(ctx context.Context, house *models.House) error
	GetHouseByID(ctx context.Context, houseID string) (*models.House, error)
	UpdateHouse(ctx context.Context, house *models.House) error
	DeleteHouse(ctx context.Context, houseID string) error
	ListHouses(ctx context.Context, limit, offset int) ([]models.House, error)

	// Member operations
	CreateMember(ctx context.Context, member *models.Member) error
	GetMemberByID(ctx context.Context, memberID string) (*models.Member, error)
	GetMemberByIdentity(ctx context.Context, houseID, domain, userID string) (*models.Member, error)
	// FindMembersByLinkkeysIdentity returns all member rows matching a
	// linkkeys (domain, user_id) tuple across every house the user belongs
	// to. Used at /auth/login to choose a house when the caller didn't
	// supply one.
	FindMembersByLinkkeysIdentity(ctx context.Context, domain, userID string) ([]models.Member, error)
	UpdateMember(ctx context.Context, member *models.Member) error
	DeleteMember(ctx context.Context, memberID string) error
	ListMembersByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Member, error)

	// Role operations
	CreateRole(ctx context.Context, role *models.Role) error
	GetRoleByID(ctx context.Context, roleID string) (*models.Role, error)
	GetRoleByName(ctx context.Context, houseID, name string) (*models.Role, error)
	UpdateRole(ctx context.Context, role *models.Role) error
	DeleteRole(ctx context.Context, roleID string) error
	ListRolesByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Role, error)
	AssignRole(ctx context.Context, memberID, roleID string) error
	RevokeRole(ctx context.Context, memberID, roleID string) error
	ListRolesForMember(ctx context.Context, memberID string) ([]models.Role, error)

	// Skill operations
	CreateSkill(ctx context.Context, skill *models.Skill) error
	GetSkillByID(ctx context.Context, skillID string) (*models.Skill, error)
	UpdateSkill(ctx context.Context, skill *models.Skill) error
	DeleteSkill(ctx context.Context, skillID string) error
	ListSkillsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Skill, error)
	AssignSkill(ctx context.Context, memberID, skillID string) error
	UnassignSkill(ctx context.Context, memberID, skillID string) error
	ListSkillsForMember(ctx context.Context, memberID string) ([]models.Skill, error)

	// Group operations
	CreateGroup(ctx context.Context, group *models.Group) error
	GetGroupByID(ctx context.Context, groupID string) (*models.Group, error)
	UpdateGroup(ctx context.Context, group *models.Group) error
	DeleteGroup(ctx context.Context, groupID string) error
	ListGroupsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Group, error)
	AddGroupMember(ctx context.Context, groupID, memberID string) error
	RemoveGroupMember(ctx context.Context, groupID, memberID string) error
	ListGroupMembers(ctx context.Context, groupID string) ([]models.Member, error)

	// Recurrence helpers — used by the worker that spawns occurrences.
	ListDueRecurringTasks(ctx context.Context, before time.Time, limit int) ([]models.Task, error)
	LatestRecurrenceChildOf(ctx context.Context, rootTaskID string) (*models.Task, error)

	// Project operations
	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectByID(ctx context.Context, projectID string) (*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error
	DeleteProject(ctx context.Context, projectID string) error
	ListProjectsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Project, error)
	AddProjectTask(ctx context.Context, projectID, taskID string, position int) error
	RemoveProjectTask(ctx context.Context, projectID, taskID string) error
	ListProjectTasks(ctx context.Context, projectID string, limit, offset int) ([]models.Task, error)

	// Member audit log
	RecordMemberAudit(ctx context.Context, audit *models.MemberAudit) error
	ListAuditsForMember(ctx context.Context, memberID string, limit, offset int) ([]models.MemberAudit, error)

	// Trusted domain operations
	CreateTrustedDomain(ctx context.Context, td *models.TrustedDomain) error
	DeleteTrustedDomain(ctx context.Context, tdID string) error
	ListTrustedDomains(ctx context.Context, houseID string) ([]models.TrustedDomain, error)
	IsDomainTrusted(ctx context.Context, houseID, domain string) (bool, error)
	// HousesTrustingDomain returns every house whose trusted_domains
	// table contains the given domain. Used at /auth/login to find
	// auto-membership opportunities for a verified identity that has
	// no member row anywhere.
	HousesTrustingDomain(ctx context.Context, domain string) ([]models.House, error)

	// Event operations
	CreateEvent(ctx context.Context, event *models.Event) error
	GetEventByID(ctx context.Context, eventID string) (*models.Event, error)
	UpdateEvent(ctx context.Context, event *models.Event) error
	DeleteEvent(ctx context.Context, eventID string) error
	ListEventsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Event, error)

	// Task operations
	CreateTask(ctx context.Context, task *models.Task) error
	GetTaskByID(ctx context.Context, taskID string) (*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
	DeleteTask(ctx context.Context, taskID string) error
	ListTasksByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Task, error)

	// Comment operations
	CreateComment(ctx context.Context, comment *models.Comment) error
	GetCommentByID(ctx context.Context, commentID string) (*models.Comment, error)
	UpdateComment(ctx context.Context, comment *models.Comment) error
	DeleteComment(ctx context.Context, commentID string) error
	ListCommentsByTarget(ctx context.Context, targetType, targetID string, limit, offset int) ([]models.Comment, error)

	// Share operations
	CreateShare(ctx context.Context, share *models.Share) error
	GetShareByID(ctx context.Context, shareID string) (*models.Share, error)
	DeleteShare(ctx context.Context, shareID string) error
	ListSharesByResource(ctx context.Context, resourceType, resourceID string) ([]models.Share, error)
	ListSharesByHouse(ctx context.Context, houseID string) ([]models.Share, error)
	GetShareAccess(ctx context.Context, domain, userID, resourceType, resourceID string) (*models.Share, error)
}
