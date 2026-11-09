package csilservices

import (
	"context"
	"slices"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/actas"
	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func (s *AuthService) GetActAsScopeSet(ctx context.Context, req csil.ActAsScopeSetRequest) (csil.ActAsScopeSetResponse, error) {
	if s.ActAs == nil {
		return csil.ActAsScopeSetResponse{}, csilrpc.Forbidden("LinkKeys delegation is not enabled")
	}
	scopes, err := actas.OfferedScopes(req.Scopes)
	if err != nil || len(req.Scopes) == 0 {
		return csil.ActAsScopeSetResponse{}, csilrpc.BadRequest("select valid, distinct scopes")
	}
	value := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	raw, err := s.ActAs.SignScopeSet(ctx, actas.ScopeSetRequest{
		GranteeDomain: value(req.GranteeDomain), GranteeUserID: value(req.GranteeUserId),
		GranteeApplicationID: value(req.GranteeApplicationId), LocalRPFingerprint: value(req.LocalRpFingerprint), Scopes: scopes,
	})
	if err != nil {
		return csil.ActAsScopeSetResponse{}, csilrpc.BadRequest("cannot sign scope set: " + err.Error())
	}
	return csil.ActAsScopeSetResponse{SignedScopeSet: raw}, nil
}

func (s *AuthService) ExchangeActAsCredential(ctx context.Context, req csil.ExchangeActAsCredentialRequest) (csil.ActAsTokenResponse, error) {
	if s.ActAs == nil || s.Store == nil || len(s.JWTSecret) == 0 {
		return csil.ActAsTokenResponse{}, csilrpc.Forbidden("LinkKeys delegation is not enabled")
	}
	verified, err := s.ActAs.VerifyCredential(ctx, req.Credential, nil)
	if err != nil {
		return csil.ActAsTokenResponse{}, csilrpc.Unauthorized("invalid or unavailable LinkKeys grant")
	}
	ttl := min(5*time.Minute, s.bearerTTL(), time.Until(verified.ExpiresAt))
	scopes := actas.MapScopes(verified.Scopes)
	if ttl < time.Second || len(scopes) == 0 || verified.Nonce == "" || verified.GrantID == "" || verified.Grantee == "" || verified.SubjectDomain == "" || verified.SubjectUserID == "" {
		return csil.ActAsTokenResponse{}, csilrpc.Unauthorized("grant has no usable authority")
	}
	// Do not provision memberships or update identity claims from a delegation.
	houses, err := buildHouseRoles(ctx, s.Store, verified.SubjectDomain, verified.SubjectUserID)
	if err != nil {
		return csil.ActAsTokenResponse{}, csilrpc.Internal("cannot load memberships")
	}
	used, err := s.Store.ConsumeActAsNonce(ctx, verified.SubjectDomain, verified.GrantID, verified.Nonce, time.Now().UTC().Add(s.ActAs.MaxPresentationAge()))
	if err != nil {
		return csil.ActAsTokenResponse{}, csilrpc.Internal("cannot record presentation")
	}
	if !used {
		return csil.ActAsTokenResponse{}, csilrpc.Unauthorized("presentation was already used")
	}
	id := auth.Identity{Domain: verified.SubjectDomain, UserID: verified.SubjectUserID, Houses: houses,
		Delegation: &auth.Delegation{GrantExpiresAt: verified.ExpiresAt.Unix(), GrantID: verified.GrantID, Grantee: verified.Grantee, Scopes: scopes}}
	token, err := auth.Mint(s.JWTSecret, id, ttl)
	if err != nil {
		return csil.ActAsTokenResponse{}, csilrpc.Internal("cannot issue delegated token")
	}
	minted, err := auth.Verify(s.JWTSecret, token)
	if err != nil {
		return csil.ActAsTokenResponse{}, csilrpc.Internal("cannot issue delegated token")
	}
	s.recordSecurityEvent(ctx, models.AuditActionLogin, id.Domain, id.UserID, houses, map[string]any{"act_as_grant_id": verified.GrantID, "act_as_grantee": verified.Grantee, "act_as_scopes": scopes})
	return csil.ActAsTokenResponse{Token: token, Domain: id.Domain, UserId: id.UserID,
		ExpiresAt: csil.Timestamp(time.Unix(minted.ExpiresAt, 0).UTC().Format(time.RFC3339)), GrantId: verified.GrantID, Grantee: verified.Grantee, Scopes: scopes}, nil
}

// VerifyDelegation checks revocation and replaces the bearer membership snapshot.
func (s *AuthService) VerifyDelegation(ctx context.Context, id *auth.Identity) error {
	if id.Delegation == nil {
		return nil
	}
	if s.ActAs == nil || s.Store == nil || id.Delegation.GrantID == "" || id.Delegation.Grantee == "" {
		return csilrpc.Unauthorized("LinkKeys delegation is not available")
	}
	revoked, err := s.ActAs.RevokedGrants(ctx, id.Domain, []string{id.Delegation.GrantID})
	if err != nil || slices.Contains(revoked, id.Delegation.GrantID) {
		return csilrpc.Unauthorized("LinkKeys grant is revoked or unavailable")
	}
	houses, err := buildHouseRoles(ctx, s.Store, id.Domain, id.UserID)
	if err != nil {
		return csilrpc.Unauthorized("cannot verify current memberships")
	}
	id.Houses = houses
	return nil
}
