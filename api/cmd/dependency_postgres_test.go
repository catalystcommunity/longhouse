//go:build integration

package cmd

import (
	"context"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// Exercises the dependency store SQL against a real Postgres: forward/reverse
// listing, ON CONFLICT idempotency, the recursive-CTE cycle check, node
// cleanup, and the house_id FK cascade.
//
// Run locally with:
//   LONGHOUSE_TEST_DB_URI=postgres://runner@127.0.0.1/longhouse_test \
//   go test -tags=integration ./cmd/...

// Fixed uuids for edge endpoints. The dependencies table has no FK to
// tasks/projects, so these need only be valid uuids, not real rows.
const (
	nA = "11111111-1111-4111-8111-111111111111"
	nB = "22222222-2222-4222-8222-222222222222"
	nC = "33333333-3333-4333-8333-333333333333"
)

func edge(houseID, dt, di, ct, ci string) *models.Dependency {
	return &models.Dependency{
		HouseID:        houseID,
		DependentType:  dt,
		DependentID:    di,
		DependencyType: ct,
		DependencyID:   ci,
	}
}

func mkHouse(t *testing.T, ctx context.Context, name string) string {
	t.Helper()
	h := &models.House{Name: name}
	if err := store.AppStore.CreateHouse(ctx, h); err != nil {
		t.Fatalf("CreateHouse: %v", err)
	}
	return h.HouseID
}

func TestDependencies_ForwardAndReverse_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Deps House")

	// A (task) depends on B (project).
	if err := store.AppStore.AddDependency(ctx, edge(house, "task", nA, "project", nB)); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	fwd, err := store.AppStore.ListDependencies(ctx, "task", nA)
	if err != nil {
		t.Fatalf("ListDependencies: %v", err)
	}
	if len(fwd) != 1 || fwd[0].DependencyID != nB || fwd[0].DependencyType != "project" {
		t.Fatalf("forward = %+v, want one edge to project nB", fwd)
	}

	rev, err := store.AppStore.ListDependents(ctx, "project", nB)
	if err != nil {
		t.Fatalf("ListDependents: %v", err)
	}
	if len(rev) != 1 || rev[0].DependentID != nA || rev[0].DependentType != "task" {
		t.Fatalf("reverse = %+v, want one edge from task nA", rev)
	}

	// A node with no edges in a direction returns empty, not error.
	if got, _ := store.AppStore.ListDependents(ctx, "task", nA); len(got) != 0 {
		t.Fatalf("nA dependents = %+v, want none", got)
	}
}

func TestDependencies_Idempotent_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Deps House")

	for i := 0; i < 3; i++ {
		if err := store.AppStore.AddDependency(ctx, edge(house, "task", nA, "task", nB)); err != nil {
			t.Fatalf("AddDependency %d: %v", i, err)
		}
	}
	fwd, _ := store.AppStore.ListDependencies(ctx, "task", nA)
	if len(fwd) != 1 {
		t.Fatalf("edges = %d, want 1 (ON CONFLICT DO NOTHING)", len(fwd))
	}
}

func TestDependencies_PathExists_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Deps House")

	// A -> B -> C (task A depends on task B depends on task C).
	mustAdd(t, ctx, edge(house, "task", nA, "task", nB))
	mustAdd(t, ctx, edge(house, "task", nB, "task", nC))

	// Forward reachability holds transitively.
	if ok, _ := store.AppStore.DependencyPathExists(ctx, "task", nA, "task", nC); !ok {
		t.Fatal("expected path A -> C")
	}
	// The reverse is NOT reachable along the directed edges...
	if ok, _ := store.AppStore.DependencyPathExists(ctx, "task", nC, "task", nA); ok {
		t.Fatal("did not expect path C -> A")
	}
	// ...which is exactly the check the handler uses to reject C -> A as a
	// cycle (path from the proposed dependency A back to dependent C is what
	// would matter; here A reaches C, so adding C depends-on A would close it).
	if ok, _ := store.AppStore.DependencyPathExists(ctx, "task", nA, "task", nC); !ok {
		t.Fatal("cycle precondition: A must reach C")
	}
}

func TestDependencies_RemoveAndCleanup_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Deps House")

	mustAdd(t, ctx, edge(house, "task", nA, "task", nB)) // nB is a dependency of nA
	mustAdd(t, ctx, edge(house, "task", nC, "task", nB)) // nB is a dependency of nC too
	mustAdd(t, ctx, edge(house, "task", nB, "task", nA)) // nB depends on nA

	// Remove a single edge.
	if err := store.AppStore.RemoveDependency(ctx, "task", nA, "task", nB); err != nil {
		t.Fatalf("RemoveDependency: %v", err)
	}
	if got, _ := store.AppStore.ListDependencies(ctx, "task", nA); len(got) != 0 {
		t.Fatalf("after remove, nA deps = %+v, want none", got)
	}

	// RemoveDependenciesForNode clears every edge touching nB in either slot.
	if err := store.AppStore.RemoveDependenciesForNode(ctx, "task", nB); err != nil {
		t.Fatalf("RemoveDependenciesForNode: %v", err)
	}
	if got, _ := store.AppStore.ListDependents(ctx, "task", nB); len(got) != 0 {
		t.Fatalf("nB dependents = %+v, want none", got)
	}
	if got, _ := store.AppStore.ListDependencies(ctx, "task", nB); len(got) != 0 {
		t.Fatalf("nB dependencies = %+v, want none", got)
	}
}

func TestDependencies_HouseCascade_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Deps House")

	mustAdd(t, ctx, edge(house, "task", nA, "task", nB))

	if err := store.AppStore.DeleteHouse(ctx, house); err != nil {
		t.Fatalf("DeleteHouse: %v", err)
	}
	// The house_id FK cascade should have removed the edge.
	if got, _ := store.AppStore.ListDependencies(ctx, "task", nA); len(got) != 0 {
		t.Fatalf("after house delete, edges = %+v, want none (cascade)", got)
	}
}

func mustAdd(t *testing.T, ctx context.Context, e *models.Dependency) {
	t.Helper()
	if err := store.AppStore.AddDependency(ctx, e); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
}
