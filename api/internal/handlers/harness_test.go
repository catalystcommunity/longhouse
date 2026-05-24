package handlers

import (
	"bytes"
	"context"
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
// real middleware stack (RequireBearer → RequireHouseMember → RequireAdmin).
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

// token mints an identity bearer carrying a single house entry
// {houseID, memberID, roles}. Authorization reads roles straight from the
// token now, so the roles passed here are what RequireHouseMember/Admin see —
// tests don't need the store's role assignments to match. A memberID with no
// seeded row still gets a valid identity (its linkkeys identity defaults to
// memberID@todandlorna.com), and a request to a different house simply finds
// no entry in the token → 403, which is what cross-house tests want.
func (h *testHarness) token(memberID, houseID string, roles ...string) string {
	h.t.Helper()
	domain, userID := "todandlorna.com", memberID
	display := ""
	if m, err := h.store.GetMemberByID(context.Background(), memberID); err == nil && m != nil {
		domain, userID, display = m.LinkkeysDomain, m.LinkkeysUserID, m.DisplayName
	}
	id := auth.Identity{
		Domain:      domain,
		UserID:      userID,
		DisplayName: display,
		Houses:      []auth.HouseRoles{{House: houseID, Member: memberID, Roles: roles}},
	}
	tok, err := auth.Mint(h.secret, id, time.Hour)
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
