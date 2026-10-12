package csilservices

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type fakeBrowserSessionStore struct {
	fakeCliAuthStore
	created       *models.CliSession
	createdHash   []byte
	session       *models.CliSession
	rotateErr     error
	rotatedOld    []byte
	rotatedNext   []byte
	grace         time.Duration
	extendedID    string
	extendedTo    time.Time
	revokedByHash []byte
	revokeErr     error
	settings      map[string][]models.HouseSetting
}

func (f *fakeBrowserSessionStore) CreateBrowserSession(_ context.Context, s *models.CliSession, hash []byte) error {
	s.SessionID = "browser-1"
	s.Kind = models.SessionKindBrowser
	f.created = s
	f.createdHash = hash
	return nil
}

func (f *fakeBrowserSessionStore) RotateBrowserRefreshToken(_ context.Context, oldHash, nextHash []byte, _ time.Time, grace time.Duration) (*models.CliSession, error) {
	f.rotatedOld = oldHash
	f.rotatedNext = nextHash
	f.grace = grace
	if f.rotateErr != nil && !errors.Is(f.rotateErr, store.ErrRefreshTokenRecentlyUsed) {
		return nil, f.rotateErr
	}
	return f.session, f.rotateErr
}

func (f *fakeBrowserSessionStore) ExtendBrowserSession(_ context.Context, id string, to time.Time) error {
	f.extendedID = id
	f.extendedTo = to
	return nil
}

func (f *fakeBrowserSessionStore) RevokeBrowserSessionByToken(_ context.Context, hash []byte, _ time.Time) (*models.CliSession, error) {
	f.revokedByHash = hash
	if f.revokeErr != nil {
		return nil, f.revokeErr
	}
	return f.session, nil
}

func (f *fakeBrowserSessionStore) GetCliSession(context.Context, string) (*models.CliSession, error) {
	if f.session == nil {
		return nil, store.ErrCliSessionInvalid
	}
	return f.session, nil
}

func (f *fakeBrowserSessionStore) GetHouseSettings(_ context.Context, houseID string) ([]models.HouseSetting, error) {
	return f.settings[houseID], nil
}

func lifetimeSetting(t *testing.T, hours uint64) []models.HouseSetting {
	t.Helper()
	raw, err := json.Marshal(hours)
	if err != nil {
		t.Fatal(err)
	}
	return []models.HouseSetting{{Key: settingSessionLifetimeHours, Value: raw}}
}

func cookieContext(value string) context.Context {
	r := httptest.NewRequest(http.MethodPost, "/api/csil/v1/rpc", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1")
	if value != "" {
		r.AddCookie(&http.Cookie{Name: browserSessionCookie, Value: value})
	}
	return csilrpc.WithHTTPRequest(context.Background(), r)
}

func queuedSessionCookie(t *testing.T, ctx context.Context) *http.Cookie {
	t.Helper()
	for _, c := range csilrpc.QueuedCookies(ctx) {
		if c.Name == browserSessionCookie {
			return c
		}
	}
	return nil
}

func requireUnauthorized(t *testing.T, err error) {
	t.Helper()
	var serviceErr *csilrpc.Error
	if !errors.As(err, &serviceErr) || serviceErr.Code != 401 {
		t.Fatalf("got %v, want 401", err)
	}
}

func liveBrowserSession() *models.CliSession {
	now := time.Now().UTC()
	return &models.CliSession{
		SessionID: "browser-1", SubjectDomain: "example.test", SubjectUserID: "user-1", DisplayName: "Test User",
		ClientName: "Safari on iPhone", Kind: models.SessionKindBrowser,
		CreatedAt: now, LastUsedAt: now, ExpiresAt: now.Add(time.Hour),
	}
}

func TestResumeSessionWithoutCookieIsUnauthorized(t *testing.T) {
	svc := &AuthService{Store: &fakeBrowserSessionStore{}, JWTSecret: []byte("secret")}
	_, err := svc.ResumeSession(cookieContext(""), csil.EmptyRequest{})
	requireUnauthorized(t, err)
}

func TestResumeSessionRotatesCookieAndSlidesExpiry(t *testing.T) {
	st := &fakeBrowserSessionStore{session: liveBrowserSession()}
	svc := &AuthService{Store: st, JWTSecret: []byte("secret"), BearerTTL: 12 * time.Hour}
	ctx := cookieContext("lh_bs_old")
	before := time.Now().UTC()

	resp, err := svc.ResumeSession(ctx, csil.EmptyRequest{})
	if err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	if !bytes.Equal(st.rotatedOld, hashSecret("lh_bs_old")) {
		t.Fatal("store did not receive the hash of the presented cookie")
	}
	if st.grace != browserRefreshReuseGrace {
		t.Fatalf("reuse grace: got %v", st.grace)
	}
	cookie := queuedSessionCookie(t, ctx)
	if cookie == nil || cookie.Value == "" || cookie.Value == "lh_bs_old" {
		t.Fatalf("cookie was not rotated: %#v", cookie)
	}
	if !bytes.Equal(st.rotatedNext, hashSecret(cookie.Value)) {
		t.Fatal("stored next hash does not match the new cookie")
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api" {
		t.Fatalf("cookie attributes: %#v", cookie)
	}
	if cookie.MaxAge != defaultSessionLifetimeHours*3600 {
		t.Fatalf("cookie MaxAge: got %d", cookie.MaxAge)
	}
	wantExpiry := before.Add(defaultSessionLifetimeHours * time.Hour)
	if st.extendedID != "browser-1" || st.extendedTo.Before(wantExpiry) || st.extendedTo.After(wantExpiry.Add(time.Minute)) {
		t.Fatalf("session extension: %q to %v", st.extendedID, st.extendedTo)
	}
	identity, err := auth.Verify(svc.JWTSecret, resp.Token)
	if err != nil {
		t.Fatalf("verify bearer: %v", err)
	}
	if identity.BrowserSessionID != "browser-1" || identity.UserID != "user-1" {
		t.Fatalf("bearer identity: %#v", identity)
	}
}

func TestResumeSessionInsideReuseGraceKeepsNewerCookie(t *testing.T) {
	st := &fakeBrowserSessionStore{session: liveBrowserSession(), rotateErr: store.ErrRefreshTokenRecentlyUsed}
	svc := &AuthService{Store: st, JWTSecret: []byte("secret")}
	ctx := cookieContext("lh_bs_old")
	resp, err := svc.ResumeSession(ctx, csil.EmptyRequest{})
	if err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("no bearer minted inside the grace window")
	}
	if c := queuedSessionCookie(t, ctx); c != nil {
		t.Fatalf("grace resume must not touch the cookie, got %#v", c)
	}
	if st.extendedID != "" {
		t.Fatal("grace resume must not extend the session")
	}
}

func TestResumeSessionReuseClearsCookie(t *testing.T) {
	st := &fakeBrowserSessionStore{rotateErr: store.ErrRefreshTokenReuse}
	svc := &AuthService{Store: st, JWTSecret: []byte("secret")}
	ctx := cookieContext("lh_bs_stolen")
	_, err := svc.ResumeSession(ctx, csil.EmptyRequest{})
	requireUnauthorized(t, err)
	if c := queuedSessionCookie(t, ctx); c == nil || c.MaxAge >= 0 {
		t.Fatalf("cookie was not cleared: %#v", c)
	}
}

func TestEndSessionRevokesAndClearsCookie(t *testing.T) {
	st := &fakeBrowserSessionStore{session: liveBrowserSession()}
	svc := &AuthService{Store: st}
	ctx := cookieContext("lh_bs_live")
	if _, err := svc.EndSession(ctx, csil.EmptyRequest{}); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	if !bytes.Equal(st.revokedByHash, hashSecret("lh_bs_live")) {
		t.Fatal("session was not revoked by cookie hash")
	}
	if c := queuedSessionCookie(t, ctx); c == nil || c.MaxAge >= 0 {
		t.Fatalf("cookie was not cleared: %#v", c)
	}
}

func TestEndSessionWithUnknownCookieSucceeds(t *testing.T) {
	st := &fakeBrowserSessionStore{revokeErr: store.ErrCliSessionInvalid}
	svc := &AuthService{Store: st}
	if _, err := svc.EndSession(cookieContext("lh_bs_gone"), csil.EmptyRequest{}); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
}

func TestRefreshUpgradesLegacyBearerToBrowserSession(t *testing.T) {
	st := &fakeBrowserSessionStore{}
	svc := &AuthService{Store: st, JWTSecret: []byte("secret"), BearerTTL: 12 * time.Hour}
	ctx := auth.WithIdentity(cookieContext(""), &auth.Identity{Domain: "example.test", UserID: "user-1"})
	resp, err := svc.Refresh(ctx, csil.EmptyRequest{})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if st.created == nil || st.created.ClientName != "Safari on iPhone" {
		t.Fatalf("browser session not created: %#v", st.created)
	}
	cookie := queuedSessionCookie(t, ctx)
	if cookie == nil || !bytes.Equal(st.createdHash, hashSecret(cookie.Value)) {
		t.Fatalf("cookie does not match the stored hash: %#v", cookie)
	}
	identity, err := auth.Verify(svc.JWTSecret, resp.Token)
	if err != nil {
		t.Fatalf("verify bearer: %v", err)
	}
	if identity.BrowserSessionID != "browser-1" {
		t.Fatalf("bearer session: got %q", identity.BrowserSessionID)
	}
}

func TestRefreshRejectsRevokedBrowserSession(t *testing.T) {
	session := liveBrowserSession()
	revoked := time.Now().UTC()
	session.RevokedAt = &revoked
	svc := &AuthService{Store: &fakeBrowserSessionStore{session: session}, JWTSecret: []byte("secret")}
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{
		Domain: "example.test", UserID: "user-1", BrowserSessionID: "browser-1",
	})
	_, err := svc.Refresh(ctx, csil.EmptyRequest{})
	requireUnauthorized(t, err)
}

func TestBrowserSessionLifetimeUsesShortestHouseSetting(t *testing.T) {
	st := &fakeBrowserSessionStore{settings: map[string][]models.HouseSetting{
		"house-a": lifetimeSetting(t, 48),
		"house-b": lifetimeSetting(t, 12),
		"house-c": lifetimeSetting(t, 0), // out of range: ignored
	}}
	svc := &AuthService{Store: st}
	houses := []auth.HouseRoles{{House: "house-a"}, {House: "house-b"}, {House: "house-c"}, {House: "house-d"}}
	if got := svc.browserSessionLifetime(context.Background(), houses); got != 12*time.Hour {
		t.Fatalf("lifetime: got %v, want 12h", got)
	}
	if got := svc.browserSessionLifetime(context.Background(), nil); got != defaultSessionLifetimeHours*time.Hour {
		t.Fatalf("default lifetime: got %v", got)
	}
}

func TestBrowserClientName(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1": "Safari on iPhone",
		"Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0":                                                                  "Firefox on Linux",
		"Mozilla/5.0 (Linux; Android 15) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Mobile Safari/537.36":                            "Chrome on Android",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0":           "Edge on Windows",
		"": "Browser",
	}
	for ua, want := range cases {
		if got := browserClientName(ua); got != want {
			t.Errorf("browserClientName(%q) = %q, want %q", ua, got, want)
		}
	}
}
