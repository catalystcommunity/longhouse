package csilservices

import (
	"context"
	"errors"

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
	d.Register("project", "ListProjects", s.listProjects)
	d.Register("project", "GetProject", s.getProject)
	d.Register("project", "CreateProject", s.createProject)
	d.Register("project", "UpdateProject", s.updateProject)
	d.Register("project", "DeleteProject", s.deleteProject)
	d.Register("project", "ListProjectTasks", s.listProjectTasks)
	d.Register("project", "AddProjectTask", s.addProjectTask)
	d.Register("project", "RemoveProjectTask", s.removeProjectTask)
	d.Register("project", "SetProjectTaskPosition", s.setProjectTaskPosition)
	d.Register("project", "ListProjectMembers", s.listProjectMembers)
	d.Register("project", "AddProjectMember", s.addProjectMember)
	d.Register("project", "RemoveProjectMember", s.removeProjectMember)
	d.Register("project", "ListProjectOwners", s.listProjectOwners)
	d.Register("project", "AddProjectOwner", s.addProjectOwner)
	d.Register("project", "RemoveProjectOwner", s.removeProjectOwner)
	d.Register("project", "ListMilestones", s.listMilestones)
	d.Register("project", "CreateMilestone", s.createMilestone)
	d.Register("project", "UpdateMilestone", s.updateMilestone)
	d.Register("project", "DeleteMilestone", s.deleteMilestone)
}

// ---- project ----------------------------------------------------------

func (s *ProjectService) listProjects(ctx context.Context, body []byte) (any, error) {
	var req csil.HouseScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	rows, err := s.Store.ListProjectsByHouse(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return projectsToCSIL(rows), nil
}

func (s *ProjectService) getProject(ctx context.Context, body []byte) (any, error) {
	var id csil.ProjectID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	p, err := s.Store.GetProjectByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, csilrpc.NotFound("project not found")
		}
		return nil, csilrpc.Internal("internal error")
	}
	if _, _, err := requireMemberForHouse(ctx, p.HouseID); err != nil {
		return nil, err
	}
	return projectToCSIL(p), nil
}

func (s *ProjectService) createProject(ctx context.Context, body []byte) (any, error) {
	var in csil.Project
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.HouseId == "" || in.Name == "" {
		return nil, csilrpc.BadRequest("house_id and name are required")
	}
	if _, _, err := requireMemberForHouse(ctx, string(in.HouseId)); err != nil {
		return nil, err
	}
	p := &models.Project{
		HouseID:     string(in.HouseId),
		Name:        in.Name,
		Description: derefStr(in.Description),
		Category:    in.Category,
		Status:      derefProjectStatus(in.Status, "active"),
	}
	if err := s.Store.CreateProject(ctx, p); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return projectToCSIL(p), nil
}

func (s *ProjectService) updateProject(ctx context.Context, body []byte) (any, error) {
	var in csil.Project
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.ProjectId == "" {
		return nil, csilrpc.BadRequest("project_id is required")
	}
	existing, err := s.Store.GetProjectByID(ctx, string(in.ProjectId))
	if err != nil {
		return nil, csilrpc.NotFound("project not found")
	}
	if _, _, err := requireRoleForHouse(ctx, existing.HouseID, "admin"); err != nil {
		return nil, err
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
		return nil, csilrpc.Internal("internal error")
	}
	return projectToCSIL(existing), nil
}

func (s *ProjectService) deleteProject(ctx context.Context, body []byte) (any, error) {
	var id csil.ProjectID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	p, err := s.Store.GetProjectByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("project not found")
	}
	if _, _, err := requireRoleForHouse(ctx, p.HouseID, "admin"); err != nil {
		return nil, err
	}
	if err := s.Store.DeleteProject(ctx, p.ProjectID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// ---- project tasks ----------------------------------------------------

func (s *ProjectService) listProjectTasks(ctx context.Context, body []byte) (any, error) {
	var req csil.ProjectScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	tasks, err := s.Store.ListProjectTasks(ctx, string(req.ProjectId), limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	out := make([]csil.Task, len(tasks))
	for i, t := range tasks {
		assignees, _ := s.Store.ListTaskAssignees(ctx, t.TaskID)
		out[i] = taskToCSIL(&t, assignees)
	}
	return out, nil
}

func (s *ProjectService) addProjectTask(ctx context.Context, body []byte) (any, error) {
	var req csil.ProjectTaskOrderRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if err := s.authzProject(ctx, string(req.ProjectId)); err != nil {
		return nil, err
	}
	if err := s.Store.AddProjectTask(ctx, string(req.ProjectId), string(req.TaskId), int(req.Position)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *ProjectService) removeProjectTask(ctx context.Context, body []byte) (any, error) {
	var req csil.ProjectTaskRef
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if err := s.authzProject(ctx, string(req.ProjectId)); err != nil {
		return nil, err
	}
	if err := s.Store.RemoveProjectTask(ctx, string(req.ProjectId), string(req.TaskId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// setProjectTaskPosition reorders a project_tasks row. Implemented as a
// remove+add since the schema's PK is (project_id, task_id) and we don't
// have a dedicated update method on the store.
func (s *ProjectService) setProjectTaskPosition(ctx context.Context, body []byte) (any, error) {
	var req csil.ProjectTaskOrderRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if err := s.authzProject(ctx, string(req.ProjectId)); err != nil {
		return nil, err
	}
	if err := s.Store.RemoveProjectTask(ctx, string(req.ProjectId), string(req.TaskId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	if err := s.Store.AddProjectTask(ctx, string(req.ProjectId), string(req.TaskId), int(req.Position)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// ---- members / owners -------------------------------------------------

func (s *ProjectService) listProjectMembers(ctx context.Context, body []byte) (any, error) {
	id, err := s.requireProjectViewer(ctx, body)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListProjectMembers(ctx, id)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return membersToCSIL(rows), nil
}

func (s *ProjectService) listProjectOwners(ctx context.Context, body []byte) (any, error) {
	id, err := s.requireProjectViewer(ctx, body)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListProjectOwners(ctx, id)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return membersToCSIL(rows), nil
}

func (s *ProjectService) addProjectMember(ctx context.Context, body []byte) (any, error) {
	var ref csil.ProjectMemberRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	if err := s.authzProject(ctx, string(ref.ProjectId)); err != nil {
		return nil, err
	}
	if err := s.Store.AddProjectMember(ctx, string(ref.ProjectId), string(ref.MemberId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *ProjectService) removeProjectMember(ctx context.Context, body []byte) (any, error) {
	var ref csil.ProjectMemberRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	if err := s.authzProject(ctx, string(ref.ProjectId)); err != nil {
		return nil, err
	}
	if err := s.Store.RemoveProjectMember(ctx, string(ref.ProjectId), string(ref.MemberId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *ProjectService) addProjectOwner(ctx context.Context, body []byte) (any, error) {
	var ref csil.ProjectOwnerRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	if err := s.authzProject(ctx, string(ref.ProjectId)); err != nil {
		return nil, err
	}
	if err := s.Store.AddProjectOwner(ctx, string(ref.ProjectId), string(ref.MemberId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *ProjectService) removeProjectOwner(ctx context.Context, body []byte) (any, error) {
	var ref csil.ProjectOwnerRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	if err := s.authzProject(ctx, string(ref.ProjectId)); err != nil {
		return nil, err
	}
	if err := s.Store.RemoveProjectOwner(ctx, string(ref.ProjectId), string(ref.MemberId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// ---- milestones -------------------------------------------------------

func (s *ProjectService) listMilestones(ctx context.Context, body []byte) (any, error) {
	id, err := s.requireProjectViewer(ctx, body)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListMilestonesByProject(ctx, id)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return milestonesToCSIL(rows), nil
}

func (s *ProjectService) createMilestone(ctx context.Context, body []byte) (any, error) {
	var in csil.Milestone
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.ProjectId == "" || in.Label == "" {
		return nil, csilrpc.BadRequest("project_id and label are required")
	}
	if err := s.authzProject(ctx, string(in.ProjectId)); err != nil {
		return nil, err
	}
	m := &models.Milestone{
		ProjectID: string(in.ProjectId),
		Label:     in.Label,
		WhenLabel: in.WhenLabel,
		State:     milestoneStateString(in.State, models.MilestoneStateFuture),
		Position:  int(in.Position),
	}
	if err := s.Store.CreateMilestone(ctx, m); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return milestoneToCSIL(m), nil
}

func (s *ProjectService) updateMilestone(ctx context.Context, body []byte) (any, error) {
	var in csil.Milestone
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.MilestoneId == "" {
		return nil, csilrpc.BadRequest("milestone_id is required")
	}
	existing, err := s.Store.GetMilestoneByID(ctx, string(in.MilestoneId))
	if err != nil {
		return nil, csilrpc.NotFound("milestone not found")
	}
	if err := s.authzProject(ctx, existing.ProjectID); err != nil {
		return nil, err
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
		return nil, csilrpc.Internal("internal error")
	}
	return milestoneToCSIL(existing), nil
}

func (s *ProjectService) deleteMilestone(ctx context.Context, body []byte) (any, error) {
	var id csil.MilestoneID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetMilestoneByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("milestone not found")
	}
	if err := s.authzProject(ctx, existing.ProjectID); err != nil {
		return nil, err
	}
	if err := s.Store.DeleteMilestone(ctx, existing.MilestoneID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// ---- helpers ----------------------------------------------------------

// requireProjectViewer decodes a project id from the body and returns it
// after confirming the caller has read access to the project's house.
// Used by listMembers/listOwners/listMilestones, which all take ProjectID
// as their entire request.
func (s *ProjectService) requireProjectViewer(ctx context.Context, body []byte) (string, error) {
	var pid csil.ProjectID
	if err := csilrpc.Decode(body, &pid); err != nil {
		return "", err
	}
	p, err := s.Store.GetProjectByID(ctx, string(pid))
	if err != nil {
		return "", csilrpc.NotFound("project not found")
	}
	if _, _, err := requireMemberForHouse(ctx, p.HouseID); err != nil {
		return "", err
	}
	return p.ProjectID, nil
}

// authzProject is the admin-or-self gate for mutations. Loads the project
// to find its house, then requires the caller to hold the admin role in
// that house. (Self-as-owner is harder to express for projects today since
// there's no single-owner column; admin is the safe baseline.)
func (s *ProjectService) authzProject(ctx context.Context, projectID string) error {
	p, err := s.Store.GetProjectByID(ctx, projectID)
	if err != nil {
		return csilrpc.NotFound("project not found")
	}
	if _, _, err := requireRoleForHouse(ctx, p.HouseID, "admin"); err != nil {
		return err
	}
	return nil
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
	switch v := (*p).(type) {
	case string:
		return v
	default:
		return fallback
	}
}

// milestoneStateString teases the underlying string out of the CSIL
// enum alias (which generates as `interface{}`). Returns fallback for
// nil/unrecognized values so the caller can't accidentally write an
// empty state to the DB enum column.
func milestoneStateString(v csil.MilestoneState, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}
