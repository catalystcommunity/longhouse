package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/store"
)

// testHarness wires a router with the in-memory store + a test JWT secret
// so per-entity tests can stamp out authenticated requests against the
// real middleware stack (RequireBearer + RequireHouseFromPath + RequireAdmin).
type testHarness struct {
	t      *testing.T
	store  *memStore
	router http.Handler
	secret []byte
}

func newHarness(t *testing.T) *testHarness {
	t.Helper()
	st := newMemStore()
	prev := store.AppStore
	store.AppStore = st
	t.Cleanup(func() { store.AppStore = prev })

	secret := []byte("harness-secret")
	router := NewRouter(&RouterDeps{
		Auth: &AuthDeps{
			JWTSecret: secret,
			IDPDomain: "idp.example",
			Store:     st,
		},
	})
	return &testHarness{t: t, store: st, router: router, secret: secret}
}

// token mints a bearer for the given member, scoped to the given house +
// roles. Tests use this to make calls AS a specific member.
func (h *testHarness) token(memberID, houseID string, roles ...string) string {
	h.t.Helper()
	tok, err := auth.Mint(h.secret, auth.Claims{
		MemberID: memberID,
		HouseID:  houseID,
		Roles:    roles,
	}, time.Hour)
	if err != nil {
		h.t.Fatalf("Mint: %v", err)
	}
	return tok
}

// do is a tiny convenience for issuing a JSON request through the router
// with the given bearer token. Returns the recorded response.
func (h *testHarness) do(method, path, token string, body any) *httptest.ResponseRecorder {
	h.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	var req *http.Request
	if reader == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, reader)
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// decodeJSON unmarshals the response body into v or t.Fatals.
func decodeJSONOrFatal(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode body: %v; body=%s", err, rec.Body.String())
	}
}
