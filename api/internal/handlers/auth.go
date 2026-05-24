package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// PKIClient is the subset of linkkeys.Client the auth handlers need. Defined
// here so tests can inject a fake without a live RP.
type PKIClient interface {
	SignRequest(callbackURL, nonce string) (string, error)
	DecryptToken(encryptedToken string) (string, error)
	VerifyAssertion(signedAssertion, expectedDomain string) (*linkkeys.Assertion, error)
}

// AuthStore is the slice of the application store the auth handlers need.
// Login/refresh enrich the token with the caller's houses+roles; /me looks
// up house names for the switcher.
type AuthStore interface {
	FindMembersByLinkkeysIdentity(ctx context.Context, domain, userID string) ([]models.Member, error)
	ListRolesForMember(ctx context.Context, memberID string) ([]models.Role, error)
	GetHouseByID(ctx context.Context, houseID string) (*models.House, error)
}

// AuthDeps bundles what the auth endpoints need.
type AuthDeps struct {
	PKI       PKIClient
	Store     AuthStore
	IDPDomain string
	JWTSecret []byte

	// Browser-flow config. IDPURL is the authorize host; CallbackURL is the
	// SPA route the IDP returns the sealed token to (also the expected
	// assertion audience). Empty when the browser flow isn't configured —
	// /auth/start then returns 500 but programmatic /auth/login still works.
	IDPURL      string
	CallbackURL string
}

// identityEnricher is the store surface buildHouseRoles needs. Both AuthStore
// and the dev-auth store satisfy it.
type identityEnricher interface {
	FindMembersByLinkkeysIdentity(ctx context.Context, domain, userID string) ([]models.Member, error)
	ListRolesForMember(ctx context.Context, memberID string) ([]models.Role, error)
}

// buildHouseRoles snapshots every house the identity belongs to, with the
// member id + role names per house. This is the DB work that used to happen
// per-request; now it happens once at mint time (login / refresh / dev-login)
// and is baked into the token.
func buildHouseRoles(ctx context.Context, store identityEnricher, domain, userID string) ([]auth.HouseRoles, error) {
	members, err := store.FindMembersByLinkkeysIdentity(ctx, domain, userID)
	if err != nil {
		return nil, err
	}
	out := make([]auth.HouseRoles, 0, len(members))
	for _, m := range members {
		roles, err := store.ListRolesForMember(ctx, m.MemberID)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(roles))
		for _, role := range roles {
			names = append(names, role.Name)
		}
		out = append(out, auth.HouseRoles{House: m.HouseID, Member: m.MemberID, Roles: names})
	}
	return out, nil
}

// optStr returns &s when s is non-empty — the csil generated types use
// `*string` for CSIL `? optional` fields and rely on omitempty for nils.
func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// loginHandler verifies a linkkeys assertion, snapshots the caller's
// houses+roles, and issues a self-contained identity token.
//
//	200 — token issued
//	400 — missing/invalid body
//	401 — assertion didn't verify
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

	houses, err := buildHouseRoles(r.Context(), d.Store, assertion.Domain, assertion.UserID)
	if err != nil {
		log.WithError(err).Error("login: enrich houses failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	d.issueIdentityToken(w, assertion.Domain, assertion.UserID, assertion.DisplayName, houses)
}

// startHandler begins the browser assertion flow: mint a stateless nonce,
// have the RP sign an auth request bound to our SPA callback, and 302 the
// browser to the IDP authorize page. No cookie, no server state — the nonce
// round-trips inside the assertion and is re-checked at /auth/complete.
func (d *AuthDeps) startHandler(w http.ResponseWriter, r *http.Request) {
	if d.PKI == nil || d.JWTSecret == nil || d.IDPURL == "" || d.CallbackURL == "" {
		writeError(w, http.StatusInternalServerError, "browser login is not configured on this server")
		return
	}
	nonce := auth.MintNonce(d.JWTSecret)
	signedRequest, err := d.PKI.SignRequest(d.CallbackURL, nonce)
	if err != nil {
		log.WithError(err).Error("auth/start: RP sign-request failed")
		writeError(w, http.StatusBadGateway, "could not reach identity service")
		return
	}
	q := url.Values{}
	q.Set("signed_request", signedRequest)
	http.Redirect(w, r, d.IDPURL+"/auth/authorize?"+q.Encode(), http.StatusFound)
}

// completeHandler finishes the browser flow: decrypt the sealed token via the
// RP, verify the assertion, re-check the stateless nonce + domain + audience,
// then enrich + mint exactly like loginHandler. Returns the token in the body
// (it never travels in a URL).
func (d *AuthDeps) completeHandler(w http.ResponseWriter, r *http.Request) {
	if d.JWTSecret == nil || d.PKI == nil || d.Store == nil {
		writeError(w, http.StatusInternalServerError, "auth not configured on this server")
		return
	}

	var req csil.CompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.EncryptedToken == "" {
		writeError(w, http.StatusBadRequest, "encrypted_token is required")
		return
	}

	signedAssertion, err := d.PKI.DecryptToken(req.EncryptedToken)
	if err != nil {
		log.WithError(err).Error("auth/complete: RP decrypt-token failed")
		writeError(w, http.StatusBadGateway, "could not decrypt token")
		return
	}

	assertion, err := d.PKI.VerifyAssertion(signedAssertion, d.IDPDomain)
	if err != nil {
		log.WithError(err).Info("auth/complete: assertion verification failed")
		writeError(w, http.StatusUnauthorized, "assertion verification failed")
		return
	}

	// The nonce we issued at /auth/start round-trips inside the assertion;
	// re-checking its HMAC + expiry proves this callback answers a login we
	// actually started, with no stored state.
	if err := auth.VerifyNonce(d.JWTSecret, assertion.Nonce); err != nil {
		log.WithError(err).Info("auth/complete: nonce rejected")
		writeError(w, http.StatusUnauthorized, "login nonce invalid or expired")
		return
	}
	if assertion.Domain != d.IDPDomain {
		writeError(w, http.StatusUnauthorized, "assertion from unexpected domain")
		return
	}
	if assertion.Audience != "" && d.CallbackURL != "" && assertion.Audience != d.CallbackURL {
		writeError(w, http.StatusUnauthorized, "assertion audience mismatch")
		return
	}

	houses, err := buildHouseRoles(r.Context(), d.Store, assertion.Domain, assertion.UserID)
	if err != nil {
		log.WithError(err).Error("auth/complete: enrich houses failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	d.issueIdentityToken(w, assertion.Domain, assertion.UserID, assertion.DisplayName, houses)
}

// refreshHandler re-mints the caller's token with a fresh houses+roles
// snapshot. Requires a still-valid bearer (composes after RequireBearer).
// This is how role/membership changes take effect inside the token's
// staleness window without forcing a full re-login.
func (d *AuthDeps) refreshHandler(w http.ResponseWriter, r *http.Request) {
	id := auth.IdentityFromContext(r.Context())
	if id == nil {
		writeError(w, http.StatusUnauthorized, "missing token")
		return
	}
	houses, err := buildHouseRoles(r.Context(), d.Store, id.Domain, id.UserID)
	if err != nil {
		log.WithError(err).Error("refresh: enrich houses failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	d.issueIdentityToken(w, id.Domain, id.UserID, id.DisplayName, houses)
}

// issueIdentityToken mints a token from an identity + house snapshot and
// writes the login response. Shared by login, refresh, and (Phase B) the
// browser callback so every path produces identical tokens.
func (d *AuthDeps) issueIdentityToken(w http.ResponseWriter, domain, userID, displayName string, houses []auth.HouseRoles) {
	tok, err := auth.Mint(d.JWTSecret, auth.Identity{
		Domain:      domain,
		UserID:      userID,
		DisplayName: displayName,
		Houses:      houses,
	}, 0)
	if err != nil {
		log.WithError(err).Error("login: jwt mint failed")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	verified, _ := auth.Verify(d.JWTSecret, tok) // safe: just minted
	writeJSON(w, http.StatusOK, csil.LoginResponse{
		Token:       tok,
		Domain:      domain,
		UserId:      userID,
		DisplayName: optStr(displayName),
		ExpiresAt:   csil.Timestamp(time.Unix(verified.ExpiresAt, 0).UTC().Format(time.RFC3339)),
	})
}

// meHandler echoes the caller's identity plus the houses they belong to.
// The houses come straight from the token (already snapshotted); we only hit
// the store to resolve display names for the switcher.
func (d *AuthDeps) meHandler(w http.ResponseWriter, r *http.Request) {
	id := auth.IdentityFromContext(r.Context())
	if id == nil {
		writeError(w, http.StatusUnauthorized, "missing token")
		return
	}

	houses := make([]csil.HouseSummary, 0, len(id.Houses))
	for _, h := range id.Houses {
		name := ""
		if d.Store != nil {
			if house, err := d.Store.GetHouseByID(r.Context(), h.House); err == nil && house != nil {
				name = house.Name
			}
		}
		houses = append(houses, csil.HouseSummary{HouseId: csil.HouseID(h.House), Name: name})
	}

	writeJSON(w, http.StatusOK, csil.MeResponse{
		Domain:      id.Domain,
		UserId:      id.UserID,
		DisplayName: optStr(id.DisplayName),
		ExpiresAt:   csil.Timestamp(time.Unix(id.ExpiresAt, 0).UTC().Format(time.RFC3339)),
		Houses:      houses,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
