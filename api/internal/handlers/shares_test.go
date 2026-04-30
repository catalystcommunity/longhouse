package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func TestShares_Create_AdminOnly(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	body := map[string]any{
		"linkkeys_domain":  "guest.example",
		"linkkeys_user_id": "ext-1",
		"resource_type":    "task",
		"resource_id":      "t-shared",
	}
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/shares", admin, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got csil.Share
	decodeJSONOrFatal(t, rec, &got)
	if got.HouseId != "h1" || got.ResourceId != "t-shared" {
		t.Errorf("share returned: %+v", got)
	}
	if got.AccessLevel == nil || *got.AccessLevel != "read" {
		t.Errorf("expected access_level read, got %v", got.AccessLevel)
	}
	// Audit row was written.
	if len(h.store.audits) == 0 || h.store.audits[len(h.store.audits)-1].Action != models.AuditActionShareCreated {
		t.Errorf("expected share_created audit; got %+v", h.store.audits)
	}
}

func TestShares_Create_NonAdminBlocked(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	body := map[string]any{
		"linkkeys_domain":  "guest.example",
		"linkkeys_user_id": "ext-1",
		"resource_type":    "task",
		"resource_id":      "t-shared",
	}
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/shares", memberTok, body)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestShares_Create_RejectsBadResourceType(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/shares", admin, map[string]any{
		"linkkeys_domain":  "g.example",
		"linkkeys_user_id": "u",
		"resource_type":    "calendar", // not in (event, task, house)
		"resource_id":      "x",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestShares_Create_RejectsMissingFields(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/shares", admin, map[string]any{
		"linkkeys_domain": "g.example",
		"resource_type":   "task",
		"resource_id":     "x",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestShares_List_Admin(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	h.store.seedShare(models.Share{HouseID: "h1", SharedBy: "m-admin1", LinkkeysDomain: "g.example", LinkkeysUserID: "u1", ResourceType: "task", ResourceID: "t1", AccessLevel: "read"})
	h.store.seedShare(models.Share{HouseID: "h1", SharedBy: "m-admin1", LinkkeysDomain: "g.example", LinkkeysUserID: "u2", ResourceType: "event", ResourceID: "e1", AccessLevel: "read"})
	h.store.seedShare(models.Share{HouseID: "h2", SharedBy: "m-admin2", LinkkeysDomain: "g.example", LinkkeysUserID: "u3", ResourceType: "task", ResourceID: "tx", AccessLevel: "read"})

	rec := h.do(http.MethodGet, "/api/v1/houses/h1/shares", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got []csil.Share
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 2 {
		t.Errorf("want 2 shares for h1, got %d (%+v)", len(got), got)
	}
}

func TestShares_List_FilterByResource(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	h.store.seedShare(models.Share{HouseID: "h1", SharedBy: "m-admin1", LinkkeysDomain: "g.example", LinkkeysUserID: "u1", ResourceType: "task", ResourceID: "t1", AccessLevel: "read"})
	h.store.seedShare(models.Share{HouseID: "h1", SharedBy: "m-admin1", LinkkeysDomain: "g.example", LinkkeysUserID: "u2", ResourceType: "task", ResourceID: "t1", AccessLevel: "read"})
	h.store.seedShare(models.Share{HouseID: "h1", SharedBy: "m-admin1", LinkkeysDomain: "g.example", LinkkeysUserID: "u3", ResourceType: "task", ResourceID: "t2", AccessLevel: "read"})
	// Different house, same resource id — must NOT leak.
	h.store.seedShare(models.Share{HouseID: "h2", SharedBy: "m-admin2", LinkkeysDomain: "g.example", LinkkeysUserID: "u4", ResourceType: "task", ResourceID: "t1", AccessLevel: "read"})

	rec := h.do(http.MethodGet, "/api/v1/houses/h1/shares?resource_type=task&resource_id=t1", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got []csil.Share
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 2 {
		t.Errorf("filter expected 2 (only h1's task t1 entries), got %d (%+v)", len(got), got)
	}
}

func TestShares_List_FilterRequiresBothParams(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodGet, "/api/v1/houses/h1/shares?resource_type=task", admin, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestShares_Delete_Admin(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	share := models.Share{ShareID: "s1", HouseID: "h1", SharedBy: "m-admin1", LinkkeysDomain: "g.example", LinkkeysUserID: "u1", ResourceType: "task", ResourceID: "t1", AccessLevel: "read"}
	h.store.seedShare(share)

	rec := h.do(http.MethodDelete, "/api/v1/houses/h1/shares/s1", admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(h.store.shares) != 0 {
		t.Errorf("share not removed: %+v", h.store.shares)
	}
	if h.store.audits[len(h.store.audits)-1].Action != models.AuditActionShareRevoked {
		t.Errorf("expected share_revoked audit; got %+v", h.store.audits)
	}
}

func TestShares_Delete_CrossHouseHidden(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	// Share belongs to h2 — admin of h1 should not be able to delete or learn it exists.
	h.store.seedShare(models.Share{ShareID: "s2", HouseID: "h2", SharedBy: "m-admin2", LinkkeysDomain: "g.example", LinkkeysUserID: "u9", ResourceType: "task", ResourceID: "tx", AccessLevel: "read"})

	rec := h.do(http.MethodDelete, "/api/v1/houses/h1/shares/s2", admin, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
	if len(h.store.shares) != 1 {
		t.Errorf("h2's share got deleted via h1 URL: %+v", h.store.shares)
	}
}

func TestShares_Delete_NotFound(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodDelete, "/api/v1/houses/h1/shares/missing", admin, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestSharedAccess_NotImplemented(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/v1/shared/access", "", map[string]any{
		"signed_assertion": "anything",
		"resource_type":    "task",
		"resource_id":      "t1",
	})
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status: got %d, want 501; body=%s", rec.Code, rec.Body.String())
	}
}

func TestShares_Delete_NonAdminBlocked(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")
	h.store.seedShare(models.Share{ShareID: "sx", HouseID: "h1", SharedBy: "m-admin1", LinkkeysDomain: "g.example", LinkkeysUserID: "u1", ResourceType: "task", ResourceID: "t1", AccessLevel: "read"})

	rec := h.do(http.MethodDelete, "/api/v1/houses/h1/shares/sx", memberTok, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}
