package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
)

func TestTasks_AnyMemberCanCreate(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", memberTok, map[string]string{
		"title": "Patch the fence",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got csil.Task
	decodeJSONOrFatal(t, rec, &got)
	if got.OwnerMemberId != "m-member1" {
		t.Errorf("owner not stamped to caller: %+v", got)
	}
	if got.Status == nil || *got.Status != "open" {
		t.Errorf("default status not 'open': %+v", got.Status)
	}
}

func TestTasks_OwnerCanUpdate(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", memberTok, map[string]string{"title": "Patch"})
	var task csil.Task
	decodeJSONOrFatal(t, rec, &task)

	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/tasks/"+string(task.TaskId), memberTok, map[string]string{
		"status": "done",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestTasks_NonOwnerNonAdminCannotUpdate(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	otherTok := h.token("m-member1", "h1", "member")

	// Admin creates a task
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", admin, map[string]string{"title": "Admin's task"})
	var task csil.Task
	decodeJSONOrFatal(t, rec, &task)

	// Other member tries to update it
	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/tasks/"+string(task.TaskId), otherTok, map[string]string{
		"status": "done",
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestTasks_AdminCanUpdateAnyone(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	otherTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", otherTok, map[string]string{"title": "Member's"})
	var task csil.Task
	decodeJSONOrFatal(t, rec, &task)

	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/tasks/"+string(task.TaskId), admin, map[string]string{
		"status": "cancelled",
	})
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestTasks_AssignmentMutuallyExclusive(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", admin, map[string]any{
		"title":                 "Bad",
		"assigned_to_member_id": "m-member1",
		"assigned_to_skill_id":  "skill-1",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400 for double-assignment", rec.Code)
	}
}

func TestTasks_MissingTitle400(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", admin, map[string]string{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestTasks_CreateWithRecurrence(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", admin, map[string]any{
		"title":                  "Take out the trash",
		"recurrence_freq":        "weekly",
		"recurrence_interval":    1,
		"recurrence_by_weekday":  []int{1, 4},
		"next_recurrence_at":     "2026-05-04T08:00:00Z",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got csil.Task
	decodeJSONOrFatal(t, rec, &got)
	if got.RecurrenceFreq == nil {
		t.Fatalf("recurrence_freq missing")
	}
	if got.RecurrenceInterval == nil || *got.RecurrenceInterval != 1 {
		t.Errorf("recurrence_interval: %v", got.RecurrenceInterval)
	}
	if len(got.RecurrenceByWeekday) != 2 {
		t.Errorf("recurrence_by_weekday: %+v", got.RecurrenceByWeekday)
	}
	if got.NextRecurrenceAt == nil {
		t.Errorf("next_recurrence_at not propagated")
	}
}

func TestTasks_CreateBadRecurrenceFreq400(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", admin, map[string]any{
		"title":           "x",
		"recurrence_freq": "decadal",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestTasks_DeleteIsSoft(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", admin, map[string]string{"title": "doomed"})
	var task csil.Task
	decodeJSONOrFatal(t, rec, &task)

	// Delete returns 204…
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/tasks/"+string(task.TaskId), admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}

	// …and the row still exists (soft delete preserves history).
	if len(h.store.tasks) != 1 {
		t.Fatalf("want row preserved, got %d tasks in store", len(h.store.tasks))
	}
	if h.store.tasks[0].DeletedAt == nil {
		t.Errorf("want deleted_at set; got nil")
	}

	// List filters it out (the in-memory ListTasksByHouse mirrors the
	// production WHERE deleted_at IS NULL).
	rec = h.do(http.MethodGet, "/api/v1/houses/h1/tasks", admin, nil)
	var listed []csil.Task
	decodeJSONOrFatal(t, rec, &listed)
	if len(listed) != 0 {
		t.Errorf("soft-deleted task should not appear in list; got %d", len(listed))
	}

	// A second delete is idempotent.
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/tasks/"+string(task.TaskId), admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("second delete: %d, want 204", rec.Code)
	}
}

func TestTasks_CreateWithParent(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", admin, map[string]string{"title": "Parent"})
	var parent csil.Task
	decodeJSONOrFatal(t, rec, &parent)

	rec = h.do(http.MethodPost, "/api/v1/houses/h1/tasks", admin, map[string]any{
		"title":          "Child",
		"parent_task_id": string(parent.TaskId),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var child csil.Task
	decodeJSONOrFatal(t, rec, &child)
	if child.ParentTaskId == nil || string(*child.ParentTaskId) != string(parent.TaskId) {
		t.Errorf("parent_task_id not propagated: %+v", child.ParentTaskId)
	}
}

func TestTasks_DeleteOwnerOrAdmin(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	// Member creates task
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", memberTok, map[string]string{"title": "Mine"})
	var task csil.Task
	decodeJSONOrFatal(t, rec, &task)

	// Other member can't delete
	other := h.token("m-other", "h1", "member")
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/tasks/"+string(task.TaskId), other, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-owner delete: got %d, want 403", rec.Code)
	}

	// Owner can
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/tasks/"+string(task.TaskId), memberTok, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("owner delete: got %d, want 204", rec.Code)
	}

	// Admin can delete a recreated one
	rec = h.do(http.MethodPost, "/api/v1/houses/h1/tasks", memberTok, map[string]string{"title": "Mine 2"})
	decodeJSONOrFatal(t, rec, &task)
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/tasks/"+string(task.TaskId), admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("admin delete: got %d, want 204", rec.Code)
	}
}
