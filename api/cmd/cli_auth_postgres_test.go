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
