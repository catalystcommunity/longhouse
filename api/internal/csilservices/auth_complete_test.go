package csilservices

import (
	"context"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"github.com/fxamacker/cbor/v2"
)

// fakeAuthStore satisfies just the two read methods issueToken touches on the
// happy path (no trusting domains, no existing members). The rest of the Store
// surface is the embedded nil interface and panics if called.
type fakeAuthStore struct {
	store.Store
}

func (fakeAuthStore) HousesTrustingDomain(context.Context, string) ([]models.House, error) {
	return nil, nil
}

func (fakeAuthStore) FindMembersByLinkkeysIdentity(context.Context, string, string) ([]models.Member, error) {
	return nil, nil
}

// issueToken now records a security audit event; the double accepts it as a
// no-op so the auth-flow tests don't need a real audit sink.
func (fakeAuthStore) RecordAuditEntry(context.Context, *models.AuditEntry) error { return nil }

// fakeCompletePKI returns a fixed assertion from VerifyAssertion so we can
// drive complete()'s audience check directly.
type fakeCompletePKI struct {
	assertion *linkkeys.Assertion
}

func (fakeCompletePKI) SignRequest(string, string) (string, error)  { return "SIGNED", nil }
func (fakeCompletePKI) DecryptToken(string) (string, error)         { return "ASSERT", nil }
func (p fakeCompletePKI) VerifyAssertion(string, string) (*linkkeys.Assertion, error) {
	return p.assertion, nil
}

const (
	testRPDomain  = "todandlorna.com"
	testIDPDomain = "todandlorna.com"
)

func completeSvc(t *testing.T, assertion *linkkeys.Assertion) *AuthService {
	t.Helper()
	secret := []byte("test-secret")
	// A real, fresh nonce so we clear the nonce gate and reach the audience check.
	assertion.Nonce = auth.MintNonce(secret)
	assertion.Domain = testIDPDomain
	return &AuthService{
		Store:     fakeAuthStore{},
		JWTSecret: secret,
		PKI:       fakeCompletePKI{assertion: assertion},
		IDPDomain: testIDPDomain,
		RPDomain:  testRPDomain,
	}
}

func callComplete(t *testing.T, svc *AuthService) (any, error) {
	t.Helper()
	body, err := cbor.Marshal(csil.CompleteRequest{EncryptedToken: "ENC"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return svc.complete(context.Background(), body)
}

// TestComplete_AcceptsRPDomainAudience: an assertion whose audience equals the
// RP domain is accepted and yields a token.
func TestComplete_AcceptsRPDomainAudience(t *testing.T) {
	svc := completeSvc(t, &linkkeys.Assertion{UserID: "u1", Audience: testRPDomain})
	resp, err := callComplete(t, svc)
	if err != nil {
		t.Fatalf("complete rejected a valid RP-domain audience: %v", err)
	}
	lr, ok := resp.(csil.LoginResponse)
	if !ok || lr.Token == "" {
		t.Fatalf("expected a LoginResponse with a token, got %#v", resp)
	}
}

// TestComplete_RejectsOtherDomainAudience: a different domain is rejected.
func TestComplete_RejectsOtherDomainAudience(t *testing.T) {
	svc := completeSvc(t, &linkkeys.Assertion{UserID: "u1", Audience: "someone-else.example"})
	if _, err := callComplete(t, svc); !isUnauthorized(err) {
		t.Fatalf("expected 401 for mismatched audience, got %v", err)
	}
}

// TestComplete_RejectsOldCallbackURLAudience guards against re-introducing the
// pre-contract-change comparison: the old callback-URL audience form must be
// rejected now that we compare against the RP domain.
func TestComplete_RejectsOldCallbackURLAudience(t *testing.T) {
	svc := completeSvc(t, &linkkeys.Assertion{
		UserID: "u1", Audience: "https://longhouse.todandlorna.com/auth/callback",
	})
	if _, err := callComplete(t, svc); !isUnauthorized(err) {
		t.Fatalf("expected 401 for old callback-URL audience, got %v", err)
	}
}

// TestComplete_ToleratesEmptyAudience: an empty audience is still accepted
// (the != "" guard), matching the back-compat behavior for IDPs that omit it.
func TestComplete_ToleratesEmptyAudience(t *testing.T) {
	svc := completeSvc(t, &linkkeys.Assertion{UserID: "u1", Audience: ""})
	if _, err := callComplete(t, svc); err != nil {
		t.Fatalf("complete rejected an empty audience: %v", err)
	}
}

func isUnauthorized(err error) bool {
	se, ok := err.(*csilrpc.Error)
	return ok && se.Code == 401
}
