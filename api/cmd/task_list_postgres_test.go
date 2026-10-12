//go:build integration

package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/csilservices"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func mkTask(t *testing.T, ctx context.Context, houseID, ownerID, title, status string, due *time.Time) string {
	t.Helper()
	task := &models.Task{HouseID: houseID, OwnerMemberID: ownerID, Title: title, Visibility: "read", Status: status, DueAt: due}
	if err := store.AppStore.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask %s: %v", title, err)
	}
	return task.TaskID
}

// listTasks unpacks a ListTasks page for the older assertions below.
func listTasks(ctx context.Context, f store.TaskFilter, limit, offset int) ([]models.Task, int64, error) {
	page, err := store.AppStore.ListTasks(ctx, f, limit, offset)
	return page.Tasks, page.Total, err
}

func titles(tasks []models.Task) []string {
	out := make([]string, len(tasks))
	for i := range tasks {
		out[i] = tasks[i].Title
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListTasks_Postgres_FiltersOrderAndTotal(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "List House")
	me := mkMember(t, ctx, house, "me")
	other := mkMember(t, ctx, house, "other")

	now := time.Now().UTC().Truncate(time.Second)
	yesterday, tomorrow, nextWeek := now.Add(-24*time.Hour), now.Add(24*time.Hour), now.Add(7*24*time.Hour)
	mkTask(t, ctx, house, me, "no-date", "open", nil)
	mkTask(t, ctx, house, me, "next-week", "open", &nextWeek)
	mkTask(t, ctx, house, me, "overdue", "in_progress", &yesterday)
	mkTask(t, ctx, house, other, "theirs-tomorrow", "open", &tomorrow)
	assigned := mkTask(t, ctx, house, other, "assigned-to-me", "open", &tomorrow)
	if err := store.AppStore.AddTaskAssignee(ctx, assigned, me); err != nil {
		t.Fatalf("AddTaskAssignee: %v", err)
	}
	inProject := mkTask(t, ctx, house, me, "in-project", "open", nil)
	mkTask(t, ctx, house, me, "done", "done", nil)
	mkTask(t, ctx, house, me, "cancelled", "cancelled", nil)

	p := &models.Project{HouseID: house, Name: "P", Visibility: "read"}
	if err := store.AppStore.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := store.AppStore.AddProjectTask(ctx, p.ProjectID, inProject, 0); err != nil {
		t.Fatalf("AddProjectTask: %v", err)
	}
	if err := store.AppStore.AddProjectTask(ctx, p.ProjectID, inProject, 1); !errors.Is(err, store.ErrAlreadyExists) {
		t.Fatalf("duplicate AddProjectTask: got %v, want ErrAlreadyExists", err)
	}

	// Open: due ascending, no due date last, then newest first.
	open, total, err := listTasks(ctx, store.TaskFilter{HouseID: house, State: store.TaskStateOpen}, 50, 0)
	if err != nil {
		t.Fatalf("ListTasks open: %v", err)
	}
	wantOpen := []string{"overdue", "assigned-to-me", "theirs-tomorrow", "next-week", "in-project", "no-date"}
	gotOpen := titles(open)
	// The two tasks due tomorrow have the same due_at: newest first.
	if total != 6 || !sameStrings(gotOpen, wantOpen) {
		t.Fatalf("open: total %d, order %v, want %v", total, gotOpen, wantOpen)
	}

	// Mine + loose + open: owner or assignee, in no project.
	mine, total, err := listTasks(ctx, store.TaskFilter{
		HouseID: house, State: store.TaskStateOpen, MineMemberID: me, Loose: true,
	}, 50, 0)
	if err != nil {
		t.Fatalf("ListTasks mine: %v", err)
	}
	if want := []string{"overdue", "assigned-to-me", "next-week", "no-date"}; total != 4 || !sameStrings(titles(mine), want) {
		t.Fatalf("mine loose: total %d, %v, want %v", total, titles(mine), want)
	}

	// Closed: most recently updated first.
	closed, total, err := listTasks(ctx, store.TaskFilter{HouseID: house, State: store.TaskStateClosed}, 50, 0)
	if err != nil || total != 2 || closed[0].Title != "cancelled" {
		t.Fatalf("closed: total %d, %v, %v", total, titles(closed), err)
	}

	// Due window [now, now+2d).
	to := now.Add(48 * time.Hour)
	due, total, err := listTasks(ctx, store.TaskFilter{HouseID: house, DueFrom: &now, DueTo: &to}, 50, 0)
	if err != nil || total != 2 || len(due) != 2 {
		t.Fatalf("due window: total %d, %v, %v", total, titles(due), err)
	}

	// Paging keeps the total and never repeats a row.
	page1, total1, _ := listTasks(ctx, store.TaskFilter{HouseID: house, State: store.TaskStateOpen}, 4, 0)
	page2, total2, _ := listTasks(ctx, store.TaskFilter{HouseID: house, State: store.TaskStateOpen}, 4, 4)
	if total1 != 6 || total2 != 6 || !sameStrings(append(titles(page1), titles(page2)...), wantOpen) {
		t.Fatalf("paging: %v + %v", titles(page1), titles(page2))
	}

	// Group starts: rows due before each boundary, in due order.
	startToday := now.Add(-time.Hour)
	farFuture := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	counted, err := store.AppStore.ListTasks(ctx, store.TaskFilter{
		HouseID: house, State: store.TaskStateOpen, DueBefore: []time.Time{startToday, nextWeek, farFuture},
	}, 1, 0)
	if err != nil {
		t.Fatalf("ListTasks due counts: %v", err)
	}
	if want := []int64{1, 3, 4}; len(counted.DueCounts) != 3 || counted.DueCounts[0] != want[0] || counted.DueCounts[1] != want[1] || counted.DueCounts[2] != want[2] {
		t.Fatalf("due counts: %v, want %v", counted.DueCounts, want)
	}

	// Project task pages: state filter, total, and the next position.
	closedInProject := mkTask(t, ctx, house, me, "closed-in-project", "done", nil)
	if err := store.AppStore.AddProjectTask(ctx, p.ProjectID, closedInProject, 7); err != nil {
		t.Fatalf("AddProjectTask: %v", err)
	}
	if next, err := store.AppStore.NextProjectTaskPosition(ctx, p.ProjectID); err != nil || next != 8 {
		t.Fatalf("next position: %d, %v", next, err)
	}
	ptOpen, ptTotal, err := store.AppStore.ListProjectTasksPage(ctx, p.ProjectID, store.TaskStateOpen, 50, 0)
	if err != nil || ptTotal != 1 || len(ptOpen) != 1 || ptOpen[0].Title != "in-project" {
		t.Fatalf("open project tasks: %v, %d, %v", titles(ptOpen), ptTotal, err)
	}
	_, ptAll, _ := store.AppStore.ListProjectTasksPage(ctx, p.ProjectID, "", 50, 0)
	if ptAll != 2 {
		t.Fatalf("all project tasks: %d, want 2", ptAll)
	}

	// Deleting the project makes its task loose again.
	op, _ := store.AppStore.NewID(ctx)
	if err := store.AppStore.SoftDeleteProject(ctx, p.ProjectID, me, op); err != nil {
		t.Fatalf("SoftDeleteProject: %v", err)
	}
	_, total, _ = listTasks(ctx, store.TaskFilter{HouseID: house, State: store.TaskStateOpen, MineMemberID: me, Loose: true}, 50, 0)
	if total != 5 {
		t.Fatalf("loose after project delete: total %d, want 5", total)
	}
}

func TestCreatePersonalTask_Postgres(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()
	house := mkHouse(t, ctx, "Personal House")
	me := mkMember(t, ctx, house, "me")
	other := mkMember(t, ctx, house, "other")

	svc := &csilservices.TaskService{Store: store.AppStore}
	asMember := func(memberID string) context.Context {
		return auth.WithIdentity(ctx, &auth.Identity{
			Domain: "soft.test", UserID: memberID,
			Houses: []auth.HouseRoles{{House: house, Member: memberID, Roles: []string{"member"}}},
		})
	}

	first, err := svc.CreatePersonalTask(asMember(me), csil.Task{HouseId: csil.HouseID(house), Title: "Call the plumber", OwnerMemberId: csil.MemberID(other)})
	if err != nil {
		t.Fatalf("CreatePersonalTask: %v", err)
	}
	if string(first.OwnerMemberId) != me || len(first.Assignees) != 1 || string(first.Assignees[0]) != me {
		t.Fatalf("owner/assignees: %#v", first)
	}
	if first.Visibility == nil || *first.Visibility != "none" {
		t.Fatalf("visibility: %v", first.Visibility)
	}
	second, err := svc.CreatePersonalTask(asMember(me), csil.Task{HouseId: csil.HouseID(house), Title: "Buy stamps"})
	if err != nil {
		t.Fatalf("second CreatePersonalTask: %v", err)
	}

	project, err := store.AppStore.GetPersonalProject(ctx, house, me)
	if err != nil || project == nil || project.Visibility != "none" {
		t.Fatalf("personal project: %#v, %v", project, err)
	}
	inProject, _ := store.AppStore.ListProjectTasks(ctx, project.ProjectID, 50, 0)
	if len(inProject) != 2 {
		t.Fatalf("personal project holds %d tasks, want 2 (one project, reused)", len(inProject))
	}

	// Another member cannot read it.
	_, err = svc.GetTask(asMember(other), second.TaskId)
	var serviceErr *csilrpc.Error
	if !errors.As(err, &serviceErr) || (serviceErr.Code != 403 && serviceErr.Code != 404) {
		t.Fatalf("other member GetTask: got %v, want 403 or 404", err)
	}

	// The project service reports a duplicate add as a conflict.
	projects := &csilservices.ProjectService{Store: store.AppStore}
	_, err = projects.AddProjectTask(asMember(me), csil.ProjectTaskOrderRequest{
		ProjectId: csil.ProjectID(project.ProjectID), TaskId: first.TaskId, Position: 5,
	})
	if !errors.As(err, &serviceErr) || serviceErr.Code != 409 {
		t.Fatalf("duplicate add: got %v, want 409", err)
	}
}
