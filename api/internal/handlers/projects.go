package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func listProjects(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	projects, err := store.AppStore.ListProjectsByHouse(r.Context(), houseFromPath(r), limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projectsToCSIL(projects))
}

func createProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	proj := &models.Project{HouseID: houseFromPath(r), Name: body.Name, Description: body.Description, Status: "active"}
	if err := store.AppStore.CreateProject(r.Context(), proj); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, projectToCSIL(proj))
}

func getProject(w http.ResponseWriter, r *http.Request) {
	p, err := projectInScope(w, r)
	if err != nil {
		return
	}
	writeJSON(w, http.StatusOK, projectToCSIL(p))
}

func updateProject(w http.ResponseWriter, r *http.Request) {
	p, err := projectInScope(w, r)
	if err != nil {
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.Description != nil {
		p.Description = *body.Description
	}
	if body.Status != nil {
		p.Status = *body.Status
	}
	if err := store.AppStore.UpdateProject(r.Context(), p); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projectToCSIL(p))
}

func deleteProject(w http.ResponseWriter, r *http.Request) {
	p, err := projectInScope(w, r)
	if err != nil {
		return
	}
	if err := store.AppStore.DeleteProject(r.Context(), p.ProjectID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listProjectTasks(w http.ResponseWriter, r *http.Request) {
	p, err := projectInScope(w, r)
	if err != nil {
		return
	}
	limit, offset := limitOffset(r)
	tasks, err := store.AppStore.ListProjectTasks(r.Context(), p.ProjectID, limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasksToCSIL(tasks))
}

func addProjectTask(w http.ResponseWriter, r *http.Request) {
	p, err := projectInScope(w, r)
	if err != nil {
		return
	}
	var body struct {
		TaskID   string `json:"task_id"`
		Position int    `json:"position"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	if body.TaskID == "" {
		writeError(w, http.StatusBadRequest, "task_id is required")
		return
	}
	if err := store.AppStore.AddProjectTask(r.Context(), p.ProjectID, body.TaskID, body.Position); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func removeProjectTask(w http.ResponseWriter, r *http.Request) {
	p, err := projectInScope(w, r)
	if err != nil {
		return
	}
	taskID := r.PathValue("task_id")
	if err := store.AppStore.RemoveProjectTask(r.Context(), p.ProjectID, taskID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// projectInScope fetches the project, returns 404/forbidden as appropriate,
// and is the gate for all per-project handlers.
func projectInScope(w http.ResponseWriter, r *http.Request) (*models.Project, error) {
	id := r.PathValue("project_id")
	p, err := store.AppStore.GetProjectByID(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return nil, err
	}
	if p.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "project belongs to a different house")
		return nil, errForbidden
	}
	return p, nil
}
