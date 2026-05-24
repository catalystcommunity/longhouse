package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func freshToken(t *testing.T, secret []byte, id Identity) string {
	t.Helper()
	tok, err := Mint(secret, id, time.Hour)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	return tok
}

// idWith builds an identity that is a member of one house with the given roles.
func idWith(domain, userID, houseID, memberID string, roles ...string) Identity {
	return Identity{
		Domain: domain,
		UserID: userID,
		Houses: []HouseRoles{{House: houseID, Member: memberID, Roles: roles}},
	}
}

func TestRequireBearer_HappyPath(t *testing.T) {
	secret := []byte("k")
	tok := freshToken(t, secret, Identity{Domain: "d", UserID: "u", DisplayName: "U"})

	var seen *Identity
	h := RequireBearer(secret)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = IdentityFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if seen == nil || seen.Domain != "d" || seen.UserID != "u" {
		t.Errorf("identity not in context: %+v", seen)
	}
}

func TestRequireBearer_MissingHeader(t *testing.T) {
	h := RequireBearer([]byte("k"))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("inner handler should not run")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestRequireBearer_MalformedHeader(t *testing.T) {
	h := RequireBearer([]byte("k"))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for _, hdr := range []string{"abc", "Token x", "Bearer", "Bearer "} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", hdr)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("header %q: got %d, want 401", hdr, rec.Code)
		}
	}
}

func TestRequireBearer_BadSignature(t *testing.T) {
	tok := freshToken(t, []byte("a"), Identity{Domain: "d", UserID: "u"})
	h := RequireBearer([]byte("b"))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("inner handler should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

// chain wires RequireBearer → RequireHouseMember → inner, mounted on a mux so
// {house_id} path values resolve. No resolver: authz comes from the token.
func chain(secret []byte, inner http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/houses/{house_id}/things",
		RequireBearer(secret)(RequireHouseMember(inner)))
	return mux
}

func TestRequireHouseMember_ReadsTokenEntry(t *testing.T) {
	secret := []byte("k")
	tok := freshToken(t, secret, idWith("d", "u", "h1", "m1", "admin", "member"))

	var seen *MemberContext
	h := chain(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/houses/h1/things", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if seen == nil || seen.MemberID != "m1" || seen.HouseID != "h1" {
		t.Fatalf("member context wrong: %+v", seen)
	}
	if !seen.HasRole("admin") || !seen.HasRole("member") {
		t.Errorf("roles not carried: %+v", seen.Roles)
	}
}

func TestRequireHouseMember_NotAMember(t *testing.T) {
	secret := []byte("k")
	// token grants h1 only; request hits h2 → no entry → 403
	tok := freshToken(t, secret, idWith("d", "u", "h1", "m1", "member"))

	h := chain(secret, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("inner handler should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/houses/h2/things", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func adminChain(secret []byte, inner http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/houses/{house_id}/admin",
		RequireBearer(secret)(RequireHouseMember(RequireAdmin(inner))))
	return mux
}

func TestRequireAdmin_AdminRolePasses(t *testing.T) {
	secret := []byte("k")
	tok := freshToken(t, secret, idWith("d", "u", "h1", "m1", "admin", "member"))

	h := adminChain(secret, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/houses/h1/admin", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status: %d", rec.Code)
	}
}

func TestRequireAdmin_NonAdminBlocked(t *testing.T) {
	secret := []byte("k")
	tok := freshToken(t, secret, idWith("d", "u", "h1", "m1", "member"))

	h := adminChain(secret, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("inner handler should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/houses/h1/admin", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}
