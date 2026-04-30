package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
)

func TestGroups_AdminCRUD(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/groups", admin, map[string]string{
		"name":        "moderators",
		"description": "the trusted few",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d, body=%s", rec.Code, rec.Body.String())
	}
	var g csil.Group
	decodeJSONOrFatal(t, rec, &g)

	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/groups/"+string(g.GroupId), admin, map[string]string{
		"description": "renamed",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d, body=%s", rec.Code, rec.Body.String())
	}

	rec = h.do(http.MethodGet, "/api/v1/houses/h1/groups", admin, nil)
	var groups []csil.Group
	decodeJSONOrFatal(t, rec, &groups)
	if len(groups) != 1 {
		t.Errorf("want 1 group, got %d", len(groups))
	}

	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/groups/"+string(g.GroupId), admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d", rec.Code)
	}
}

func TestGroups_NonAdminCannotCreate(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/groups", memberTok, map[string]string{"name": "x"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestGroups_AdminAddRemoveMember(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/groups", admin, map[string]string{"name": "team"})
	var g csil.Group
	decodeJSONOrFatal(t, rec, &g)

	rec = h.do(http.MethodPost,
		"/api/v1/houses/h1/groups/"+string(g.GroupId)+"/members/m-member1", admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add: %d, body=%s", rec.Code, rec.Body.String())
	}

	rec = h.do(http.MethodGet,
		"/api/v1/houses/h1/groups/"+string(g.GroupId)+"/members", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list members: %d", rec.Code)
	}
	var got []csil.Member
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 1 || got[0].MemberId != "m-member1" {
		t.Errorf("group members: %+v", got)
	}

	rec = h.do(http.MethodDelete,
		"/api/v1/houses/h1/groups/"+string(g.GroupId)+"/members/m-member1", admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("remove: %d", rec.Code)
	}
}

func TestGroups_CrossHouseUpdateBlocked(t *testing.T) {
	h := newHarness(t)
	admin1, admin2 := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h2/groups", admin2, map[string]string{"name": "h2only"})
	var g csil.Group
	decodeJSONOrFatal(t, rec, &g)

	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/groups/"+string(g.GroupId), admin1, map[string]string{
		"name": "stolen",
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}
