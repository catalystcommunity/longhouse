package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func copyCookies(t *testing.T, rec *httptest.ResponseRecorder, req *http.Request) {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
}

func TestIdentityRoundTrip(t *testing.T) {
	m := New("super-secret", false)

	rec := httptest.NewRecorder()
	id := Identity{Domain: "idp.example", UserID: "u1", DisplayName: "Tod"}
	if err := m.SetIdentity(rec, id); err != nil {
		t.Fatalf("SetIdentity: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	copyCookies(t, rec, req)

	got, err := m.GetIdentity(req)
	if err != nil {
		t.Fatalf("GetIdentity: %v", err)
	}
	if got.Domain != "idp.example" || got.UserID != "u1" || got.DisplayName != "Tod" {
		t.Errorf("identity mismatch: %+v", got)
	}
	if got.ExpiresAt.Before(time.Now()) {
		t.Errorf("ExpiresAt should be in the future: %v", got.ExpiresAt)
	}
}

func TestGetIdentity_Missing(t *testing.T) {
	m := New("s", false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := m.GetIdentity(req); err != ErrMissing {
		t.Errorf("got %v, want ErrMissing", err)
	}
}

func TestGetIdentity_Tampered(t *testing.T) {
	m := New("s", false)
	rec := httptest.NewRecorder()
	if err := m.SetIdentity(rec, Identity{Domain: "idp.example", UserID: "u1"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		c.Value = strings.ReplaceAll(c.Value, ".", "X") + "X"
		req.AddCookie(c)
	}
	if _, err := m.GetIdentity(req); err != ErrInvalid {
		t.Errorf("got %v, want ErrInvalid", err)
	}
}

func TestGetIdentity_WrongSecret(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := New("alice", false).SetIdentity(rec, Identity{Domain: "idp.example", UserID: "u1"}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	copyCookies(t, rec, req)

	if _, err := New("bob", false).GetIdentity(req); err != ErrInvalid {
		t.Errorf("got %v, want ErrInvalid", err)
	}
}

func TestClearIdentity(t *testing.T) {
	m := New("s", false)
	rec := httptest.NewRecorder()
	m.ClearIdentity(rec)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != SessionCookie || cookies[0].MaxAge >= 0 {
		t.Errorf("clear cookie looks wrong: %+v", cookies)
	}
}

func TestLoginStateConsume(t *testing.T) {
	m := New("s", false)
	rec := httptest.NewRecorder()
	if err := m.SetLoginState(rec, LoginState{Nonce: "abc"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	copyCookies(t, rec, req)

	// Consume writes a clearing cookie into a new recorder.
	rec2 := httptest.NewRecorder()
	state, err := m.ConsumeLoginState(rec2, req)
	if err != nil {
		t.Fatalf("ConsumeLoginState: %v", err)
	}
	if state.Nonce != "abc" {
		t.Errorf("nonce: got %q", state.Nonce)
	}

	cleared := false
	for _, c := range rec2.Result().Cookies() {
		if c.Name == LoginStateCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("expected login-state cookie to be cleared on consume")
	}
}

func TestNew_PanicsOnEmptySecret(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty secret")
		}
	}()
	_ = New("", false)
}
