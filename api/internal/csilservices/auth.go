package csilservices

import (
	"context"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// AuthService implements the CSIL AuthService over the CSIL-RPC dispatcher.
//
//	Login    — programmatic: caller already holds a verified linkkeys assertion
//	Complete — browser callback: SPA posts the IDP's sealed token
//	Refresh  — re-mint with fresh houses+roles snapshot (needs bearer)
//	Me       — identity + accessible houses (needs bearer)
//
// PKI is optional; when nil, Login/Complete refuse with InternalError so a
// misconfigured deploy never issues tokens against an unverified assertion.
// Refresh and Me work whenever the JWT secret is set, because they don't
// touch linkkeys.
type AuthService struct {
	Store     store.Store
	JWTSecret []byte
	PKI       PKIClient
	IDPDomain string

	// RPDomain is our relying-party DNS identity — the value linkkeys binds
	// each assertion to via its `audience` claim. We compare it against
	// assertion.Audience in complete(). In this single-IDP self-RP deploy it
	// equals IDPDomain. (linkkeys used to put the callback URL here; it now
	// emits the RP domain, which is why complete() no longer compares against
	// CallbackURL.)
	RPDomain string

	// Browser-flow config; used only by the GET /api/v1/auth/start handler
	// in serve.go (kept as a 302 endpoint outside the dispatcher). Stashed
	// here so a single struct carries everything the auth layer needs.
	IDPURL      string
	CallbackURL string
}

// PKIClient is the subset of linkkeys.Client the auth service uses. Kept
// here so tests can inject a fake without a live RP.
type PKIClient interface {
	SignRequest(callbackURL, nonce string) (string, error)
	DecryptToken(encryptedToken string) (string, error)
	VerifyAssertion(signedAssertion, expectedDomain string) (*linkkeys.Assertion, error)
}

// UserInfoFetcher is an optional capability a PKIClient may implement to redeem
// a verified assertion for the full linkkeys UserInfo (display name + released
// claims) — richer than the assertion itself. The CSIL-RPC/TCP transport
// implements it; the legacy HTTP sidecar does not. Enrichment is best-effort:
// login never blocks on it, and any failure or empty result falls back to the
// assertion's own fields (see enrichDisplayName).
type UserInfoFetcher interface {
	FetchUserInfo(token, domain string) (*linkkeys.UserInfo, error)
}

// fetchClaims redeems a verified assertion for the claim set the user's consent
// + the IDP's policy released to us (claim_type → value). Best-effort and
// strictly optional: returns nil when the PKI transport can't fetch UserInfo
// (the HTTP shim), the fetch fails, or nothing was released — callers must
// degrade to the assertion's own fields. We log released claim TYPES (never
// values) so the dogfood logs show what richer identity is actually available.
func (s *AuthService) fetchClaims(token string, a *linkkeys.Assertion) map[string]string {
	fetcher, ok := s.PKI.(UserInfoFetcher)
	if !ok {
		return nil
	}
	info, err := fetcher.FetchUserInfo(token, a.Domain)
	if err != nil {
		log.WithError(err).Debug("auth: userinfo-fetch failed; proceeding without claims")
		return nil
	}
	if info == nil {
		return nil
	}
	claims := info.ClaimValues()
	// The UserInfo envelope carries the display name out of band; fold it in as
	// a display_name claim when it didn't arrive as one, so reconciliation has a
	// single source of truth.
	if info.DisplayName != "" {
		if _, ok := claims["display_name"]; !ok {
			if claims == nil {
				claims = make(map[string]string, 1)
			}
			claims["display_name"] = info.DisplayName
		}
	}
	if len(claims) > 0 {
		types := make([]string, 0, len(claims))
		for t := range claims {
			types = append(types, t)
		}
		log.WithFields(log.Fields{
			"domain":      a.Domain,
			"user_id":     a.UserID,
			"claim_types": types,
		}).Debug("auth: linkkeys released claims")
	}
	return claims
}

// resolveDisplayName prefers a released display_name claim, falling back to the
// assertion's display name (i.e. pre-claims behavior) when none was granted.
func resolveDisplayName(claims map[string]string, a *linkkeys.Assertion) string {
	if v, ok := claims["display_name"]; ok && v != "" {
		return v
	}
	return a.DisplayName
}

func (s *AuthService) Register(d *csilrpc.Dispatcher) {
	d.RegisterPublic("auth", "Login", s.login)
	d.RegisterPublic("auth", "Complete", s.complete)
	d.Register("auth", "Refresh", s.refresh)
	d.Register("auth", "Logout", s.logout)
	d.Register("auth", "Me", s.me)
}

// logout records a logout security event for the bearer's identity. Tokens are
// stateless (HMAC, no server-side revocation list), so this is an audit marker
// — not a revocation; the bearer remains valid until its exp. Recorded per
// house so each house admin sees their members' logouts.
func (s *AuthService) logout(ctx context.Context, _ []byte) (any, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	s.recordSecurityEvent(ctx, models.AuditActionLogout, id.Domain, id.UserID, id.Houses, nil)
	return csil.EmptyResponse{}, nil
}

// recordSecurityEvent appends an auth audit entry. With houses, it fans out one
// entry per house (actor = that house's member) so per-house admins see it.
// With none, it writes a single global-scope (house_id NULL) entry. Best-effort.
func (s *AuthService) recordSecurityEvent(ctx context.Context, action, domain, userID string, houses []auth.HouseRoles, detail models.JSONMap) {
	if s.Store == nil {
		return
	}
	if len(houses) == 0 {
		s.recordOne(ctx, &models.AuditEntry{
			Service: "auth", Action: action, ActorDomain: domain, ActorUserID: userID,
			Outcome: models.AuditOutcomeOK, Detail: detail,
		})
		return
	}
	for i := range houses {
		houseID := houses[i].House
		memberID := houses[i].Member
		s.recordOne(ctx, &models.AuditEntry{
			Service: "auth", Action: action,
			HouseID: &houseID, ActorMemberID: &memberID,
			ActorDomain: domain, ActorUserID: userID,
			Outcome: models.AuditOutcomeOK, Detail: detail,
		})
	}
}

// recordSecurityFailure writes a single global-scope (house_id NULL) entry for
// an unattributable auth failure — there is no resolved member/house to charge
// it to, so it lives in the global security scope.
func (s *AuthService) recordSecurityFailure(ctx context.Context, domain, userID, reason string) {
	if s.Store == nil {
		return
	}
	s.recordOne(ctx, &models.AuditEntry{
		Service: "auth", Action: models.AuditActionLoginFailed,
		ActorDomain: domain, ActorUserID: userID,
		Outcome: models.AuditOutcomeDenied,
		Detail:  models.JSONMap{"reason": reason},
	})
}

func (s *AuthService) recordOne(ctx context.Context, e *models.AuditEntry) {
	if err := s.Store.RecordAuditEntry(ctx, e); err != nil {
		log.WithError(err).WithField("action", e.Action).Warn("audit: security event record failed")
	}
}

func (s *AuthService) login(ctx context.Context, body []byte) (any, error) {
	if s.PKI == nil || s.JWTSecret == nil {
		return nil, csilrpc.Internal("auth not configured")
	}
	var req csil.LoginRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if req.SignedAssertion == "" {
		return nil, csilrpc.BadRequest("signed_assertion is required")
	}
	assertion, err := s.PKI.VerifyAssertion(req.SignedAssertion, s.IDPDomain)
	if err != nil {
		log.WithError(err).Info("auth.Login: assertion verification failed")
		s.recordSecurityFailure(ctx, "", "", "login: assertion verification failed")
		return nil, csilrpc.Unauthorized("assertion verification failed")
	}
	claims := s.fetchClaims(req.SignedAssertion, assertion)
	display := resolveDisplayName(claims, assertion)
	return s.issueToken(ctx, assertion.Domain, assertion.UserID, display, claims, models.AuditActionLogin)
}

func (s *AuthService) complete(ctx context.Context, body []byte) (any, error) {
	if s.PKI == nil || s.JWTSecret == nil {
		return nil, csilrpc.Internal("auth not configured")
	}
	var req csil.CompleteRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if req.EncryptedToken == "" {
		return nil, csilrpc.BadRequest("encrypted_token is required")
	}
	signed, err := s.PKI.DecryptToken(req.EncryptedToken)
	if err != nil {
		log.WithError(err).Error("auth.Complete: RP decrypt-token failed")
		s.recordSecurityFailure(ctx, "", "", "complete: token decrypt failed")
		return nil, csilrpc.NewError(502, "could not decrypt token")
	}
	assertion, err := s.PKI.VerifyAssertion(signed, s.IDPDomain)
	if err != nil {
		log.WithError(err).Info("auth.Complete: assertion verification failed")
		s.recordSecurityFailure(ctx, "", "", "complete: assertion verification failed")
		return nil, csilrpc.Unauthorized("assertion verification failed")
	}
	if err := auth.VerifyNonce(s.JWTSecret, assertion.Nonce); err != nil {
		log.WithError(err).Info("auth.Complete: nonce rejected")
		s.recordSecurityFailure(ctx, assertion.Domain, assertion.UserID, "complete: nonce invalid or expired")
		return nil, csilrpc.Unauthorized("login nonce invalid or expired")
	}
	if assertion.Domain != s.IDPDomain {
		s.recordSecurityFailure(ctx, assertion.Domain, assertion.UserID, "complete: assertion from unexpected domain")
		return nil, csilrpc.Unauthorized("assertion from unexpected domain")
	}
	// The audience binds the assertion to our RP domain (NOT the callback
	// URL — linkkeys changed that contract). Tolerate an empty audience for
	// back-compat with older IDPs that don't set it.
	if assertion.Audience != "" && s.RPDomain != "" && assertion.Audience != s.RPDomain {
		s.recordSecurityFailure(ctx, assertion.Domain, assertion.UserID, "complete: assertion audience mismatch")
		return nil, csilrpc.Unauthorized("assertion audience mismatch")
	}
	claims := s.fetchClaims(signed, assertion)
	display := resolveDisplayName(claims, assertion)
	return s.issueToken(ctx, assertion.Domain, assertion.UserID, display, claims, models.AuditActionLogin)
}

func (s *AuthService) refresh(ctx context.Context, _ []byte) (any, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	// Refresh re-mints from the bearer without re-contacting linkkeys, so there
	// are no fresh claims to reconcile — pass nil and keep the cached identity.
	return s.issueToken(ctx, id.Domain, id.UserID, id.DisplayName, nil, models.AuditActionRefresh)
}

func (s *AuthService) me(ctx context.Context, _ []byte) (any, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	houses := make([]csil.HouseSummary, 0, len(id.Houses))
	for _, h := range id.Houses {
		name := ""
		if s.Store != nil {
			if house, err := s.Store.GetHouseByID(ctx, h.House); err == nil && house != nil {
				name = house.Name
			}
		}
		roles := make([]string, len(h.Roles))
		copy(roles, h.Roles)
		houses = append(houses, csil.HouseSummary{
			HouseId:  csil.HouseID(h.House),
			Name:     name,
			MemberId: csil.MemberID(h.Member),
			Roles:    roles,
		})
	}
	return csil.MeResponse{
		Domain:      id.Domain,
		UserId:      id.UserID,
		DisplayName: strPtrCopy(id.DisplayName),
		ExpiresAt:   csil.Timestamp(time.Unix(id.ExpiresAt, 0).UTC().Format(time.RFC3339)),
		Houses:      houses,
	}, nil
}

// issueToken snapshots the identity's houses+roles and mints a fresh JWT.
// Shared by Login / Complete / Refresh / DevAuthService.DevLogin so every
// path produces an identical token shape. claims is the linkkeys claim set for
// this redemption (nil on refresh/dev-login); it seeds/reconciles the
// claim-backed member fields and is otherwise ignored.
func (s *AuthService) issueToken(ctx context.Context, domain, userID, displayName string, claims map[string]string, action string) (csil.LoginResponse, error) {
	// Auto-provision: if this verified identity's domain is in any house's
	// trusted_domains table and they aren't already a member there, insert
	// a member row + grant the canonical "member" role before we snapshot
	// houses for the token. Failures here are logged but don't block the
	// login — worst case the user gets the same empty-houses token they
	// would have had before.
	if err := autoProvisionFromTrustedDomains(ctx, s.Store, domain, userID, displayName); err != nil {
		log.WithError(err).Warn("auth: auto-provision from trusted domains failed (continuing)")
	}
	// Reconcile claim-backed member fields AFTER auto-provision (so freshly
	// created rows get seeded too) and before we snapshot. Best-effort: a
	// failure never blocks login.
	if err := reconcileMemberClaims(ctx, s.Store, domain, userID, claims); err != nil {
		log.WithError(err).Warn("auth: reconcile member claims failed (continuing)")
	}
	houses, err := buildHouseRoles(ctx, s.Store, domain, userID)
	if err != nil {
		log.WithError(err).Error("auth: enrich houses failed")
		return csil.LoginResponse{}, csilrpc.Internal("internal error")
	}
	tok, err := auth.Mint(s.JWTSecret, auth.Identity{
		Domain:      domain,
		UserID:      userID,
		DisplayName: displayName,
		Houses:      houses,
	}, 0)
	if err != nil {
		log.WithError(err).Error("auth: jwt mint failed")
		return csil.LoginResponse{}, csilrpc.Internal("internal error")
	}
	verified, _ := auth.Verify(s.JWTSecret, tok) // safe: just minted
	// Security audit: one entry per house the identity belongs to, so each
	// house admin sees the login/refresh of their members. An identity with no
	// houses still records a single global-scope entry.
	s.recordSecurityEvent(ctx, action, domain, userID, houses, nil)
	return csil.LoginResponse{
		Token:       tok,
		Domain:      domain,
		UserId:      userID,
		DisplayName: strPtrCopy(displayName),
		ExpiresAt:   csil.Timestamp(time.Unix(verified.ExpiresAt, 0).UTC().Format(time.RFC3339)),
	}, nil
}

// reconcileMemberClaims seeds/updates the claim-backed fields on every member
// row for this identity (an identity is a member in zero or more houses, each
// its own row). For each field: while the stored value still equals its mirror
// the user hasn't overridden it, so we track the released claim; once the user
// has diverged we leave their value but keep the mirror current. A claim that
// wasn't released leaves its columns untouched. nil/empty claims is a no-op, so
// refresh and dev-login (which carry no fresh claims) never disturb the fields.
func reconcileMemberClaims(ctx context.Context, st store.Store, domain, userID string, claims map[string]string) error {
	if st == nil || len(claims) == 0 {
		return nil
	}
	members, err := st.FindMembersByLinkkeysIdentity(ctx, domain, userID)
	if err != nil {
		return err
	}
	for i := range members {
		m := &members[i]
		changed := reconcileField(&m.DisplayName, &m.DisplayNameClaimed, claims, "display_name")
		changed = reconcileField(&m.Email, &m.EmailClaimed, claims, "email") || changed
		changed = reconcileField(&m.AvatarURL, &m.AvatarURLClaimed, claims, "avatar_url") || changed
		if !changed {
			continue
		}
		if err := st.UpdateMember(ctx, m); err != nil {
			// One bad row shouldn't abort the others or the login.
			log.WithError(err).WithField("member_id", m.MemberID).
				Warn("auth: reconcile member claims: update failed; skipping row")
		}
	}
	return nil
}

// reconcileField applies one claim to a (field, mirror) pair and reports whether
// either changed. Tracks upstream while field == mirror (or field is empty);
// preserves a user override (field != mirror) but always advances the mirror to
// the latest released value. Returns false when the claim wasn't released.
func reconcileField(field, mirror *string, claims map[string]string, claimType string) bool {
	val, ok := claims[claimType]
	if !ok {
		return false
	}
	changed := false
	if (*field == "" || *field == *mirror) && *field != val {
		*field = val
		changed = true
	}
	if *mirror != val {
		*mirror = val
		changed = true
	}
	return changed
}

// buildHouseRoles snapshots every house the identity belongs to. Used by
// issueToken to bake roles into the bearer instead of looking them up per
// request. Identical to the helper that used to live in handlers/auth.go.
func buildHouseRoles(ctx context.Context, st store.Store, domain, userID string) ([]auth.HouseRoles, error) {
	members, err := st.FindMembersByLinkkeysIdentity(ctx, domain, userID)
	if err != nil {
		return nil, err
	}
	out := make([]auth.HouseRoles, 0, len(members))
	for _, m := range members {
		// A deactivated membership grants no house roles — the person was
		// denied access to that house, so their token simply won't carry it.
		// Reactivating the member (admin action) restores it on next login.
		if m.DeactivatedAt != nil {
			continue
		}
		roles, err := st.ListRolesForMember(ctx, m.MemberID)
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

// autoProvisionFromTrustedDomains creates a members row + member-role
// grant for the verified identity in every house whose trusted_domains
// table contains their domain — provided they aren't already a member
// there. Existing memberships are left alone. Errors on individual
// houses are logged and skipped so one misconfigured row can't lock a
// user out of every house they should land in.
func autoProvisionFromTrustedDomains(ctx context.Context, st store.Store, domain, userID, displayName string) error {
	if st == nil || domain == "" || userID == "" {
		return nil
	}
	trusting, err := st.HousesTrustingDomain(ctx, domain)
	if err != nil {
		return err
	}
	if len(trusting) == 0 {
		return nil
	}
	existing, err := st.FindMembersByLinkkeysIdentity(ctx, domain, userID)
	if err != nil {
		return err
	}
	// Any existing membership (active OR deactivated) means "leave alone" — we
	// never recreate a duplicate, and crucially we do NOT reactivate a
	// deactivated member here: deactivation is an intentional denial that
	// auto-provision must not silently undo. A deactivated member just gets no
	// roles (buildHouseRoles skips them) until an admin reactivates them.
	already := make(map[string]bool, len(existing))
	for _, m := range existing {
		already[m.HouseID] = true
	}
	for _, h := range trusting {
		if already[h.HouseID] {
			continue
		}
		m := &models.Member{
			HouseID:        h.HouseID,
			LinkkeysDomain: domain,
			LinkkeysUserID: userID,
			DisplayName:    displayName,
		}
		if err := st.CreateMember(ctx, m); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"house_id": h.HouseID,
				"domain":   domain,
				"user_id":  userID,
			}).Warn("auto-provision: create member failed; skipping house")
			continue
		}
		// Grant the canonical "member" role so the freshly-minted bearer
		// can list resources in this house immediately. Absent member role
		// (e.g. a malformed house) just means no role assignment — the
		// member row still exists so admins can fix it from the SPA.
		if role, rerr := st.GetRoleByName(ctx, h.HouseID, models.RoleMember); rerr == nil && role != nil {
			_ = st.AssignRole(ctx, m.MemberID, role.RoleID)
		}
		// Append-only audit so house admins can see auto-joins in the
		// member log without a separate event stream.
		_ = st.RecordMemberAudit(ctx, &models.MemberAudit{
			HouseID:         h.HouseID,
			SubjectMemberID: m.MemberID,
			Action:          models.AuditActionMemberAutoCreated,
			Detail: models.JSONMap{
				"via":             "trusted_domain",
				"linkkeys_domain": domain,
			},
		})
	}
	return nil
}

// loadMemberDisplayName is used by DevAuthService to enrich the picker
// listing with display names; reaches the store, never panics. Returns the
// member's display_name or its linkkeys_user_id as a fallback.
func loadMemberDisplayName(m *models.Member) string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	return m.LinkkeysUserID
}
