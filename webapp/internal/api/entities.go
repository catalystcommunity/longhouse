// Wire types for the per-entity api endpoints. Hand-mirrored from the
// CSIL spec until we lift the generated package out of the api module
// (see package doc on client.go).
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ----- Member -----

type Member struct {
	MemberID       string `json:"member_id"`
	HouseID        string `json:"house_id"`
	LinkkeysDomain string `json:"linkkeys_domain"`
	LinkkeysUserID string `json:"linkkeys_user_id"`
	DisplayName    string `json:"display_name,omitempty"`
}

type UpdateMemberRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
}

// ----- Role -----

type Role struct {
	RoleID      string `json:"role_id"`
	HouseID     string `json:"house_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type UpdateRoleRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ----- Skill -----

type Skill struct {
	SkillID     string `json:"skill_id"`
	HouseID     string `json:"house_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type CreateSkillRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ----- Event -----

type Event struct {
	EventID       string `json:"event_id"`
	HouseID       string `json:"house_id"`
	OwnerMemberID string `json:"owner_member_id"`
	Title         string `json:"title"`
	Description   string `json:"description,omitempty"`
	Location      string `json:"location,omitempty"`
	StartsAt      string `json:"starts_at,omitempty"`
	EndsAt        string `json:"ends_at,omitempty"`
	AllDay        bool   `json:"all_day,omitempty"`
}

type CreateEventRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
	StartsAt    string `json:"starts_at,omitempty"`
	EndsAt      string `json:"ends_at,omitempty"`
	AllDay      bool   `json:"all_day,omitempty"`
}

// ----- Group -----

type Group struct {
	GroupID     string `json:"group_id"`
	HouseID     string `json:"house_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ----- MemberAudit -----

type MemberAudit struct {
	AuditID         string  `json:"audit_id"`
	HouseID         string  `json:"house_id"`
	SubjectMemberID string  `json:"subject_member_id"`
	ActorMemberID   *string `json:"actor_member_id,omitempty"`
	Action          string  `json:"action"`
	TargetType      *string `json:"target_type,omitempty"`
	TargetID        *string `json:"target_id,omitempty"`
	Detail          *string `json:"detail,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

// ----- Comment -----

type Comment struct {
	CommentID  string `json:"comment_id"`
	HouseID    string `json:"house_id"`
	MemberID   string `json:"member_id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
}

type CreateCommentRequest struct {
	Body string `json:"body"`
}

// ----- TrustedDomain -----

type TrustedDomain struct {
	TrustedDomainID string `json:"trusted_domain_id"`
	HouseID         string `json:"house_id"`
	Domain          string `json:"domain"`
}

type CreateTrustedDomainRequest struct {
	Domain string `json:"domain"`
}

// ----- Project -----

type Project struct {
	ProjectID   string `json:"project_id"`
	HouseID     string `json:"house_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ----- Task -----

type Task struct {
	TaskID             string  `json:"task_id"`
	HouseID            string  `json:"house_id"`
	OwnerMemberID      string  `json:"owner_member_id"`
	AssignedToMemberID *string `json:"assigned_to_member_id,omitempty"`
	AssignedToSkillID  *string `json:"assigned_to_skill_id,omitempty"`
	ParentTaskID       *string `json:"parent_task_id,omitempty"`
	Title              string  `json:"title"`
	Description        string  `json:"description,omitempty"`
	Status             string  `json:"status,omitempty"`
}

type CreateTaskRequest struct {
	Title               string  `json:"title"`
	Description         string  `json:"description,omitempty"`
	AssignedToMemberID  *string `json:"assigned_to_member_id,omitempty"`
	AssignedToSkillID   *string `json:"assigned_to_skill_id,omitempty"`
	ParentTaskID        *string `json:"parent_task_id,omitempty"`
	RecurrenceFreq      *string `json:"recurrence_freq,omitempty"`
	RecurrenceInterval  int     `json:"recurrence_interval,omitempty"`
	RecurrenceByWeekday []int   `json:"recurrence_by_weekday,omitempty"`
	NextRecurrenceAt    string  `json:"next_recurrence_at,omitempty"`
}

type UpdateTaskRequest struct {
	Title              *string `json:"title,omitempty"`
	Description        *string `json:"description,omitempty"`
	Status             *string `json:"status,omitempty"`
	AssignedToMemberID *string `json:"assigned_to_member_id,omitempty"`
	AssignedToSkillID  *string `json:"assigned_to_skill_id,omitempty"`
}

// ----- Share -----

type Share struct {
	ShareID        string `json:"share_id"`
	HouseID        string `json:"house_id"`
	SharedBy       string `json:"shared_by"`
	LinkkeysDomain string `json:"linkkeys_domain"`
	LinkkeysUserID string `json:"linkkeys_user_id"`
	ResourceType   string `json:"resource_type"`
	ResourceID     string `json:"resource_id"`
	AccessLevel    string `json:"access_level,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	ExpiresAt      string `json:"expires_at,omitempty"`
}

type CreateShareRequest struct {
	LinkkeysDomain string `json:"linkkeys_domain"`
	LinkkeysUserID string `json:"linkkeys_user_id"`
	ResourceType   string `json:"resource_type"`
	ResourceID     string `json:"resource_id"`
	ExpiresAt      string `json:"expires_at,omitempty"`
}

// ----- HTTP plumbing -----

// do issues an authenticated request, decoding JSON when out is non-nil
// and returning a typed error for non-2xx responses.
func (c *Client) do(method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return fmt.Errorf("build %s %s: %w", method, path, err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("do %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("api %s %s: %s: %s", method, path, resp.Status, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode %s %s: %w", method, path, err)
		}
	}
	return nil
}

// ----- Members -----

func (c *Client) ListMembers(houseID string) ([]Member, error) {
	var out []Member
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/members", nil, &out)
}

func (c *Client) UpdateMember(houseID, memberID string, body UpdateMemberRequest) (*Member, error) {
	var out Member
	return &out, c.do(http.MethodPatch, "/api/v1/houses/"+houseID+"/members/"+memberID, body, &out)
}

// ----- Roles -----

func (c *Client) ListRoles(houseID string) ([]Role, error) {
	var out []Role
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/roles", nil, &out)
}

func (c *Client) CreateRole(houseID string, body CreateRoleRequest) (*Role, error) {
	var out Role
	return &out, c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/roles", body, &out)
}

func (c *Client) UpdateRole(houseID, roleID string, body UpdateRoleRequest) (*Role, error) {
	var out Role
	return &out, c.do(http.MethodPatch, "/api/v1/houses/"+houseID+"/roles/"+roleID, body, &out)
}

func (c *Client) DeleteRole(houseID, roleID string) error {
	return c.do(http.MethodDelete, "/api/v1/houses/"+houseID+"/roles/"+roleID, nil, nil)
}

func (c *Client) ListMemberRoles(houseID, memberID string) ([]Role, error) {
	var out []Role
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/members/"+memberID+"/roles", nil, &out)
}

func (c *Client) GrantRole(houseID, memberID, roleID string) error {
	return c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/members/"+memberID+"/roles/"+roleID, nil, nil)
}

func (c *Client) RevokeRole(houseID, memberID, roleID string) error {
	return c.do(http.MethodDelete, "/api/v1/houses/"+houseID+"/members/"+memberID+"/roles/"+roleID, nil, nil)
}

// ----- Skills -----

func (c *Client) ListSkills(houseID string) ([]Skill, error) {
	var out []Skill
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/skills", nil, &out)
}

func (c *Client) CreateSkill(houseID string, body CreateSkillRequest) (*Skill, error) {
	var out Skill
	return &out, c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/skills", body, &out)
}

func (c *Client) DeleteSkill(houseID, skillID string) error {
	return c.do(http.MethodDelete, "/api/v1/houses/"+houseID+"/skills/"+skillID, nil, nil)
}

func (c *Client) ListMemberSkills(houseID, memberID string) ([]Skill, error) {
	var out []Skill
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/members/"+memberID+"/skills", nil, &out)
}

func (c *Client) AddMemberSkill(houseID, memberID, skillID string) error {
	return c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/members/"+memberID+"/skills/"+skillID, nil, nil)
}

func (c *Client) RemoveMemberSkill(houseID, memberID, skillID string) error {
	return c.do(http.MethodDelete, "/api/v1/houses/"+houseID+"/members/"+memberID+"/skills/"+skillID, nil, nil)
}

// ----- Groups -----

func (c *Client) ListGroups(houseID string) ([]Group, error) {
	var out []Group
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/groups", nil, &out)
}

func (c *Client) CreateGroup(houseID string, body CreateGroupRequest) (*Group, error) {
	var out Group
	return &out, c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/groups", body, &out)
}

func (c *Client) DeleteGroup(houseID, groupID string) error {
	return c.do(http.MethodDelete, "/api/v1/houses/"+houseID+"/groups/"+groupID, nil, nil)
}

func (c *Client) ListGroupMembers(houseID, groupID string) ([]Member, error) {
	var out []Member
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/groups/"+groupID+"/members", nil, &out)
}

func (c *Client) AddGroupMember(houseID, groupID, memberID string) error {
	return c.do(http.MethodPost,
		"/api/v1/houses/"+houseID+"/groups/"+groupID+"/members/"+memberID, nil, nil)
}

func (c *Client) RemoveGroupMember(houseID, groupID, memberID string) error {
	return c.do(http.MethodDelete,
		"/api/v1/houses/"+houseID+"/groups/"+groupID+"/members/"+memberID, nil, nil)
}

// ----- Events -----

func (c *Client) ListEvents(houseID string) ([]Event, error) {
	var out []Event
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/events", nil, &out)
}

func (c *Client) CreateEvent(houseID string, body CreateEventRequest) (*Event, error) {
	var out Event
	return &out, c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/events", body, &out)
}

func (c *Client) GetEvent(houseID, eventID string) (*Event, error) {
	var out Event
	return &out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/events/"+eventID, nil, &out)
}

// ----- Member audits -----

func (c *Client) ListAuditsForMember(houseID, memberID string) ([]MemberAudit, error) {
	var out []MemberAudit
	return out, c.do(http.MethodGet,
		"/api/v1/houses/"+houseID+"/members/"+memberID+"/audits", nil, &out)
}

// ----- Comments -----

func (c *Client) ListComments(houseID, targetType, targetID string) ([]Comment, error) {
	var out []Comment
	return out, c.do(http.MethodGet,
		"/api/v1/houses/"+houseID+"/comments/"+targetType+"/"+targetID, nil, &out)
}

func (c *Client) CreateComment(houseID, targetType, targetID string, body CreateCommentRequest) (*Comment, error) {
	var out Comment
	return &out, c.do(http.MethodPost,
		"/api/v1/houses/"+houseID+"/comments/"+targetType+"/"+targetID, body, &out)
}

func (c *Client) DeleteComment(houseID, commentID string) error {
	return c.do(http.MethodDelete, "/api/v1/houses/"+houseID+"/comments/"+commentID, nil, nil)
}

// ----- Trusted domains -----

func (c *Client) ListTrustedDomains(houseID string) ([]TrustedDomain, error) {
	var out []TrustedDomain
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/trusted-domains", nil, &out)
}

func (c *Client) CreateTrustedDomain(houseID string, body CreateTrustedDomainRequest) (*TrustedDomain, error) {
	var out TrustedDomain
	return &out, c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/trusted-domains", body, &out)
}

func (c *Client) DeleteTrustedDomain(houseID, tdID string) error {
	return c.do(http.MethodDelete, "/api/v1/houses/"+houseID+"/trusted-domains/"+tdID, nil, nil)
}

// ----- Projects -----

func (c *Client) ListProjects(houseID string) ([]Project, error) {
	var out []Project
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/projects", nil, &out)
}

func (c *Client) CreateProject(houseID string, body CreateProjectRequest) (*Project, error) {
	var out Project
	return &out, c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/projects", body, &out)
}

func (c *Client) GetProject(houseID, projectID string) (*Project, error) {
	var out Project
	return &out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/projects/"+projectID, nil, &out)
}

func (c *Client) ListProjectTasks(houseID, projectID string) ([]Task, error) {
	var out []Task
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/projects/"+projectID+"/tasks", nil, &out)
}

func (c *Client) AddProjectTask(houseID, projectID, taskID string, position int) error {
	body := struct {
		TaskID   string `json:"task_id"`
		Position int    `json:"position"`
	}{TaskID: taskID, Position: position}
	return c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/projects/"+projectID+"/tasks", body, nil)
}

// ----- Tasks -----

func (c *Client) ListTasks(houseID string) ([]Task, error) {
	var out []Task
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/tasks", nil, &out)
}

func (c *Client) CreateTask(houseID string, body CreateTaskRequest) (*Task, error) {
	var out Task
	return &out, c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/tasks", body, &out)
}

func (c *Client) GetTask(houseID, taskID string) (*Task, error) {
	var out Task
	return &out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/tasks/"+taskID, nil, &out)
}

func (c *Client) UpdateTask(houseID, taskID string, body UpdateTaskRequest) (*Task, error) {
	var out Task
	return &out, c.do(http.MethodPatch, "/api/v1/houses/"+houseID+"/tasks/"+taskID, body, &out)
}

// ----- Shares -----

func (c *Client) ListShares(houseID string) ([]Share, error) {
	var out []Share
	return out, c.do(http.MethodGet, "/api/v1/houses/"+houseID+"/shares", nil, &out)
}

func (c *Client) ListSharesByResource(houseID, resourceType, resourceID string) ([]Share, error) {
	var out []Share
	return out, c.do(http.MethodGet,
		"/api/v1/houses/"+houseID+"/shares?resource_type="+resourceType+"&resource_id="+resourceID, nil, &out)
}

func (c *Client) CreateShare(houseID string, body CreateShareRequest) (*Share, error) {
	var out Share
	return &out, c.do(http.MethodPost, "/api/v1/houses/"+houseID+"/shares", body, &out)
}

func (c *Client) DeleteShare(houseID, shareID string) error {
	return c.do(http.MethodDelete, "/api/v1/houses/"+houseID+"/shares/"+shareID, nil, nil)
}

// WithToken returns a copy of the client carrying a per-request token.
// Useful for building a session-scoped client without mutating shared state.
func (c *Client) WithToken(token string) *Client {
	cp := *c
	cp.Token = token
	return &cp
}
