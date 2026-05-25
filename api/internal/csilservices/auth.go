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

func (s *AuthService) Register(d *csilrpc.Dispatcher) {
	d.RegisterPublic("auth", "Login", s.login)
	d.RegisterPublic("auth", "Complete", s.complete)
	d.Register("auth", "Refresh", s.refresh)
	d.Register("auth", "Me", s.me)
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
		return nil, csilrpc.Unauthorized("assertion verification failed")
	}
	return s.issueToken(ctx, assertion.Domain, assertion.UserID, assertion.DisplayName)
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
		return nil, csilrpc.NewError(502, "could not decrypt token")
	}
	assertion, err := s.PKI.VerifyAssertion(signed, s.IDPDomain)
	if err != nil {
		log.WithError(err).Info("auth.Complete: assertion verification failed")
		return nil, csilrpc.Unauthorized("assertion verification failed")
	}
	if err := auth.VerifyNonce(s.JWTSecret, assertion.Nonce); err != nil {
		log.WithError(err).Info("auth.Complete: nonce rejected")
		return nil, csilrpc.Unauthorized("login nonce invalid or expired")
	}
	if assertion.Domain != s.IDPDomain {
		return nil, csilrpc.Unauthorized("assertion from unexpected domain")
	}
	if assertion.Audience != "" && s.CallbackURL != "" && assertion.Audience != s.CallbackURL {
		return nil, csilrpc.Unauthorized("assertion audience mismatch")
	}
	return s.issueToken(ctx, assertion.Domain, assertion.UserID, assertion.DisplayName)
}

func (s *AuthService) refresh(ctx context.Context, _ []byte) (any, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return s.issueToken(ctx, id.Domain, id.UserID, id.DisplayName)
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
// path produces an identical token shape.
func (s *AuthService) issueToken(ctx context.Context, domain, userID, displayName string) (csil.LoginResponse, error) {
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
	return csil.LoginResponse{
		Token:       tok,
		Domain:      domain,
		UserId:      userID,
		DisplayName: strPtrCopy(displayName),
		ExpiresAt:   csil.Timestamp(time.Unix(verified.ExpiresAt, 0).UTC().Format(time.RFC3339)),
	}, nil
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

// loadMemberDisplayName is used by DevAuthService to enrich the picker
// listing with display names; reaches the store, never panics. Returns the
// member's display_name or its linkkeys_user_id as a fallback.
func loadMemberDisplayName(m *models.Member) string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	return m.LinkkeysUserID
}
