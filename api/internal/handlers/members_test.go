package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
)

func TestMembers_List(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodGet, "/api/v1/houses/h1/members", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got []csil.Member
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 2 {
		t.Errorf("want 2 members in h1, got %d", len(got))
	}
}

func TestMembers_UpdateSelf_Allowed(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")
	displayName := "Member One"

	rec := h.do(http.MethodPatch, "/api/v1/houses/h1/members/m-member1", memberTok, map[string]any{
		"display_name": displayName,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got csil.Member
	decodeJSONOrFatal(t, rec, &got)
	if got.DisplayName == nil || *got.DisplayName != displayName {
		t.Errorf("display_name not updated: %+v", got.DisplayName)
	}
}

func TestMembers_UpdateOther_AsNonAdminBlocked(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPatch, "/api/v1/houses/h1/members/m-admin1", memberTok, map[string]any{
		"display_name": "hijack",
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestMembers_UpdateOther_AsAdminAllowed(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)
	displayName := "Renamed"

	rec := h.do(http.MethodPatch, "/api/v1/houses/h1/members/m-member1", admin, map[string]any{
		"display_name": displayName,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestMembers_AuditLog_AdminOnly(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	// Generate some audit history by granting a role.
	var adminRoleID string
	for _, r := range h.store.roles {
		if r.HouseID == "h1" && r.Name == "admin" {
			adminRoleID = r.RoleID
		}
	}
	rec := h.do(http.MethodPost,
		"/api/v1/houses/h1/members/m-member1/roles/"+adminRoleID, admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("grant: %d", rec.Code)
	}

	// Admin can read.
	rec = h.do(http.MethodGet,
		"/api/v1/houses/h1/members/m-member1/audits", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin list audits: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got []csil.MemberAudit
	decodeJSONOrFatal(t, rec, &got)
	if len(got) == 0 {
		t.Errorf("expected at least one audit row from the role grant")
	}
	// Detail JSON survives the round-trip.
	if got[0].Detail == nil || *got[0].Detail == "" {
		t.Errorf("expected detail json, got %v", got[0].Detail)
	}

	// Non-admin can't.
	memberTok := h.token("m-member1", "h1", "member")
	rec = h.do(http.MethodGet,
		"/api/v1/houses/h1/members/m-member1/audits", memberTok, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin status: %d, want 403", rec.Code)
	}
}

func TestMembers_GetCrossHouseBlocked(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	// Token for h1 trying to fetch a member that belongs to h2 via h1's URL.
	rec := h.do(http.MethodGet, "/api/v1/houses/h1/members/m-admin2", admin, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}
