package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func listGroups(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	groups, err := store.AppStore.ListGroupsByHouse(r.Context(), houseFromPath(r), limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, groupsToCSIL(groups))
}

func createGroup(w http.ResponseWriter, r *http.Request) {
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
	g := &models.Group{HouseID: houseFromPath(r), Name: body.Name, Description: body.Description}
	if err := store.AppStore.CreateGroup(r.Context(), g); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, groupToCSIL(g))
}

func updateGroup(w http.ResponseWriter, r *http.Request) {
	g, err := groupInScope(w, r)
	if err != nil {
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
		g.Name = *body.Name
	}
	if body.Description != nil {
		g.Description = *body.Description
	}
	if err := store.AppStore.UpdateGroup(r.Context(), g); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, groupToCSIL(g))
}

func deleteGroup(w http.ResponseWriter, r *http.Request) {
	g, err := groupInScope(w, r)
	if err != nil {
		return
	}
	if err := store.AppStore.DeleteGroup(r.Context(), g.GroupID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func addGroupMember(w http.ResponseWriter, r *http.Request) {
	g, err := groupInScope(w, r)
	if err != nil {
		return
	}
	memberID := r.PathValue("member_id")
	if err := store.AppStore.AddGroupMember(r.Context(), g.GroupID, memberID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func removeGroupMember(w http.ResponseWriter, r *http.Request) {
	g, err := groupInScope(w, r)
	if err != nil {
		return
	}
	memberID := r.PathValue("member_id")
	if err := store.AppStore.RemoveGroupMember(r.Context(), g.GroupID, memberID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listGroupMembers(w http.ResponseWriter, r *http.Request) {
	g, err := groupInScope(w, r)
	if err != nil {
		return
	}
	members, err := store.AppStore.ListGroupMembers(r.Context(), g.GroupID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membersToCSIL(members))
}

func groupInScope(w http.ResponseWriter, r *http.Request) (*models.Group, error) {
	id := r.PathValue("group_id")
	g, err := store.AppStore.GetGroupByID(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return nil, err
	}
	if g.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "group belongs to a different house")
		return nil, errForbidden
	}
	return g, nil
}
