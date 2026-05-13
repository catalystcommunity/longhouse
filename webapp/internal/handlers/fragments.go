package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/catalystcommunity/longhouse/webapp/internal/api"
	log "github.com/sirupsen/logrus"
)

// Fragment endpoints back the dashboard's vanilla-JS widgets. They return
// JSON, not HTML — requireAuth's `/app/fragments/` branch already returns
// 401 (rather than redirecting to /login) so the JS can react cleanly.

const (
	priorityTaskLimit = 8
	calendarWindowDays = 5 // server window; client may render fewer
)

// fragmentPriorityTasks returns open/in-progress tasks for the dashboard,
// ordered: assigned-to-me first, then by due_at ascending (no-due-date
// last), then by title for stability. Capped at priorityTaskLimit.
func (d *Deps) fragmentPriorityTasks(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	tasks, err := d.sessionAPI(id).ListTasks(id.HouseID)
	if err != nil {
		writeFragmentError(w, "loading tasks", err)
		return
	}
	var open []api.Task
	for _, t := range tasks {
		if t.Status == "done" || t.Status == "cancelled" {
			continue
		}
		open = append(open, t)
	}
	mine := id.MemberID
	sort.SliceStable(open, func(i, j int) bool {
		ai := open[i].AssignedToMemberID != nil && *open[i].AssignedToMemberID == mine
		aj := open[j].AssignedToMemberID != nil && *open[j].AssignedToMemberID == mine
		if ai != aj {
			return ai
		}
		di := dueAtKey(open[i].DueAt)
		dj := dueAtKey(open[j].DueAt)
		if di != dj {
			return di < dj
		}
		return open[i].Title < open[j].Title
	})
	if len(open) > priorityTaskLimit {
		open = open[:priorityTaskLimit]
	}
	writeJSON(w, open)
}

// dueAtKey returns a sortable string for due_at: empty/unset sorts after
// any real date by using a sentinel that compares greater than any RFC3339
// timestamp.
func dueAtKey(due *string) string {
	if due == nil || *due == "" {
		return "~" // '~' (0x7E) sorts after digits/'Z' in ASCII
	}
	return *due
}

// fragmentCalendar returns events with a parseable RFC3339 StartsAt within
// a window starting now and extending calendarWindowDays days. The client
// buckets and lays them out; we return the raw rows so server timezone
// assumptions stay out of the picture.
func (d *Deps) fragmentCalendar(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	events, err := d.sessionAPI(id).ListEvents(id.HouseID)
	if err != nil {
		writeFragmentError(w, "loading events", err)
		return
	}
	now := time.Now().UTC()
	end := now.AddDate(0, 0, calendarWindowDays)
	var window []api.Event
	for _, e := range events {
		if e.StartsAt == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, e.StartsAt)
		if err != nil {
			continue
		}
		// Keep events whose start falls in the window, plus all-day events
		// where the date alone is in range. Cheap filter: any event whose
		// start is within [now-1d, end] survives — gives the client a
		// little slack for timezone bucketing.
		if t.Before(now.AddDate(0, 0, -1)) || t.After(end) {
			continue
		}
		window = append(window, e)
	}
	sort.Slice(window, func(i, j int) bool { return window[i].StartsAt < window[j].StartsAt })
	writeJSON(w, window)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if v == nil {
		_, _ = w.Write([]byte("[]"))
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.WithError(err).Error("encode fragment json")
	}
}

func writeFragmentError(w http.ResponseWriter, what string, err error) {
	log.WithError(err).Errorf("fragment: %s failed", what)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": what})
}
