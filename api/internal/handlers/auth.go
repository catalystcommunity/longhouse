package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// PKIClient is the subset of linkkeys.Client that handlers need. Defined
// here so tests can inject a fake without a live sidecar.
type PKIClient interface {
	VerifyAssertion(signedAssertion, expectedDomain string) (*linkkeys.Assertion, error)
}

// LoginStore is the subset of the application Store used by the login
// handler. The trusted-domain auto-membership flow needs more than the
// simple member lookup.
type LoginStore interface {
	FindMembersByLinkkeysIdentity(ctx context.Context, domain, userID string) ([]models.Member, error)
	ListRolesForMember(ctx context.Context, memberID string) ([]models.Role, error)
	GetHouseByID(ctx context.Context, houseID string) (*models.House, error)
	HousesTrustingDomain(ctx context.Context, domain string) ([]models.House, error)
	IsDomainTrusted(ctx context.Context, houseID, domain string) (bool, error)
	GetRoleByName(ctx context.Context, houseID, name string) (*models.Role, error)
	CreateMember(ctx context.Context, member *models.Member) error
	UpdateMember(ctx context.Context, member *models.Member) error
	AssignRole(ctx context.Context, memberID, roleID string) error
	RecordMemberAudit(ctx context.Context, audit *models.MemberAudit) error
}

// AuthDeps bundles everything POST /auth/login needs.
type AuthDeps struct {
	PKI       PKIClient
	Store     LoginStore
	IDPDomain string
	JWTSecret []byte
}

// loginHandler verifies a linkkeys assertion, finds (or creates, if the
// domain is trusted) the caller's member rows, and issues a JWT scoped to
// one house. Status codes:
//
//   200 — token returned (caller specified a house, or has exactly one
//         option — whether existing membership or trusted-domain auto-join)
//   409 — caller has more than one option; body lists them
//   401 — assertion didn't verify
//   403 — verified, but caller has no member row anywhere AND no house
//         trusts their linkkeys domain
func (d *AuthDeps) loginHandler(w http.ResponseWriter, r *http.Request) {
	if d.JWTSecret == nil || d.PKI == nil || d.Store == nil {
		writeError(w, http.StatusInternalServerError, "auth not configured on this server")
		return
	}

	var req csil.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.SignedAssertion == "" {
		writeError(w, http.StatusBadRequest, "signed_assertion is required")
		return
	}

	assertion, err := d.PKI.VerifyAssertion(req.SignedAssertion, d.IDPDomain)
	if err != nil {
		log.WithError(err).Info("login: assertion verification failed")
		writeError(w, http.StatusUnauthorized, "assertion verification failed")
		return
	}

	members, err := d.Store.FindMembersByLinkkeysIdentity(r.Context(), assertion.Domain, assertion.UserID)
	if err != nil {
		log.WithError(err).Error("login: lookup members by identity failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	memberByHouse := map[string]*models.Member{}
	for i := range members {
		memberByHouse[members[i].HouseID] = &members[i]
	}

	// Houses where this domain is trusted. Auto-join targets when the
	// caller picks one of these but isn't yet a member.
	trustedHouses, err := d.Store.HousesTrustingDomain(r.Context(), assertion.Domain)
	if err != nil {
		log.WithError(err).Error("login: lookup trusted houses failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	available := availableHouses(members, trustedHouses)
	if len(available) == 0 {
		writeError(w, http.StatusForbidden, "no membership and no trusted domain in any house")
		return
	}

	desired := ""
	if req.HouseId != nil {
		desired = string(*req.HouseId)
	}
	chosenHouseID, err := pickAvailableHouse(available, desired)
	if err != nil {
		writeJSON(w, http.StatusConflict, csil.LoginMultiHouseResponse{
			Error:  "house_id required: caller can log in to multiple houses",
			Houses: housesSummaryFromIDs(r.Context(), d.Store, available),
		})
		return
	}

	// Resolve to a member row. If the caller is logging in via trusted
	// domain to a house they're not yet in, create the member + assign
	// the canonical 'member' role + record the audit.
	chosen, ok := memberByHouse[chosenHouseID]
	if !ok {
		chosen, err = d.autoCreateMember(r.Context(), assertion, chosenHouseID)
		if err != nil {
			log.WithError(err).Error("login: auto-create via trusted domain failed")
			writeError(w, http.StatusInternalServerError, "could not provision membership")
			return
		}
	} else if chosen.DisplayName == "" && assertion.DisplayName != "" {
		// First-time display name — populate from the IDP. Failure here
		// is non-fatal; we just log and proceed without it.
		chosen.DisplayName = assertion.DisplayName
		if err := d.Store.UpdateMember(r.Context(), chosen); err != nil {
			log.WithError(err).Warn("login: failed to backfill display_name")
		}
	}

	roles, err := d.Store.ListRolesForMember(r.Context(), chosen.MemberID)
	if err != nil {
		log.WithError(err).Error("login: list roles failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, role.Name)
	}

	tok, err := auth.Mint(d.JWTSecret, auth.Claims{
		MemberID: chosen.MemberID,
		HouseID:  chosen.HouseID,
		Roles:    roleNames,
	}, 0)
	if err != nil {
		log.WithError(err).Error("login: jwt mint failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	verified, _ := auth.Verify(d.JWTSecret, tok) // safe: just minted
	writeJSON(w, http.StatusOK, csil.LoginResponse{
		Token:     tok,
		MemberId:  csil.MemberID(chosen.MemberID),
		HouseId:   csil.HouseID(chosen.HouseID),
		Roles:     roleNames,
		ExpiresAt: csil.Timestamp(time.Unix(verified.ExpiresAt, 0).UTC().Format(time.RFC3339)),
	})
}

// autoCreateMember provisions a member row in the target house using the
// verified linkkeys assertion's identity, assigns the canonical 'member'
// role (if it exists), and records an audit row. Called only when the
// caller picked a house their domain is trusted in but they're not yet a
// member of.
func (d *AuthDeps) autoCreateMember(ctx context.Context, a *linkkeys.Assertion, houseID string) (*models.Member, error) {
	// Defense in depth: re-verify trust at point of write. Avoids a TOCTOU
	// where the trusted_domains row was removed between the listing call
	// above and the member insert.
	trusted, err := d.Store.IsDomainTrusted(ctx, houseID, a.Domain)
	if err != nil {
		return nil, err
	}
	if !trusted {
		return nil, errors.New("login: domain not trusted in selected house")
	}
	m := &models.Member{
		HouseID:        houseID,
		LinkkeysDomain: a.Domain,
		LinkkeysUserID: a.UserID,
		DisplayName:    a.DisplayName,
	}
	if err := d.Store.CreateMember(ctx, m); err != nil {
		return nil, err
	}
	// Assign the canonical 'member' role for this house. If the role is
	// missing (older houses, or houses created without seeding canonicals),
	// the JWT just lacks it — the user can still log in but won't pass any
	// `requireMember` checks that look for that exact role name. Existing
	// houses created via SeedInitialAdmin always have it.
	if role, err := d.Store.GetRoleByName(ctx, houseID, models.RoleMember); err == nil && role != nil {
		_ = d.Store.AssignRole(ctx, m.MemberID, role.RoleID)
	}
	_ = d.Store.RecordMemberAudit(ctx, &models.MemberAudit{
		HouseID:         houseID,
		SubjectMemberID: m.MemberID,
		Action:          models.AuditActionMemberAutoCreated,
		Detail: models.JSONMap{
			"linkkeys_domain":  a.Domain,
			"linkkeys_user_id": a.UserID,
			"via":              "trusted_domain",
		},
	})
	return m, nil
}

// availableHouses returns the deduplicated list of house IDs the caller
// can log in to: existing membership ∪ houses trusting their domain.
func availableHouses(members []models.Member, trusted []models.House) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range members {
		if !seen[m.HouseID] {
			seen[m.HouseID] = true
			out = append(out, m.HouseID)
		}
	}
	for _, h := range trusted {
		if !seen[h.HouseID] {
			seen[h.HouseID] = true
			out = append(out, h.HouseID)
		}
	}
	return out
}

func pickAvailableHouse(available []string, desiredHouse string) (string, error) {
	if desiredHouse != "" {
		for _, h := range available {
			if h == desiredHouse {
				return h, nil
			}
		}
		return "", errNoSuchHouse
	}
	if len(available) == 1 {
		return available[0], nil
	}
	return "", errMultipleHouses
}

// meHandler echoes the caller's verified token claims.
func meHandler(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	if c == nil {
		writeError(w, http.StatusUnauthorized, "missing token")
		return
	}
	writeJSON(w, http.StatusOK, csil.MeResponse{
		MemberId:  csil.MemberID(c.MemberID),
		HouseId:   csil.HouseID(c.HouseID),
		Roles:     c.Roles,
		ExpiresAt: csil.Timestamp(time.Unix(c.ExpiresAt, 0).UTC().Format(time.RFC3339)),
	})
}

var (
	errMultipleHouses = errors.New("multiple houses; caller must specify house_id")
	errNoSuchHouse    = errors.New("caller cannot log in to the specified house")
)

func housesSummaryFromIDs(ctx context.Context, store LoginStore, ids []string) []csil.HouseSummary {
	out := make([]csil.HouseSummary, 0, len(ids))
	for _, id := range ids {
		name := ""
		if h, err := store.GetHouseByID(ctx, id); err == nil && h != nil {
			name = h.Name
		}
		out = append(out, csil.HouseSummary{HouseId: csil.HouseID(id), Name: name})
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
