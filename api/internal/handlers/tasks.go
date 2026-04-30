package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

var errForbidden = errors.New("handlers: forbidden")

func listTasks(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	tasks, err := store.AppStore.ListTasksByHouse(r.Context(), houseFromPath(r), limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasksToCSIL(tasks))
}

func createTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title              string  `json:"title"`
		Description        string  `json:"description"`
		AssignedToMemberID  *string `json:"assigned_to_member_id"`
		AssignedToSkillID   *string `json:"assigned_to_skill_id"`
		ParentTaskID        *string `json:"parent_task_id"`
		RecurrenceFreq      *string `json:"recurrence_freq"`
		RecurrenceInterval  int     `json:"recurrence_interval"`
		RecurrenceByWeekday []int   `json:"recurrence_by_weekday"`
		NextRecurrenceAt    string  `json:"next_recurrence_at"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	if body.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if body.AssignedToMemberID != nil && body.AssignedToSkillID != nil {
		writeError(w, http.StatusBadRequest, "assign to a member or a skill, not both")
		return
	}
	if body.RecurrenceFreq != nil {
		switch *body.RecurrenceFreq {
		case "hourly", "daily", "weekly", "monthly", "quarterly", "yearly":
			// ok
		default:
			writeError(w, http.StatusBadRequest, "recurrence_freq must be one of hourly|daily|weekly|monthly|quarterly|yearly")
			return
		}
	}
	if body.RecurrenceInterval < 0 {
		writeError(w, http.StatusBadRequest, "recurrence_interval must be >= 1")
		return
	}
	nextRec, err := parseOptionalRFC3339(body.NextRecurrenceAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "next_recurrence_at: "+err.Error())
		return
	}
	interval := body.RecurrenceInterval
	if interval == 0 {
		interval = 1
	}
	t := &models.Task{
		HouseID:             houseFromPath(r),
		OwnerMemberID:       callerMemberID(r),
		Title:               body.Title,
		Description:         body.Description,
		Status:              "open",
		AssignedToMemberID:  body.AssignedToMemberID,
		AssignedToSkillID:   body.AssignedToSkillID,
		ParentTaskID:        body.ParentTaskID,
		RecurrenceFreq:      body.RecurrenceFreq,
		RecurrenceInterval:  interval,
		RecurrenceByWeekday: models.IntList(body.RecurrenceByWeekday),
		NextRecurrenceAt:    nextRec,
	}
	if err := store.AppStore.CreateTask(r.Context(), t); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, taskToCSIL(t))
}

func getTask(w http.ResponseWriter, r *http.Request) {
	t, err := taskInScope(w, r)
	if err != nil {
		return
	}
	writeJSON(w, http.StatusOK, taskToCSIL(t))
}

func updateTask(w http.ResponseWriter, r *http.Request) {
	t, err := taskInScope(w, r)
	if err != nil {
		return
	}
	if !requireOwnerOrAdmin(w, r, t.OwnerMemberID) {
		return
	}
	var body struct {
		Title              *string `json:"title"`
		Description        *string `json:"description"`
		Status             *string `json:"status"`
		AssignedToMemberID *string `json:"assigned_to_member_id"`
		AssignedToSkillID  *string `json:"assigned_to_skill_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Title != nil {
		t.Title = *body.Title
	}
	if body.Description != nil {
		t.Description = *body.Description
	}
	if body.Status != nil {
		t.Status = *body.Status
	}
	if body.AssignedToMemberID != nil {
		t.AssignedToMemberID = body.AssignedToMemberID
	}
	if body.AssignedToSkillID != nil {
		t.AssignedToSkillID = body.AssignedToSkillID
	}
	if t.AssignedToMemberID != nil && t.AssignedToSkillID != nil {
		writeError(w, http.StatusBadRequest, "assign to a member or a skill, not both")
		return
	}
	if err := store.AppStore.UpdateTask(r.Context(), t); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, taskToCSIL(t))
}

// deleteTask soft-deletes (sets deleted_at). Hard-deletes are intentionally
// not exposed: the migration plan calls for the row to remain so previously-
// spawned recurrence children stay linked, comments referencing the task
// don't dangle, and audit history isn't lost. A future admin-only "purge"
// endpoint can hard-delete if needed.
func deleteTask(w http.ResponseWriter, r *http.Request) {
	t, err := taskInScope(w, r)
	if err != nil {
		return
	}
	if !requireOwnerOrAdmin(w, r, t.OwnerMemberID) {
		return
	}
	if t.DeletedAt != nil {
		w.WriteHeader(http.StatusNoContent) // idempotent
		return
	}
	now := time.Now().UTC()
	t.DeletedAt = &now
	if err := store.AppStore.UpdateTask(r.Context(), t); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func taskInScope(w http.ResponseWriter, r *http.Request) (*models.Task, error) {
	id := r.PathValue("task_id")
	t, err := store.AppStore.GetTaskByID(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return nil, err
	}
	if t.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "task belongs to a different house")
		return nil, errForbidden
	}
	return t, nil
}
