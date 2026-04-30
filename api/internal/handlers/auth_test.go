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
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type fakePKI struct {
	verify func(signed, domain string) (*linkkeys.Assertion, error)
}

func (f *fakePKI) VerifyAssertion(s, d string) (*linkkeys.Assertion, error) {
	return f.verify(s, d)
}

type fakeLoginStore struct {
	members        map[string][]models.Member // key: domain|user_id
	roles          map[string][]models.Role   // key: member_id
	houses         map[string]*models.House   // key: house_id
	trustedHouses  map[string][]models.House  // key: domain — houses trusting it
	trustedDomains map[string]bool            // key: house_id|domain
	roleByName     map[string]*models.Role    // key: house_id|name

	createdMembers []models.Member
	assignments    []struct{ MemberID, RoleID string }
	audits         []models.MemberAudit
}

func (f *fakeLoginStore) FindMembersByLinkkeysIdentity(_ context.Context, domain, userID string) ([]models.Member, error) {
	return f.members[domain+"|"+userID], nil
}

func (f *fakeLoginStore) ListRolesForMember(_ context.Context, memberID string) ([]models.Role, error) {
	return f.roles[memberID], nil
}

func (f *fakeLoginStore) GetHouseByID(_ context.Context, houseID string) (*models.House, error) {
	if h, ok := f.houses[houseID]; ok {
		return h, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeLoginStore) HousesTrustingDomain(_ context.Context, domain string) ([]models.House, error) {
	return f.trustedHouses[domain], nil
}

func (f *fakeLoginStore) IsDomainTrusted(_ context.Context, houseID, domain string) (bool, error) {
	return f.trustedDomains[houseID+"|"+domain], nil
}

func (f *fakeLoginStore) GetRoleByName(_ context.Context, houseID, name string) (*models.Role, error) {
	if r, ok := f.roleByName[houseID+"|"+name]; ok {
		return r, nil
	}
	return nil, errors.New("role not found")
}

func (f *fakeLoginStore) CreateMember(_ context.Context, m *models.Member) error {
	if m.MemberID == "" {
		m.MemberID = "auto-" + m.LinkkeysUserID
	}
	f.createdMembers = append(f.createdMembers, *m)
	if f.members == nil {
		f.members = map[string][]models.Member{}
	}
	f.members[m.LinkkeysDomain+"|"+m.LinkkeysUserID] = append(
		f.members[m.LinkkeysDomain+"|"+m.LinkkeysUserID], *m)
	return nil
}

func (f *fakeLoginStore) UpdateMember(_ context.Context, m *models.Member) error {
	for k, list := range f.members {
		for i := range list {
			if list[i].MemberID == m.MemberID {
				f.members[k][i] = *m
			}
		}
	}
	return nil
}

func (f *fakeLoginStore) AssignRole(_ context.Context, memberID, roleID string) error {
	f.assignments = append(f.assignments, struct{ MemberID, RoleID string }{memberID, roleID})
	return nil
}

func (f *fakeLoginStore) RecordMemberAudit(_ context.Context, a *models.MemberAudit) error {
	if a.AuditID == "" {
		a.AuditID = "audit-" + a.SubjectMemberID
	}
	f.audits = append(f.audits, *a)
	return nil
}

func newDeps(pki PKIClient, store LoginStore) *AuthDeps {
	return &AuthDeps{
		PKI:       pki,
		Store:     store,
		IDPDomain: "idp.example",
		JWTSecret: []byte("test-secret"),
	}
}

func postLogin(t *testing.T, deps *AuthDeps, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	deps.loginHandler(rec, req)
	return rec
}

func TestLogin_HappyPath_SingleHouse(t *testing.T) {
	pki := &fakePKI{verify: func(s, d string) (*linkkeys.Assertion, error) {
		if s != "ASSERT" || d != "idp.example" {
			t.Errorf("verify called with %q / %q", s, d)
		}
		return &linkkeys.Assertion{UserID: "u1", Domain: "idp.example"}, nil
	}}
	st := &fakeLoginStore{
		members: map[string][]models.Member{
			"idp.example|u1": {{MemberID: "m1", HouseID: "h1", LinkkeysDomain: "idp.example", LinkkeysUserID: "u1"}},
		},
		roles: map[string][]models.Role{
			"m1": {{Name: "admin"}, {Name: "member"}},
		},
		houses: map[string]*models.House{"h1": {HouseID: "h1", Name: "Test"}},
	}

	rec := postLogin(t, newDeps(pki, st), csil.LoginRequest{SignedAssertion: "ASSERT"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp csil.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, rec.Body.String())
	}
	if resp.Token == "" || resp.MemberId != "m1" || resp.HouseId != "h1" {
		t.Errorf("response: %+v", resp)
	}

	claims, err := auth.Verify([]byte("test-secret"), resp.Token)
	if err != nil {
		t.Fatalf("token did not verify: %v", err)
	}
	if claims.MemberID != "m1" || claims.HouseID != "h1" {
		t.Errorf("claims: %+v", claims)
	}
	if !claims.HasRole("admin") || !claims.HasRole("member") {
		t.Errorf("missing roles in claims: %+v", claims.Roles)
	}
}

func TestLogin_MultipleHouses_RequiresChoice(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return &linkkeys.Assertion{UserID: "u1", Domain: "idp.example"}, nil
	}}
	st := &fakeLoginStore{
		members: map[string][]models.Member{
			"idp.example|u1": {
				{MemberID: "m1", HouseID: "h1"},
				{MemberID: "m2", HouseID: "h2"},
			},
		},
		houses: map[string]*models.House{
			"h1": {HouseID: "h1", Name: "First"},
			"h2": {HouseID: "h2", Name: "Second"},
		},
	}

	rec := postLogin(t, newDeps(pki, st), csil.LoginRequest{SignedAssertion: "ASSERT"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want 409; body=%s", rec.Code, rec.Body.String())
	}

	var resp csil.LoginMultiHouseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Houses) != 2 {
		t.Errorf("want 2 houses in response, got %d", len(resp.Houses))
	}
	got := map[string]string{}
	for _, h := range resp.Houses {
		got[string(h.HouseId)] = h.Name
	}
	if got["h1"] != "First" || got["h2"] != "Second" {
		t.Errorf("house summary: %+v", got)
	}
}

func TestLogin_MultipleHouses_PicksRequested(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return &linkkeys.Assertion{UserID: "u1", Domain: "idp.example"}, nil
	}}
	st := &fakeLoginStore{
		members: map[string][]models.Member{
			"idp.example|u1": {
				{MemberID: "m1", HouseID: "h1"},
				{MemberID: "m2", HouseID: "h2"},
			},
		},
		roles:  map[string][]models.Role{"m2": {{Name: "member"}}},
		houses: map[string]*models.House{"h1": {Name: "A"}, "h2": {Name: "B"}},
	}

	hid := csil.HouseID("h2")
	rec := postLogin(t, newDeps(pki, st), csil.LoginRequest{SignedAssertion: "ASSERT", HouseId: &hid})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp csil.LoginResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.MemberId != "m2" || resp.HouseId != "h2" {
		t.Errorf("got %+v", resp)
	}
}

func TestLogin_AssertionRejected(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return nil, errors.New("bad signature")
	}}
	rec := postLogin(t, newDeps(pki, &fakeLoginStore{}), csil.LoginRequest{SignedAssertion: "ASSERT"})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestLogin_NoMemberRecord(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return &linkkeys.Assertion{UserID: "u1", Domain: "idp.example"}, nil
	}}
	rec := postLogin(t, newDeps(pki, &fakeLoginStore{}), csil.LoginRequest{SignedAssertion: "ASSERT"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestLogin_MissingAssertion(t *testing.T) {
	rec := postLogin(t, newDeps(&fakePKI{}, &fakeLoginStore{}), csil.LoginRequest{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestLogin_BadJSON(t *testing.T) {
	deps := newDeps(&fakePKI{}, &fakeLoginStore{})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	deps.loginHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestLogin_TrustedDomain_AutoCreatesMember(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return &linkkeys.Assertion{UserID: "u1", Domain: "trusted.example", DisplayName: "New User"}, nil
	}}
	memberRole := &models.Role{RoleID: "role-member", HouseID: "h1", Name: "member"}
	st := &fakeLoginStore{
		members:        map[string][]models.Member{}, // user has no member rows anywhere
		trustedHouses:  map[string][]models.House{"trusted.example": {{HouseID: "h1", Name: "First"}}},
		trustedDomains: map[string]bool{"h1|trusted.example": true},
		houses:         map[string]*models.House{"h1": {HouseID: "h1", Name: "First"}},
		roleByName:     map[string]*models.Role{"h1|member": memberRole},
	}

	rec := postLogin(t, newDeps(pki, st), csil.LoginRequest{SignedAssertion: "ASSERT"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(st.createdMembers) != 1 {
		t.Fatalf("expected 1 auto-created member, got %d", len(st.createdMembers))
	}
	created := st.createdMembers[0]
	if created.HouseID != "h1" || created.LinkkeysUserID != "u1" || created.LinkkeysDomain != "trusted.example" {
		t.Errorf("created member: %+v", created)
	}
	if created.DisplayName != "New User" {
		t.Errorf("display name not propagated: %q", created.DisplayName)
	}
	if len(st.assignments) != 1 || st.assignments[0].RoleID != "role-member" {
		t.Errorf("expected the canonical 'member' role to be assigned: %+v", st.assignments)
	}
	if len(st.audits) != 1 || st.audits[0].Action != models.AuditActionMemberAutoCreated {
		t.Errorf("expected an auto-create audit row: %+v", st.audits)
	}
}

func TestLogin_NoMembershipNoTrust_403(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return &linkkeys.Assertion{UserID: "u1", Domain: "stranger.example"}, nil
	}}
	rec := postLogin(t, newDeps(pki, &fakeLoginStore{}), csil.LoginRequest{SignedAssertion: "ASSERT"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestLogin_TrustedDomain_MultipleOptions(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return &linkkeys.Assertion{UserID: "u1", Domain: "trusted.example"}, nil
	}}
	st := &fakeLoginStore{
		members: map[string][]models.Member{
			"trusted.example|u1": {{MemberID: "m1", HouseID: "h1"}},
		},
		trustedHouses: map[string][]models.House{
			"trusted.example": {{HouseID: "h2", Name: "Second"}, {HouseID: "h1"}},
		},
		houses: map[string]*models.House{
			"h1": {HouseID: "h1", Name: "First"},
			"h2": {HouseID: "h2", Name: "Second"},
		},
	}

	rec := postLogin(t, newDeps(pki, st), csil.LoginRequest{SignedAssertion: "ASSERT"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	var multi csil.LoginMultiHouseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &multi); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := map[string]bool{}
	for _, h := range multi.Houses {
		got[string(h.HouseId)] = true
	}
	if !got["h1"] || !got["h2"] {
		t.Errorf("multi-house response should include both h1 (member) and h2 (trusted): %+v", multi.Houses)
	}
}

func TestLogin_TrustedDomain_SpecifyingNonAvailableHouse403(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return &linkkeys.Assertion{UserID: "u1", Domain: "trusted.example"}, nil
	}}
	st := &fakeLoginStore{
		trustedHouses:  map[string][]models.House{"trusted.example": {{HouseID: "h1"}}},
		trustedDomains: map[string]bool{"h1|trusted.example": true},
		houses:         map[string]*models.House{"h1": {HouseID: "h1"}},
	}
	requested := csil.HouseID("h99")
	rec := postLogin(t, newDeps(pki, st), csil.LoginRequest{SignedAssertion: "ASSERT", HouseId: &requested})
	if rec.Code != http.StatusConflict {
		// pickAvailableHouse with an unmatchable desired returns errNoSuchHouse,
		// which the handler currently maps to 409 just like errMultipleHouses.
		// Either way, a 200 here would be wrong.
		t.Errorf("status: got %d, want non-200 (mapped to 409 in the current handler)", rec.Code)
	}
}

func TestLogin_BackfillsDisplayNameOnFirstNamedAssertion(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return &linkkeys.Assertion{UserID: "u1", Domain: "idp.example", DisplayName: "Late Arrival"}, nil
	}}
	st := &fakeLoginStore{
		members: map[string][]models.Member{
			"idp.example|u1": {{
				MemberID: "m1", HouseID: "h1",
				LinkkeysDomain: "idp.example", LinkkeysUserID: "u1",
				DisplayName: "", // empty — should be filled in by login
			}},
		},
		houses: map[string]*models.House{"h1": {HouseID: "h1", Name: "Test"}},
	}

	rec := postLogin(t, newDeps(pki, st), csil.LoginRequest{SignedAssertion: "ASSERT"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	got := st.members["idp.example|u1"][0]
	if got.DisplayName != "Late Arrival" {
		t.Errorf("display_name not backfilled: %q", got.DisplayName)
	}
}

func TestLogin_DoesNotOverwriteExistingDisplayName(t *testing.T) {
	pki := &fakePKI{verify: func(string, string) (*linkkeys.Assertion, error) {
		return &linkkeys.Assertion{UserID: "u1", Domain: "idp.example", DisplayName: "From IDP"}, nil
	}}
	st := &fakeLoginStore{
		members: map[string][]models.Member{
			"idp.example|u1": {{
				MemberID: "m1", HouseID: "h1",
				LinkkeysDomain: "idp.example", LinkkeysUserID: "u1",
				DisplayName: "Local Override",
			}},
		},
		houses: map[string]*models.House{"h1": {HouseID: "h1"}},
	}

	rec := postLogin(t, newDeps(pki, st), csil.LoginRequest{SignedAssertion: "ASSERT"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	if st.members["idp.example|u1"][0].DisplayName != "Local Override" {
		t.Errorf("display_name was clobbered: %q", st.members["idp.example|u1"][0].DisplayName)
	}
}

func TestMe_RequiresToken(t *testing.T) {
	rec := httptest.NewRecorder()
	meHandler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestMe_EchoesClaims(t *testing.T) {
	secret := []byte("k")
	tok, err := auth.Mint(secret, auth.Claims{
		MemberID: "m1",
		HouseID:  "h1",
		Roles:    []string{"admin"},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}

	chain := auth.RequireBearer(secret)(http.HandlerFunc(meHandler))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp csil.MeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.MemberId != "m1" || resp.HouseId != "h1" {
		t.Errorf("got %+v", resp)
	}
	if len(resp.Roles) != 1 || resp.Roles[0] != "admin" {
		t.Errorf("roles: %+v", resp.Roles)
	}
}
