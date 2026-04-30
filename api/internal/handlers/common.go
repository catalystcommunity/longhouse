package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

// houseFromPath extracts the house_id from a route registered under
// `/api/v1/houses/{house_id}/...`. Routes wrapped by RequireHouseFromPath
// have already verified the URL matches the JWT, so callers can trust this
// without re-checking.
func houseFromPath(r *http.Request) string {
	return r.PathValue("house_id")
}

// decodeJSON parses the request body or writes a 400 and returns an error.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return err
	}
	return nil
}

// notFoundOr500 maps a store error to either 404 (gorm.ErrRecordNotFound)
// or 500. Keeps boilerplate down across handlers.
func notFoundOr500(w http.ResponseWriter, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}

// limitOffset reads ?limit=&offset= from query, with sane defaults+caps.
func limitOffset(r *http.Request) (int, int) {
	limit := 50
	offset := 0
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		if n, err := parsePositiveInt(v); err == nil && n <= 500 {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			offset = n
		}
	}
	return limit, offset
}

// strPtr returns a pointer to a string literal — handy for filling
// optional fields in CSIL types from constant tags.
func strPtr(s string) *string { return &s }

func parsePositiveInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// requireOwnerOrAdmin lets the request through when the caller is the
// resource owner (their member_id equals ownerMemberID) or has the admin
// role. Otherwise writes 403 and returns false.
func requireOwnerOrAdmin(w http.ResponseWriter, r *http.Request, ownerMemberID string) bool {
	c := auth.FromContext(r.Context())
	if c == nil {
		writeError(w, http.StatusUnauthorized, "missing token")
		return false
	}
	if c.MemberID == ownerMemberID || c.HasRole(models.RoleAdmin) {
		return true
	}
	writeError(w, http.StatusForbidden, "owner or admin required")
	return false
}

// callerMemberID returns the calling member's ID. Used by handlers that
// stamp the caller as the owner of new rows.
func callerMemberID(r *http.Request) string {
	if c := auth.FromContext(r.Context()); c != nil {
		return c.MemberID
	}
	return ""
}

// recordRoleAudit centralizes the bookkeeping for role grants/revokes so
// every code path emits a consistent audit row.
func recordRoleAudit(ctx context.Context, action, houseID, subjectMemberID, actorMemberID, roleID, roleName string) error {
	audit := &models.MemberAudit{
		HouseID:         houseID,
		SubjectMemberID: subjectMemberID,
		Action:          action,
		TargetType:      strPtr("role"),
		TargetID:        &roleID,
		Detail:          models.JSONMap{"role_name": roleName},
	}
	if actorMemberID != "" && actorMemberID != subjectMemberID {
		audit.ActorMemberID = &actorMemberID
	}
	return store.AppStore.RecordMemberAudit(ctx, audit)
}
