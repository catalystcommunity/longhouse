package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/webapp/internal/api"
	"github.com/catalystcommunity/longhouse/webapp/internal/session"
)

// TestUnregisteredAppPath_Returns404 pins the fix for the recursion
// regression: prior to /app/{$}, GET /app/anything-not-registered
// silently fell through to the dashboard handler, so the dashboard's
// HTMX widgets recursively rendered the layout into themselves.
func TestUnregisteredAppPath_Returns404(t *testing.T) {
	d := viewDeps(&fakeSessionAPI{})
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/no-such-thing", nil, cookies))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unregistered /app/* should 404, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func ptr(s string) *string { return &s }

func TestFragmentPriorityTasks_AssignedToMeFirstThenByDue(t *testing.T) {
	fake := &fakeSessionAPI{
		listTasks: func(h string) ([]api.Task, error) {
			return []api.Task{
				{TaskID: "t-other-soon", Title: "Other soon", Status: "open", AssignedToMemberID: ptr("someone-else"), DueAt: ptr("2026-05-13T09:00:00Z")},
				{TaskID: "t-mine-late", Title: "Mine late", Status: "open", AssignedToMemberID: ptr("me"), DueAt: ptr("2026-06-01T09:00:00Z")},
				{TaskID: "t-mine-early", Title: "Mine early", Status: "in_progress", AssignedToMemberID: ptr("me"), DueAt: ptr("2026-05-12T09:00:00Z")},
				{TaskID: "t-mine-nodue", Title: "Mine no due", Status: "open", AssignedToMemberID: ptr("me")},
				{TaskID: "t-done", Title: "Already done", Status: "done", AssignedToMemberID: ptr("me")},
			}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "me", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/fragments/priority-tasks", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got []api.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v; body=%s", err, rec.Body.String())
	}
	// Done task dropped.
	for _, x := range got {
		if x.Status == "done" || x.Status == "cancelled" {
			t.Errorf("done/cancelled leaked: %+v", x)
		}
	}
	// Order: mine-early, mine-late, mine-nodue, other-soon.
	want := []string{"t-mine-early", "t-mine-late", "t-mine-nodue", "t-other-soon"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d (%+v)", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].TaskID != id {
			t.Errorf("order[%d]: got %q, want %q (full=%+v)", i, got[i].TaskID, id, got)
		}
	}
}

func TestFragmentPriorityTasks_RespectsCap(t *testing.T) {
	var many []api.Task
	for i := 0; i < priorityTaskLimit+5; i++ {
		many = append(many, api.Task{TaskID: "t", Title: "x", Status: "open"})
	}
	fake := &fakeSessionAPI{listTasks: func(string) ([]api.Task, error) { return many, nil }}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/fragments/priority-tasks", nil, cookies))
	var got []api.Task
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got) != priorityTaskLimit {
		t.Errorf("cap: got %d, want %d", len(got), priorityTaskLimit)
	}
}

func TestFragmentCalendar_WindowAndSort(t *testing.T) {
	now := time.Now().UTC()
	in2h := now.Add(2 * time.Hour).Format(time.RFC3339)
	in1d := now.AddDate(0, 0, 1).Format(time.RFC3339)
	in10d := now.AddDate(0, 0, 10).Format(time.RFC3339)
	last := now.AddDate(0, 0, -10).Format(time.RFC3339)

	fake := &fakeSessionAPI{
		listEvents: func(string) ([]api.Event, error) {
			return []api.Event{
				{EventID: "later", Title: "Later", StartsAt: in1d},
				{EventID: "way-out", Title: "Way out", StartsAt: in10d},
				{EventID: "ancient", Title: "Ancient", StartsAt: last},
				{EventID: "soon", Title: "Soon", StartsAt: in2h},
				{EventID: "no-time", Title: "Untimed"},
			}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/fragments/calendar", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got []api.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 events in window, got %d (%+v)", len(got), got)
	}
	if got[0].EventID != "soon" || got[1].EventID != "later" {
		t.Errorf("not sorted ascending: %+v", got)
	}
}

// Fragment endpoints must return 401 for anonymous callers (not redirect),
// so the JS in the dashboard can react cleanly. requireAuth has a special
// case for /app/fragments/ that does this.
func TestFragments_AnonymousReturns401NotRedirect(t *testing.T) {
	d := viewDeps(&fakeSessionAPI{})
	for _, path := range []string{"/app/fragments/priority-tasks", "/app/fragments/calendar"} {
		rec := httptest.NewRecorder()
		NewRouter(d).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d (Location=%q)",
				path, rec.Code, rec.Header().Get("Location"))
		}
	}
}

// TestAdminDropdown_RendersWhenAdmin smoke-tests the layout change: admin
// users see one "Admin" toggle and the dropdown menu links underneath,
// not four sibling top-level links.
func TestAdminDropdown_RendersWhenAdmin(t *testing.T) {
	d := viewDeps(&fakeSessionAPI{})
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"admin"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/dashboard", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"nav-dropdown", "nav-dropdown-toggle", "Admin", "/app/admin/roles", "/app/admin/groups"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard body missing %q", want)
		}
	}
	if strings.Contains(body, "Admin · Roles") {
		t.Errorf("old flat admin links still present in nav")
	}
}
