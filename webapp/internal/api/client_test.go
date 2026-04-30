package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogin_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var got LoginRequest
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.SignedAssertion != "ASSERT" {
			t.Errorf("signed_assertion: %q", got.SignedAssertion)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LoginResponse{
			Token: "tok-1", MemberID: "m1", HouseID: "h1", ExpiresAt: "2026-04-29T12:00:00Z",
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	resp, err := c.Login("ASSERT", nil)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.Token != "tok-1" || resp.MemberID != "m1" || resp.HouseID != "h1" {
		t.Errorf("got %+v", resp)
	}
}

func TestLogin_MultiHouseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"pick one","houses":[{"house_id":"h1","name":"First"},{"house_id":"h2","name":"Second"}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Login("ASSERT", nil)
	if err == nil {
		t.Fatal("expected MultiHouseError")
	}
	if !errors.Is(err, ErrMultipleHouses) {
		t.Errorf("error doesn't unwrap to ErrMultipleHouses: %v", err)
	}
	var multi *MultiHouseError
	if !errors.As(err, &multi) {
		t.Fatalf("expected *MultiHouseError, got %T", err)
	}
	if len(multi.Houses) != 2 || multi.Houses[0].HouseID != "h1" || multi.Houses[1].Name != "Second" {
		t.Errorf("houses: %+v", multi.Houses)
	}
}

func TestLogin_PassesHouseID(t *testing.T) {
	var seen LoginRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &seen)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LoginResponse{Token: "t", MemberID: "m", HouseID: "h2"})
	}))
	defer srv.Close()

	hid := "h2"
	if _, err := New(srv.URL).Login("ASSERT", &hid); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if seen.HouseID == nil || *seen.HouseID != "h2" {
		t.Errorf("house_id not propagated: %+v", seen)
	}
}

func TestLogin_5xxBubblesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	_, err := New(srv.URL).Login("ASSERT", nil)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got %v", err)
	}
}
