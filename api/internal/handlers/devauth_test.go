package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// fakeDevAuthStore is the dev-auth equivalent of fakeLoginStore from
// auth_test.go. Kept separate so the two test suites can evolve
// independently — dev-auth has narrower needs.
type fakeDevAuthStore struct {
	members         map[string]*models.Member  // member_id → member
	houses          []models.House             // returned by ListHouses
	membersByHouse  map[string][]models.Member // house_id → members
	rolesByMember   map[string][]models.Role
	failListHouses  bool
	failListMembers bool
}

func (f *fakeDevAuthStore) GetMemberByID(_ context.Context, id string) (*models.Member, error) {
	if m, ok := f.members[id]; ok {
		return m, nil
	}
	return nil, nil
}

func (f *fakeDevAuthStore) ListRolesForMember(_ context.Context, id string) ([]models.Role, error) {
	return f.rolesByMember[id], nil
}

func (f *fakeDevAuthStore) ListHouses(_ context.Context, _, _ int) ([]models.House, error) {
	if f.failListHouses {
		return nil, errors.New("boom")
	}
	return f.houses, nil
}

func (f *fakeDevAuthStore) ListMembersByHouse(_ context.Context, houseID string, _, _ int) ([]models.Member, error) {
	if f.failListMembers {
		return nil, errors.New("boom")
	}
	return f.membersByHouse[houseID], nil
}

func (f *fakeDevAuthStore) FindMembersByLinkkeysIdentity(_ context.Context, domain, userID string) ([]models.Member, error) {
	out := []models.Member{}
	for _, m := range f.members {
		if m.LinkkeysDomain == domain && m.LinkkeysUserID == userID {
			out = append(out, *m)
		}
	}
	return out, nil
}

func newFakeDevStore() *fakeDevAuthStore {
	memTod := &models.Member{MemberID: "m-tod", HouseID: "h-1", DisplayName: "Tod Hansmann", LinkkeysDomain: "todandlorna.com", LinkkeysUserID: "tod"}
	memLorna := &models.Member{MemberID: "m-lorna", HouseID: "h-1", DisplayName: "Lorna Hansmann", LinkkeysDomain: "todandlorna.com", LinkkeysUserID: "lorna"}
	memAcme := &models.Member{MemberID: "m-acme", HouseID: "h-2", DisplayName: "Acme Admin", LinkkeysDomain: "acme.example", LinkkeysUserID: "admin"}
	return &fakeDevAuthStore{
		members: map[string]*models.Member{
			"m-tod":   memTod,
			"m-lorna": memLorna,
			"m-acme":  memAcme,
		},
		houses: []models.House{
			{HouseID: "h-1", Name: "Longhouse"},
			{HouseID: "h-2", Name: "Acme HQ"},
		},
		membersByHouse: map[string][]models.Member{
			"h-1": {*memTod, *memLorna},
			"h-2": {*memAcme},
		},
		rolesByMember: map[string][]models.Role{
			"m-tod":   {{RoleID: "r-admin", HouseID: "h-1", Name: models.RoleAdmin}, {RoleID: "r-member", HouseID: "h-1", Name: models.RoleMember}},
			"m-lorna": {{RoleID: "r-member", HouseID: "h-1", Name: models.RoleMember}},
			"m-acme":  {{RoleID: "r-admin", HouseID: "h-2", Name: models.RoleAdmin}},
		},
	}
}

// ── /dev/login ──────────────────────────────────────────────────────────────

func TestDevLogin(t *testing.T) {
	secret := []byte("test-secret-do-not-use-in-prod")
	store := newFakeDevStore()
	deps := &DevAuthDeps{Store: store, JWTSecret: secret, Env: "dev"}

	for _, tc := range []struct {
		name       string
		body       string
		wantStatus int
		// checked on success — the minted identity:
		wantDomain string
		wantUser   string
	}{
		{
			name:       "happy path mints identity token",
			body:       `{"member_id":"m-tod"}`,
			wantStatus: http.StatusOK,
			wantDomain: "todandlorna.com",
			wantUser:   "tod",
		},
		{
			name:       "house_id is accepted but ignored",
			body:       `{"member_id":"m-acme","house_id":"h-2"}`,
			wantStatus: http.StatusOK,
			wantDomain: "acme.example",
			wantUser:   "admin",
		},
		{
			name:       "missing member_id",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed json",
			body:       `not-json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown member",
			body:       `{"member_id":"m-ghost"}`,
			wantStatus: http.StatusBadRequest,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/dev/login", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			deps.loginHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}

			var resp csil.LoginResponse
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("decode resp: %v", err)
			}
			if resp.Domain != tc.wantDomain || resp.UserId != tc.wantUser {
				t.Errorf("identity in response: got %s/%s, want %s/%s", resp.Domain, resp.UserId, tc.wantDomain, tc.wantUser)
			}

			// The dev token is indistinguishable from a real one: it verifies
			// against the same secret and decodes to the same identity.
			id, err := auth.Verify(secret, resp.Token)
			if err != nil {
				t.Fatalf("verify minted token: %v", err)
			}
			if id.Domain != tc.wantDomain || id.UserID != tc.wantUser {
				t.Errorf("token identity mismatch: got %+v", id)
			}
		})
	}
}

// ── /dev/users ──────────────────────────────────────────────────────────────

func TestDevUsers(t *testing.T) {
	store := newFakeDevStore()
	deps := &DevAuthDeps{Store: store, JWTSecret: []byte("x"), Env: "dev"}

	req := httptest.NewRequest("GET", "/api/v1/dev/users", nil)
	rr := httptest.NewRecorder()
	deps.usersHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Users []DevUserEntry `json:"users"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, want := len(resp.Users), 3; got != want {
		t.Fatalf("user count: got %d, want %d", got, want)
	}
	// Spot-check that house name + roles came through.
	foundTod := false
	for _, u := range resp.Users {
		if u.MemberID == "m-tod" {
			foundTod = true
			if u.HouseName != "Longhouse" {
				t.Errorf("tod house_name: got %q", u.HouseName)
			}
			if !rolesEqual(u.Roles, []string{models.RoleAdmin, models.RoleMember}) {
				t.Errorf("tod roles: %v", u.Roles)
			}
		}
	}
	if !foundTod {
		t.Error("did not find m-tod in /dev/users response")
	}
}

// ── router-level: endpoints only registered when DevAuth deps are set ──

func TestRouter_DevAuthEndpointsHiddenWhenDisabled(t *testing.T) {
	// No DevAuth in RouterDeps → the routes are not registered → 404.
	deps := &RouterDeps{
		Auth: &AuthDeps{
			PKI:       &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) { return nil, errors.New("unused") }},
			Store:     &fakeAuthStore{},
			IDPDomain: "x",
			JWTSecret: []byte("x"),
		},
	}
	h := NewRouter(deps)

	for _, path := range []string{"/api/v1/dev/login", "/api/v1/dev/users"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, bytes.NewReader(nil))
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", path, rr.Code)
		}
	}
}

func TestRouter_DevAuthEndpointsRegisteredWhenEnabled(t *testing.T) {
	deps := &RouterDeps{
		Auth: &AuthDeps{
			PKI:       &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) { return nil, errors.New("unused") }},
			Store:     &fakeAuthStore{},
			IDPDomain: "x",
			JWTSecret: []byte("x"),
		},
		DevAuth:  &DevAuthDeps{Store: newFakeDevStore(), JWTSecret: []byte("x"), Env: "dev"},
	}
	h := NewRouter(deps)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/dev/users", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("/dev/users: got %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
}

func rolesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	have := map[string]bool{}
	for _, r := range a {
		have[r] = true
	}
	for _, r := range b {
		if !have[r] {
			return false
		}
	}
	return true
}
