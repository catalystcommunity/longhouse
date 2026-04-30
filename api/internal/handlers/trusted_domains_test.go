package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
)

func TestTrustedDomains_AdminCRUD(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	// Initially empty.
	rec := h.do(http.MethodGet, "/api/v1/houses/h1/trusted-domains", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list initial: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got []csil.TrustedDomain
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}

	// Add one.
	rec = h.do(http.MethodPost, "/api/v1/houses/h1/trusted-domains", admin, map[string]string{
		"domain": "todandlorna.com",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d, body=%s", rec.Code, rec.Body.String())
	}
	var td csil.TrustedDomain
	decodeJSONOrFatal(t, rec, &td)
	if td.Domain != "todandlorna.com" || td.HouseId != "h1" || td.TrustedDomainId == "" {
		t.Errorf("created: %+v", td)
	}

	// List shows the new entry.
	rec = h.do(http.MethodGet, "/api/v1/houses/h1/trusted-domains", admin, nil)
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 1 {
		t.Errorf("list after create: got %d, want 1", len(got))
	}

	// Audit row was recorded.
	if len(h.store.audits) == 0 {
		t.Error("expected audit row for trusted-domain add")
	}

	// Delete.
	rec = h.do(http.MethodDelete,
		"/api/v1/houses/h1/trusted-domains/"+string(td.TrustedDomainId), admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d", rec.Code)
	}

	rec = h.do(http.MethodGet, "/api/v1/houses/h1/trusted-domains", admin, nil)
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 0 {
		t.Errorf("list after delete: got %d, want 0", len(got))
	}
}

func TestTrustedDomains_NonAdminBlocked(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/trusted-domains", memberTok,
		map[string]string{"domain": "x.example"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestTrustedDomains_DeleteFromOtherHouseBlocked(t *testing.T) {
	h := newHarness(t)
	admin1, admin2 := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h2/trusted-domains", admin2, map[string]string{
		"domain": "h2-only.example",
	})
	var td csil.TrustedDomain
	decodeJSONOrFatal(t, rec, &td)

	// admin1 tries to delete via h1's URL.
	rec = h.do(http.MethodDelete,
		"/api/v1/houses/h1/trusted-domains/"+string(td.TrustedDomainId), admin1, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404 (not visible from h1)", rec.Code)
	}
}

func TestTrustedDomains_DomainRequired(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/trusted-domains", admin, map[string]string{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}
