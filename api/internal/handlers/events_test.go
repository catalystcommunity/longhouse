package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
)

func TestEvents_AnyMemberCanCreate(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/events", memberTok, map[string]any{
		"title":     "Standup",
		"location":  "Kitchen",
		"starts_at": "2026-05-01T09:00:00Z",
		"ends_at":   "2026-05-01T09:15:00Z",
		"all_day":   false,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got csil.Event
	decodeJSONOrFatal(t, rec, &got)
	if got.OwnerMemberId != "m-member1" {
		t.Errorf("owner not stamped to caller: %+v", got)
	}
	if got.StartsAt == nil || string(*got.StartsAt) == "" {
		t.Errorf("starts_at should round-trip: %+v", got.StartsAt)
	}
}

func TestEvents_BadStartsAtRejected(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/events", admin, map[string]string{
		"title":     "Bad",
		"starts_at": "not a date",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestEvents_NonOwnerCannotUpdate(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	otherTok := h.token("m-member1", "h1", "member")

	// Admin creates the event.
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/events", admin, map[string]string{"title": "Adm"})
	var e csil.Event
	decodeJSONOrFatal(t, rec, &e)

	// Member tries to update.
	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/events/"+string(e.EventId), otherTok, map[string]string{
		"title": "Hijack",
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestEvents_AdminCanUpdateAnyone(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/events", memberTok, map[string]string{"title": "M"})
	var e csil.Event
	decodeJSONOrFatal(t, rec, &e)

	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/events/"+string(e.EventId), admin, map[string]string{
		"title": "By admin",
	})
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestEvents_ListAndGet(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	for _, title := range []string{"a", "b"} {
		_ = h.do(http.MethodPost, "/api/v1/houses/h1/events", admin, map[string]string{"title": title})
	}
	rec := h.do(http.MethodGet, "/api/v1/houses/h1/events", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var got []csil.Event
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 2 {
		t.Errorf("want 2 events, got %d", len(got))
	}

	rec = h.do(http.MethodGet, "/api/v1/houses/h1/events/"+string(got[0].EventId), admin, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("get: %d", rec.Code)
	}
}

func TestEvents_CrossHouseGetForbidden(t *testing.T) {
	h := newHarness(t)
	admin1, admin2 := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h2/events", admin2, map[string]string{"title": "h2 only"})
	var e csil.Event
	decodeJSONOrFatal(t, rec, &e)

	rec = h.do(http.MethodGet, "/api/v1/houses/h1/events/"+string(e.EventId), admin1, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestEvents_DeleteOwnerOrAdmin(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/events", memberTok, map[string]string{"title": "x"})
	var e csil.Event
	decodeJSONOrFatal(t, rec, &e)

	other := h.token("m-other", "h1", "member")
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/events/"+string(e.EventId), other, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-owner delete: got %d, want 403", rec.Code)
	}

	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/events/"+string(e.EventId), admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("admin delete: got %d, want 204", rec.Code)
	}
}
