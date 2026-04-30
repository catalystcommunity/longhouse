package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func freshToken(t *testing.T, secret []byte, c Claims) string {
	t.Helper()
	tok, err := Mint(secret, c, time.Hour)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	return tok
}

func TestRequireBearer_HappyPath(t *testing.T) {
	secret := []byte("k")
	tok := freshToken(t, secret, Claims{MemberID: "m1", HouseID: "h1", Roles: []string{"admin"}})

	var seen *Claims
	h := RequireBearer(secret)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if seen == nil || seen.MemberID != "m1" {
		t.Errorf("claims not in context: %+v", seen)
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
	tok := freshToken(t, []byte("a"), Claims{MemberID: "m1"})
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

func TestRequireAdmin_AdminRolePasses(t *testing.T) {
	secret := []byte("k")
	tok := freshToken(t, secret, Claims{MemberID: "m1", HouseID: "h1", Roles: []string{"admin", "member"}})

	chain := RequireBearer(secret)(RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status: %d", rec.Code)
	}
}

func TestRequireAdmin_NonAdminBlocked(t *testing.T) {
	secret := []byte("k")
	tok := freshToken(t, secret, Claims{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	chain := RequireBearer(secret)(RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("inner handler should not run")
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestRequireHouseFromPath_Match(t *testing.T) {
	secret := []byte("k")
	tok := freshToken(t, secret, Claims{MemberID: "m1", HouseID: "h1"})

	mux := http.NewServeMux()
	chain := RequireBearer(secret)(RequireHouseFromPath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	mux.Handle("GET /api/v1/houses/{house_id}/things", chain)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/houses/h1/things", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestRequireHouseFromPath_Mismatch(t *testing.T) {
	secret := []byte("k")
	tok := freshToken(t, secret, Claims{MemberID: "m1", HouseID: "h1"})

	mux := http.NewServeMux()
	chain := RequireBearer(secret)(RequireHouseFromPath(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("inner handler should not run")
	})))
	mux.Handle("GET /api/v1/houses/{house_id}/things", chain)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/houses/h2/things", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}
