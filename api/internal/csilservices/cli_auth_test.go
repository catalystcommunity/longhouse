package csilservices

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type fakeCliAuthStore struct {
	store.Store
	created      *models.CliLoginRequest
	inspected    *models.CliLoginRequest
	exchange     *models.CliSession
	exchangeErr  error
	rotated      *models.CliSession
	rotateErr    error
	approvedCode []byte
	approvedID   auth.Identity
	rotatedOld   []byte
	rotatedNext  []byte
	revokedID    string
	listed       []models.CliSession
}

func (f *fakeCliAuthStore) CreateCliLogin(_ context.Context, login *models.CliLoginRequest) error {
	f.created = login
	return nil
}

func (f *fakeCliAuthStore) GetCliLoginByUserCode(context.Context, []byte, time.Time) (*models.CliLoginRequest, error) {
	if f.inspected == nil {
		return nil, store.ErrCliLoginNotFound
	}
	return f.inspected, nil
}

func (f *fakeCliAuthStore) ApproveCliLogin(_ context.Context, code []byte, domain, userID, displayName string, _ time.Time) error {
	f.approvedCode = code
	f.approvedID = auth.Identity{Domain: domain, UserID: userID, DisplayName: displayName}
	return nil
}

func (f *fakeCliAuthStore) DenyCliLogin(context.Context, []byte, time.Time) error { return nil }

func (f *fakeCliAuthStore) ExchangeCliLogin(context.Context, []byte, []byte, time.Time, time.Time) (*models.CliSession, error) {
	return f.exchange, f.exchangeErr
}

func (f *fakeCliAuthStore) RotateCliRefreshToken(_ context.Context, oldHash, nextHash []byte, _ time.Time) (*models.CliSession, error) {
	f.rotatedOld = oldHash
	f.rotatedNext = nextHash
	return f.rotated, f.rotateErr
}

func (f *fakeCliAuthStore) ListCliSessions(context.Context, string, string) ([]models.CliSession, error) {
	return f.listed, nil
}

func (f *fakeCliAuthStore) RevokeCliSession(_ context.Context, sessionID, _, _ string, _ time.Time) error {
	f.revokedID = sessionID
	return nil
}

func (f *fakeCliAuthStore) HousesTrustingDomain(context.Context, string) ([]models.House, error) {
	return nil, nil
}

func (f *fakeCliAuthStore) FindMembersByLinkkeysIdentity(context.Context, string, string) ([]models.Member, error) {
	return nil, nil
}

func (f *fakeCliAuthStore) RecordAuditEntry(context.Context, *models.AuditEntry) error { return nil }

func TestBeginCliLoginStoresOnlyCodeHashes(t *testing.T) {
	st := &fakeCliAuthStore{}
	svc := &AuthService{Store: st, JWTSecret: []byte("secret"), CallbackURL: "https://longhouse.example/auth/callback"}
	response, err := svc.BeginCliLogin(context.Background(), csil.BeginCliLoginRequest{ClientName: "workstation"})
	if err != nil {
		t.Fatalf("BeginCliLogin: %v", err)
	}
	if st.created == nil {
		t.Fatal("login request was not stored")
	}
	if response.DeviceCode == "" || response.UserCode == "" {
		t.Fatal("response omitted a login code")
	}
	if bytes.Contains(st.created.DeviceCodeHash, []byte(response.DeviceCode)) || bytes.Contains(st.created.UserCodeHash, []byte(response.UserCode)) {
		t.Fatal("store received a plaintext login code")
	}
	if !strings.HasPrefix(response.VerificationUrl, "https://longhouse.example/cli/authorize?code=") {
		t.Fatalf("verification URL: %q", response.VerificationUrl)
	}
}

func TestApproveCliLoginUsesBearerIdentity(t *testing.T) {
	st := &fakeCliAuthStore{}
	svc := &AuthService{Store: st}
	id := &auth.Identity{Domain: "example.test", UserID: "user-1", DisplayName: "Test User"}
	ctx := auth.WithIdentity(context.Background(), id)
	if _, err := svc.ApproveCliLogin(ctx, csil.ApproveCliLoginRequest{UserCode: "ABCD-EFGH"}); err != nil {
		t.Fatalf("ApproveCliLogin: %v", err)
	}
	if st.approvedID.Domain != id.Domain || st.approvedID.UserID != id.UserID || st.approvedID.DisplayName != id.DisplayName {
		t.Fatalf("approved identity: %#v", st.approvedID)
	}
	if bytes.Equal(st.approvedCode, []byte("ABCDEFGH")) {
		t.Fatal("approval passed a plaintext user code to the store")
	}
}

func TestExchangeCliLoginPendingIsNotAnError(t *testing.T) {
	svc := &AuthService{Store: &fakeCliAuthStore{exchangeErr: store.ErrCliLoginPending}}
	response, err := svc.ExchangeCliLogin(context.Background(), csil.ExchangeCliLoginRequest{DeviceCode: "device"})
	if err != nil {
		t.Fatalf("ExchangeCliLogin: %v", err)
	}
	if response.Status != csil.CliLoginStatus("pending") || response.Session != nil {
		t.Fatalf("pending response: %#v", response)
	}
}

func TestExchangeCliLoginUsesConfiguredBearerTTL(t *testing.T) {
	now := time.Now().UTC()
	st := &fakeCliAuthStore{exchange: &models.CliSession{
		SessionID: "session-1", SubjectDomain: "example.test", SubjectUserID: "user-1",
		ClientName: "workstation", CreatedAt: now, LastUsedAt: now, ExpiresAt: now.Add(24 * time.Hour),
	}}
	svc := &AuthService{
		Store: st, JWTSecret: []byte("secret"), BearerTTL: 37 * time.Minute, RefreshTokenTTL: 24 * time.Hour,
	}
	response, err := svc.ExchangeCliLogin(context.Background(), csil.ExchangeCliLoginRequest{DeviceCode: "device"})
	if err != nil {
		t.Fatalf("ExchangeCliLogin: %v", err)
	}
	if response.Session == nil {
		t.Fatal("exchange did not return a session")
	}
	identity, err := auth.Verify(svc.JWTSecret, response.Session.Token)
	if err != nil {
		t.Fatalf("verify bearer: %v", err)
	}
	gotTTL := time.Duration(identity.ExpiresAt-identity.IssuedAt) * time.Second
	if gotTTL != 37*time.Minute {
		t.Fatalf("bearer TTL: got %v, want 37m", gotTTL)
	}
	if identity.CliSessionID != "session-1" {
		t.Fatalf("bearer CLI session: got %q, want session-1", identity.CliSessionID)
	}
	if response.Session.RefreshToken == "" || response.Session.SessionId != "session-1" {
		t.Fatalf("CLI token response: %#v", response.Session)
	}
}

func TestBearerRefreshRejectsCliSession(t *testing.T) {
	svc := &AuthService{}
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{
		Domain: "example.test", UserID: "user-1", CliSessionID: "session-1",
	})
	_, err := svc.Refresh(ctx, csil.EmptyRequest{})
	var serviceErr *csilrpc.Error
	if !errors.As(err, &serviceErr) || serviceErr.Code != 401 {
		t.Fatalf("CLI bearer refresh error: got %v, want 401", err)
	}
}

func TestExchangeCliLoginCapsBearerAtSessionExpiry(t *testing.T) {
	now := time.Now().UTC()
	st := &fakeCliAuthStore{exchange: &models.CliSession{
		SessionID: "session-1", SubjectDomain: "example.test", SubjectUserID: "user-1",
		ClientName: "workstation", CreatedAt: now, LastUsedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	}}
	svc := &AuthService{Store: st, JWTSecret: []byte("secret"), BearerTTL: 12 * time.Hour}
	response, err := svc.ExchangeCliLogin(context.Background(), csil.ExchangeCliLoginRequest{DeviceCode: "device"})
	if err != nil {
		t.Fatalf("ExchangeCliLogin: %v", err)
	}
	identity, err := auth.Verify(svc.JWTSecret, response.Session.Token)
	if err != nil {
		t.Fatalf("verify bearer: %v", err)
	}
	gotTTL := identity.ExpiresAt - identity.IssuedAt
	if gotTTL < 298 || gotTTL > 300 {
		t.Fatalf("bearer TTL: got %ds, want no more than the five-minute session lifetime", gotTTL)
	}
}

func TestRefreshSessionMapsReuseToUnauthorized(t *testing.T) {
	svc := &AuthService{Store: &fakeCliAuthStore{rotateErr: store.ErrRefreshTokenReuse}}
	_, err := svc.RefreshSession(context.Background(), csil.RefreshSessionRequest{RefreshToken: "old"})
	var serviceErr *csilrpc.Error
	if !errors.As(err, &serviceErr) || serviceErr.Code != 401 {
		t.Fatalf("refresh reuse error: got %v, want 401", err)
	}
}
