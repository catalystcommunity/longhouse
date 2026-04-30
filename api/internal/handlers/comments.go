package handlers

import (
	"net/http"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// commentToCSIL converts the DB row to its wire shape.
func commentToCSIL(c *models.Comment) csil.Comment {
	return csil.Comment{
		CommentId:   csil.CommentID(c.CommentID),
		HouseId:     csil.HouseID(c.HouseID),
		MemberId:    csil.MemberID(c.MemberID),
		TargetType:  csil.TargetType(c.TargetType),
		TargetId:    c.TargetID,
		Body:        c.Body,
		CreatedAt:   ts(c.CreatedAt),
		UpdatedAt:   ts(c.UpdatedAt),
	}
}

// listComments returns the comment thread for an event/task in the caller's
// house. Path: /api/v1/houses/{house_id}/comments/{target_type}/{target_id}.
func listComments(w http.ResponseWriter, r *http.Request) {
	targetType := r.PathValue("target_type")
	targetID := r.PathValue("target_id")
	if !validCommentTarget(targetType) {
		writeError(w, http.StatusBadRequest, "target_type must be 'event' or 'task'")
		return
	}
	if !commentTargetInHouse(w, r, targetType, targetID) {
		return
	}

	limit, offset := limitOffset(r)
	comments, err := store.AppStore.ListCommentsByTarget(r.Context(), targetType, targetID, limit, offset)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	out := make([]csil.Comment, 0, len(comments))
	for i := range comments {
		out = append(out, commentToCSIL(&comments[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// createComment posts a new comment to an event or task. Owner of the
// comment is always the caller. Any house member may comment.
func createComment(w http.ResponseWriter, r *http.Request) {
	targetType := r.PathValue("target_type")
	targetID := r.PathValue("target_id")
	if !validCommentTarget(targetType) {
		writeError(w, http.StatusBadRequest, "target_type must be 'event' or 'task'")
		return
	}
	if !commentTargetInHouse(w, r, targetType, targetID) {
		return
	}

	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.Body == "" {
		writeError(w, http.StatusBadRequest, "body is required")
		return
	}
	c := &models.Comment{
		HouseID:    houseFromPath(r),
		MemberID:   callerMemberID(r),
		TargetType: targetType,
		TargetID:   targetID,
		Body:       body.Body,
	}
	if err := store.AppStore.CreateComment(r.Context(), c); err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, commentToCSIL(c))
}

// deleteComment removes a single comment. Author or admin only.
func deleteComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("comment_id")
	c, err := store.AppStore.GetCommentByID(r.Context(), commentID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if c.HouseID != houseFromPath(r) {
		writeError(w, http.StatusForbidden, "comment belongs to a different house")
		return
	}
	if !requireOwnerOrAdmin(w, r, c.MemberID) {
		return
	}
	if err := store.AppStore.DeleteComment(r.Context(), commentID); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validCommentTarget(t string) bool {
	return t == "event" || t == "task"
}

// commentTargetInHouse confirms the target row belongs to the caller's
// house. Without this, a member of one house could read or post comments
// on another house's resources via the URL.
func commentTargetInHouse(w http.ResponseWriter, r *http.Request, targetType, targetID string) bool {
	houseID := houseFromPath(r)
	switch targetType {
	case "task":
		t, err := store.AppStore.GetTaskByID(r.Context(), targetID)
		if err != nil {
			notFoundOr500(w, err)
			return false
		}
		if t.HouseID != houseID {
			writeError(w, http.StatusForbidden, "target task belongs to a different house")
			return false
		}
	case "event":
		e, err := store.AppStore.GetEventByID(r.Context(), targetID)
		if err != nil {
			notFoundOr500(w, err)
			return false
		}
		if e.HouseID != houseID {
			writeError(w, http.StatusForbidden, "target event belongs to a different house")
			return false
		}
	}
	return true
}
