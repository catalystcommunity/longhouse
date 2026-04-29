package linkkeys

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(srv *httptest.Server) *Client {
	return &Client{
		BaseURL:    srv.URL,
		APIKey:     "test-key",
		HTTPClient: srv.Client(),
	}
}

func TestSignRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1alpha/sign-request.json" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth header: got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var in map[string]string
		if err := json.Unmarshal(body, &in); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if in["callback_url"] != "https://rp.example/cb" || in["nonce"] != "n1" {
			t.Errorf("body: %+v", in)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"signed_request":"SIGNED"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.SignRequest("https://rp.example/cb", "n1")
	if err != nil {
		t.Fatalf("SignRequest: %v", err)
	}
	if got != "SIGNED" {
		t.Errorf("got %q, want SIGNED", got)
	}
}

func TestDecryptToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"signed_assertion":"ASSERT"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.DecryptToken("ENC")
	if err != nil {
		t.Fatalf("DecryptToken: %v", err)
	}
	if got != "ASSERT" {
		t.Errorf("got %q, want ASSERT", got)
	}
}

func TestVerifyAssertion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"verified":true,"assertion":{"user_id":"u1","domain":"idp.example","nonce":"n1","display_name":"Tod"}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.VerifyAssertion("ASSERT", "idp.example")
	if err != nil {
		t.Fatalf("VerifyAssertion: %v", err)
	}
	if got.UserID != "u1" || got.Domain != "idp.example" || got.Nonce != "n1" || got.DisplayName != "Tod" {
		t.Errorf("assertion mismatch: %+v", got)
	}
}

func TestVerifyAssertion_NotVerified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"verified":false,"assertion":{}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	if _, err := c.VerifyAssertion("ASSERT", "idp.example"); err == nil {
		t.Fatal("expected error when verified=false")
	}
}

func TestPost_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("nope"))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.SignRequest("cb", "n")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got %v", err)
	}
}
