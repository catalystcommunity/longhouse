package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// taskInH1 creates a task owned by m-admin1 in h1 and returns its ID. Most
// comment tests need an existing target; this is the cheap way to make one.
func taskInH1(t *testing.T, h *testHarness, adminTok string) string {
	t.Helper()
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/tasks", adminTok, map[string]string{"title": "T"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task: %d, body=%s", rec.Code, rec.Body.String())
	}
	var task csil.Task
	decodeJSONOrFatal(t, rec, &task)
	return string(task.TaskId)
}

func TestComments_AnyMemberCanPost(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	taskID := taskInH1(t, h, admin)

	memberTok := h.token("m-member1", "h1", "member")
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/comments/task/"+taskID, memberTok, map[string]string{
		"body": "first thoughts",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got csil.Comment
	decodeJSONOrFatal(t, rec, &got)
	if got.MemberId != "m-member1" || got.Body != "first thoughts" {
		t.Errorf("comment: %+v", got)
	}

	// Show up in the list.
	rec = h.do(http.MethodGet, "/api/v1/houses/h1/comments/task/"+taskID, memberTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var listed []csil.Comment
	decodeJSONOrFatal(t, rec, &listed)
	if len(listed) != 1 {
		t.Errorf("want 1 comment, got %d", len(listed))
	}
}

func TestComments_BodyRequired(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	taskID := taskInH1(t, h, admin)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/comments/task/"+taskID, admin, map[string]string{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
	rec = h.do(http.MethodPost, "/api/v1/houses/h1/comments/task/"+taskID, admin, map[string]string{
		"body": "   ",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("whitespace-only body: got %d, want 400", rec.Code)
	}
}

func TestComments_BadTargetType400(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/comments/banana/whatever", admin,
		map[string]string{"body": "x"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestComments_TargetCrossHouseBlocked(t *testing.T) {
	h := newHarness(t)
	admin1, admin2 := setupTwoHousesWithAdmins(h)
	taskInH2 := taskInH1 // identical pattern; just call against h2 admin
	taskID := func() string {
		rec := h.do(http.MethodPost, "/api/v1/houses/h2/tasks", admin2, map[string]string{"title": "T2"})
		var task csil.Task
		decodeJSONOrFatal(t, rec, &task)
		return string(task.TaskId)
	}()
	_ = taskInH2

	// admin1 with h1 token tries to comment on a task that lives in h2.
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/comments/task/"+taskID, admin1,
		map[string]string{"body": "stalking from h1"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestComments_DeleteAuthorOrAdmin(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	taskID := taskInH1(t, h, admin)
	memberTok := h.token("m-member1", "h1", "member")

	// Member writes the comment.
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/comments/task/"+taskID, memberTok,
		map[string]string{"body": "hi"})
	var posted csil.Comment
	decodeJSONOrFatal(t, rec, &posted)

	// Another member can't delete.
	other := h.token("m-other", "h1", "member")
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/comments/"+string(posted.CommentId), other, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-author delete: got %d, want 403", rec.Code)
	}

	// Author can.
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/comments/"+string(posted.CommentId), memberTok, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("author delete: got %d, want 204", rec.Code)
	}

	// Re-post + admin deletes.
	rec = h.do(http.MethodPost, "/api/v1/houses/h1/comments/task/"+taskID, memberTok,
		map[string]string{"body": "again"})
	decodeJSONOrFatal(t, rec, &posted)
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/comments/"+string(posted.CommentId), admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("admin delete: got %d, want 204", rec.Code)
	}
}

func TestComments_OnEventTarget(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	h.store.seedEvent(models.Event{EventID: "e-1", HouseID: "h1", OwnerMemberID: "m-admin1", Title: "Standup"})

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/comments/event/e-1", admin,
		map[string]string{"body": "ok"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
}
