package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func listSkills(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	skills, err := store.AppStore.ListSkillsByHouse(r.Context(), houseFromPath(r), limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skillsToCSIL(skills))
}

func createSkill(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	skill := &models.Skill{HouseID: houseFromPath(r), Name: body.Name, Description: body.Description}
	if err := store.AppStore.CreateSkill(r.Context(), skill); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, skillToCSIL(skill))
}

func updateSkill(w http.ResponseWriter, r *http.Request) {
	skillID := r.PathValue("skill_id")
	skill, err := store.AppStore.GetSkillByID(r.Context(), skillID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if skill.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "skill belongs to a different house")
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Name != nil {
		skill.Name = *body.Name
	}
	if body.Description != nil {
		skill.Description = *body.Description
	}
	if err := store.AppStore.UpdateSkill(r.Context(), skill); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skillToCSIL(skill))
}

func deleteSkill(w http.ResponseWriter, r *http.Request) {
	skillID := r.PathValue("skill_id")
	skill, err := store.AppStore.GetSkillByID(r.Context(), skillID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if skill.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "skill belongs to a different house")
		return
	}
	if err := store.AppStore.DeleteSkill(r.Context(), skillID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addMemberSkill: any member may add a skill to themselves; admin may add
// to anyone in the house.
func addMemberSkill(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("member_id")
	skillID := r.PathValue("skill_id")
	if !skillBelongsToHouse(w, r, skillID) {
		return
	}
	if !memberCanModifySkills(w, r, memberID) {
		return
	}
	if err := store.AppStore.AssignSkill(r.Context(), memberID, skillID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func removeMemberSkill(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("member_id")
	skillID := r.PathValue("skill_id")
	if !skillBelongsToHouse(w, r, skillID) {
		return
	}
	if !memberCanModifySkills(w, r, memberID) {
		return
	}
	if err := store.AppStore.UnassignSkill(r.Context(), memberID, skillID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listMemberSkills(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("member_id")
	skills, err := store.AppStore.ListSkillsForMember(r.Context(), memberID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skillsToCSIL(skills))
}

func skillBelongsToHouse(w http.ResponseWriter, r *http.Request, skillID string) bool {
	skill, err := store.AppStore.GetSkillByID(r.Context(), skillID)
	if err != nil {
		notFoundOr500(w, err)
		return false
	}
	if skill.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "skill belongs to a different house")
		return false
	}
	return true
}

func memberCanModifySkills(w http.ResponseWriter, r *http.Request, memberID string) bool {
	c := auth.FromContext(r.Context())
	if c == nil {
		writeError(w, http.StatusUnauthorized, "missing token")
		return false
	}
	if c.MemberID == memberID || c.HasRole(models.RoleAdmin) {
		return true
	}
	writeError(w, http.StatusForbidden, "members may only modify their own skills")
	return false
}
