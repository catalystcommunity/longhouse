// Package actas makes longhouse a LinkKeys act-as AUDIENCE with the regular-RP
// Go SDK. It signs longhouse's scope sets and verifies the credentials that
// grantee applications present to AuthService.exchange-act-as-credential.
//
// The services see only the Provider interface, so they are testable with a
// fake. Verifier is the SDK-backed Provider; its key material comes through
// the KeySource interface, so the SDK conformance vectors can drive it
// without a network.
package actas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	api "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
	"github.com/catalystcommunity/longhouse/api/internal/auth"
)

// RequestDigestInput is the text whose SHA-256 is the presentation request
// digest of exchange-act-as-credential (csil/longhouse.csil).
const RequestDigestInput = "longhouse/AuthService/exchange-act-as-credential"

// RequestDigest returns SHA-256(RequestDigestInput).
func RequestDigest() []byte {
	sum := sha256.Sum256([]byte(RequestDigestInput))
	return sum[:]
}

// Verified is an accepted credential, reduced to what longhouse needs.
type Verified struct {
	GrantID string
	// SubjectUserID is the LinkKeys account id of the user at SubjectDomain.
	SubjectUserID string
	SubjectDomain string
	// Application names the grantee, for the audit log:
	// "<application_id>@<domain>" or "local-rp:<fingerprint>".
	Application string
	// Grantee names the grantee exactly: "<user_id>@<domain>/<application_id>"
	// or "local-rp:<fingerprint>".
	Grantee string
	// Scopes are the approved scopes that are longhouse Scope words. Unknown
	// words are dropped.
	Scopes    []string
	ExpiresAt time.Time
	// Nonce is the presentation nonce (hex). The caller records it.
	Nonce string
}

// ScopeSetRequest names the grantee application that asks for a scope set.
type ScopeSetRequest struct {
	GranteeDomain        string
	GranteeUserID        string
	GranteeApplicationID string
	LocalRPFingerprint   string
	// Scopes are the scopes to offer. nil offers every scope. The set
	// lists them in auth.DelegationScopeOrder.
	Scopes []string
}

// Provider is what AuthService needs from act-as.
type Provider interface {
	// SignScopeSet returns CBOR(SignedActAsScopeSet).
	SignScopeSet(ctx context.Context, req ScopeSetRequest) ([]byte, error)
	// VerifyCredential checks CBOR(ActAsCredential). knownRevoked holds
	// grant ids the caller already knows are revoked.
	VerifyCredential(ctx context.Context, credential []byte, knownRevoked []string) (Verified, error)
	// RevokedGrants returns the ids among grantIDs that subjectDomain
	// revoked.
	RevokedGrants(ctx context.Context, subjectDomain string, grantIDs []string) ([]string, error)
	// MaxPresentationAge is how long a presentation nonce must be held.
	MaxPresentationAge() time.Duration
}

// KeySource supplies verified key material.
type KeySource interface {
	// OwnScopeSetKeys returns ALL attested keys of longhouse's own instance,
	// including expired and revoked ones (AttestedKeyRefs).
	OwnScopeSetKeys(ctx context.Context, own api.ApplicationRef, instanceID string) ([]regularrp.ApplicationKeyRef, error)
	// GranteeInstanceKeys returns the usable keys of a grantee instance.
	GranteeInstanceKeys(ctx context.Context, ref regularrp.InstanceRef) ([]regularrp.ApplicationKeyRef, error)
	// DomainKeys returns the signing keys of a home domain, with its key
	// revocations applied.
	DomainKeys(ctx context.Context, domain string) ([]api.DomainPublicKey, error)
	// RevokedGrants returns the ids of grants with a verified revocation.
	RevokedGrants(ctx context.Context, domain string, grantIDs []string) ([]string, error)
}

// Verifier is the SDK-backed Provider.
type Verifier struct {
	Own        api.ApplicationRef
	InstanceID string
	Signers    []regularrp.ApplicationSigner
	Keys       KeySource
	// Digest is the expected presentation request digest. Default:
	// RequestDigest().
	Digest      []byte
	MaxAge      time.Duration
	SkewSeconds int64
	// ScopeSetTTL is the life of a signed scope set. Default 24 hours.
	ScopeSetTTL time.Duration
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

var _ Provider = (*Verifier)(nil)

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// MaxPresentationAge implements Provider.
func (v *Verifier) MaxPresentationAge() time.Duration {
	return v.maxAge() + 2*time.Duration(v.skew())*time.Second
}
func (v *Verifier) maxAge() time.Duration {
	if v.MaxAge > 0 {
		return v.MaxAge
	}
	return 5 * time.Minute
}
func (v *Verifier) skew() int64 {
	if v.SkewSeconds > 0 {
		return v.SkewSeconds
	}
	return 60
}

// SignScopeSet implements Provider. It offers req.Scopes, or every
// longhouse scope when req.Scopes is nil.
func (v *Verifier) SignScopeSet(_ context.Context, req ScopeSetRequest) ([]byte, error) {
	grantee := api.GranteeRef{}
	if req.LocalRPFingerprint != "" {
		if req.GranteeDomain != "" || req.GranteeUserID != "" || req.GranteeApplicationID != "" {
			return nil, errors.New("choose one grantee type")
		}
		fingerprint, err := hex.DecodeString(req.LocalRPFingerprint)
		if err != nil || len(fingerprint) != 32 {
			return nil, errors.New("local RP fingerprint must be 32 bytes of hex")
		}
		grantee.LocalRpDescriptorFingerprint = &req.LocalRPFingerprint
	} else {
		if req.GranteeDomain == "" || req.GranteeUserID == "" || req.GranteeApplicationID == "" {
			return nil, errors.New("grantee domain, user ID and application ID are required")
		}
		grantee.Application = &api.ApplicationRef{SubjectUserId: req.GranteeUserID, SubjectDomain: strings.ToLower(req.GranteeDomain), ApplicationId: req.GranteeApplicationID}
	}
	now := v.now().UTC()
	ttl := v.ScopeSetTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	offer, err := OfferedScopes(req.Scopes)
	if err != nil {
		return nil, err
	}
	entries := make([]api.ActAsScopeEntry, 0, len(offer))
	for _, s := range offer {
		d := auth.DelegationScopes[s]
		entries = append(entries, api.ActAsScopeEntry{Scope: s, Description: &d})
	}
	lang := "en"
	set := api.ActAsScopeSet{
		Audience:  v.Own,
		Grantee:   grantee,
		Entries:   entries,
		Language:  &lang,
		IssuedAt:  regularrp.FormatActAsTime(now),
		ExpiresAt: regularrp.FormatActAsTime(now.Add(ttl)),
	}
	signed, err := regularrp.SignScopeSet(set, v.InstanceID, v.Signers)
	if err != nil {
		return nil, err
	}
	return api.EncodeSignedActAsScopeSet(signed), nil
}

// VerifyCredential implements Provider: the SDK's audience checklist plus
// key resolution and a live revocation lookup.
func (v *Verifier) VerifyCredential(ctx context.Context, raw []byte, knownRevoked []string) (Verified, error) {
	cred, err := api.DecodeActAsCredential(raw)
	if err != nil {
		return Verified{}, fmt.Errorf("credential does not decode: %w", err)
	}
	grant, err := regularrp.DecodeGrant(cred.Grant)
	if err != nil {
		return Verified{}, err
	}
	signerInstance, err := regularrp.CredentialScopeSetSigner(cred)
	if err != nil {
		return Verified{}, err
	}
	ownKeys, err := v.Keys.OwnScopeSetKeys(ctx, v.Own, signerInstance)
	if err != nil {
		return Verified{}, fmt.Errorf("resolve own scope-set keys: %w", err)
	}
	var granteeKeys []regularrp.ApplicationKeyRef
	if inst, ok := regularrp.CredentialSignerInstance(cred); ok && grant.Grantee.Application != nil {
		app := grant.Grantee.Application
		granteeKeys, err = v.Keys.GranteeInstanceKeys(ctx, regularrp.InstanceRef{
			SubjectUserID: app.SubjectUserId,
			SubjectDomain: app.SubjectDomain,
			ApplicationID: app.ApplicationId,
			InstanceID:    inst,
		})
		if err != nil {
			return Verified{}, fmt.Errorf("resolve grantee keys: %w", err)
		}
	}
	domainKeys, err := v.Keys.DomainKeys(ctx, grant.SubjectDomain)
	if err != nil {
		return Verified{}, fmt.Errorf("resolve home domain keys: %w", err)
	}
	revoked := append([]string{}, knownRevoked...)
	live, err := v.Keys.RevokedGrants(ctx, grant.SubjectDomain, []string{grant.GrantId})
	if err != nil {
		return Verified{}, fmt.Errorf("resolve grant revocations: %w", err)
	}
	revoked = append(revoked, live...)
	digest := v.Digest
	if digest == nil {
		digest = RequestDigest()
	}
	skew := v.skew()
	verified, err := regularrp.VerifyCredential(cred, regularrp.AudienceContext{
		OwnApplication:            v.Own,
		OwnScopeSetKeys:           ownKeys,
		IssuerDomainKeys:          domainKeys,
		GranteeInstanceKeys:       granteeKeys,
		ExpectedRequestDigest:     digest,
		RevokedGrantIDs:           revoked,
		MaxPresentationAgeSeconds: int64(v.maxAge() / time.Second),
		Now:                       v.now(),
		SkewSeconds:               skew,
		RevokedKeyPolicy:          regularrp.AcceptBeforeRevocation,
	})
	if err != nil {
		return Verified{}, err
	}
	expires, err := time.Parse(time.RFC3339, verified.ExpiresAt)
	if err != nil {
		return Verified{}, fmt.Errorf("grant expires_at: %w", err)
	}
	return Verified{
		GrantID:       verified.GrantID,
		SubjectUserID: grant.UserId,
		SubjectDomain: strings.ToLower(grant.SubjectDomain),
		Application:   GranteeName(grant.Grantee),
		Grantee:       GranteeKey(grant.Grantee),
		Scopes:        MapScopes(verified.ApprovedScope),
		ExpiresAt:     expires.UTC(),
		Nonce:         hex.EncodeToString(verified.Nonce),
	}, nil
}

// RevokedGrants implements Provider.
func (v *Verifier) RevokedGrants(ctx context.Context, domain string, grantIDs []string) ([]string, error) {
	return v.Keys.RevokedGrants(ctx, domain, grantIDs)
}

// GranteeName is the audit name of a grantee.
// GranteeKey names a grantee with every field of its ApplicationRef, in
// the LONGHOUSE_ALLOWED_APPLICATIONS form.
func GranteeKey(g api.GranteeRef) string {
	if a := g.Application; a != nil {
		return a.SubjectUserId + "@" + strings.ToLower(a.SubjectDomain) + "/" + a.ApplicationId
	}
	return GranteeName(g)
}

func GranteeName(g api.GranteeRef) string {
	if g.Application != nil {
		return g.Application.ApplicationId + "@" + strings.ToLower(g.Application.SubjectDomain)
	}
	if g.LocalRpDescriptorFingerprint != nil {
		return "local-rp:" + *g.LocalRpDescriptorFingerprint
	}
	return "unknown"
}

// OfferedScopes checks a requested scope list and returns it in
// auth.DelegationScopeOrder. nil means every scope. An empty list, an unknown word, or a
// repeated word is an error.
func OfferedScopes(requested []string) ([]string, error) {
	if requested == nil {
		return auth.DelegationScopeOrder, nil
	}
	if len(requested) == 0 {
		return nil, errors.New("scopes must not be empty")
	}
	want := map[string]bool{}
	for _, s := range requested {
		if _, ok := auth.DelegationScopes[s]; !ok {
			return nil, fmt.Errorf("unknown scope %q", s)
		}
		if want[s] {
			return nil, fmt.Errorf("scope %q is repeated", s)
		}
		want[s] = true
	}
	out := make([]string, 0, len(want))
	for _, s := range auth.DelegationScopeOrder {
		if want[s] {
			out = append(out, s)
		}
	}
	return out, nil
}

// MapScopes keeps the approved scopes that are longhouse Scope words, in the
// scope order, without duplicates.
func MapScopes(approved []string) []string {
	want := map[string]bool{}
	for _, s := range approved {
		want[s] = true
	}
	var out []string
	for _, s := range auth.DelegationScopeOrder {
		if want[s] {
			out = append(out, s)
		}
	}
	return out
}

// RPKeySource is the production KeySource: longhouse's own LinkKeys RP over
// the SDK's pinned transport, with the SDK's CachedResolver for
// application keys.
type RPKeySource struct {
	Transport api.Transport
	Resolver  *regularrp.CachedResolver
}

// NewRPKeySource builds an RPKeySource over a transport.
func NewRPKeySource(t api.Transport) *RPKeySource {
	return &RPKeySource{Transport: t, Resolver: regularrp.NewCachedResolver(t, regularrp.CachedResolverOptions{})}
}

// OwnScopeSetKeys implements KeySource with AttestedKeyRefs (NOT
// UsableKeyRefs: a scope set signed before a key rotation must still
// verify).
func (r *RPKeySource) OwnScopeSetKeys(ctx context.Context, own api.ApplicationRef, instanceID string) ([]regularrp.ApplicationKeyRef, error) {
	res, err := r.Resolver.Resolve(ctx, regularrp.InstanceRef{
		SubjectUserID: own.SubjectUserId,
		SubjectDomain: own.SubjectDomain,
		ApplicationID: own.ApplicationId,
		InstanceID:    instanceID,
	}, nil)
	if err != nil {
		return nil, err
	}
	if res.Freshness == regularrp.FreshnessStale || res.RPCacheStatus == "stale" {
		return nil, errors.New("application keys are stale")
	}
	return res.Keys.AttestedKeyRefs(), nil
}

// GranteeInstanceKeys implements KeySource with UsableKeyRefs.
func (r *RPKeySource) GranteeInstanceKeys(ctx context.Context, ref regularrp.InstanceRef) ([]regularrp.ApplicationKeyRef, error) {
	res, err := r.Resolver.Resolve(ctx, ref, nil)
	if err != nil {
		return nil, err
	}
	if res.Freshness == regularrp.FreshnessStale || res.RPCacheStatus == "stale" {
		return nil, errors.New("application keys are stale")
	}
	return res.Keys.UsableKeyRefs(), nil
}

// DomainKeys implements KeySource through Rp/resolve-domain-keys. A key
// named by any revocation certificate in the response is dropped. That
// can only refuse more, never accept more, so it is safe without checking
// the certificate signatures here.
func (r *RPKeySource) DomainKeys(ctx context.Context, domain string) ([]api.DomainPublicKey, error) {
	resp, err := api.NewRpClient(r.Transport).ResolveDomainKeys(ctx, api.RpResolveDomainKeysRequest{Domain: domain})
	if err != nil {
		return nil, err
	}
	if resp.CacheStatus == "stale" {
		return nil, errors.New("domain keys are stale")
	}
	if !equalASCIIDomain(resp.Domain, domain) {
		return nil, errors.New("RP domain-key response names another domain")
	}
	revoked := map[string]bool{}
	for _, c := range resp.Revocations {
		revoked[c.TargetKeyId] = true
	}
	var out []api.DomainPublicKey
	for _, k := range resp.Keys {
		if !revoked[k.KeyId] {
			out = append(out, k)
		}
	}
	return out, nil
}

// RevokedGrants refuses invalid revocation evidence. A failed check cannot
// turn a returned revocation into permission to use the grant.
func (r *RPKeySource) RevokedGrants(ctx context.Context, domain string, grantIDs []string) ([]string, error) {
	var out []string
	for start := 0; start < len(grantIDs); start += regularrp.MaxRevocationLookupIDs {
		chunk := grantIDs[start:min(start+regularrp.MaxRevocationLookupIDs, len(grantIDs))]
		response, err := api.NewRpClient(r.Transport).ResolveActAsRevocations(ctx, api.RpResolveActAsRevocationsRequest{SubjectDomain: domain, GrantIds: chunk})
		if err != nil {
			return nil, err
		}
		if len(response.Revocations) == 0 {
			continue
		}
		keys, err := r.DomainKeys(ctx, domain)
		if err != nil {
			return nil, err
		}
		for _, signed := range response.Revocations {
			revocation, err := regularrp.VerifyGrantRevocation(signed, keys, domain)
			if err != nil {
				return nil, fmt.Errorf("invalid grant revocation: %w", err)
			}
			if !slices.Contains(chunk, revocation.GrantId) {
				return nil, errors.New("revocation names an unrequested grant")
			}
			out = append(out, revocation.GrantId)
		}
	}
	return out, nil
}

func equalASCIIDomain(a, b string) bool {
	for _, s := range []string{a, b} {
		for i := 0; i < len(s); i++ {
			if s[i] > 127 {
				return false
			}
		}
	}
	return strings.ToLower(a) == strings.ToLower(b)
}
