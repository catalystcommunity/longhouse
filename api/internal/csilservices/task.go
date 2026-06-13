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
	d.Register("task", "ListTasks", s.listTasks)
	d.Register("task", "GetTask", s.getTask)
	d.Register("task", "CreateTask", s.createTask)
	d.Register("task", "UpdateTask", s.updateTask)
	d.Register("task", "DeleteTask", s.deleteTask)
	d.Register("task", "SetTaskVisibility", s.setTaskVisibility)
	d.Register("task", "ListTaskGrants", s.listTaskGrants)
	d.Register("task", "PutTaskGrant", s.putTaskGrant)
	d.Register("task", "DeleteTaskGrant", s.deleteTaskGrant)
}

func (s *TaskService) listTasks(ctx context.Context, body []byte) (any, error) {
	var req csil.HouseScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	id, memberID, err := requireMemberForHouse(ctx, string(req.HouseId))
	if err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	tasks, err := s.Store.ListTasksByHouse(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
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

func (s *TaskService) getTask(ctx context.Context, body []byte) (any, error) {
	var id csil.TaskID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	t, err := s.Store.GetTaskByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, csilrpc.NotFound("task not found")
		}
		return nil, csilrpc.Internal("internal error")
	}
	ident, memberID, err := requireMemberForHouse(ctx, t.HouseID)
	if err != nil {
		return nil, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, t.HouseID, memberID)
	if !canRead(pol.taskAccess(ctx, t, g)) {
		// Don't leak existence of a task the caller can't see.
		return nil, csilrpc.NotFound("task not found")
	}
	assignees, _ := s.Store.ListTaskAssignees(ctx, t.TaskID)
	return taskToCSIL(t, assignees), nil
}

func (s *TaskService) createTask(ctx context.Context, body []byte) (any, error) {
	var in csil.Task
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.HouseId == "" {
		return nil, csilrpc.BadRequest("house_id is required")
	}
	if in.Title == "" {
		return nil, csilrpc.BadRequest("title is required")
	}
	_, callerMemberID, err := requireMemberForHouse(ctx, string(in.HouseId))
	if err != nil {
		return nil, err
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
		if f, ok := (*in.RecurrenceFreq).(string); ok && f != "" {
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
		return nil, csilrpc.Internal("internal error")
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
				return nil, csilrpc.Internal("could not attach assignee")
			}
		}
	}
	assignees, _ := s.Store.ListTaskAssignees(ctx, t.TaskID)
	return taskToCSIL(t, assignees), nil
}

func (s *TaskService) updateTask(ctx context.Context, body []byte) (any, error) {
	var in csil.Task
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.TaskId == "" {
		return nil, csilrpc.BadRequest("task_id is required")
	}
	existing, err := s.Store.GetTaskByID(ctx, string(in.TaskId))
	if err != nil {
		return nil, csilrpc.NotFound("task not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	// Content edits require `edit`. Visibility and grants are governance and
	// are changed only via set-task-visibility / *-task-grant (full). An
	// inbound Visibility here is ignored. See docs/rbac.md §7.
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, existing.HouseID, memberID)
	if !canEdit(pol.taskAccess(ctx, existing, g)) {
		return nil, csilrpc.Forbidden("you need edit access to change this task")
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
		if f, ok := (*in.RecurrenceFreq).(string); ok {
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
		return nil, csilrpc.Internal("internal error")
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
func (s *TaskService) deleteTask(ctx context.Context, body []byte) (any, error) {
	var id csil.TaskID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetTaskByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("task not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	// Delete is governance — requires full (owner/admin resolve to full).
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, existing.HouseID, memberID)
	if !canFull(pol.taskAccess(ctx, existing, g)) {
		return nil, csilrpc.Forbidden("you need full access to delete this task")
	}
	if existing.DeletedAt != nil {
		return csil.EmptyResponse{}, nil // idempotent
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteTask(ctx, existing.TaskID, memberID, opID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, existing.HouseID, "task", existing.TaskID, opID, existing)
	return csil.EmptyResponse{}, nil
}

// setTaskVisibility changes the task's house-at-large visibility. Requires
// full, and the requested level is bounded by the umbrella guardrail (a task
// can't be more visible than the MIN of its containers). See docs/rbac.md.
func (s *TaskService) setTaskVisibility(ctx context.Context, body []byte) (any, error) {
	var in csil.SetTaskVisibilityRequest
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.TaskId == "" {
		return nil, csilrpc.BadRequest("task_id is required")
	}
	want := accessLevelVal(in.Visibility, "")
	if !validAccessLevel(want) {
		return nil, csilrpc.BadRequest("invalid visibility")
	}
	existing, err := s.Store.GetTaskByID(ctx, string(in.TaskId))
	if err != nil {
		return nil, csilrpc.NotFound("task not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, existing.HouseID, memberID)
	if !canFull(pol.taskAccess(ctx, existing, g)) {
		return nil, csilrpc.Forbidden("you need full access to change visibility")
	}
	// A free-floating task (no parent, no projects) is pinned to read: it can
	// be neither raised above read nor made private. Privacy requires a
	// containing project. See docs/rbac.md.
	if pol.isFreeFloating(ctx, existing) {
		if want != models.AccessRead {
			return nil, csilrpc.BadRequest("a task with no project or parent must stay read; add it to a project to change visibility")
		}
	} else {
		ceil := pol.maxAllowedTaskVisibility(ctx, existing)
		if models.AccessRank(want) > models.AccessRank(ceil) {
			return nil, csilrpc.BadRequest("visibility cannot exceed the task's least-visible project or parent (" + ceil + ")")
		}
	}
	existing.Visibility = want
	if err := s.Store.UpdateTask(ctx, existing); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	assignees, _ := s.Store.ListTaskAssignees(ctx, existing.TaskID)
	return taskToCSIL(existing, assignees), nil
}

// listTaskGrants returns the task's explicit grants. Viewing the access list
// is governance — requires full.
func (s *TaskService) listTaskGrants(ctx context.Context, body []byte) (any, error) {
	var id csil.TaskID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
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
func (s *TaskService) putTaskGrant(ctx context.Context, body []byte) (any, error) {
	var in csil.PutTaskGrantRequest
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	gt := granteeTypeVal(in.GranteeType)
	if gt != models.GranteeMember && gt != models.GranteeGroup {
		return nil, csilrpc.BadRequest("invalid grantee_type")
	}
	if in.GranteeId == "" {
		return nil, csilrpc.BadRequest("grantee_id is required")
	}
	level := accessLevelVal(in.AccessLevel, "")
	if !validAccessLevel(level) {
		return nil, csilrpc.BadRequest("invalid access_level")
	}
	t, _, err := s.requireTaskFull(ctx, string(in.TaskId))
	if err != nil {
		return nil, err
	}
	grant := &models.TaskGrant{
		TaskID: t.TaskID, HouseID: t.HouseID,
		GranteeType: gt, GranteeID: in.GranteeId, AccessLevel: level,
	}
	if err := s.Store.PutTaskGrant(ctx, grant); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// deleteTaskGrant removes a grant from the task.
func (s *TaskService) deleteTaskGrant(ctx context.Context, body []byte) (any, error) {
	var in csil.TaskGrantRef
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	gt := granteeTypeVal(in.GranteeType)
	if gt == "" || in.GranteeId == "" {
		return nil, csilrpc.BadRequest("grantee_type and grantee_id are required")
	}
	t, _, err := s.requireTaskFull(ctx, string(in.TaskId))
	if err != nil {
		return nil, err
	}
	if err := s.Store.DeleteTaskGrant(ctx, t.TaskID, gt, in.GranteeId); err != nil {
		return nil, csilrpc.Internal("internal error")
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
	switch v := (*p).(type) {
	case string:
		return v
	default:
		return fallback
	}
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
