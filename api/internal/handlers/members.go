package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/catalystcommunity/longhouse/api/internal/store"
)

func listMembers(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	members, err := store.AppStore.ListMembersByHouse(r.Context(), houseFromPath(r), limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membersToCSIL(members))
}

func getMember(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("member_id")
	m, err := store.AppStore.GetMemberByID(r.Context(), memberID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if m.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "member belongs to a different house")
		return
	}
	writeJSON(w, http.StatusOK, memberToCSIL(m))
}

// listMemberAudits returns the audit history for a single member. Admin-
// only — a non-admin shouldn't be able to see who granted them what.
// The route layer applies RequireAdmin; this handler only checks scope.
func listMemberAudits(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("member_id")
	m, err := store.AppStore.GetMemberByID(r.Context(), memberID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if m.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "member belongs to a different house")
		return
	}
	limit, offset := limitOffset(r)
	audits, err := store.AppStore.ListAuditsForMember(r.Context(), memberID, limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, auditsToCSIL(audits))
}

// updateMember currently allows just a display-name change. Admins can
// edit anyone; non-admins can only edit themselves.
func updateMember(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("member_id")
	m, err := store.AppStore.GetMemberByID(r.Context(), memberID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if m.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "member belongs to a different house")
		return
	}
	if !requireOwnerOrAdmin(w, r, memberID) {
		return
	}
	var body struct {
		DisplayName *string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.DisplayName != nil {
		m.DisplayName = *body.DisplayName
	}
	if err := store.AppStore.UpdateMember(r.Context(), m); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, memberToCSIL(m))
}
