package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// ── shared fakes ─────────────────────────────────────────────────────────────

type fakePKI struct {
	sign    func(cb, nonce string) (string, error)
	decrypt func(enc string) (string, error)
	verify  func(signed, domain string) (*linkkeys.Assertion, error)
}

func (f *fakePKI) SignRequest(cb, n string) (string, error) {
	if f.sign == nil {
		return "signed-request", nil
	}
	return f.sign(cb, n)
}
func (f *fakePKI) DecryptToken(e string) (string, error) {
	if f.decrypt == nil {
		return "signed-assertion", nil
	}
	return f.decrypt(e)
}
func (f *fakePKI) VerifyAssertion(s, d string) (*linkkeys.Assertion, error) {
	if f.verify == nil {
		return nil, nil
	}
	return f.verify(s, d)
}

type fakeAuthStore struct {
	membersByIdentity map[string][]models.Member // key: domain|user_id
	rolesByMember     map[string][]models.Role    // key: member_id
	houses            map[string]*models.House     // key: house_id
}

func (f *fakeAuthStore) FindMembersByLinkkeysIdentity(_ context.Context, domain, userID string) ([]models.Member, error) {
	return f.membersByIdentity[domain+"|"+userID], nil
}
func (f *fakeAuthStore) ListRolesForMember(_ context.Context, memberID string) ([]models.Role, error) {
	return f.rolesByMember[memberID], nil
}
func (f *fakeAuthStore) GetHouseByID(_ context.Context, houseID string) (*models.House, error) {
	if h, ok := f.houses[houseID]; ok {
		return h, nil
	}
	return nil, errors.New("not found")
}

// ── POST /api/v1/auth/login ──────────────────────────────────────────────────

func TestLogin(t *testing.T) {
	secret := []byte("login-secret")
	store := &fakeAuthStore{
		membersByIdentity: map[string][]models.Member{
			"todandlorna.com|tod": {{MemberID: "m-1", HouseID: "h-1"}},
		},
		rolesByMember: map[string][]models.Role{
			"m-1": {{Name: "admin"}, {Name: "member"}},
		},
	}

	for _, tc := range []struct {
		name       string
		body       string
		verify     func(string, string) (*linkkeys.Assertion, error)
		wantStatus int
	}{
		{
			name: "happy path mints enriched token",
			body: `{"signed_assertion":"good"}`,
			verify: func(string, string) (*linkkeys.Assertion, error) {
				return &linkkeys.Assertion{Domain: "todandlorna.com", UserID: "tod", DisplayName: "Tod Hansmann"}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing assertion",
			body:       `{}`,
			verify:     func(string, string) (*linkkeys.Assertion, error) { return nil, nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed json",
			body:       `nope`,
			verify:     func(string, string) (*linkkeys.Assertion, error) { return nil, nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "assertion fails verification",
			body:       `{"signed_assertion":"bad"}`,
			verify:     func(string, string) (*linkkeys.Assertion, error) { return nil, errors.New("bad sig") },
			wantStatus: http.StatusUnauthorized,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := &AuthDeps{
				PKI:       &fakePKI{verify: tc.verify},
				Store:     store,
				IDPDomain: "todandlorna.com",
				JWTSecret: secret,
			}
			req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			deps.loginHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d (body %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			var resp csil.LoginResponse
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp.Domain != "todandlorna.com" || resp.UserId != "tod" {
				t.Errorf("identity in response wrong: %+v", resp)
			}
			// token verifies and carries the enriched house + roles
			id, err := auth.Verify(secret, resp.Token)
			if err != nil {
				t.Fatalf("verify minted token: %v", err)
			}
			hr := id.House("h-1")
			if hr == nil {
				t.Fatalf("token missing house h-1: %+v", id.Houses)
			}
			if hr.Member != "m-1" || len(hr.Roles) != 2 {
				t.Errorf("house entry wrong: %+v", hr)
			}
		})
	}
}

// ── GET /api/v1/auth/start ───────────────────────────────────────────────────

func TestStart(t *testing.T) {
	deps := &AuthDeps{
		PKI:         &fakePKI{sign: func(cb, nonce string) (string, error) { return "SIGNEDREQ", nil }},
		JWTSecret:   []byte("k"),
		IDPURL:      "https://idp.example",
		CallbackURL: "https://app.example/auth/callback",
	}
	req := httptest.NewRequest("GET", "/api/v1/auth/start", nil)
	rr := httptest.NewRecorder()
	deps.startHandler(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status: got %d, want 302 (body %s)", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if loc != "https://idp.example/auth/authorize?signed_request=SIGNEDREQ" {
		t.Errorf("redirect: %q", loc)
	}
}

func TestStart_NotConfigured(t *testing.T) {
	deps := &AuthDeps{PKI: &fakePKI{}, JWTSecret: []byte("k")} // no IDPURL/CallbackURL
	rr := httptest.NewRecorder()
	deps.startHandler(rr, httptest.NewRequest("GET", "/api/v1/auth/start", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rr.Code)
	}
}

// ── POST /api/v1/auth/complete ───────────────────────────────────────────────

func TestComplete(t *testing.T) {
	secret := []byte("complete-secret")
	const idpDomain = "todandlorna.com"
	const callback = "https://app.example/auth/callback"

	store := &fakeAuthStore{
		membersByIdentity: map[string][]models.Member{
			idpDomain + "|tod": {{MemberID: "m-1", HouseID: "h-1"}},
		},
		rolesByMember: map[string][]models.Role{"m-1": {{Name: "admin"}}},
	}

	// assertionWith returns a verify func producing an assertion with the
	// given nonce/domain/audience.
	assertionWith := func(nonce, domain, aud string) func(string, string) (*linkkeys.Assertion, error) {
		return func(string, string) (*linkkeys.Assertion, error) {
			return &linkkeys.Assertion{UserID: "tod", Domain: domain, Audience: aud, Nonce: nonce, DisplayName: "Tod"}, nil
		}
	}

	goodNonce := auth.MintNonce(secret)

	for _, tc := range []struct {
		name       string
		body       string
		decrypt    func(string) (string, error)
		verify     func(string, string) (*linkkeys.Assertion, error)
		wantStatus int
	}{
		{
			name:       "happy path",
			body:       `{"encrypted_token":"sealed"}`,
			verify:     assertionWith(goodNonce, idpDomain, callback),
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing token",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "decrypt fails",
			body:       `{"encrypted_token":"sealed"}`,
			decrypt:    func(string) (string, error) { return "", errors.New("seal broken") },
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "bad nonce",
			body:       `{"encrypted_token":"sealed"}`,
			verify:     assertionWith("forged-nonce", idpDomain, callback),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong domain",
			body:       `{"encrypted_token":"sealed"}`,
			verify:     assertionWith(goodNonce, "evil.example", callback),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "audience mismatch",
			body:       `{"encrypted_token":"sealed"}`,
			verify:     assertionWith(goodNonce, idpDomain, "https://phish.example/cb"),
			wantStatus: http.StatusUnauthorized,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := &AuthDeps{
				PKI:         &fakePKI{decrypt: tc.decrypt, verify: tc.verify},
				Store:       store,
				IDPDomain:   idpDomain,
				JWTSecret:   secret,
				IDPURL:      "https://idp.example",
				CallbackURL: callback,
			}
			req := httptest.NewRequest("POST", "/api/v1/auth/complete", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			deps.completeHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d (body %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			var resp csil.LoginResponse
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			id, err := auth.Verify(secret, resp.Token)
			if err != nil {
				t.Fatalf("verify token: %v", err)
			}
			if id.UserID != "tod" || id.House("h-1") == nil {
				t.Errorf("token wrong: %+v", id)
			}
		})
	}
}

// ── GET /api/v1/me ───────────────────────────────────────────────────────────

func TestMe(t *testing.T) {
	secret := []byte("me-secret")
	deps := &AuthDeps{
		Store: &fakeAuthStore{
			houses: map[string]*models.House{
				"h-1": {HouseID: "h-1", Name: "Longhouse"},
				"h-2": {HouseID: "h-2", Name: "Acme HQ"},
			},
		},
		JWTSecret: secret,
	}

	// token already carries the houses (snapshotted at mint); /me just adds names
	tok, err := auth.Mint(secret, auth.Identity{
		Domain: "todandlorna.com", UserID: "tod", DisplayName: "Tod",
		Houses: []auth.HouseRoles{
			{House: "h-1", Member: "m-1", Roles: []string{"admin"}},
			{House: "h-2", Member: "m-2", Roles: []string{"member"}},
		},
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	chain := auth.RequireBearer(secret)(http.HandlerFunc(deps.meHandler))
	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d (body %s)", rr.Code, rr.Body.String())
	}
	var resp csil.MeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Domain != "todandlorna.com" || len(resp.Houses) != 2 {
		t.Fatalf("me wrong: %+v", resp)
	}
	if resp.Houses[0].Name != "Longhouse" {
		t.Errorf("house name not resolved: %+v", resp.Houses[0])
	}
}

func TestMe_RequiresToken(t *testing.T) {
	deps := &AuthDeps{Store: &fakeAuthStore{}, JWTSecret: []byte("k")}
	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	rr := httptest.NewRecorder()
	deps.meHandler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

// ── POST /api/v1/auth/refresh ────────────────────────────────────────────────

func TestRefresh_ReEnriches(t *testing.T) {
	secret := []byte("refresh-secret")
	deps := &AuthDeps{
		Store: &fakeAuthStore{
			membersByIdentity: map[string][]models.Member{
				"todandlorna.com|tod": {{MemberID: "m-1", HouseID: "h-1"}},
			},
			rolesByMember: map[string][]models.Role{
				// note: only "member" now — simulates an admin role having been revoked
				"m-1": {{Name: "member"}},
			},
		},
		JWTSecret: secret,
	}

	// caller still holds an old token that claimed admin
	old, _ := auth.Mint(secret, auth.Identity{
		Domain: "todandlorna.com", UserID: "tod",
		Houses: []auth.HouseRoles{{House: "h-1", Member: "m-1", Roles: []string{"admin", "member"}}},
	}, time.Hour)

	chain := auth.RequireBearer(secret)(http.HandlerFunc(deps.refreshHandler))
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+old)
	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d (body %s)", rr.Code, rr.Body.String())
	}
	var resp csil.LoginResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	id, err := auth.Verify(secret, resp.Token)
	if err != nil {
		t.Fatal(err)
	}
	hr := id.House("h-1")
	if hr == nil {
		t.Fatalf("token missing house h-1")
	}
	for _, role := range hr.Roles {
		if role == "admin" {
			t.Errorf("refresh should have dropped the revoked admin role: %+v", hr.Roles)
		}
	}
}
