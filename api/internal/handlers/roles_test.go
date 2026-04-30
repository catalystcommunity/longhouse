package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
)

func setupTwoHousesWithAdmins(h *testHarness) (admin1, admin2 string) {
	h.store.seedHouse("h1", "First")
	h.store.seedHouse("h2", "Second")
	h.store.seedMember("h1", "m-admin1", "todandlorna.com", "u1", "admin", "member")
	h.store.seedMember("h1", "m-member1", "todandlorna.com", "u2", "member")
	h.store.seedMember("h2", "m-admin2", "todandlorna.com", "u3", "admin", "member")
	return h.token("m-admin1", "h1", "admin", "member"),
		h.token("m-admin2", "h2", "admin", "member")
}

func TestRoles_Create_Admin(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/roles", admin, map[string]string{
		"name":        "moderator",
		"description": "trusted contributor",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var got csil.Role
	decodeJSONOrFatal(t, rec, &got)
	if got.Name != "moderator" || got.HouseId != "h1" || got.RoleId == "" {
		t.Errorf("created role: %+v", got)
	}
}

func TestRoles_Create_NonAdminBlocked(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/roles", memberTok, map[string]string{"name": "x"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestRoles_List_AnyMember(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodGet, "/api/v1/houses/h1/roles", memberTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var got []csil.Role
	decodeJSONOrFatal(t, rec, &got)
	// h1 was seeded with admin + member roles via seedMember
	if len(got) < 2 {
		t.Errorf("want >=2 roles, got %d", len(got))
	}
}

func TestRoles_CrossHouseBlocked(t *testing.T) {
	h := newHarness(t)
	admin1, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodGet, "/api/v1/houses/h2/roles", admin1, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403 (token is for h1, URL is h2)", rec.Code)
	}
}

func TestRoles_GrantAndListMemberRoles(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	// Find the existing admin role's id (seeded by seedMember).
	var adminRoleID string
	for _, r := range h.store.roles {
		if r.HouseID == "h1" && r.Name == "admin" {
			adminRoleID = r.RoleID
		}
	}
	if adminRoleID == "" {
		t.Fatal("seeded admin role not found")
	}

	rec := h.do(http.MethodPost,
		"/api/v1/houses/h1/members/m-member1/roles/"+adminRoleID, admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("grant status: %d, body=%s", rec.Code, rec.Body.String())
	}

	// Confirm via list-member-roles.
	rec = h.do(http.MethodGet, "/api/v1/houses/h1/members/m-member1/roles", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status: %d", rec.Code)
	}
	var roles []csil.Role
	decodeJSONOrFatal(t, rec, &roles)
	hasAdmin := false
	for _, r := range roles {
		if r.Name == "admin" {
			hasAdmin = true
		}
	}
	if !hasAdmin {
		t.Errorf("expected member1 to now have admin; got %+v", roles)
	}

	// And the audit row was recorded.
	if len(h.store.audits) == 0 {
		t.Error("expected an audit row for the role grant")
	}
}

func TestRoles_RevokeNonAdminBlocked(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	var adminRoleID string
	for _, r := range h.store.roles {
		if r.HouseID == "h1" && r.Name == "admin" {
			adminRoleID = r.RoleID
		}
	}
	rec := h.do(http.MethodDelete,
		"/api/v1/houses/h1/members/m-admin1/roles/"+adminRoleID, memberTok, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestRoles_DeleteRoleFromOtherHouseBlocked(t *testing.T) {
	h := newHarness(t)
	admin1, _ := setupTwoHousesWithAdmins(h)

	// Grab a role belonging to h2.
	var foreignRoleID string
	for _, r := range h.store.roles {
		if r.HouseID == "h2" {
			foreignRoleID = r.RoleID
		}
	}
	if foreignRoleID == "" {
		t.Fatal("no h2 role seeded")
	}
	rec := h.do(http.MethodDelete, "/api/v1/houses/h1/roles/"+foreignRoleID, admin1, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403 (role belongs to h2)", rec.Code)
	}
}
