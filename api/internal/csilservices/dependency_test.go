package csilservices

import (
	"context"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"github.com/fxamacker/cbor/v2"
)

// fakeDepStore implements just the slice of the Store surface the
// DependencyService + its policy resolution touch. Everything else is the
// embedded (nil) interface and panics if hit, keeping the test honest about
// its dependencies. Access resolution is driven entirely by task ownership /
// visibility and project ownership / visibility, since there are no grants,
// groups, ancestors, or containing projects in these fixtures.
type fakeDepStore struct {
	store.Store
	tasks    map[string]*models.Task
	projects map[string]*models.Project
	edges    []models.Dependency
}

func (f *fakeDepStore) GetTaskByID(_ context.Context, id string) (*models.Task, error) {
	if t, ok := f.tasks[id]; ok {
		return t, nil
	}
	return nil, errNotFound
}

func (f *fakeDepStore) GetProjectByID(_ context.Context, id string) (*models.Project, error) {
	if p, ok := f.projects[id]; ok {
		return p, nil
	}
	return nil, errNotFound
}

// Policy-resolution stubs: empty everywhere so access = own visibility +
// owner/admin only.
func (f *fakeDepStore) ListGroupIDsForMember(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (f *fakeDepStore) GetTaskAncestors(_ context.Context, _ string) ([]models.Task, error) {
	return nil, nil
}
func (f *fakeDepStore) ListTaskGrants(_ context.Context, _ string) ([]models.TaskGrant, error) {
	return nil, nil
}
func (f *fakeDepStore) ListProjectsForTask(_ context.Context, _ string) ([]models.Project, error) {
	return nil, nil
}
func (f *fakeDepStore) ListProjectGrants(_ context.Context, _ string) ([]models.ProjectGrant, error) {
	return nil, nil
}

func (f *fakeDepStore) AddDependency(_ context.Context, dep *models.Dependency) error {
	for _, e := range f.edges {
		if e.DependentType == dep.DependentType && e.DependentID == dep.DependentID &&
			e.DependencyType == dep.DependencyType && e.DependencyID == dep.DependencyID {
			return nil // ON CONFLICT DO NOTHING
		}
	}
	f.edges = append(f.edges, *dep)
	return nil
}

func (f *fakeDepStore) RemoveDependency(_ context.Context, dt, di, ct, ci string) error {
	out := f.edges[:0]
	for _, e := range f.edges {
		if e.DependentType == dt && e.DependentID == di && e.DependencyType == ct && e.DependencyID == ci {
			continue
		}
		out = append(out, e)
	}
	f.edges = out
	return nil
}

func (f *fakeDepStore) ListDependencies(_ context.Context, nt, ni string) ([]models.Dependency, error) {
	var out []models.Dependency
	for _, e := range f.edges {
		if e.DependentType == nt && e.DependentID == ni {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeDepStore) ListDependents(_ context.Context, nt, ni string) ([]models.Dependency, error) {
	var out []models.Dependency
	for _, e := range f.edges {
		if e.DependencyType == nt && e.DependencyID == ni {
			out = append(out, e)
		}
	}
	return out, nil
}

// DependencyPathExists mirrors the DB CTE: BFS forward from (ft,fi) along
// dependent->dependency edges, reporting whether (tt,ti) is reachable.
func (f *fakeDepStore) DependencyPathExists(_ context.Context, ft, fi, tt, ti string) (bool, error) {
	type node struct{ t, i string }
	seen := map[node]bool{}
	queue := []node{{ft, fi}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.t == tt && cur.i == ti {
			return true, nil
		}
		if seen[cur] {
			continue
		}
		seen[cur] = true
		for _, e := range f.edges {
			if e.DependentType == cur.t && e.DependentID == cur.i {
				queue = append(queue, node{e.DependencyType, e.DependencyID})
			}
		}
	}
	return false, nil
}

// ---- helpers ----------------------------------------------------------

var testEnc, _ = cbor.CoreDetEncOptions().EncMode()

func enc(t *testing.T, v any) []byte {
	t.Helper()
	b, err := testEnc.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// ctxAs builds a request context carrying an identity for `member` in house
// `houseID` with the given roles.
func ctxAs(houseID, member string, roles ...string) context.Context {
	id := &auth.Identity{
		Domain: "test",
		UserID: "u-" + member,
		Houses: []auth.HouseRoles{{House: houseID, Member: member, Roles: roles}},
	}
	return auth.WithIdentity(context.Background(), id)
}

func ref(dt, di, ct, ci string) csil.DependencyRef {
	return csil.DependencyRef{
		DependentType:  csil.DependencyNodeType(dt),
		DependentId:    di,
		DependencyType: csil.DependencyNodeType(ct),
		DependencyId:   ci,
	}
}

func nodeIDs(ns []csil.DependencyNode) map[string]bool {
	m := map[string]bool{}
	for _, n := range ns {
		m[n.Id] = true
	}
	return m
}

// errCode pulls the HTTP status out of a csilrpc error, or 0 if not one.
func errCode(err error) int {
	if e, ok := err.(*csilrpc.Error); ok {
		return e.Code
	}
	return 0
}

// ---- tests ------------------------------------------------------------

const h1 = "house-1"

func TestAddAndGetDependency(t *testing.T) {
	fs := &fakeDepStore{
		tasks: map[string]*models.Task{
			"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "me", Status: "open"},
			"b": {TaskID: "b", HouseID: h1, Title: "B", OwnerMemberID: "me", Status: "done"},
		},
	}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")

	// A depends on B.
	if _, err := svc.AddDependency(ctx, ref("task", "a", "task", "b")); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Forward view of A: depends on B; nothing depends on A.
	out, err := svc.GetDependencies(ctx, csil.DependencyTarget{Type: "task", Id: "a"})
	if err != nil {
		t.Fatalf("get a: %v", err)
	}
	g := out
	if !nodeIDs(g.Dependencies)["b"] || len(g.Dependencies) != 1 {
		t.Fatalf("A.dependencies = %+v, want [b]", g.Dependencies)
	}
	if g.Dependencies[0].Title != "B" {
		t.Fatalf("enriched title = %q, want B", g.Dependencies[0].Title)
	}
	if len(g.Dependents) != 0 {
		t.Fatalf("A.dependents = %+v, want []", g.Dependents)
	}

	// Reverse view of B: A depends on B.
	out, err = svc.GetDependencies(ctx, csil.DependencyTarget{Type: "task", Id: "b"})
	if err != nil {
		t.Fatalf("get b: %v", err)
	}
	g = out
	if !nodeIDs(g.Dependents)["a"] || len(g.Dependents) != 1 {
		t.Fatalf("B.dependents = %+v, want [a]", g.Dependents)
	}
	if len(g.Dependencies) != 0 {
		t.Fatalf("B.dependencies = %+v, want []", g.Dependencies)
	}
}

func TestAddDependencyIsIdempotent(t *testing.T) {
	fs := &fakeDepStore{tasks: map[string]*models.Task{
		"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "me"},
		"b": {TaskID: "b", HouseID: h1, Title: "B", OwnerMemberID: "me"},
	}}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")
	body := ref("task", "a", "task", "b")
	for i := 0; i < 2; i++ {
		if _, err := svc.AddDependency(ctx, body); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	if len(fs.edges) != 1 {
		t.Fatalf("edges = %d, want 1 (dedup)", len(fs.edges))
	}
}

func TestSelfDependencyRejected(t *testing.T) {
	fs := &fakeDepStore{tasks: map[string]*models.Task{
		"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "me"},
	}}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")
	_, err := svc.AddDependency(ctx, ref("task", "a", "task", "a"))
	if errCode(err) != 400 {
		t.Fatalf("self-dep: got %v (code %d), want 400", err, errCode(err))
	}
}

func TestCycleRejected(t *testing.T) {
	fs := &fakeDepStore{tasks: map[string]*models.Task{
		"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "me"},
		"b": {TaskID: "b", HouseID: h1, Title: "B", OwnerMemberID: "me"},
		"c": {TaskID: "c", HouseID: h1, Title: "C", OwnerMemberID: "me"},
	}}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")
	// A->B->C, then C->A would close a loop.
	if _, err := svc.AddDependency(ctx, ref("task", "a", "task", "b")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddDependency(ctx, ref("task", "b", "task", "c")); err != nil {
		t.Fatal(err)
	}
	_, err := svc.AddDependency(ctx, ref("task", "c", "task", "a"))
	if errCode(err) != 409 {
		t.Fatalf("cycle: got %v (code %d), want 409", err, errCode(err))
	}
}

func TestAddRequiresEditOnDependent(t *testing.T) {
	fs := &fakeDepStore{tasks: map[string]*models.Task{
		// Caller is NOT the owner and visibility is read → read but not edit.
		"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "other", Visibility: "read"},
		"b": {TaskID: "b", HouseID: h1, Title: "B", OwnerMemberID: "me"},
	}}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")
	_, err := svc.AddDependency(ctx, ref("task", "a", "task", "b"))
	if errCode(err) != 403 {
		t.Fatalf("edit-gate: got %v (code %d), want 403", err, errCode(err))
	}
}

func TestAddRequiresReadOnDependency(t *testing.T) {
	fs := &fakeDepStore{tasks: map[string]*models.Task{
		"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "me"},
		// Caller can't read B (not owner, visibility none).
		"b": {TaskID: "b", HouseID: h1, Title: "B", OwnerMemberID: "other", Visibility: "none"},
	}}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")
	_, err := svc.AddDependency(ctx, ref("task", "a", "task", "b"))
	// Unreadable dependency is reported as not-found (don't leak existence).
	if errCode(err) != 404 {
		t.Fatalf("read-gate: got %v (code %d), want 404", err, errCode(err))
	}
}

func TestCrossHouseRejected(t *testing.T) {
	fs := &fakeDepStore{tasks: map[string]*models.Task{
		"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "me"},
		"b": {TaskID: "b", HouseID: "house-2", Title: "B", OwnerMemberID: "me"},
	}}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")
	_, err := svc.AddDependency(ctx, ref("task", "a", "task", "b"))
	if errCode(err) != 400 {
		t.Fatalf("cross-house: got %v (code %d), want 400", err, errCode(err))
	}
}

func TestGetFiltersUnreadableNodes(t *testing.T) {
	fs := &fakeDepStore{tasks: map[string]*models.Task{
		"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "me"},
		// Pre-existing edge to a now-unreadable node (e.g. it became private).
		"secret": {TaskID: "secret", HouseID: h1, Title: "Secret", OwnerMemberID: "other", Visibility: "none"},
		"pub":    {TaskID: "pub", HouseID: h1, Title: "Public", OwnerMemberID: "other", Visibility: "read"},
	}}
	fs.edges = []models.Dependency{
		{HouseID: h1, DependentType: "task", DependentID: "a", DependencyType: "task", DependencyID: "secret"},
		{HouseID: h1, DependentType: "task", DependentID: "a", DependencyType: "task", DependencyID: "pub"},
	}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")
	out, err := svc.GetDependencies(ctx, csil.DependencyTarget{Type: "task", Id: "a"})
	if err != nil {
		t.Fatal(err)
	}
	g := out
	ids := nodeIDs(g.Dependencies)
	if ids["secret"] {
		t.Fatalf("unreadable node leaked: %+v", g.Dependencies)
	}
	if !ids["pub"] || len(g.Dependencies) != 1 {
		t.Fatalf("dependencies = %+v, want [pub] only", g.Dependencies)
	}
}

func TestRemoveDependency(t *testing.T) {
	fs := &fakeDepStore{tasks: map[string]*models.Task{
		"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "me"},
		"b": {TaskID: "b", HouseID: h1, Title: "B", OwnerMemberID: "me"},
	}}
	fs.edges = []models.Dependency{
		{HouseID: h1, DependentType: "task", DependentID: "a", DependencyType: "task", DependencyID: "b"},
	}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")
	if _, err := svc.RemoveDependency(ctx, ref("task", "a", "task", "b")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(fs.edges) != 0 {
		t.Fatalf("edges = %d, want 0", len(fs.edges))
	}
}

// A task may depend on a project (mixed endpoints).
func TestMixedTaskProjectEdge(t *testing.T) {
	fs := &fakeDepStore{
		tasks: map[string]*models.Task{
			"a": {TaskID: "a", HouseID: h1, Title: "A", OwnerMemberID: "me"},
		},
		projects: map[string]*models.Project{
			"p": {ProjectID: "p", HouseID: h1, Name: "Proj", Status: "active", Visibility: "read"},
		},
	}
	svc := &DependencyService{Store: fs}
	ctx := ctxAs(h1, "me", "member")
	if _, err := svc.AddDependency(ctx, ref("task", "a", "project", "p")); err != nil {
		t.Fatalf("add mixed: %v", err)
	}
	out, err := svc.GetDependencies(ctx, csil.DependencyTarget{Type: "task", Id: "a"})
	if err != nil {
		t.Fatal(err)
	}
	g := out
	if len(g.Dependencies) != 1 || g.Dependencies[0].Id != "p" || g.Dependencies[0].Title != "Proj" {
		t.Fatalf("mixed dependency = %+v, want project p", g.Dependencies)
	}
}
