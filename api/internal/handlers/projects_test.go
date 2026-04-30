package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
)

func TestProjects_AnyMemberCanCreate(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/projects", memberTok, map[string]string{
		"name": "Spring cleanup",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var p csil.Project
	decodeJSONOrFatal(t, rec, &p)
	if p.Name != "Spring cleanup" || p.HouseId != "h1" {
		t.Errorf("project: %+v", p)
	}
}

func TestProjects_AdminCanUpdate(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/projects", admin, map[string]string{"name": "X"})
	var p csil.Project
	decodeJSONOrFatal(t, rec, &p)

	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/projects/"+string(p.ProjectId), admin, map[string]string{
		"name":   "Renamed",
		"status": "archived",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got csil.Project
	decodeJSONOrFatal(t, rec, &got)
	if got.Name != "Renamed" {
		t.Errorf("name: %+v", got)
	}
}

func TestProjects_NonAdminCannotUpdate(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/projects", admin, map[string]string{"name": "X"})
	var p csil.Project
	decodeJSONOrFatal(t, rec, &p)

	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/projects/"+string(p.ProjectId), memberTok, map[string]string{
		"name": "Hijack",
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestProjectTasks_AddListRemove(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	// Create project
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/projects", admin, map[string]string{"name": "P"})
	var p csil.Project
	decodeJSONOrFatal(t, rec, &p)

	// Create a task
	rec = h.do(http.MethodPost, "/api/v1/houses/h1/tasks", admin, map[string]string{"title": "Cut wood"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("task create: %d, body=%s", rec.Code, rec.Body.String())
	}
	var task csil.Task
	decodeJSONOrFatal(t, rec, &task)

	// Link task to project
	rec = h.do(http.MethodPost, "/api/v1/houses/h1/projects/"+string(p.ProjectId)+"/tasks", admin,
		map[string]any{"task_id": string(task.TaskId), "position": 1})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("link: %d, body=%s", rec.Code, rec.Body.String())
	}

	// List project tasks
	rec = h.do(http.MethodGet, "/api/v1/houses/h1/projects/"+string(p.ProjectId)+"/tasks", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var got []csil.Task
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 1 || got[0].Title != "Cut wood" {
		t.Errorf("project tasks: %+v", got)
	}

	// Remove
	rec = h.do(http.MethodDelete,
		"/api/v1/houses/h1/projects/"+string(p.ProjectId)+"/tasks/"+string(task.TaskId), admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("unlink: %d", rec.Code)
	}
}
