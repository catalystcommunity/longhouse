//go:build integration

package cmd

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// backdateEvent ages the notification_event whose body matches, so cull tests
// don't have to wait months. Uses a raw connection — there's no store method
// to rewrite created_at, by design.
func backdateEvent(t *testing.T, uri, body string, hours int) {
	t.Helper()
	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(
		`UPDATE notification_events SET created_at = timezone('utc', now()) + make_interval(hours => $1) WHERE body = $2`,
		hours, body,
	); err != nil {
		t.Fatalf("backdate exec: %v", err)
	}
}

// seedHouseWithMembers creates a house and n members, returning their ids.
func seedHouseWithMembers(t *testing.T, ctx context.Context, names ...string) (string, []string) {
	t.Helper()
	h := &models.House{Name: "Notif Test House"}
	if err := store.AppStore.CreateHouse(ctx, h); err != nil {
		t.Fatalf("CreateHouse: %v", err)
	}
	ids := make([]string, 0, len(names))
	for i, n := range names {
		m := &models.Member{
			HouseID:        h.HouseID,
			LinkkeysDomain: "test.example",
			LinkkeysUserID: n + "@test.example",
			DisplayName:    n,
		}
		if err := store.AppStore.CreateMember(ctx, m); err != nil {
			t.Fatalf("CreateMember[%d]: %v", i, err)
		}
		ids = append(ids, m.MemberID)
	}
	return h.HouseID, ids
}

// One comment with N watchers => 1 event snapshot + N per-recipient rows, all
// written atomically, with the author excluded.
func TestCreateCommentWithNotifications_FanOut(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()

	houseID, ids := seedHouseWithMembers(t, ctx, "owner", "alice", "bob")
	owner, alice, bob := ids[0], ids[1], ids[2]

	task := &models.Task{HouseID: houseID, OwnerMemberID: owner, Title: "Fix the fence"}
	if err := store.AppStore.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	for _, m := range []string{alice, bob} {
		if err := store.AppStore.AddTaskAssignee(ctx, task.TaskID, m); err != nil {
			t.Fatalf("AddTaskAssignee: %v", err)
		}
	}

	// alice comments → recipients are owner + bob (not alice).
	comment := &models.Comment{HouseID: houseID, MemberID: alice, TargetType: "task", TargetID: task.TaskID, Body: "on it"}
	tt, tid := "task", task.TaskID
	actor := alice
	event := &models.NotificationEvent{
		HouseID: houseID, Kind: "comment_created",
		ActorMemberID: &actor, ActorName: "alice",
		TargetType: &tt, TargetID: &tid, TargetTitle: task.Title, Body: "on it",
	}
	if err := store.AppStore.CreateCommentWithNotifications(ctx, comment, event, []string{owner, bob}); err != nil {
		t.Fatalf("CreateCommentWithNotifications: %v", err)
	}

	// owner sees one unread notification, self-contained.
	if c, err := store.AppStore.CountUnreadNotifications(ctx, houseID, owner); err != nil || c != 1 {
		t.Fatalf("owner unread = %d, err %v; want 1", c, err)
	}
	if c, _ := store.AppStore.CountUnreadNotifications(ctx, houseID, bob); c != 1 {
		t.Fatalf("bob unread = %d; want 1", c)
	}
	// alice (author) gets nothing.
	if c, _ := store.AppStore.CountUnreadNotifications(ctx, houseID, alice); c != 0 {
		t.Fatalf("alice unread = %d; want 0 (author excluded)", c)
	}

	feed, err := store.AppStore.ListNotificationsByMember(ctx, houseID, owner, false, 50, 0)
	if err != nil || len(feed) != 1 {
		t.Fatalf("owner feed len = %d, err %v; want 1", len(feed), err)
	}
	item := feed[0]
	if item.ActorName != "alice" || item.TargetTitle != "Fix the fence" || item.Body != "on it" || item.Kind != "comment_created" {
		t.Fatalf("snapshot wrong: %+v", item)
	}
	if item.ReadAt != nil {
		t.Fatalf("new notification should be unread")
	}

	// The snapshot survives deletion of the source task (no FK back to it).
	if err := store.AppStore.DeleteTask(ctx, task.TaskID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if feed, _ := store.AppStore.ListNotificationsByMember(ctx, houseID, owner, false, 50, 0); len(feed) != 1 {
		t.Fatalf("after task delete, owner feed len = %d; want 1 (snapshot independent)", len(feed))
	}
}

func TestMarkRead_And_MarkAllRead(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()

	houseID, ids := seedHouseWithMembers(t, ctx, "owner", "alice")
	owner, alice := ids[0], ids[1]
	task := &models.Task{HouseID: houseID, OwnerMemberID: owner, Title: "T"}
	if err := store.AppStore.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	// Two comments by alice => owner gets two notifications.
	for _, b := range []string{"first", "second"} {
		c := &models.Comment{HouseID: houseID, MemberID: alice, TargetType: "task", TargetID: task.TaskID, Body: b}
		e := &models.NotificationEvent{HouseID: houseID, Kind: "comment_created", ActorName: "alice", TargetTitle: "T", Body: b}
		if err := store.AppStore.CreateCommentWithNotifications(ctx, c, e, []string{owner}); err != nil {
			t.Fatal(err)
		}
	}
	if c, _ := store.AppStore.CountUnreadNotifications(ctx, houseID, owner); c != 2 {
		t.Fatalf("unread = %d; want 2", c)
	}

	feed, _ := store.AppStore.ListNotificationsByMember(ctx, houseID, owner, false, 50, 0)
	if err := store.AppStore.MarkNotificationRead(ctx, feed[0].NotificationID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if c, _ := store.AppStore.CountUnreadNotifications(ctx, houseID, owner); c != 1 {
		t.Fatalf("after mark one read, unread = %d; want 1", c)
	}
	if unread, _ := store.AppStore.ListNotificationsByMember(ctx, houseID, owner, true, 50, 0); len(unread) != 1 {
		t.Fatalf("unread-only feed = %d; want 1", len(unread))
	}

	if err := store.AppStore.MarkAllNotificationsRead(ctx, houseID, owner, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if c, _ := store.AppStore.CountUnreadNotifications(ctx, houseID, owner); c != 0 {
		t.Fatalf("after mark all read, unread = %d; want 0", c)
	}
}

// Culling deletes old events and cascades their per-recipient rows, while
// leaving recent ones untouched.
func TestCullNotificationEventsBefore(t *testing.T) {
	uri := requireTestDB(t)
	freshStore(t, uri)
	ctx := context.Background()

	houseID, ids := seedHouseWithMembers(t, ctx, "owner", "alice")
	owner, alice := ids[0], ids[1]
	task := &models.Task{HouseID: houseID, OwnerMemberID: owner, Title: "T"}
	if err := store.AppStore.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	mk := func(body string) {
		c := &models.Comment{HouseID: houseID, MemberID: alice, TargetType: "task", TargetID: task.TaskID, Body: body}
		e := &models.NotificationEvent{HouseID: houseID, Kind: "comment_created", ActorName: "alice", TargetTitle: "T", Body: body}
		if err := store.AppStore.CreateCommentWithNotifications(ctx, c, e, []string{owner}); err != nil {
			t.Fatal(err)
		}
	}
	mk("old")
	mk("new")

	// Backdate the "old" event well past retention (~200 days).
	backdateEvent(t, uri, "old", -200*24)

	cutoff := time.Now().UTC().Add(-180 * 24 * time.Hour)
	n, err := store.AppStore.CullNotificationEventsBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("cull: %v", err)
	}
	if n != 1 {
		t.Fatalf("culled %d events; want 1", n)
	}
	// The recent one (and its per-recipient row) remain.
	if c, _ := store.AppStore.CountUnreadNotifications(ctx, houseID, owner); c != 1 {
		t.Fatalf("after cull, owner unread = %d; want 1", c)
	}
}
