package handlers

import "github.com/catalystcommunity/longhouse/webapp/internal/api"

// SessionAPI is the surface area of api.Client that view handlers use to
// talk to the api on behalf of a logged-in user. It's an interface so
// tests can inject a fake without spinning up an HTTP server.
//
// New methods land here as views start needing them; the production type
// is *api.Client, which already implements the full surface.
type SessionAPI interface {
	ListMembers(houseID string) ([]api.Member, error)
	UpdateMember(houseID, memberID string, body api.UpdateMemberRequest) (*api.Member, error)

	ListRoles(houseID string) ([]api.Role, error)
	CreateRole(houseID string, body api.CreateRoleRequest) (*api.Role, error)
	DeleteRole(houseID, roleID string) error
	ListMemberRoles(houseID, memberID string) ([]api.Role, error)
	GrantRole(houseID, memberID, roleID string) error
	RevokeRole(houseID, memberID, roleID string) error

	ListAuditsForMember(houseID, memberID string) ([]api.MemberAudit, error)

	ListEvents(houseID string) ([]api.Event, error)
	CreateEvent(houseID string, body api.CreateEventRequest) (*api.Event, error)
	GetEvent(houseID, eventID string) (*api.Event, error)

	ListComments(houseID, targetType, targetID string) ([]api.Comment, error)
	CreateComment(houseID, targetType, targetID string, body api.CreateCommentRequest) (*api.Comment, error)
	DeleteComment(houseID, commentID string) error

	ListGroups(houseID string) ([]api.Group, error)
	CreateGroup(houseID string, body api.CreateGroupRequest) (*api.Group, error)
	DeleteGroup(houseID, groupID string) error
	ListGroupMembers(houseID, groupID string) ([]api.Member, error)
	AddGroupMember(houseID, groupID, memberID string) error
	RemoveGroupMember(houseID, groupID, memberID string) error

	ListTrustedDomains(houseID string) ([]api.TrustedDomain, error)
	CreateTrustedDomain(houseID string, body api.CreateTrustedDomainRequest) (*api.TrustedDomain, error)
	DeleteTrustedDomain(houseID, tdID string) error

	ListSkills(houseID string) ([]api.Skill, error)
	CreateSkill(houseID string, body api.CreateSkillRequest) (*api.Skill, error)
	DeleteSkill(houseID, skillID string) error
	ListMemberSkills(houseID, memberID string) ([]api.Skill, error)
	AddMemberSkill(houseID, memberID, skillID string) error
	RemoveMemberSkill(houseID, memberID, skillID string) error

	ListProjects(houseID string) ([]api.Project, error)
	CreateProject(houseID string, body api.CreateProjectRequest) (*api.Project, error)
	GetProject(houseID, projectID string) (*api.Project, error)
	ListProjectTasks(houseID, projectID string) ([]api.Task, error)
	AddProjectTask(houseID, projectID, taskID string, position int) error

	ListTasks(houseID string) ([]api.Task, error)
	CreateTask(houseID string, body api.CreateTaskRequest) (*api.Task, error)
	GetTask(houseID, taskID string) (*api.Task, error)
	UpdateTask(houseID, taskID string, body api.UpdateTaskRequest) (*api.Task, error)

	ListShares(houseID string) ([]api.Share, error)
	CreateShare(houseID string, body api.CreateShareRequest) (*api.Share, error)
	DeleteShare(houseID, shareID string) error
}
