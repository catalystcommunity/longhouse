package csilservices

import (
	"context"
	"errors"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

// TaskService implements the CSIL TaskService over the dispatcher. Assignees
// are now a set (task_assignees join), not a single field — the methods here
// reconcile that set against the inbound Task.Assignees list. Task tag and
// estimate_minutes are passed through as opaque values; validation lives at
// the SPA layer for now (CSIL just constrains presence + type).
type TaskService struct{ Store store.Store }

func (s *TaskService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("task", "ListTasks", csilrpc.Route(s.ListTasks, csil.DecodeTaskListTasksRequest, csil.EncodeTaskListTasksResponse))
	d.RegisterTyped("task", "GetTask", csilrpc.Route(s.GetTask, csil.DecodeTaskGetTaskRequest, csil.EncodeTaskGetTaskResponse))
	d.RegisterTyped("task", "CreateTask", csilrpc.Route(s.CreateTask, csil.DecodeTaskCreateTaskRequest, csil.EncodeTaskCreateTaskResponse))
	d.RegisterTyped("task", "UpdateTask", csilrpc.Route(s.UpdateTask, csil.DecodeTaskUpdateTaskRequest, csil.EncodeTaskUpdateTaskResponse))
	d.RegisterTyped("task", "DeleteTask", csilrpc.Route(s.DeleteTask, csil.DecodeTaskDeleteTaskRequest, csil.EncodeTaskDeleteTaskResponse))
	d.RegisterTyped("task", "SetTaskVisibility", csilrpc.Route(s.SetTaskVisibility, csil.DecodeTaskSetTaskVisibilityRequest, csil.EncodeTaskSetTaskVisibilityResponse))
	d.RegisterTyped("task", "ListTaskGrants", csilrpc.Route(s.ListTaskGrants, csil.DecodeTaskListTaskGrantsRequest, csil.EncodeTaskListTaskGrantsResponse))
	d.RegisterTyped("task", "PutTaskGrant", csilrpc.Route(s.PutTaskGrant, csil.DecodeTaskPutTaskGrantRequest, csil.EncodeTaskPutTaskGrantResponse))
	d.RegisterTyped("task", "DeleteTaskGrant", csilrpc.Route(s.DeleteTaskGrant, csil.DecodeTaskDeleteTaskGrantRequest, csil.EncodeTaskDeleteTaskGrantResponse))
}

func (s *TaskService) ListTasks(ctx context.Context, req csil.HouseScopedListRequest) (csil.TaskList, error) {
	id, memberID, err := requireMemberForHouse(ctx, string(req.HouseId))
	if err != nil {
		return csil.TaskList{}, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	tasks, err := s.Store.ListTasksByHouse(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return csil.TaskList{}, csilrpc.Internal("internal error")
	}
	// Resolve effective access per task and filter to what the caller may
	// read. hidden_count reports how many rows in this page were withheld —
	// "you can hide a task, you can't hide that you hid one". See docs/rbac.md.
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, id, string(req.HouseId), memberID)
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

func (s *TaskService) GetTask(ctx context.Context, id csil.TaskID) (csil.Task, error) {
	t, err := s.Store.GetTaskByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return csil.Task{}, csilrpc.NotFound("task not found")
		}
		return csil.Task{}, csilrpc.Internal("internal error")
	}
	ident, memberID, err := requireMemberForHouse(ctx, t.HouseID)
	if err != nil {
		return csil.Task{}, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, t.HouseID, memberID)
	if !canRead(pol.taskAccess(ctx, t, g)) {
		// Don't leak existence of a task the caller can't see.
		return csil.Task{}, csilrpc.NotFound("task not found")
	}
	assignees, _ := s.Store.ListTaskAssignees(ctx, t.TaskID)
	return taskToCSIL(t, assignees), nil
}

func (s *TaskService) CreateTask(ctx context.Context, in csil.Task) (csil.Task, error) {
	if in.HouseId == "" {
		return csil.Task{}, csilrpc.BadRequest("house_id is required")
	}
	if in.Title == "" {
		return csil.Task{}, csilrpc.BadRequest("title is required")
	}
	_, callerMemberID, err := requireMemberForHouse(ctx, string(in.HouseId))
	if err != nil {
		return csil.Task{}, err
	}
	owner := callerMemberID
	if in.OwnerMemberId != "" {
		owner = string(in.OwnerMemberId)
	}
	t := &models.Task{
		HouseID:       string(in.HouseId),
		OwnerMemberID: owner,
		Title:         in.Title,
		Description:   derefStr(in.Description),
		Status:        derefTaskStatus(in.Status, "open"),
		Tag:           in.Tag,
		DueAt:         tsToTimePtr(in.DueAt),
	}
	if in.EstimateMinutes != nil {
		v := int(*in.EstimateMinutes)
		t.EstimateMinutes = &v
	}
	if in.AssignedToSkillId != nil {
		v := string(*in.AssignedToSkillId)
		t.AssignedToSkillID = &v
	}
	if in.ParentTaskId != nil {
		v := string(*in.ParentTaskId)
		t.ParentTaskID = &v
	}
	// Visibility at create. A task isn't in any project yet, so the only
	// possible container is a parent task:
	//   * Top-level (no parent): pinned to read — no free-floating private
	//     tasks. To get a private task, attach it to a private project and
	//     then set-task-visibility. See docs/rbac.md.
	//   * Subtask: inherit the umbrella — clamp the requested level (default
	//     read) down to the parent's visibility.
	if in.ParentTaskId != nil {
		requestedVis := accessLevelPtr(in.Visibility, models.AccessRead)
		ceil := models.AccessRead
		if parent, perr := s.Store.GetTaskByID(ctx, string(*in.ParentTaskId)); perr == nil {
			ceil = visibilityOf(parent.Visibility)
		}
		t.Visibility = models.MinAccess(requestedVis, ceil)
	} else {
		t.Visibility = models.AccessRead
	}
	// Recurrence: freq=nil means not recurring. Empty string also disables.
	// The worker reads next_recurrence_at as the spawn anchor; seed it from
	// due_at when set, otherwise leave nil and the root is dormant until
	// edited.
	if in.RecurrenceFreq != nil {
		if f := string(*in.RecurrenceFreq); f != "" {
			t.RecurrenceFreq = &f
		}
	}
	if in.RecurrenceInterval != nil && *in.RecurrenceInterval > 0 {
		t.RecurrenceInterval = int(*in.RecurrenceInterval)
	}
	if len(in.RecurrenceByWeekday) > 0 {
		w := make(models.IntList, len(in.RecurrenceByWeekday))
		for i, d := range in.RecurrenceByWeekday {
			w[i] = int(d)
		}
		t.RecurrenceByWeekday = w
	}
	if in.RecurrenceBySetpos != nil && *in.RecurrenceBySetpos != 0 {
		v := int(*in.RecurrenceBySetpos)
		t.RecurrenceBySetpos = &v
	}
	if in.NextRecurrenceAt != nil {
		t.NextRecurrenceAt = tsToTimePtr(in.NextRecurrenceAt)
	} else if t.RecurrenceFreq != nil && t.DueAt != nil {
		anchor := *t.DueAt
		t.NextRecurrenceAt = &anchor
	}
	if err := s.Store.CreateTask(ctx, t); err != nil {
		return csil.Task{}, csilrpc.Internal("internal error")
	}
	// Default-assignee policy at create time:
	//   * Assignees omitted from the request (nil slice on the wire) →
	//     - Subtask (parent_task_id set): copy parent's assignees.
	//     - Otherwise: assign the creating member.
	//   * Assignees explicitly empty ([] on the wire) → no assignees.
	//     The SPA uses this to create a project task with no default
	//     assignee, so the project can hand the work out later.
	//   * Assignees explicitly listed → use as-is.
	switch {
	case in.Assignees == nil:
		if in.ParentTaskId != nil {
			parents, _ := s.Store.ListTaskAssignees(ctx, string(*in.ParentTaskId))
			for _, a := range parents {
				_ = s.Store.AddTaskAssignee(ctx, t.TaskID, a.MemberID)
			}
		} else {
			_ = s.Store.AddTaskAssignee(ctx, t.TaskID, callerMemberID)
		}
	default:
		for _, mid := range in.Assignees {
			if mid == "" {
				continue
			}
			if err := s.Store.AddTaskAssignee(ctx, t.TaskID, string(mid)); err != nil {
				return csil.Task{}, csilrpc.Internal("could not attach assignee")
			}
		}
	}
	assignees, _ := s.Store.ListTaskAssignees(ctx, t.TaskID)
	return taskToCSIL(t, assignees), nil
}

func (s *TaskService) UpdateTask(ctx context.Context, in csil.Task) (csil.Task, error) {
	if in.TaskId == "" {
		return csil.Task{}, csilrpc.BadRequest("task_id is required")
	}
	existing, err := s.Store.GetTaskByID(ctx, string(in.TaskId))
	if err != nil {
		return csil.Task{}, csilrpc.NotFound("task not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return csil.Task{}, err
	}
	// Content edits require `edit`. Visibility and grants are governance and
	// are changed only via set-task-visibility / *-task-grant (full). An
	// inbound Visibility here is ignored. See docs/rbac.md §7.
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, existing.HouseID, memberID)
	if !canEdit(pol.taskAccess(ctx, existing, g)) {
		return csil.Task{}, csilrpc.Forbidden("you need edit access to change this task")
	}
	if in.Title != "" {
		existing.Title = in.Title
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	if in.Status != nil {
		existing.Status = derefTaskStatus(in.Status, existing.Status)
	}
	existing.Tag = in.Tag
	if in.EstimateMinutes != nil {
		v := int(*in.EstimateMinutes)
		existing.EstimateMinutes = &v
	}
	existing.DueAt = tsToTimePtr(in.DueAt)
	// Recurrence edits mirror Event semantics: empty-string freq clears the
	// schedule and wipes next/by_weekday/by_setpos; a non-empty freq toggles
	// recurrence on (seeding next_recurrence_at from due_at when nothing
	// else specifies it).
	wasRecurring := existing.RecurrenceFreq != nil
	if in.RecurrenceFreq != nil {
		f := string(*in.RecurrenceFreq)
		if f == "" {
			existing.RecurrenceFreq = nil
			existing.NextRecurrenceAt = nil
			existing.RecurrenceByWeekday = nil
			existing.RecurrenceBySetpos = nil
		} else {
			existing.RecurrenceFreq = &f
			if !wasRecurring && existing.NextRecurrenceAt == nil && existing.DueAt != nil {
				anchor := *existing.DueAt
				existing.NextRecurrenceAt = &anchor
			}
		}
	}
	if in.RecurrenceInterval != nil && *in.RecurrenceInterval > 0 {
		existing.RecurrenceInterval = int(*in.RecurrenceInterval)
	}
	if in.RecurrenceByWeekday != nil {
		if len(in.RecurrenceByWeekday) == 0 {
			existing.RecurrenceByWeekday = nil
		} else {
			w := make(models.IntList, len(in.RecurrenceByWeekday))
			for i, d := range in.RecurrenceByWeekday {
				w[i] = int(d)
			}
			existing.RecurrenceByWeekday = w
		}
	}
	if in.RecurrenceBySetpos != nil {
		if *in.RecurrenceBySetpos == 0 {
			existing.RecurrenceBySetpos = nil
		} else {
			v := int(*in.RecurrenceBySetpos)
			existing.RecurrenceBySetpos = &v
		}
	}
	if in.NextRecurrenceAt != nil {
		existing.NextRecurrenceAt = tsToTimePtr(in.NextRecurrenceAt)
	}
	if err := s.Store.UpdateTask(ctx, existing); err != nil {
		return csil.Task{}, csilrpc.Internal("internal error")
	}
	// Replace the assignees set if the caller provided one. A nil slice
	// means "don't touch"; an empty slice means "clear all assignees".
	if in.Assignees != nil {
		cur, _ := s.Store.ListTaskAssignees(ctx, existing.TaskID)
		curSet := map[string]bool{}
		for _, m := range cur {
			curSet[m.MemberID] = true
		}
		want := map[string]bool{}
		for _, mid := range in.Assignees {
			if mid == "" {
				continue
			}
			want[string(mid)] = true
		}
		for mid := range want {
			if !curSet[mid] {
				_ = s.Store.AddTaskAssignee(ctx, existing.TaskID, mid)
			}
		}
		for mid := range curSet {
			if !want[mid] {
				_ = s.Store.RemoveTaskAssignee(ctx, existing.TaskID, mid)
			}
		}
	}
	assignees, _ := s.Store.ListTaskAssignees(ctx, existing.TaskID)
	return taskToCSIL(existing, assignees), nil
}

// deleteTask soft-deletes (sets deleted_at). Hard-deletes are intentionally
// not exposed — see the historical handler comment for context. The
// recurrence worker reads `deleted_at IS NULL` to skip soft-deleted rows.
func (s *TaskService) DeleteTask(ctx context.Context, id csil.TaskID) (csil.EmptyResponse, error) {
	existing, err := s.Store.GetTaskByID(ctx, string(id))
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("task not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	// Delete is governance — requires full (owner/admin resolve to full).
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, existing.HouseID, memberID)
	if !canFull(pol.taskAccess(ctx, existing, g)) {
		return csil.EmptyResponse{}, csilrpc.Forbidden("you need full access to delete this task")
	}
	if existing.DeletedAt != nil {
		return csil.EmptyResponse{}, nil // idempotent
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteTask(ctx, existing.TaskID, memberID, opID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, existing.HouseID, "task", existing.TaskID, opID, existing)
	return csil.EmptyResponse{}, nil
}

// setTaskVisibility changes the task's house-at-large visibility. Requires
// full, and the requested level is bounded by the umbrella guardrail (a task
// can't be more visible than the MIN of its containers). See docs/rbac.md.
func (s *TaskService) SetTaskVisibility(ctx context.Context, in csil.SetTaskVisibilityRequest) (csil.Task, error) {
	if in.TaskId == "" {
		return csil.Task{}, csilrpc.BadRequest("task_id is required")
	}
	want := accessLevelVal(in.Visibility, "")
	if !validAccessLevel(want) {
		return csil.Task{}, csilrpc.BadRequest("invalid visibility")
	}
	existing, err := s.Store.GetTaskByID(ctx, string(in.TaskId))
	if err != nil {
		return csil.Task{}, csilrpc.NotFound("task not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return csil.Task{}, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, existing.HouseID, memberID)
	if !canFull(pol.taskAccess(ctx, existing, g)) {
		return csil.Task{}, csilrpc.Forbidden("you need full access to change visibility")
	}
	// A free-floating task (no parent, no projects) is pinned to read: it can
	// be neither raised above read nor made private. Privacy requires a
	// containing project. See docs/rbac.md.
	if pol.isFreeFloating(ctx, existing) {
		if want != models.AccessRead {
			return csil.Task{}, csilrpc.BadRequest("a task with no project or parent must stay read; add it to a project to change visibility")
		}
	} else {
		ceil := pol.maxAllowedTaskVisibility(ctx, existing)
		if models.AccessRank(want) > models.AccessRank(ceil) {
			return csil.Task{}, csilrpc.BadRequest("visibility cannot exceed the task's least-visible project or parent (" + ceil + ")")
		}
	}
	existing.Visibility = want
	if err := s.Store.UpdateTask(ctx, existing); err != nil {
		return csil.Task{}, csilrpc.Internal("internal error")
	}
	assignees, _ := s.Store.ListTaskAssignees(ctx, existing.TaskID)
	return taskToCSIL(existing, assignees), nil
}

// listTaskGrants returns the task's explicit grants. Viewing the access list
// is governance — requires full.
func (s *TaskService) ListTaskGrants(ctx context.Context, id csil.TaskID) ([]csil.Grant, error) {
	t, _, err := s.requireTaskFull(ctx, string(id))
	if err != nil {
		return nil, err
	}
	grants, err := s.Store.ListTaskGrants(ctx, t.TaskID)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return taskGrantsToCSIL(grants), nil
}

// putTaskGrant adds or updates a (grantee, level) grant on the task.
func (s *TaskService) PutTaskGrant(ctx context.Context, in csil.PutTaskGrantRequest) (csil.EmptyResponse, error) {
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
	t, _, err := s.requireTaskFull(ctx, string(in.TaskId))
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	grant := &models.TaskGrant{
		TaskID: t.TaskID, HouseID: t.HouseID,
		GranteeType: gt, GranteeID: in.GranteeId, AccessLevel: level,
	}
	if err := s.Store.PutTaskGrant(ctx, grant); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// deleteTaskGrant removes a grant from the task.
func (s *TaskService) DeleteTaskGrant(ctx context.Context, in csil.TaskGrantRef) (csil.EmptyResponse, error) {
	gt := granteeTypeVal(in.GranteeType)
	if gt == "" || in.GranteeId == "" {
		return csil.EmptyResponse{}, csilrpc.BadRequest("grantee_type and grantee_id are required")
	}
	t, _, err := s.requireTaskFull(ctx, string(in.TaskId))
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.DeleteTaskGrant(ctx, t.TaskID, gt, in.GranteeId); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// requireTaskFull loads a task and confirms the caller has full access.
func (s *TaskService) requireTaskFull(ctx context.Context, taskID string) (*models.Task, grantee, error) {
	if taskID == "" {
		return nil, grantee{}, csilrpc.BadRequest("task_id is required")
	}
	t, err := s.Store.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, grantee{}, csilrpc.NotFound("task not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, t.HouseID)
	if err != nil {
		return nil, grantee{}, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, t.HouseID, memberID)
	if !canFull(pol.taskAccess(ctx, t, g)) {
		return nil, grantee{}, csilrpc.Forbidden("you need full access to manage this task")
	}
	return t, g, nil
}

// ---- small helpers ----------------------------------------------------

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefTaskStatus(p *csil.TaskStatus, fallback string) string {
	if p == nil {
		return fallback
	}
	if s := string(*p); s != "" {
		return s
	}
	return fallback
}

func tsToTimePtr(p *csil.Timestamp) *time.Time {
	if p == nil || *p == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, string(*p))
	if err != nil {
		return nil
	}
	return &t
}
