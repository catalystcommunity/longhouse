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
}

func (s *TaskService) listTasks(ctx context.Context, body []byte) (any, error) {
	var req csil.HouseScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	tasks, err := s.Store.ListTasksByHouse(ctx, string(req.HouseId), limit, offset)
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
	if _, _, err := requireMemberForHouse(ctx, t.HouseID); err != nil {
		return nil, err
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
	id, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	if callerMemberID != existing.OwnerMemberID {
		if _, err := requireRole(id, existing.HouseID, "admin"); err != nil {
			return nil, csilrpc.Forbidden("only the task owner or a house admin may edit this task")
		}
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
	ident, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	if callerMemberID != existing.OwnerMemberID {
		if _, err := requireRole(ident, existing.HouseID, "admin"); err != nil {
			return nil, csilrpc.Forbidden("only the task owner or a house admin may delete this task")
		}
	}
	if existing.DeletedAt != nil {
		return csil.EmptyResponse{}, nil // idempotent
	}
	now := time.Now().UTC()
	existing.DeletedAt = &now
	if err := s.Store.UpdateTask(ctx, existing); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
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
