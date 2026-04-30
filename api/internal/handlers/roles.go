package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// listRoles returns all roles for the caller's house. Any authenticated
// member may read; mutations are admin-only.
func listRoles(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	roles, err := store.AppStore.ListRolesByHouse(r.Context(), houseFromPath(r), limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rolesToCSIL(roles))
}

func createRole(w http.ResponseWriter, r *http.Request) {
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
	role := &models.Role{HouseID: houseFromPath(r), Name: body.Name, Description: body.Description}
	if err := store.AppStore.CreateRole(r.Context(), role); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, roleToCSIL(role))
}

func updateRole(w http.ResponseWriter, r *http.Request) {
	roleID := r.PathValue("role_id")
	role, err := store.AppStore.GetRoleByID(r.Context(), roleID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if role.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "role belongs to a different house")
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
		role.Name = *body.Name
	}
	if body.Description != nil {
		role.Description = *body.Description
	}
	if err := store.AppStore.UpdateRole(r.Context(), role); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roleToCSIL(role))
}

func deleteRole(w http.ResponseWriter, r *http.Request) {
	roleID := r.PathValue("role_id")
	role, err := store.AppStore.GetRoleByID(r.Context(), roleID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if role.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "role belongs to a different house")
		return
	}
	if err := store.AppStore.DeleteRole(r.Context(), roleID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// grantRole adds a role to a member. Admin-only. Records an audit row.
func grantRole(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("member_id")
	roleID := r.PathValue("role_id")
	role, err := store.AppStore.GetRoleByID(r.Context(), roleID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if role.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "role belongs to a different house")
		return
	}
	if err := store.AppStore.AssignRole(r.Context(), memberID, roleID); err != nil {
		notFoundOr500(w, err)
		return
	}
	if auditErr := recordRoleAudit(r.Context(), models.AuditActionRoleGranted, role.HouseID, memberID, callerMemberID(r), roleID, role.Name); auditErr != nil {
		log.WithError(auditErr).Warn("recording role-grant audit failed")
	}
	w.WriteHeader(http.StatusNoContent)
}

func revokeRole(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("member_id")
	roleID := r.PathValue("role_id")
	role, err := store.AppStore.GetRoleByID(r.Context(), roleID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if role.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "role belongs to a different house")
		return
	}
	if err := store.AppStore.RevokeRole(r.Context(), memberID, roleID); err != nil {
		notFoundOr500(w, err)
		return
	}
	if auditErr := recordRoleAudit(r.Context(), models.AuditActionRoleRevoked, role.HouseID, memberID, callerMemberID(r), roleID, role.Name); auditErr != nil {
		log.WithError(auditErr).Warn("recording role-revoke audit failed")
	}
	w.WriteHeader(http.StatusNoContent)
}

// listMemberRoles returns the roles attached to a single member. Visible
// to any house member; finer-grained visibility (e.g. "only admins can see
// other members' roles") can come with the permissions table.
func listMemberRoles(w http.ResponseWriter, r *http.Request) {
	memberID := r.PathValue("member_id")
	roles, err := store.AppStore.ListRolesForMember(r.Context(), memberID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	out := rolesToCSIL(roles)
	if out == nil {
		out = []csil.Role{}
	}
	writeJSON(w, http.StatusOK, out)
}
