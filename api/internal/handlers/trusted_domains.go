package handlers

import (
	"net/http"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// trustedDomainToCSIL converts the DB row to the wire shape. Inlined here
// rather than in convert.go because it's only used by these handlers.
func trustedDomainToCSIL(td *models.TrustedDomain) csil.TrustedDomain {
	return csil.TrustedDomain{
		TrustedDomainId: csil.TrustedDomainID(td.TrustedDomainID),
		HouseId:         csil.HouseID(td.HouseID),
		Domain:          td.Domain,
		CreatedAt:       ts(td.CreatedAt),
	}
}

func listTrustedDomains(w http.ResponseWriter, r *http.Request) {
	tds, err := store.AppStore.ListTrustedDomains(r.Context(), houseFromPath(r))
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	out := make([]csil.TrustedDomain, 0, len(tds))
	for i := range tds {
		out = append(out, trustedDomainToCSIL(&tds[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func createTrustedDomain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain string `json:"domain"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	body.Domain = strings.TrimSpace(strings.ToLower(body.Domain))
	if body.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	td := &models.TrustedDomain{HouseID: houseFromPath(r), Domain: body.Domain}
	if err := store.AppStore.CreateTrustedDomain(r.Context(), td); err != nil {
		notFoundOr500(w, err)
		return
	}
	if auditErr := store.AppStore.RecordMemberAudit(r.Context(), &models.MemberAudit{
		HouseID:         td.HouseID,
		SubjectMemberID: callerMemberID(r),
		Action:          models.AuditActionTrustedDomainAdded,
		TargetType:      strPtr("trusted_domain"),
		TargetID:        &td.TrustedDomainID,
		Detail:          models.JSONMap{"domain": body.Domain},
	}); auditErr != nil {
		log.WithError(auditErr).Warn("recording trusted-domain-add audit failed")
	}
	writeJSON(w, http.StatusCreated, trustedDomainToCSIL(td))
}

func deleteTrustedDomain(w http.ResponseWriter, r *http.Request) {
	tdID := r.PathValue("trusted_domain_id")
	// Confirm the row belongs to the URL's house before deleting; the
	// store's DeleteTrustedDomain doesn't check.
	tds, err := store.AppStore.ListTrustedDomains(r.Context(), houseFromPath(r))
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	var found *models.TrustedDomain
	for i := range tds {
		if tds[i].TrustedDomainID == tdID {
			found = &tds[i]
			break
		}
	}
	if found == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := store.AppStore.DeleteTrustedDomain(r.Context(), tdID); err != nil {
		notFoundOr500(w, err)
		return
	}
	if auditErr := store.AppStore.RecordMemberAudit(r.Context(), &models.MemberAudit{
		HouseID:         found.HouseID,
		SubjectMemberID: callerMemberID(r),
		Action:          models.AuditActionTrustedDomainRemoved,
		TargetType:      strPtr("trusted_domain"),
		TargetID:        &found.TrustedDomainID,
		Detail:          models.JSONMap{"domain": found.Domain},
	}); auditErr != nil {
		log.WithError(auditErr).Warn("recording trusted-domain-remove audit failed")
	}
	w.WriteHeader(http.StatusNoContent)
}
