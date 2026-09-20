package cmd

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/transport"
)

func TestCredentialsRoundTripUsesPrivateFile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("LONGHOUSE_CONFIG_DIR", configDir)
	want := &cliCredentials{
		URL: "https://longhouse.example", Token: "access", Domain: "example.test",
		UserID: "user-1", RefreshToken: "refresh", SessionID: "session-1",
		ExpiresAt:        time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		RefreshExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	}
	if err := writeCredentials(want); err != nil {
		t.Fatalf("writeCredentials: %v", err)
	}
	path := filepath.Join(configDir, "session.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credential mode: got %o, want 600", info.Mode().Perm())
	}
	got, err := readCredentials()
	if err != nil {
		t.Fatalf("readCredentials: %v", err)
	}
	if got.RefreshToken != want.RefreshToken || got.SessionID != want.SessionID || got.URL != want.URL {
		t.Fatalf("credentials: got %#v, want %#v", got, want)
	}
}

func TestCliURLRequiresHTTPOrigin(t *testing.T) {
	t.Setenv("LONGHOUSE_URL", "")
	for _, raw := range []string{"", "ftp://longhouse.example", "https://user@longhouse.example", "https://longhouse.example?q=x"} {
		_, err := cliURL(map[string]string{"url": raw}, nil)
		if err == nil {
			t.Errorf("cliURL accepted %q", raw)
		}
	}
	got, err := cliURL(map[string]string{"url": "https://longhouse.example/"}, nil)
	if err != nil || got != "https://longhouse.example" {
		t.Fatalf("cliURL valid origin: got %q, %v", got, err)
	}
}

func TestCliURLDoesNotSendSavedCredentialsToAnotherServer(t *testing.T) {
	t.Setenv("LONGHOUSE_URL", "")
	credentials := &cliCredentials{URL: "https://one.example"}

	got, err := cliURL(map[string]string{"url": "https://one.example/"}, credentials)
	if err != nil || got != "https://one.example" {
		t.Fatalf("cliURL matching origin: got %q, %v", got, err)
	}
	if _, err := cliURL(map[string]string{"url": "https://two.example"}, credentials); err == nil {
		t.Fatal("cliURL accepted an origin that does not own the saved credentials")
	}
}

func TestInvalidSessionError(t *testing.T) {
	if !isInvalidSessionError(&longhouseServiceError{Code: http.StatusUnauthorized}) {
		t.Fatal("unauthorized service response should identify an invalid session")
	}
	if !isInvalidSessionError(transport.StatusError{Code: transport.StatusUnauthenticated.Code()}) {
		t.Fatal("unauthenticated transport response should identify an invalid session")
	}
	if isInvalidSessionError(errors.New("network error")) {
		t.Fatal("network error should not identify an invalid session")
	}
}

func TestAccessTokenNeedsRefresh(t *testing.T) {
	if accessTokenNeedsRefresh(&cliCredentials{ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}) {
		t.Fatal("one-hour bearer should not refresh yet")
	}
	if !accessTokenNeedsRefresh(&cliCredentials{ExpiresAt: time.Now().Add(time.Minute).UTC().Format(time.RFC3339)}) {
		t.Fatal("one-minute bearer should refresh")
	}
}
