package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func listEvents(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	events, err := store.AppStore.ListEventsByHouse(r.Context(), houseFromPath(r), limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, eventsToCSIL(events))
}

func createEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Location    string `json:"location"`
		StartsAt    string `json:"starts_at"`
		EndsAt      string `json:"ends_at"`
		AllDay      bool   `json:"all_day"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	if body.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	starts, err := parseOptionalRFC3339(body.StartsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "starts_at: "+err.Error())
		return
	}
	ends, err := parseOptionalRFC3339(body.EndsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ends_at: "+err.Error())
		return
	}
	e := &models.Event{
		HouseID:       houseFromPath(r),
		OwnerMemberID: callerMemberID(r),
		Title:         body.Title,
		Description:   body.Description,
		Location:      body.Location,
		StartsAt:      starts,
		EndsAt:        ends,
		AllDay:        body.AllDay,
	}
	if err := store.AppStore.CreateEvent(r.Context(), e); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, eventToCSIL(e))
}

func getEvent(w http.ResponseWriter, r *http.Request) {
	e, err := eventInScope(w, r)
	if err != nil {
		return
	}
	writeJSON(w, http.StatusOK, eventToCSIL(e))
}

func updateEvent(w http.ResponseWriter, r *http.Request) {
	e, err := eventInScope(w, r)
	if err != nil {
		return
	}
	if !requireOwnerOrAdmin(w, r, e.OwnerMemberID) {
		return
	}
	var body struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Location    *string `json:"location"`
		StartsAt    *string `json:"starts_at"`
		EndsAt      *string `json:"ends_at"`
		AllDay      *bool   `json:"all_day"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Title != nil {
		e.Title = *body.Title
	}
	if body.Description != nil {
		e.Description = *body.Description
	}
	if body.Location != nil {
		e.Location = *body.Location
	}
	if body.StartsAt != nil {
		s, err := parseOptionalRFC3339(*body.StartsAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "starts_at: "+err.Error())
			return
		}
		e.StartsAt = s
	}
	if body.EndsAt != nil {
		s, err := parseOptionalRFC3339(*body.EndsAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "ends_at: "+err.Error())
			return
		}
		e.EndsAt = s
	}
	if body.AllDay != nil {
		e.AllDay = *body.AllDay
	}
	if err := store.AppStore.UpdateEvent(r.Context(), e); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, eventToCSIL(e))
}

func deleteEvent(w http.ResponseWriter, r *http.Request) {
	e, err := eventInScope(w, r)
	if err != nil {
		return
	}
	if !requireOwnerOrAdmin(w, r, e.OwnerMemberID) {
		return
	}
	if err := store.AppStore.DeleteEvent(r.Context(), e.EventID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func eventInScope(w http.ResponseWriter, r *http.Request) (*models.Event, error) {
	id := r.PathValue("event_id")
	e, err := store.AppStore.GetEventByID(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return nil, err
	}
	if e.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "event belongs to a different house")
		return nil, errForbidden
	}
	return e, nil
}

// parseOptionalRFC3339 returns nil + nil for empty input; otherwise parses
// strict RFC3339 (the same format CSIL Timestamps emit).
func parseOptionalRFC3339(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	utc := t.UTC()
	return &utc, nil
}
