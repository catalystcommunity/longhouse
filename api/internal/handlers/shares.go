package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// validShareResourceTypes mirrors the CHECK constraint on the shares table.
// Keep in sync with coredb/migrations/000001_baseline.sql.
var validShareResourceTypes = map[string]bool{
	"event": true,
	"task":  true,
	"house": true,
}

func listShares(w http.ResponseWriter, r *http.Request) {
	houseID := houseFromPath(r)
	q := r.URL.Query()
	resourceType := q.Get("resource_type")
	resourceID := q.Get("resource_id")

	var rows []models.Share
	var err error
	if resourceType != "" || resourceID != "" {
		// Filtering by resource — both must be present.
		if resourceType == "" || resourceID == "" {
			writeError(w, http.StatusBadRequest, "resource_type and resource_id must be provided together")
			return
		}
		rows, err = store.AppStore.ListSharesByResource(r.Context(), resourceType, resourceID)
		if err != nil {
			notFoundOr500(w, err)
			return
		}
		// Filter to only shares that belong to the URL's house.
		filtered := rows[:0]
		for i := range rows {
			if rows[i].HouseID == houseID {
				filtered = append(filtered, rows[i])
			}
		}
		rows = filtered
	} else {
		rows, err = store.AppStore.ListSharesByHouse(r.Context(), houseID)
		if err != nil {
			notFoundOr500(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, sharesToCSIL(rows))
}

func createShare(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LinkkeysDomain string  `json:"linkkeys_domain"`
		LinkkeysUserID string  `json:"linkkeys_user_id"`
		ResourceType   string  `json:"resource_type"`
		ResourceID     string  `json:"resource_id"`
		ExpiresAt      *string `json:"expires_at,omitempty"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	body.LinkkeysDomain = strings.TrimSpace(strings.ToLower(body.LinkkeysDomain))
	body.LinkkeysUserID = strings.TrimSpace(body.LinkkeysUserID)
	body.ResourceType = strings.TrimSpace(strings.ToLower(body.ResourceType))
	body.ResourceID = strings.TrimSpace(body.ResourceID)

	if body.LinkkeysDomain == "" || body.LinkkeysUserID == "" {
		writeError(w, http.StatusBadRequest, "linkkeys_domain and linkkeys_user_id are required")
		return
	}
	if !validShareResourceTypes[body.ResourceType] {
		writeError(w, http.StatusBadRequest, "resource_type must be one of event, task, house")
		return
	}
	if body.ResourceID == "" {
		writeError(w, http.StatusBadRequest, "resource_id is required")
		return
	}

	share := &models.Share{
		HouseID:        houseFromPath(r),
		SharedBy:       callerMemberID(r),
		LinkkeysDomain: body.LinkkeysDomain,
		LinkkeysUserID: body.LinkkeysUserID,
		ResourceType:   body.ResourceType,
		ResourceID:     body.ResourceID,
		AccessLevel:    "read",
	}
	if body.ExpiresAt != nil && *body.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *body.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "expires_at must be RFC3339")
			return
		}
		share.ExpiresAt = &t
	}

	if err := store.AppStore.CreateShare(r.Context(), share); err != nil {
		notFoundOr500(w, err)
		return
	}
	if auditErr := store.AppStore.RecordMemberAudit(r.Context(), &models.MemberAudit{
		HouseID:         share.HouseID,
		SubjectMemberID: callerMemberID(r),
		Action:          models.AuditActionShareCreated,
		TargetType:      strPtr("share"),
		TargetID:        &share.ShareID,
		Detail: models.JSONMap{
			"linkkeys_domain":  share.LinkkeysDomain,
			"linkkeys_user_id": share.LinkkeysUserID,
			"resource_type":    share.ResourceType,
			"resource_id":      share.ResourceID,
		},
	}); auditErr != nil {
		log.WithError(auditErr).Warn("recording share-create audit failed")
	}
	writeJSON(w, http.StatusCreated, shareToCSIL(share))
}

// sharedAccessHandler is the placeholder for the external-access path
// (an outside linkkeys identity using a share to read a shared resource).
// Schema, store ops, and admin grant/revoke are in place; the verification
// + scoped-token flow is intentionally not built yet — return 501 so callers
// get a clear signal instead of a 404.
func sharedAccessHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "share access is not yet implemented")
}

func deleteShare(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("share_id")
	share, err := store.AppStore.GetShareByID(r.Context(), shareID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if share.HouseID != houseFromPath(r) {
		// Don't leak existence across houses.
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := store.AppStore.DeleteShare(r.Context(), shareID); err != nil {
		notFoundOr500(w, err)
		return
	}
	if auditErr := store.AppStore.RecordMemberAudit(r.Context(), &models.MemberAudit{
		HouseID:         share.HouseID,
		SubjectMemberID: callerMemberID(r),
		Action:          models.AuditActionShareRevoked,
		TargetType:      strPtr("share"),
		TargetID:        &share.ShareID,
		Detail: models.JSONMap{
			"linkkeys_domain":  share.LinkkeysDomain,
			"linkkeys_user_id": share.LinkkeysUserID,
			"resource_type":    share.ResourceType,
			"resource_id":      share.ResourceID,
		},
	}); auditErr != nil {
		log.WithError(auditErr).Warn("recording share-revoke audit failed")
	}
	w.WriteHeader(http.StatusNoContent)
}
