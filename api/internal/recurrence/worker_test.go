package recurrence

import (
	"context"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// fakeWorkerStore is a tiny in-memory implementation of WorkerStore used
// only by these tests. Keeps the test self-contained — no need to pull in
// the handlers' larger memStore.
type fakeWorkerStore struct {
	due        []models.Task
	priors     map[string]*models.Task // root_task_id → prior child
	created    []models.Task
	updated    []models.Task
	comments   []models.Comment
	// task_id → assignees living on that task. Tests pre-seed assignees on
	// the root; the worker copies them onto each spawned child.
	assignees     map[string][]models.Member
	failCreate    bool
	failUpdate    bool
}

func (f *fakeWorkerStore) ListDueRecurringTasks(_ context.Context, _ time.Time, _ int) ([]models.Task, error) {
	return f.due, nil
}
func (f *fakeWorkerStore) LatestRecurrenceChildOf(_ context.Context, rootID string) (*models.Task, error) {
	if t, ok := f.priors[rootID]; ok {
		return t, nil
	}
	return nil, nil
}
func (f *fakeWorkerStore) CreateTask(_ context.Context, t *models.Task) error {
	if f.failCreate {
		return errBoom("create")
	}
	t.TaskID = "child-new"
	f.created = append(f.created, *t)
	return nil
}
func (f *fakeWorkerStore) UpdateTask(_ context.Context, t *models.Task) error {
	if f.failUpdate {
		return errBoom("update")
	}
	f.updated = append(f.updated, *t)
	return nil
}
func (f *fakeWorkerStore) CreateComment(_ context.Context, c *models.Comment) error {
	c.CommentID = "comment-new"
	f.comments = append(f.comments, *c)
	return nil
}

func (f *fakeWorkerStore) ListTaskAssignees(_ context.Context, taskID string) ([]models.Member, error) {
	if f.assignees == nil {
		return nil, nil
	}
	return f.assignees[taskID], nil
}

func (f *fakeWorkerStore) AddTaskAssignee(_ context.Context, taskID, memberID string) error {
	if f.assignees == nil {
		f.assignees = map[string][]models.Member{}
	}
	f.assignees[taskID] = append(f.assignees[taskID], models.Member{MemberID: memberID})
	return nil
}

type errBoom string

func (e errBoom) Error() string { return string(e) }

func dueDailyRoot(t *testing.T, id, dueAt string) models.Task {
	t.Helper()
	freq := Daily
	due := mustParse(t, dueAt)
	return models.Task{
		TaskID:             id,
		HouseID:            "h1",
		OwnerMemberID:      "m1",
		Title:              "Take out trash",
		Status:             "open",
		RecurrenceFreq:     &freq,
		RecurrenceInterval: 1,
		NextRecurrenceAt:   &due,
	}
}

func TestTick_NoPriorChild_SpawnsAndAdvances(t *testing.T) {
	store := &fakeWorkerStore{
		due: []models.Task{dueDailyRoot(t, "root-1", "2026-05-01T09:00:00Z")},
	}
	now := mustParse(t, "2026-05-01T09:01:00Z")

	res, err := Tick(context.Background(), store, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Considered != 1 || res.Spawned != 1 || res.MissedComments != 0 || len(res.Errors) != 0 {
		t.Errorf("result: %+v", res)
	}
	if len(store.created) != 1 {
		t.Fatalf("want 1 child, got %d", len(store.created))
	}
	if len(store.updated) != 1 {
		t.Fatalf("want root updated once, got %d", len(store.updated))
	}
	if store.updated[0].NextRecurrenceAt == nil ||
		!store.updated[0].NextRecurrenceAt.Equal(mustParse(t, "2026-05-02T09:00:00Z")) {
		t.Errorf("root NextRecurrenceAt: %v", store.updated[0].NextRecurrenceAt)
	}
}

func TestTick_PriorChildIncomplete_PostsComment(t *testing.T) {
	root := dueDailyRoot(t, "root-1", "2026-05-01T09:00:00Z")
	prior := &models.Task{TaskID: "child-prev", HouseID: "h1", OwnerMemberID: "m1", Status: "open"}
	store := &fakeWorkerStore{
		due:    []models.Task{root},
		priors: map[string]*models.Task{"root-1": prior},
	}
	now := mustParse(t, "2026-05-01T09:01:00Z")

	res, err := Tick(context.Background(), store, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Spawned != 1 {
		t.Errorf("Spawned: %d", res.Spawned)
	}
	if res.MissedComments != 1 {
		t.Errorf("MissedComments: %d", res.MissedComments)
	}
	if len(store.comments) != 1 || store.comments[0].TargetID != "child-prev" {
		t.Errorf("expected one missed-comment on the prior child: %+v", store.comments)
	}
	if store.comments[0].Body == "" {
		t.Errorf("missed-comment body should not be empty")
	}
}

func TestTick_PriorChildDone_NoMissedComment(t *testing.T) {
	root := dueDailyRoot(t, "root-1", "2026-05-01T09:00:00Z")
	prior := &models.Task{TaskID: "child-prev", HouseID: "h1", OwnerMemberID: "m1", Status: "done"}
	store := &fakeWorkerStore{
		due:    []models.Task{root},
		priors: map[string]*models.Task{"root-1": prior},
	}
	now := mustParse(t, "2026-05-01T09:01:00Z")

	res, _ := Tick(context.Background(), store, now)
	if res.MissedComments != 0 {
		t.Errorf("MissedComments: %d, want 0", res.MissedComments)
	}
	if len(store.comments) != 0 {
		t.Errorf("no comments should be posted; got %d", len(store.comments))
	}
}

func TestTick_MultipleDueTasks(t *testing.T) {
	store := &fakeWorkerStore{
		due: []models.Task{
			dueDailyRoot(t, "root-a", "2026-05-01T09:00:00Z"),
			dueDailyRoot(t, "root-b", "2026-05-01T08:00:00Z"),
			dueDailyRoot(t, "root-c", "2026-05-01T10:00:00Z"),
		},
	}
	res, err := Tick(context.Background(), store, mustParse(t, "2026-05-01T11:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Considered != 3 || res.Spawned != 3 {
		t.Errorf("result: %+v", res)
	}
	if len(store.created) != 3 || len(store.updated) != 3 {
		t.Errorf("expected 3 created + 3 updated, got %d+%d", len(store.created), len(store.updated))
	}
}

func TestTick_CreateChildErrorRecorded(t *testing.T) {
	store := &fakeWorkerStore{
		due:        []models.Task{dueDailyRoot(t, "root-1", "2026-05-01T09:00:00Z")},
		failCreate: true,
	}
	res, err := Tick(context.Background(), store, mustParse(t, "2026-05-01T09:01:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Spawned != 0 {
		t.Errorf("Spawned should be 0 on create-error: %d", res.Spawned)
	}
	if len(res.Errors) == 0 {
		t.Errorf("expected at least one error in result")
	}
}

func TestTick_NilStoreRejected(t *testing.T) {
	if _, err := Tick(context.Background(), nil, time.Now()); err == nil {
		t.Error("expected error for nil store")
	}
}

// TestTick_AssigneesCopiedToChild — the old single assigned_to_member_id
// column moved to a task_assignees join, and the recurrence worker now
// owns the cross-row copy. Verify every assignee on the root lands on
// the spawned child, in the right house.
func TestTick_AssigneesCopiedToChild(t *testing.T) {
	root := dueDailyRoot(t, "root-1", "2026-05-01T09:00:00Z")
	store := &fakeWorkerStore{
		due: []models.Task{root},
		assignees: map[string][]models.Member{
			root.TaskID: {{MemberID: "m-alice"}, {MemberID: "m-bob"}},
		},
	}
	res, err := Tick(context.Background(), store, mustParse(t, "2026-05-01T09:01:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Spawned != 1 {
		t.Fatalf("Spawned: got %d, want 1", res.Spawned)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	got := store.assignees["child-new"]
	if len(got) != 2 {
		t.Fatalf("child assignees: got %d (%+v), want 2", len(got), got)
	}
	want := map[string]bool{"m-alice": true, "m-bob": true}
	for _, m := range got {
		if !want[m.MemberID] {
			t.Errorf("unexpected assignee on child: %q", m.MemberID)
		}
	}
}
