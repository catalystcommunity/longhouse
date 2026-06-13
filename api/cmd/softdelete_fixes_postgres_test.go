//go:build integration

package cmd

import (
	"context"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// Locks in the deferred-item fixes: GetXByID hides trashed rows (so mutators
// reject trashed targets), member purge is blocked by dependents instead of
// FK-erroring, and the delete-op audit detail is retrievable for restore re-arm.

func TestGetByID_HidesTrashedRow_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Hide House")
	actor := mkMember(t, ctx, house, "actor")

	p := &models.Project{HouseID: house, Name: "P", Visibility: "read"}
	if err := store.AppStore.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	// Live: fetchable.
	if _, err := store.AppStore.GetProjectByID(ctx, p.ProjectID); err != nil {
		t.Fatalf("live GetProjectByID: %v", err)
	}
	op, _ := store.AppStore.NewID(ctx)
	if err := store.AppStore.SoftDeleteProject(ctx, p.ProjectID, actor, op); err != nil {
		t.Fatalf("SoftDeleteProject: %v", err)
	}
	// Trashed: GetByID now reports not-found, so every mutator that fetches by
	// id rejects it.
	if _, err := store.AppStore.GetProjectByID(ctx, p.ProjectID); err == nil {
		t.Fatalf("GetProjectByID returned a trashed project; want not-found")
	}
}

func TestMember_DeactivateReactivate_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Deact House")
	admin := mkMember(t, ctx, house, "admin")
	owner := mkMember(t, ctx, house, "owner")

	// The member owns a task; deactivation must not touch that content.
	task := &models.Task{HouseID: house, OwnerMemberID: owner, Title: "owned", Visibility: "read", Status: "open"}
	if err := store.AppStore.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := store.AppStore.DeactivateMember(ctx, owner, admin); err != nil {
		t.Fatalf("DeactivateMember: %v", err)
	}
	// Member record + owned task still exist; the member is just flagged.
	m, err := store.AppStore.GetMemberByID(ctx, owner)
	if err != nil {
		t.Fatalf("GetMemberByID after deactivate: %v", err)
	}
	if m.DeactivatedAt == nil {
		t.Fatalf("DeactivatedAt not set")
	}
	if m.DeactivatedByMemberID == nil || *m.DeactivatedByMemberID != admin {
		t.Fatalf("DeactivatedByMemberID not recorded: %+v", m.DeactivatedByMemberID)
	}
	if _, err := store.AppStore.GetTaskByID(ctx, task.TaskID); err != nil {
		t.Fatalf("owned task should be untouched by deactivation: %v", err)
	}
	// Deactivated members still appear in the house list (admins manage them).
	list, _ := store.AppStore.ListMembersByHouse(ctx, house, 50, 0)
	if len(list) != 2 {
		t.Fatalf("member list = %d, want 2 (deactivation doesn't hide)", len(list))
	}

	if err := store.AppStore.ReactivateMember(ctx, owner); err != nil {
		t.Fatalf("ReactivateMember: %v", err)
	}
	m, _ = store.AppStore.GetMemberByID(ctx, owner)
	if m.DeactivatedAt != nil {
		t.Fatalf("DeactivatedAt not cleared on reactivate")
	}
}

func TestGetDeleteOpDetail_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Detail House")
	op, _ := store.AppStore.NewID(ctx)

	// Simulate the delete audit entry a "this & future" event delete records.
	if err := store.AppStore.RecordAuditEntry(ctx, &models.AuditEntry{
		HouseID: &house, Service: "event", Method: "DeleteEventAndFuture",
		Action: models.AuditActionDelete, Outcome: models.AuditOutcomeOK,
		Detail: models.JSONMap{
			"deleted_op_id":              op,
			"mode":                       "this_and_future",
			"root_event_id":              "root-123",
			"root_prior_recurrence_freq": "weekly",
		},
	}); err != nil {
		t.Fatalf("RecordAuditEntry: %v", err)
	}
	detail, err := store.AppStore.GetDeleteOpDetail(ctx, house, op)
	if err != nil {
		t.Fatalf("GetDeleteOpDetail: %v", err)
	}
	if detail == nil || detail["root_event_id"] != "root-123" || detail["root_prior_recurrence_freq"] != "weekly" {
		t.Fatalf("detail = %+v, want the recorded re-arm fields", detail)
	}
}
