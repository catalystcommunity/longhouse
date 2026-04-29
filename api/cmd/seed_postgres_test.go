//go:build integration

package cmd

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres"
	_ "github.com/jackc/pgx/v4/stdlib"
)

// These tests exercise SeedInitialAdmin against a real Postgres running
// inside the CI job (see .reactorcide/jobs/api-test-postgres.yaml). They
// are gated behind the `integration` build tag so `go test ./...` stays
// fast on dev boxes that don't have Postgres.
//
// Once the build tag is enabled, we expect a working test database — these
// tests fail loudly rather than skipping, because under -tags=integration
// the absence of a DB is a configuration error, not "this box doesn't run
// integration tests."
//
// Run locally with:
//   LONGHOUSE_TEST_DB_URI=postgres://runner@127.0.0.1/longhouse_test \
//   go test -tags=integration ./cmd/...

func requireTestDB(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("LONGHOUSE_TEST_DB_URI")
	if uri == "" {
		t.Fatal("LONGHOUSE_TEST_DB_URI is not set; integration tests need a Postgres test DB")
	}
	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatalf("LONGHOUSE_TEST_DB_URI invalid: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("cannot reach test DB at %s: %v", uri, err)
	}
	return uri
}

// freshStore drops and recreates the public schema on the target database,
// then reruns migrations and re-initializes the global store. This gives
// each test a clean slate without needing separate DB instances.
func freshStore(t *testing.T, uri string) {
	t.Helper()

	prevURI := config.DbUri
	config.DbUri = uri
	t.Cleanup(func() { config.DbUri = prevURI })

	// Use a short-lived raw connection to reset the schema. Migrations run
	// as DDL, so we nuke whatever was there first.
	resetSchema(t, uri)

	if err := RunMigrations(); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	prev := store.AppStore
	s := &postgres.PostgresStore{}
	cleanup, err := s.Initialize()
	if err != nil {
		t.Fatalf("store.Initialize: %v", err)
	}
	store.AppStore = s
	t.Cleanup(func() {
		store.AppStore = prev
		if cleanup != nil {
			cleanup()
		}
	})
}

func resetSchema(t *testing.T, uri string) {
	t.Helper()
	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	stmts := []string{
		`DROP SCHEMA IF EXISTS public CASCADE`,
		`CREATE SCHEMA public`,
		`GRANT ALL ON SCHEMA public TO public`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("reset schema (%q): %v", s, err)
		}
	}
}

func TestSeedInitialAdmin_Postgres_CreatesHouseAndAdmin(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)

	const domain = "integration.example"
	const userID = "01234567-89ab-4def-8123-456789abcdef"
	withConfig(t, domain, userID, "Integration House")

	if err := SeedInitialAdmin(); err != nil {
		t.Fatalf("SeedInitialAdmin: %v", err)
	}

	ctx := context.Background()
	houses, err := store.AppStore.ListHouses(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListHouses: %v", err)
	}
	if len(houses) != 1 {
		t.Fatalf("want 1 house, got %d", len(houses))
	}
	if houses[0].Name != "Integration House" {
		t.Errorf("house name: %q", houses[0].Name)
	}
	if houses[0].HouseID == "" {
		t.Errorf("house_id should be server-generated, got empty")
	}

	m, err := store.AppStore.GetMemberByIdentity(ctx, houses[0].HouseID, domain, userID)
	if err != nil {
		t.Fatalf("GetMemberByIdentity: %v", err)
	}
	if m.MemberID == "" {
		t.Errorf("member_id should be server-generated, got empty")
	}
	hasAdmin := false
	for _, r := range m.Roles {
		if r == "admin" {
			hasAdmin = true
		}
	}
	if !hasAdmin {
		t.Errorf("member missing admin role: %v", m.Roles)
	}
}

func TestSeedInitialAdmin_Postgres_Idempotent(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)

	withConfig(t, "integration.example", "01234567-89ab-4def-8123-456789abcdef", "Integration House")

	if err := SeedInitialAdmin(); err != nil {
		t.Fatalf("first SeedInitialAdmin: %v", err)
	}
	if err := SeedInitialAdmin(); err != nil {
		t.Fatalf("second SeedInitialAdmin: %v", err)
	}

	ctx := context.Background()
	houses, err := store.AppStore.ListHouses(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListHouses: %v", err)
	}
	if len(houses) != 1 {
		t.Fatalf("want 1 house after re-run, got %d", len(houses))
	}
	members, err := store.AppStore.ListMembersByHouse(ctx, houses[0].HouseID, 10, 0)
	if err != nil {
		t.Fatalf("ListMembersByHouse: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("want 1 member after re-run, got %d", len(members))
	}
}

func TestSeedInitialAdmin_Postgres_NoOpOnInvalidUUID(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)

	withConfig(t, "integration.example", "not-a-uuid", "Integration House")

	if err := SeedInitialAdmin(); err != nil {
		t.Fatalf("SeedInitialAdmin: %v", err)
	}

	ctx := context.Background()
	houses, err := store.AppStore.ListHouses(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListHouses: %v", err)
	}
	if len(houses) != 0 {
		t.Errorf("want 0 houses (invalid UUID should skip), got %d", len(houses))
	}
}

