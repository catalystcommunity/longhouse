package handlers

import (
	"net/http"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
)

func TestSkills_AdminCRUD(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	// Create
	rec := h.do(http.MethodPost, "/api/v1/houses/h1/skills", admin, map[string]string{
		"name": "carpentry",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d, body=%s", rec.Code, rec.Body.String())
	}
	var s csil.Skill
	decodeJSONOrFatal(t, rec, &s)

	// Update
	rec = h.do(http.MethodPatch, "/api/v1/houses/h1/skills/"+string(s.SkillId), admin, map[string]string{
		"description": "rough framing",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d, body=%s", rec.Code, rec.Body.String())
	}

	// List
	rec = h.do(http.MethodGet, "/api/v1/houses/h1/skills", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var skills []csil.Skill
	decodeJSONOrFatal(t, rec, &skills)
	if len(skills) != 1 {
		t.Errorf("want 1, got %d", len(skills))
	}

	// Delete
	rec = h.do(http.MethodDelete, "/api/v1/houses/h1/skills/"+string(s.SkillId), admin, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d", rec.Code)
	}
}

func TestSkills_NonAdminCannotCreate(t *testing.T) {
	h := newHarness(t)
	setupTwoHousesWithAdmins(h)
	memberTok := h.token("m-member1", "h1", "member")

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/skills", memberTok, map[string]string{"name": "x"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestSkills_MemberCanSelfAssign(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/skills", admin, map[string]string{"name": "carpentry"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	var s csil.Skill
	decodeJSONOrFatal(t, rec, &s)

	memberTok := h.token("m-member1", "h1", "member")
	rec = h.do(http.MethodPost,
		"/api/v1/houses/h1/members/m-member1/skills/"+string(s.SkillId), memberTok, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("self-assign: %d, body=%s", rec.Code, rec.Body.String())
	}

	rec = h.do(http.MethodGet,
		"/api/v1/houses/h1/members/m-member1/skills", memberTok, nil)
	var got []csil.Skill
	decodeJSONOrFatal(t, rec, &got)
	if len(got) != 1 || got[0].Name != "carpentry" {
		t.Errorf("self-skill not visible: %+v", got)
	}
}

func TestSkills_MemberCannotAssignToOther(t *testing.T) {
	h := newHarness(t)
	admin, _ := setupTwoHousesWithAdmins(h)

	rec := h.do(http.MethodPost, "/api/v1/houses/h1/skills", admin, map[string]string{"name": "carpentry"})
	var s csil.Skill
	decodeJSONOrFatal(t, rec, &s)

	memberTok := h.token("m-member1", "h1", "member")
	rec = h.do(http.MethodPost,
		"/api/v1/houses/h1/members/m-admin1/skills/"+string(s.SkillId), memberTok, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}
