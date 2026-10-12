//go:build integration

package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func TestCliRefreshRotation_Postgres_RevokesOnReuse(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	deviceHash := bytes.Repeat([]byte{1}, 32)
	userHash := bytes.Repeat([]byte{2}, 32)
	firstRefreshHash := bytes.Repeat([]byte{3}, 32)
	secondRefreshHash := bytes.Repeat([]byte{4}, 32)

	login := &models.CliLoginRequest{
		DeviceCodeHash: deviceHash,
		UserCodeHash:   userHash,
		ClientName:     "integration CLI",
		Status:         "pending",
		CreatedAt:      now,
		ExpiresAt:      now.Add(10 * time.Minute),
	}
	if err := store.AppStore.CreateCliLogin(ctx, login); err != nil {
		t.Fatalf("CreateCliLogin: %v", err)
	}
	if err := store.AppStore.ApproveCliLogin(ctx, userHash, "example.test", "user-1", "Test User", now); err != nil {
		t.Fatalf("ApproveCliLogin: %v", err)
	}
	session, err := store.AppStore.ExchangeCliLogin(ctx, deviceHash, firstRefreshHash, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("ExchangeCliLogin: %v", err)
	}
	if session.SessionID == "" {
		t.Fatal("exchange returned an empty session id")
	}
	if _, err := store.AppStore.RotateCliRefreshToken(ctx, firstRefreshHash, secondRefreshHash, now.Add(time.Minute)); err != nil {
		t.Fatalf("RotateCliRefreshToken: %v", err)
	}
	if _, err := store.AppStore.RotateCliRefreshToken(ctx, firstRefreshHash, bytes.Repeat([]byte{5}, 32), now.Add(2*time.Minute)); !errors.Is(err, store.ErrRefreshTokenReuse) {
		t.Fatalf("old-token reuse: got %v, want ErrRefreshTokenReuse", err)
	}
	sessions, err := store.AppStore.ListCliSessions(ctx, "example.test", "user-1")
	if err != nil {
		t.Fatalf("ListCliSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].RevokedAt == nil {
		t.Fatalf("session was not revoked after reuse: %#v", sessions)
	}
}

func TestBrowserRefreshRotation_Postgres_GraceKindAndRevoke(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	first := bytes.Repeat([]byte{11}, 32)
	second := bytes.Repeat([]byte{12}, 32)
	third := bytes.Repeat([]byte{13}, 32)

	session := &models.CliSession{
		SubjectDomain: "example.test",
		SubjectUserID: "user-1",
		DisplayName:   "Test User",
		ClientName:    "Firefox on Linux",
		CreatedAt:     now,
		LastUsedAt:    now,
		ExpiresAt:     now.Add(time.Hour),
	}
	if err := store.AppStore.CreateBrowserSession(ctx, session, first); err != nil {
		t.Fatalf("CreateBrowserSession: %v", err)
	}
	if session.Kind != models.SessionKindBrowser {
		t.Fatalf("kind: got %q", session.Kind)
	}

	// A browser token is not a CLI token.
	if _, err := store.AppStore.RotateCliRefreshToken(ctx, first, third, now); !errors.Is(err, store.ErrCliSessionInvalid) {
		t.Fatalf("CLI rotate of a browser token: got %v, want ErrCliSessionInvalid", err)
	}

	if _, err := store.AppStore.RotateBrowserRefreshToken(ctx, first, second, now.Add(time.Second), time.Minute); err != nil {
		t.Fatalf("RotateBrowserRefreshToken: %v", err)
	}
	// A second tab with the old cookie inside the grace window.
	got, err := store.AppStore.RotateBrowserRefreshToken(ctx, first, third, now.Add(10*time.Second), time.Minute)
	if !errors.Is(err, store.ErrRefreshTokenRecentlyUsed) || got == nil || got.SessionID != session.SessionID {
		t.Fatalf("grace reuse: got %v, %v", got, err)
	}

	newExpiry := now.Add(48 * time.Hour)
	if err := store.AppStore.ExtendBrowserSession(ctx, session.SessionID, newExpiry); err != nil {
		t.Fatalf("ExtendBrowserSession: %v", err)
	}
	loaded, err := store.AppStore.GetCliSession(ctx, session.SessionID)
	if err != nil || !loaded.ExpiresAt.Equal(newExpiry) {
		t.Fatalf("extended session: %#v, %v", loaded, err)
	}

	// The same old cookie after the grace window is theft: revoke.
	if _, err := store.AppStore.RotateBrowserRefreshToken(ctx, first, third, now.Add(5*time.Minute), time.Minute); !errors.Is(err, store.ErrRefreshTokenReuse) {
		t.Fatalf("late reuse: got %v, want ErrRefreshTokenReuse", err)
	}
	if _, err := store.AppStore.RotateBrowserRefreshToken(ctx, second, third, now.Add(6*time.Minute), time.Minute); !errors.Is(err, store.ErrCliSessionInvalid) {
		t.Fatalf("rotate after revoke: got %v, want ErrCliSessionInvalid", err)
	}

	// Revoke-by-token on a fresh session.
	other := &models.CliSession{
		SubjectDomain: "example.test", SubjectUserID: "user-1", ClientName: "Safari on iPhone",
		CreatedAt: now, LastUsedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	otherToken := bytes.Repeat([]byte{21}, 32)
	if err := store.AppStore.CreateBrowserSession(ctx, other, otherToken); err != nil {
		t.Fatalf("CreateBrowserSession: %v", err)
	}
	revoked, err := store.AppStore.RevokeBrowserSessionByToken(ctx, otherToken, now)
	if err != nil || revoked.SessionID != other.SessionID {
		t.Fatalf("RevokeBrowserSessionByToken: %#v, %v", revoked, err)
	}
	if _, err := store.AppStore.RotateBrowserRefreshToken(ctx, otherToken, third, now.Add(time.Second), time.Minute); !errors.Is(err, store.ErrCliSessionInvalid) {
		t.Fatalf("rotate after end-session: got %v, want ErrCliSessionInvalid", err)
	}
}
