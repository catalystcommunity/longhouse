//go:build integration

package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// Validates the partitioned audit_log (migration 000015): inserts route to a
// monthly partition and return the generated id, house-scoped + filtered +
// keyset queries work, the global (house_id NULL) scope is separate, and the
// partition-maintenance worker SQL creates/drops partitions.

func sp(s string) *string { return &s }

func TestAudit_RecordListFilterAndScope_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Audit House")
	actor := mkMember(t, ctx, house, "auditor")

	for i := 0; i < 3; i++ {
		e := &models.AuditEntry{
			HouseID: &house, ActorMemberID: &actor,
			Service: "task", Method: "DeleteTask", Action: models.AuditActionDelete,
			ResourceType: sp("task"), Outcome: models.AuditOutcomeOK,
		}
		if err := store.AppStore.RecordAuditEntry(ctx, e); err != nil {
			t.Fatalf("RecordAuditEntry: %v", err)
		}
		if e.AuditID == "" {
			t.Fatalf("audit_id not populated on insert (RETURNING on partitioned table)")
		}
		if e.CreatedAt.IsZero() {
			t.Fatalf("created_at not stamped")
		}
	}
	// Unattributable failure → global (house_id NULL) scope.
	if err := store.AppStore.RecordAuditEntry(ctx, &models.AuditEntry{
		Service: "auth", Action: models.AuditActionLoginFailed, Outcome: models.AuditOutcomeDenied,
	}); err != nil {
		t.Fatalf("RecordAuditEntry(global): %v", err)
	}

	// House scope sees only its 3 rows.
	rows, err := store.AppStore.ListAuditEntries(ctx, house, models.AuditFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("house scope = %d rows, want 3", len(rows))
	}
	// Action filter that matches nothing.
	if none, _ := store.AppStore.ListAuditEntries(ctx, house, models.AuditFilter{Action: sp("create"), Limit: 10}); len(none) != 0 {
		t.Fatalf("action filter leaked %d rows", len(none))
	}
	// Global scope is separate and holds the one failure.
	g, err := store.AppStore.ListAuditEntries(ctx, "", models.AuditFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListAuditEntries(global): %v", err)
	}
	if len(g) != 1 || g[0].Action != models.AuditActionLoginFailed {
		t.Fatalf("global scope = %+v, want one login_failed", g)
	}

	// Keyset pagination: first page of 2, then the rest via the cursor.
	page1, _ := store.AppStore.ListAuditEntries(ctx, house, models.AuditFilter{Limit: 2})
	if len(page1) != 2 {
		t.Fatalf("page1 = %d, want 2", len(page1))
	}
	if page1[0].CreatedAt.Before(page1[1].CreatedAt) {
		t.Fatalf("results not newest-first")
	}
	last := page1[len(page1)-1]
	page2, _ := store.AppStore.ListAuditEntries(ctx, house, models.AuditFilter{
		Limit: 10, CursorCreatedAt: &last.CreatedAt, CursorAuditID: &last.AuditID,
	})
	if len(page2) != 1 {
		t.Fatalf("page2 = %d, want 1 (the remaining row)", len(page2))
	}
}

func TestAudit_PartitionMaintenance_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	now := time.Now().UTC()

	// Idempotent: creating ahead twice must not error.
	if err := store.AppStore.CreateAuditPartitionsAhead(ctx, now, 3); err != nil {
		t.Fatalf("CreateAuditPartitionsAhead: %v", err)
	}
	if err := store.AppStore.CreateAuditPartitionsAhead(ctx, now, 3); err != nil {
		t.Fatalf("CreateAuditPartitionsAhead(again): %v", err)
	}
	// A huge retention window drops nothing recent.
	dropped, err := store.AppStore.DropAuditPartitionsBefore(ctx, now, 240)
	if err != nil {
		t.Fatalf("DropAuditPartitionsBefore: %v", err)
	}
	if dropped != 0 {
		t.Fatalf("dropped %d recent partitions, want 0", dropped)
	}
	// Pretending "now" is far in the future drops the partitions we created.
	future := now.AddDate(5, 0, 0)
	dropped, err = store.AppStore.DropAuditPartitionsBefore(ctx, future, 1)
	if err != nil {
		t.Fatalf("DropAuditPartitionsBefore(future): %v", err)
	}
	if dropped == 0 {
		t.Fatalf("expected to drop aged-out partitions, dropped 0")
	}
}
