package handlers

// Dev-auth handlers. These are registered ONLY when
// config.DevAuthAllowed() returns true (env != prod AND
// LONGHOUSE_DEV_AUTH_ENABLED=true). When disabled they're absent from
// the router — clients see the normal 404 from http.ServeMux, no
// per-request middleware branch.
//
// The contract is deliberately small:
//   GET  /api/v1/dev/users  → list every (house, member) so a dev tool
//                             can present a picker.
//   POST /api/v1/dev/login  → {"member_id":..., "house_id":...} →
//                             mints a real JWT for that member using
//                             the same auth.Mint path as production
//                             /auth/login. The middleware path is
//                             unchanged; only the *origin* of the JWT
//                             differs.
//
// "Logging in as another user" — impersonation as a real product
// feature — is explicitly out of scope. If we ever want it, it's a
// distinct handler with admin role checks and an audit row; this one
// is for development convenience and refuses to exist outside dev/nonprod.

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// DevAuthStore is the slice of the application store the dev endpoints need.
// Kept narrow so tests can fake it cheaply. FindMembersByLinkkeysIdentity +
// ListRolesForMember let dev-login enrich the token exactly like real login.
type DevAuthStore interface {
	GetMemberByID(ctx context.Context, memberID string) (*models.Member, error)
	FindMembersByLinkkeysIdentity(ctx context.Context, domain, userID string) ([]models.Member, error)
	ListRolesForMember(ctx context.Context, memberID string) ([]models.Role, error)
	ListHouses(ctx context.Context, limit, offset int) ([]models.House, error)
	ListMembersByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Member, error)
}

// DevAuthDeps bundles what the dev-auth handlers need at runtime. Constructed
// in cmd/serve.go iff config.DevAuthAllowed() is true.
type DevAuthDeps struct {
	Store     DevAuthStore
	JWTSecret []byte
	Env       string // logged on every issuance for grep-ability
}

// DevUserEntry is one row in the /api/v1/dev/users listing.
type DevUserEntry struct {
	MemberID       string   `json:"member_id"`
	HouseID        string   `json:"house_id"`
	HouseName      string   `json:"house_name"`
	DisplayName    string   `json:"display_name,omitempty"`
	LinkkeysDomain string   `json:"linkkeys_domain,omitempty"`
	LinkkeysUserID string   `json:"linkkeys_user_id,omitempty"`
	Roles          []string `json:"roles"`
}

// DevLoginRequest is the body of POST /api/v1/dev/login. We accept a
// member_id because the picker lists members, but the minted token is
// identity-scoped — we read the member's linkkeys identity and mint from
// that. (house_id is accepted but ignored; kept so old callers don't break.)
type DevLoginRequest struct {
	MemberID string `json:"member_id"`
	HouseID  string `json:"house_id,omitempty"`
}

// usersHandler lists every member across every house. Used by dev tooling
// (e.g., the SPA's /dev-login picker) so devs can see who's available to
// sign in as. House and member counts are capped at 1000 each — generous for
// any plausible local DB, and a cheap upper bound to keep pathological seeds
// from melting the picker.
func (d *DevAuthDeps) usersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	houses, err := d.Store.ListHouses(ctx, 1000, 0)
	if err != nil {
		log.WithError(err).Error("dev-auth: list houses failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := []DevUserEntry{}
	for _, h := range houses {
		members, err := d.Store.ListMembersByHouse(ctx, h.HouseID, 1000, 0)
		if err != nil {
			log.WithError(err).WithField("house_id", h.HouseID).Error("dev-auth: list members failed")
			continue
		}
		for _, m := range members {
			roles, _ := d.Store.ListRolesForMember(ctx, m.MemberID)
			roleNames := make([]string, 0, len(roles))
			for _, role := range roles {
				roleNames = append(roleNames, role.Name)
			}
			out = append(out, DevUserEntry{
				MemberID:       m.MemberID,
				HouseID:        m.HouseID,
				HouseName:      h.Name,
				DisplayName:    m.DisplayName,
				LinkkeysDomain: m.LinkkeysDomain,
				LinkkeysUserID: m.LinkkeysUserID,
				Roles:          roleNames,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

// loginHandler mints an identity JWT for the supplied member without any
// linkkeys exchange. The member row must exist; we read its linkkeys
// identity (domain, user_id) and mint from that — the resulting token is
// indistinguishable from one minted by the real assertion flow.
func (d *DevAuthDeps) loginHandler(w http.ResponseWriter, r *http.Request) {
	if d.JWTSecret == nil || d.Store == nil {
		writeError(w, http.StatusInternalServerError, "dev-auth not configured")
		return
	}

	var req DevLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.MemberID == "" {
		writeError(w, http.StatusBadRequest, "member_id is required")
		return
	}

	member, err := d.Store.GetMemberByID(r.Context(), req.MemberID)
	if err != nil || member == nil {
		writeError(w, http.StatusBadRequest, "member not found")
		return
	}

	// Enrich exactly like real login so the dev token is indistinguishable:
	// full per-house membership + roles for this member's identity.
	houses, err := buildHouseRoles(r.Context(), d.Store, member.LinkkeysDomain, member.LinkkeysUserID)
	if err != nil {
		log.WithError(err).Error("dev-auth: enrich houses failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	tok, err := auth.Mint(d.JWTSecret, auth.Identity{
		Domain:      member.LinkkeysDomain,
		UserID:      member.LinkkeysUserID,
		DisplayName: member.DisplayName,
		Houses:      houses,
	}, 0)
	if err != nil {
		log.WithError(err).Error("dev-auth: jwt mint failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	log.WithFields(log.Fields{
		"env":              d.Env,
		"linkkeys_domain":  member.LinkkeysDomain,
		"linkkeys_user_id": member.LinkkeysUserID,
		"houses":           len(houses),
	}).Warn("DEV-AUTH: minted identity JWT without assertion verification")

	verified, _ := auth.Verify(d.JWTSecret, tok)
	writeJSON(w, http.StatusOK, csil.LoginResponse{
		Token:       tok,
		Domain:      member.LinkkeysDomain,
		UserId:      member.LinkkeysUserID,
		DisplayName: optStr(member.DisplayName),
		ExpiresAt:   csil.Timestamp(time.Unix(verified.ExpiresAt, 0).UTC().Format(time.RFC3339)),
	})
}
