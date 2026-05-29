package csilservices

import (
	"context"
	"errors"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// fakeCommentStore implements just the read methods resolveTarget needs; the
// rest of the Store surface is satisfied by the embedded (nil) interface and
// will panic if called, which keeps these tests honest about their deps.
type fakeCommentStore struct {
	store.Store
	tasks          map[string]*models.Task
	taskAssignees  map[string][]models.Member
	projects       map[string]*models.Project
	projectMembers map[string][]models.Member
	projectOwners  map[string][]models.Member
}

var errNotFound = errors.New("not found")

func (f *fakeCommentStore) GetTaskByID(_ context.Context, id string) (*models.Task, error) {
	if t, ok := f.tasks[id]; ok {
		return t, nil
	}
	return nil, errNotFound
}

func (f *fakeCommentStore) ListTaskAssignees(_ context.Context, id string) ([]models.Member, error) {
	return f.taskAssignees[id], nil
}

func (f *fakeCommentStore) GetProjectByID(_ context.Context, id string) (*models.Project, error) {
	if p, ok := f.projects[id]; ok {
		return p, nil
	}
	return nil, errNotFound
}

func (f *fakeCommentStore) ListProjectMembers(_ context.Context, id string) ([]models.Member, error) {
	return f.projectMembers[id], nil
}

func (f *fakeCommentStore) ListProjectOwners(_ context.Context, id string) ([]models.Member, error) {
	return f.projectOwners[id], nil
}

func members(ids ...string) []models.Member {
	out := make([]models.Member, 0, len(ids))
	for _, id := range ids {
		out = append(out, models.Member{MemberID: id})
	}
	return out
}

func TestDedupExcept(t *testing.T) {
	cases := []struct {
		name   string
		ids    []string
		except string
		want   []string
	}{
		{"empty", nil, "x", []string{}},
		{"drops except", []string{"a", "b", "a"}, "a", []string{"b"}},
		{"dedups", []string{"a", "b", "b", "c"}, "z", []string{"a", "b", "c"}},
		{"skips blanks", []string{"", "a", ""}, "z", []string{"a"}},
		{"all excluded", []string{"a", "a"}, "a", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupExcept(tc.ids, tc.except)
			if len(got) != len(tc.want) {
				t.Fatalf("len: got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}

func TestMemberDisplayName(t *testing.T) {
	cases := []struct {
		m    models.Member
		want string
	}{
		{models.Member{DisplayName: "Sam Fielder"}, "Sam Fielder"},
		{models.Member{DisplayName: "  ", LinkkeysUserID: "sam@x"}, "sam@x"},
		{models.Member{MemberID: "m1"}, "m1"},
	}
	for _, tc := range cases {
		if got := memberDisplayName(&tc.m); got != tc.want {
			t.Errorf("memberDisplayName(%+v) = %q, want %q", tc.m, got, tc.want)
		}
	}
}

// A task's watchers are its owner plus all assignees; the comment author is
// removed and the set is deduped, so an author who is also an assignee is not
// notified about their own comment.
func TestTaskCommentRecipients(t *testing.T) {
	fs := &fakeCommentStore{
		tasks:         map[string]*models.Task{"t1": {TaskID: "t1", HouseID: "h1", Title: "Fix fence", OwnerMemberID: "owner"}},
		taskAssignees: map[string][]models.Member{"t1": members("alice", "bob", "owner")},
	}
	svc := &CommentService{Store: fs}
	ti, err := svc.resolveTarget(context.Background(), "task", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if ti.houseID != "h1" || ti.title != "Fix fence" {
		t.Fatalf("target info: %+v", ti)
	}
	// Author = alice (an assignee). Expect owner + bob, not alice, no dup owner.
	got := dedupExcept(ti.watchers, "alice")
	want := map[string]bool{"owner": true, "bob": true}
	if len(got) != len(want) {
		t.Fatalf("recipients = %v, want keys %v", got, want)
	}
	for _, id := range got {
		if !want[id] {
			t.Fatalf("unexpected recipient %q in %v", id, got)
		}
	}
}

// A project's watchers are its members plus owners, deduped across the two
// sets, with the author removed.
func TestProjectCommentRecipients(t *testing.T) {
	fs := &fakeCommentStore{
		projects:       map[string]*models.Project{"p1": {ProjectID: "p1", HouseID: "h1", Name: "Barn rebuild"}},
		projectMembers: map[string][]models.Member{"p1": members("alice", "bob")},
		projectOwners:  map[string][]models.Member{"p1": members("bob", "carol")},
	}
	svc := &CommentService{Store: fs}
	ti, err := svc.resolveTarget(context.Background(), "project", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if ti.title != "Barn rebuild" {
		t.Fatalf("title = %q", ti.title)
	}
	// Author = carol. Expect alice + bob (bob deduped across member+owner).
	got := dedupExcept(ti.watchers, "carol")
	want := map[string]bool{"alice": true, "bob": true}
	if len(got) != len(want) {
		t.Fatalf("recipients = %v, want keys %v", got, want)
	}
	for _, id := range got {
		if !want[id] {
			t.Fatalf("unexpected recipient %q in %v", id, got)
		}
	}
}

func TestResolveTargetUnknownAndMissing(t *testing.T) {
	svc := &CommentService{Store: &fakeCommentStore{tasks: map[string]*models.Task{}}}
	if _, err := svc.resolveTarget(context.Background(), "bogus", "x"); err == nil {
		t.Error("expected error for unknown target type")
	}
	if _, err := svc.resolveTarget(context.Background(), "task", "missing"); err == nil {
		t.Error("expected error for missing task")
	}
}
