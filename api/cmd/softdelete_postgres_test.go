//go:build integration

package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// Exercises the soft-delete / trash / restore / purge store SQL against a real
// Postgres (which also validates migration 000014). Run with:
//   LONGHOUSE_TEST_DB_URI=postgres://runner@127.0.0.1/longhouse_test \
//   go test -tags=integration ./cmd/...

func mkMember(t *testing.T, ctx context.Context, houseID, userID string) string {
	t.Helper()
	m := &models.Member{
		HouseID:        houseID,
		LinkkeysDomain: "soft.test",
		LinkkeysUserID: userID,
		DisplayName:    userID,
	}
	if err := store.AppStore.CreateMember(ctx, m); err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	return m.MemberID
}

func TestSoftDelete_ProjectTrashRestorePurge_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "SD House")
	actor := mkMember(t, ctx, house, "actor")

	p := &models.Project{HouseID: house, Name: "Doomed", Visibility: "read"}
	if err := store.AppStore.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if list, _ := store.AppStore.ListProjectsByHouse(ctx, house, 50, 0); len(list) != 1 {
		t.Fatalf("want 1 live project, got %d", len(list))
	}

	// Soft delete and confirm it leaves the live list but lands in the trash
	// with the actor + op recorded.
	opID, err := store.AppStore.NewID(ctx)
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	if err := store.AppStore.SoftDeleteProject(ctx, p.ProjectID, actor, opID); err != nil {
		t.Fatalf("SoftDeleteProject: %v", err)
	}
	if list, _ := store.AppStore.ListProjectsByHouse(ctx, house, 50, 0); len(list) != 0 {
		t.Fatalf("soft-deleted project still listed (%d)", len(list))
	}
	trash, err := store.AppStore.ListTrash(ctx, house, 50, 0)
	if err != nil {
		t.Fatalf("ListTrash: %v", err)
	}
	if len(trash) != 1 || trash[0].ResourceType != "project" || trash[0].ResourceID != p.ProjectID {
		t.Fatalf("trash = %+v, want one project row", trash)
	}
	if trash[0].Title != "Doomed" {
		t.Fatalf("trash title = %q, want Doomed", trash[0].Title)
	}
	if trash[0].DeletedByMemberID == nil || *trash[0].DeletedByMemberID != actor {
		t.Fatalf("deleted_by_member_id not recorded: %+v", trash[0].DeletedByMemberID)
	}

	// ResourceHouseID + FindDeletedOpID resolve correctly.
	if h, _ := store.AppStore.ResourceHouseID(ctx, "project", p.ProjectID); h != house {
		t.Fatalf("ResourceHouseID = %q, want %q", h, house)
	}
	if got, _ := store.AppStore.FindDeletedOpID(ctx, "project", p.ProjectID); got != opID {
		t.Fatalf("FindDeletedOpID = %q, want %q", got, opID)
	}

	// Restore by op brings it back to the live list.
	if err := store.AppStore.RestoreProjectsByOp(ctx, opID); err != nil {
		t.Fatalf("RestoreProjectsByOp: %v", err)
	}
	if list, _ := store.AppStore.ListProjectsByHouse(ctx, house, 50, 0); len(list) != 1 {
		t.Fatalf("restore failed: %d live projects", len(list))
	}

	// Soft delete again, then purge everything older than the future cutoff.
	op2, _ := store.AppStore.NewID(ctx)
	if err := store.AppStore.SoftDeleteProject(ctx, p.ProjectID, actor, op2); err != nil {
		t.Fatalf("SoftDeleteProject(2): %v", err)
	}
	n, err := store.AppStore.PurgeAllSoftDeletedBefore(ctx, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("PurgeAllSoftDeletedBefore: %v", err)
	}
	if n < 1 {
		t.Fatalf("purge removed %d rows, want >= 1", n)
	}
	if _, err := store.AppStore.GetProjectByID(ctx, p.ProjectID); err == nil {
		t.Fatalf("project still exists after purge")
	}
	if trash, _ := store.AppStore.ListTrash(ctx, house, 50, 0); len(trash) != 0 {
		t.Fatalf("trash not empty after purge: %+v", trash)
	}
}

func TestSoftDelete_TaskPurgeResource_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Task SD House")
	owner := mkMember(t, ctx, house, "owner")

	task := &models.Task{HouseID: house, OwnerMemberID: owner, Title: "Trash me", Visibility: "read", Status: "open"}
	if err := store.AppStore.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	op, _ := store.AppStore.NewID(ctx)
	if err := store.AppStore.SoftDeleteTask(ctx, task.TaskID, owner, op); err != nil {
		t.Fatalf("SoftDeleteTask: %v", err)
	}
	if list, _ := store.AppStore.ListTasksByHouse(ctx, house, 50, 0); len(list) != 0 {
		t.Fatalf("soft-deleted task still listed")
	}
	// PurgeResource (the admin "purge now" path) hard-deletes the single item.
	if err := store.AppStore.PurgeResource(ctx, "task", task.TaskID); err != nil {
		t.Fatalf("PurgeResource: %v", err)
	}
	if _, err := store.AppStore.GetTaskByID(ctx, task.TaskID); err == nil {
		t.Fatalf("task still exists after PurgeResource")
	}
}
