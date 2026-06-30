package csilservices

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

// ProjectService combines project mutations with the per-project members,
// owners, tasks and milestone surfaces — the SPA reads all four from a
// project detail page, so co-locating them keeps the call graph tight.
type ProjectService struct{ Store store.Store }

func (s *ProjectService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("project", "ListProjects", csilrpc.Route(s.ListProjects, csil.DecodeProjectListProjectsRequest, csil.EncodeProjectListProjectsResponse))
	d.RegisterTyped("project", "GetProject", csilrpc.Route(s.GetProject, csil.DecodeProjectGetProjectRequest, csil.EncodeProjectGetProjectResponse))
	d.RegisterTyped("project", "CreateProject", csilrpc.Route(s.CreateProject, csil.DecodeProjectCreateProjectRequest, csil.EncodeProjectCreateProjectResponse))
	d.RegisterTyped("project", "UpdateProject", csilrpc.Route(s.UpdateProject, csil.DecodeProjectUpdateProjectRequest, csil.EncodeProjectUpdateProjectResponse))
	d.RegisterTyped("project", "DeleteProject", csilrpc.Route(s.DeleteProject, csil.DecodeProjectDeleteProjectRequest, csil.EncodeProjectDeleteProjectResponse))
	d.RegisterTyped("project", "ListProjectTasks", csilrpc.Route(s.ListProjectTasks, csil.DecodeProjectListProjectTasksRequest, csil.EncodeProjectListProjectTasksResponse))
	d.RegisterTyped("project", "AddProjectTask", csilrpc.Route(s.AddProjectTask, csil.DecodeProjectAddProjectTaskRequest, csil.EncodeProjectAddProjectTaskResponse))
	d.RegisterTyped("project", "RemoveProjectTask", csilrpc.Route(s.RemoveProjectTask, csil.DecodeProjectRemoveProjectTaskRequest, csil.EncodeProjectRemoveProjectTaskResponse))
	d.RegisterTyped("project", "SetProjectTaskPosition", csilrpc.Route(s.SetProjectTaskPosition, csil.DecodeProjectSetProjectTaskPositionRequest, csil.EncodeProjectSetProjectTaskPositionResponse))
	d.RegisterTyped("project", "ListProjectMembers", csilrpc.Route(s.ListProjectMembers, csil.DecodeProjectListProjectMembersRequest, csil.EncodeProjectListProjectMembersResponse))
	d.RegisterTyped("project", "AddProjectMember", csilrpc.Route(s.AddProjectMember, csil.DecodeProjectAddProjectMemberRequest, csil.EncodeProjectAddProjectMemberResponse))
	d.RegisterTyped("project", "RemoveProjectMember", csilrpc.Route(s.RemoveProjectMember, csil.DecodeProjectRemoveProjectMemberRequest, csil.EncodeProjectRemoveProjectMemberResponse))
	d.RegisterTyped("project", "ListProjectOwners", csilrpc.Route(s.ListProjectOwners, csil.DecodeProjectListProjectOwnersRequest, csil.EncodeProjectListProjectOwnersResponse))
	d.RegisterTyped("project", "AddProjectOwner", csilrpc.Route(s.AddProjectOwner, csil.DecodeProjectAddProjectOwnerRequest, csil.EncodeProjectAddProjectOwnerResponse))
	d.RegisterTyped("project", "RemoveProjectOwner", csilrpc.Route(s.RemoveProjectOwner, csil.DecodeProjectRemoveProjectOwnerRequest, csil.EncodeProjectRemoveProjectOwnerResponse))
	d.RegisterTyped("project", "ListMilestones", csilrpc.Route(s.ListMilestones, csil.DecodeProjectListMilestonesRequest, csil.EncodeProjectListMilestonesResponse))
	d.RegisterTyped("project", "CreateMilestone", csilrpc.Route(s.CreateMilestone, csil.DecodeProjectCreateMilestoneRequest, csil.EncodeProjectCreateMilestoneResponse))
	d.RegisterTyped("project", "UpdateMilestone", csilrpc.Route(s.UpdateMilestone, csil.DecodeProjectUpdateMilestoneRequest, csil.EncodeProjectUpdateMilestoneResponse))
	d.RegisterTyped("project", "DeleteMilestone", csilrpc.Route(s.DeleteMilestone, csil.DecodeProjectDeleteMilestoneRequest, csil.EncodeProjectDeleteMilestoneResponse))
	d.RegisterTyped("project", "SetProjectVisibility", csilrpc.Route(s.SetProjectVisibility, csil.DecodeProjectSetProjectVisibilityRequest, csil.EncodeProjectSetProjectVisibilityResponse))
	d.RegisterTyped("project", "ListProjectGrants", csilrpc.Route(s.ListProjectGrants, csil.DecodeProjectListProjectGrantsRequest, csil.EncodeProjectListProjectGrantsResponse))
	d.RegisterTyped("project", "PutProjectGrant", csilrpc.Route(s.PutProjectGrant, csil.DecodeProjectPutProjectGrantRequest, csil.EncodeProjectPutProjectGrantResponse))
	d.RegisterTyped("project", "DeleteProjectGrant", csilrpc.Route(s.DeleteProjectGrant, csil.DecodeProjectDeleteProjectGrantRequest, csil.EncodeProjectDeleteProjectGrantResponse))
}

// ---- project ----------------------------------------------------------

func (s *ProjectService) ListProjects(ctx context.Context, req csil.HouseScopedListRequest) (csil.ProjectList, error) {
	ident, memberID, err := requireMemberForHouse(ctx, string(req.HouseId))
	if err != nil {
		return csil.ProjectList{}, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	rows, err := s.Store.ListProjectsByHouse(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return csil.ProjectList{}, csilrpc.Internal("internal error")
	}
	// Filter to projects the caller may read; report the withheld count so a
	// private project's existence isn't itself concealed. See docs/rbac.md.
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, string(req.HouseId), memberID)
	out := make([]csil.Project, 0, len(rows))
	hidden := uint64(0)
	for i := range rows {
		if canRead(pol.projectAccess(ctx, &rows[i], g)) {
			out = append(out, projectToCSIL(&rows[i]))
		} else {
			hidden++
		}
	}
	return csil.ProjectList{Projects: out, HiddenCount: hidden}, nil
}

func (s *ProjectService) GetProject(ctx context.Context, id csil.ProjectID) (csil.Project, error) {
	p, err := s.Store.GetProjectByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return csil.Project{}, csilrpc.NotFound("project not found")
		}
		return csil.Project{}, csilrpc.Internal("internal error")
	}
	if p.DeletedAt != nil {
		return csil.Project{}, csilrpc.NotFound("project not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, p.HouseID)
	if err != nil {
		return csil.Project{}, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, p.HouseID, memberID)
	if !canRead(pol.projectAccess(ctx, p, g)) {
		return csil.Project{}, csilrpc.NotFound("project not found")
	}
	return projectToCSIL(p), nil
}

func (s *ProjectService) CreateProject(ctx context.Context, in csil.Project) (csil.Project, error) {
	if in.HouseId == "" || in.Name == "" {
		return csil.Project{}, csilrpc.BadRequest("house_id and name are required")
	}
	_, callerMemberID, err := requireMemberForHouse(ctx, string(in.HouseId))
	if err != nil {
		return csil.Project{}, err
	}
	// Visibility: honor an explicit request, else stamp the house default
	// (default_project_visibility, falling back to read). See docs/rbac.md.
	vis := accessLevelPtr(in.Visibility, "")
	if !validAccessLevel(vis) {
		vis = s.defaultProjectVisibility(ctx, string(in.HouseId))
	}
	creator := callerMemberID
	p := &models.Project{
		HouseID:           string(in.HouseId),
		Name:              in.Name,
		Description:       derefStr(in.Description),
		Category:          in.Category,
		Status:            derefProjectStatus(in.Status, "active"),
		Visibility:        vis,
		CreatedByMemberID: &creator,
	}
	if err := s.Store.CreateProject(ctx, p); err != nil {
		return csil.Project{}, csilrpc.Internal("internal error")
	}
	// Seed the creator as an owner (full) grant so they retain governance even
	// if they later drop the creator fallback (e.g. reassigned).
	_ = s.Store.AddProjectOwner(ctx, p.ProjectID, callerMemberID)
	return projectToCSIL(p), nil
}

// defaultProjectVisibility reads the house setting, falling back to read.
func (s *ProjectService) defaultProjectVisibility(ctx context.Context, houseID string) string {
	raw, ok, err := readHouseSetting(ctx, s.Store, houseID, settingDefaultProjectVisibility)
	if err != nil || !ok {
		return models.AccessRead
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil || !validAccessLevel(v) {
		return models.AccessRead
	}
	return v
}

func (s *ProjectService) UpdateProject(ctx context.Context, in csil.Project) (csil.Project, error) {
	if in.ProjectId == "" {
		return csil.Project{}, csilrpc.BadRequest("project_id is required")
	}
	existing, err := s.Store.GetProjectByID(ctx, string(in.ProjectId))
	if err != nil {
		return csil.Project{}, csilrpc.NotFound("project not found")
	}
	// Content edits require `edit`. Visibility/grants/created_by are
	// governance — managed via the dedicated ops (full). An inbound
	// Visibility / CreatedByMemberId here is ignored. See docs/rbac.md §7.
	if _, _, err := s.requireProjectAccess(ctx, existing, models.AccessEdit); err != nil {
		return csil.Project{}, err
	}
	if in.Name != "" {
		existing.Name = in.Name
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	existing.Category = in.Category
	if in.Status != nil {
		existing.Status = derefProjectStatus(in.Status, existing.Status)
	}
	if err := s.Store.UpdateProject(ctx, existing); err != nil {
		return csil.Project{}, csilrpc.Internal("internal error")
	}
	return projectToCSIL(existing), nil
}

func (s *ProjectService) DeleteProject(ctx context.Context, id csil.ProjectID) (csil.EmptyResponse, error) {
	p, err := s.Store.GetProjectByID(ctx, string(id))
	if err != nil || p.DeletedAt != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("project not found")
	}
	// Delete is governance — requires full.
	_, g, err := s.requireProjectAccess(ctx, p, models.AccessFull)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	// Soft delete into the trash: a configurable-retention purge worker
	// removes it for good later; an admin can restore it until then.
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteProject(ctx, p.ProjectID, g.memberID, opID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, p.HouseID, "project", p.ProjectID, opID, p)
	return csil.EmptyResponse{}, nil
}

// ---- project tasks ----------------------------------------------------

func (s *ProjectService) ListProjectTasks(ctx context.Context, req csil.ProjectScopedListRequest) (csil.TaskList, error) {
	ident, memberID, err := requireMemberForHouse(ctx, string(req.HouseId))
	if err != nil {
		return csil.TaskList{}, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	tasks, err := s.Store.ListProjectTasks(ctx, string(req.ProjectId), limit, offset)
	if err != nil {
		return csil.TaskList{}, csilrpc.Internal("internal error")
	}
	// Even within a project, a task may carry tighter visibility — resolve
	// each and filter, reporting the withheld count. See docs/rbac.md.
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, string(req.HouseId), memberID)
	out := make([]csil.Task, 0, len(tasks))
	hidden := uint64(0)
	for i := range tasks {
		if canRead(pol.taskAccess(ctx, &tasks[i], g)) {
			assignees, _ := s.Store.ListTaskAssignees(ctx, tasks[i].TaskID)
			out = append(out, taskToCSIL(&tasks[i], assignees))
		} else {
			hidden++
		}
	}
	return csil.TaskList{Tasks: out, HiddenCount: hidden}, nil
}

func (s *ProjectService) AddProjectTask(ctx context.Context, req csil.ProjectTaskOrderRequest) (csil.EmptyResponse, error) {
	if err := s.authzProject(ctx, string(req.ProjectId), models.AccessEdit); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.AddProjectTask(ctx, string(req.ProjectId), string(req.TaskId), int(req.Position)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *ProjectService) RemoveProjectTask(ctx context.Context, req csil.ProjectTaskRef) (csil.EmptyResponse, error) {
	if err := s.authzProject(ctx, string(req.ProjectId), models.AccessEdit); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.RemoveProjectTask(ctx, string(req.ProjectId), string(req.TaskId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// SetProjectTaskPosition reorders a project_tasks row. Implemented as a
// remove+add since the schema's PK is (project_id, task_id) and we don't
// have a dedicated update method on the store.
func (s *ProjectService) SetProjectTaskPosition(ctx context.Context, req csil.ProjectTaskOrderRequest) (csil.EmptyResponse, error) {
	if err := s.authzProject(ctx, string(req.ProjectId), models.AccessEdit); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.RemoveProjectTask(ctx, string(req.ProjectId), string(req.TaskId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if err := s.Store.AddProjectTask(ctx, string(req.ProjectId), string(req.TaskId), int(req.Position)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// ---- members / owners -------------------------------------------------

func (s *ProjectService) ListProjectMembers(ctx context.Context, id csil.ProjectID) ([]csil.Member, error) {
	pid, err := s.requireProjectViewer(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListProjectMembers(ctx, pid)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return membersToCSIL(rows), nil
}

func (s *ProjectService) ListProjectOwners(ctx context.Context, id csil.ProjectID) ([]csil.Member, error) {
	pid, err := s.requireProjectViewer(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListProjectOwners(ctx, pid)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return membersToCSIL(rows), nil
}

func (s *ProjectService) AddProjectMember(ctx context.Context, ref csil.ProjectMemberRef) (csil.EmptyResponse, error) {
	if err := s.authzProject(ctx, string(ref.ProjectId), models.AccessEdit); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.AddProjectMember(ctx, string(ref.ProjectId), string(ref.MemberId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *ProjectService) RemoveProjectMember(ctx context.Context, ref csil.ProjectMemberRef) (csil.EmptyResponse, error) {
	if err := s.authzProject(ctx, string(ref.ProjectId), models.AccessEdit); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.RemoveProjectMember(ctx, string(ref.ProjectId), string(ref.MemberId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *ProjectService) AddProjectOwner(ctx context.Context, ref csil.ProjectOwnerRef) (csil.EmptyResponse, error) {
	if err := s.authzProject(ctx, string(ref.ProjectId), models.AccessEdit); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.AddProjectOwner(ctx, string(ref.ProjectId), string(ref.MemberId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *ProjectService) RemoveProjectOwner(ctx context.Context, ref csil.ProjectOwnerRef) (csil.EmptyResponse, error) {
	if err := s.authzProject(ctx, string(ref.ProjectId), models.AccessEdit); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.RemoveProjectOwner(ctx, string(ref.ProjectId), string(ref.MemberId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// ---- milestones -------------------------------------------------------

func (s *ProjectService) ListMilestones(ctx context.Context, id csil.ProjectID) ([]csil.Milestone, error) {
	pid, err := s.requireProjectViewer(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListMilestonesByProject(ctx, pid)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return milestonesToCSIL(rows), nil
}

func (s *ProjectService) CreateMilestone(ctx context.Context, in csil.Milestone) (csil.Milestone, error) {
	if in.ProjectId == "" || in.Label == "" {
		return csil.Milestone{}, csilrpc.BadRequest("project_id and label are required")
	}
	if err := s.authzProject(ctx, string(in.ProjectId), models.AccessEdit); err != nil {
		return csil.Milestone{}, err
	}
	m := &models.Milestone{
		ProjectID: string(in.ProjectId),
		Label:     in.Label,
		WhenLabel: in.WhenLabel,
		State:     milestoneStateString(in.State, models.MilestoneStateFuture),
		Position:  int(in.Position),
	}
	if err := s.Store.CreateMilestone(ctx, m); err != nil {
		return csil.Milestone{}, csilrpc.Internal("internal error")
	}
	return milestoneToCSIL(m), nil
}

func (s *ProjectService) UpdateMilestone(ctx context.Context, in csil.Milestone) (csil.Milestone, error) {
	if in.MilestoneId == "" {
		return csil.Milestone{}, csilrpc.BadRequest("milestone_id is required")
	}
	existing, err := s.Store.GetMilestoneByID(ctx, string(in.MilestoneId))
	if err != nil {
		return csil.Milestone{}, csilrpc.NotFound("milestone not found")
	}
	if err := s.authzProject(ctx, existing.ProjectID, models.AccessEdit); err != nil {
		return csil.Milestone{}, err
	}
	if in.Label != "" {
		existing.Label = in.Label
	}
	if in.WhenLabel != "" {
		existing.WhenLabel = in.WhenLabel
	}
	if v := milestoneStateString(in.State, ""); v != "" {
		existing.State = v
	}
	if in.Position != 0 {
		existing.Position = int(in.Position)
	}
	if err := s.Store.UpdateMilestone(ctx, existing); err != nil {
		return csil.Milestone{}, csilrpc.Internal("internal error")
	}
	return milestoneToCSIL(existing), nil
}

func (s *ProjectService) DeleteMilestone(ctx context.Context, id csil.MilestoneID) (csil.EmptyResponse, error) {
	existing, err := s.Store.GetMilestoneByID(ctx, string(id))
	if err != nil || existing.DeletedAt != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("milestone not found")
	}
	if err := s.authzProject(ctx, existing.ProjectID, models.AccessEdit); err != nil {
		return csil.EmptyResponse{}, err
	}
	// Resolve the caller's member id in the project's house to stamp
	// deleted_by_member_id (milestones carry project_id, not house_id).
	p, err := s.Store.GetProjectByID(ctx, existing.ProjectID)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	_, callerMemberID, err := requireMemberForHouse(ctx, p.HouseID)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteMilestone(ctx, existing.MilestoneID, callerMemberID, opID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, p.HouseID, "milestone", existing.MilestoneID, opID, existing)
	return csil.EmptyResponse{}, nil
}

// ---- helpers ----------------------------------------------------------

// requireProjectViewer takes a project id and returns it after confirming
// the caller has read access to the project's house. Used by
// listMembers/listOwners/listMilestones, which all take ProjectID as their
// entire request.
func (s *ProjectService) requireProjectViewer(ctx context.Context, pid csil.ProjectID) (string, error) {
	p, err := s.Store.GetProjectByID(ctx, string(pid))
	if err != nil {
		return "", csilrpc.NotFound("project not found")
	}
	if _, _, err := requireMemberForHouse(ctx, p.HouseID); err != nil {
		return "", err
	}
	return p.ProjectID, nil
}

// requireProjectAccess loads (if needed) and gates a project mutation by the
// caller's resolved access level. Returns the identity grantee for reuse.
// `need` is the minimum level (edit for content, full for governance).
func (s *ProjectService) requireProjectAccess(ctx context.Context, p *models.Project, need string) (*auth.Identity, grantee, error) {
	ident, memberID, err := requireMemberForHouse(ctx, p.HouseID)
	if err != nil {
		return nil, grantee{}, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, p.HouseID, memberID)
	level := pol.projectAccess(ctx, p, g)
	if models.AccessRank(level) < models.AccessRank(need) {
		return nil, grantee{}, csilrpc.Forbidden("you need " + need + " access to this project")
	}
	return ident, g, nil
}

// authzProject is the grant-based gate for project mutations that take a
// project id. `need` is the minimum access level. Replaces the old admin-only
// check now that projects carry grants. See docs/rbac.md §7.
func (s *ProjectService) authzProject(ctx context.Context, projectID, need string) error {
	p, err := s.Store.GetProjectByID(ctx, projectID)
	if err != nil {
		return csilrpc.NotFound("project not found")
	}
	if _, _, err := s.requireProjectAccess(ctx, p, need); err != nil {
		return err
	}
	return nil
}

// ---- visibility + grants ----------------------------------------------

// setProjectVisibility changes the project's house-at-large visibility.
// Requires full. Projects have no container, so there is no umbrella ceiling.
func (s *ProjectService) SetProjectVisibility(ctx context.Context, in csil.SetProjectVisibilityRequest) (csil.Project, error) {
	if in.ProjectId == "" {
		return csil.Project{}, csilrpc.BadRequest("project_id is required")
	}
	want := accessLevelVal(in.Visibility, "")
	if !validAccessLevel(want) {
		return csil.Project{}, csilrpc.BadRequest("invalid visibility")
	}
	p, err := s.Store.GetProjectByID(ctx, string(in.ProjectId))
	if err != nil {
		return csil.Project{}, csilrpc.NotFound("project not found")
	}
	if _, _, err := s.requireProjectAccess(ctx, p, models.AccessFull); err != nil {
		return csil.Project{}, err
	}
	p.Visibility = want
	if err := s.Store.UpdateProject(ctx, p); err != nil {
		return csil.Project{}, csilrpc.Internal("internal error")
	}
	return projectToCSIL(p), nil
}

// ListProjectGrants returns the project's explicit grants (governance — full).
func (s *ProjectService) ListProjectGrants(ctx context.Context, id csil.ProjectID) ([]csil.Grant, error) {
	p, err := s.Store.GetProjectByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("project not found")
	}
	if _, _, err := s.requireProjectAccess(ctx, p, models.AccessFull); err != nil {
		return nil, err
	}
	grants, err := s.Store.ListProjectGrants(ctx, p.ProjectID)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return projectGrantsToCSIL(grants), nil
}

func (s *ProjectService) PutProjectGrant(ctx context.Context, in csil.PutProjectGrantRequest) (csil.EmptyResponse, error) {
	gt := granteeTypeVal(in.GranteeType)
	if gt != models.GranteeMember && gt != models.GranteeGroup {
		return csil.EmptyResponse{}, csilrpc.BadRequest("invalid grantee_type")
	}
	if in.GranteeId == "" {
		return csil.EmptyResponse{}, csilrpc.BadRequest("grantee_id is required")
	}
	level := accessLevelVal(in.AccessLevel, "")
	if !validAccessLevel(level) {
		return csil.EmptyResponse{}, csilrpc.BadRequest("invalid access_level")
	}
	p, err := s.Store.GetProjectByID(ctx, string(in.ProjectId))
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("project not found")
	}
	if _, _, err := s.requireProjectAccess(ctx, p, models.AccessFull); err != nil {
		return csil.EmptyResponse{}, err
	}
	grant := &models.ProjectGrant{
		ProjectID: p.ProjectID, HouseID: p.HouseID,
		GranteeType: gt, GranteeID: in.GranteeId, AccessLevel: level,
	}
	if err := s.Store.PutProjectGrant(ctx, grant); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *ProjectService) DeleteProjectGrant(ctx context.Context, in csil.ProjectGrantRef) (csil.EmptyResponse, error) {
	gt := granteeTypeVal(in.GranteeType)
	if gt == "" || in.GranteeId == "" {
		return csil.EmptyResponse{}, csilrpc.BadRequest("grantee_type and grantee_id are required")
	}
	p, err := s.Store.GetProjectByID(ctx, string(in.ProjectId))
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("project not found")
	}
	if _, _, err := s.requireProjectAccess(ctx, p, models.AccessFull); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.DeleteProjectGrant(ctx, p.ProjectID, gt, in.GranteeId); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// requireRoleForHouse is the role-gated counterpart of requireMemberForHouse.
func requireRoleForHouse(ctx context.Context, houseID string, anyOf ...string) (string, string, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return "", "", err
	}
	memberID, err := requireRole(id, houseID, anyOf...)
	if err != nil {
		return "", "", err
	}
	return string(id.UserID), memberID, nil
}

func derefProjectStatus(p *csil.ProjectStatus, fallback string) string {
	if p == nil {
		return fallback
	}
	if s := string(*p); s != "" {
		return s
	}
	return fallback
}

// milestoneStateString returns the underlying string of the CSIL MilestoneState
// enum, or fallback for an empty value so the caller can't accidentally write an
// empty state to the DB enum column.
func milestoneStateString(v csil.MilestoneState, fallback string) string {
	if s := string(v); s != "" {
		return s
	}
	return fallback
}
