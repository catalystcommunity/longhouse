package recurrence

import (
	"errors"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

func TestNext_Hourly(t *testing.T) {
	from := mustParse(t, "2026-05-01T08:00:00Z")
	got, err := Next(from, Hourly, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := mustParse(t, "2026-05-01T09:00:00Z")
	if !got.Equal(want) {
		t.Errorf("hourly+1: got %s, want %s", got, want)
	}

	got, _ = Next(from, Hourly, 4, nil)
	want = mustParse(t, "2026-05-01T12:00:00Z")
	if !got.Equal(want) {
		t.Errorf("hourly+4: got %s, want %s", got, want)
	}
}

func TestNext_Daily(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Daily, 1, nil)
	if !got.Equal(mustParse(t, "2026-05-02T09:00:00Z")) {
		t.Errorf("daily+1: got %s", got)
	}
}

func TestNext_Weekly_NoByWeekday(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z") // Friday
	got, _ := Next(from, Weekly, 1, nil)
	if !got.Equal(mustParse(t, "2026-05-08T09:00:00Z")) {
		t.Errorf("weekly+1: got %s", got)
	}
}

func TestNext_Weekly_Weekdays(t *testing.T) {
	// Friday → next weekday-ish occurrence with interval=1 lands seven days
	// later (also a Friday); byWeekday=[1..5] (Mon..Fri) accepts it.
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Weekly, 1, []int{1, 2, 3, 4, 5})
	if w := got.Weekday(); w < time.Monday || w > time.Friday {
		t.Errorf("weekday filter accepted non-weekday: %v", w)
	}
}

func TestNext_Weekly_Weekends(t *testing.T) {
	// Friday → next, with byWeekday = Sat/Sun, should land on Sat the
	// week after (since interval=1 first jumps 7 days, then we forward-
	// scan to the next allowed weekday).
	from := mustParse(t, "2026-05-01T09:00:00Z") // Friday
	got, _ := Next(from, Weekly, 1, []int{0, 6})
	w := got.Weekday()
	if w != time.Saturday && w != time.Sunday {
		t.Errorf("weekend filter: got %v", w)
	}
}

func TestNext_EveryFewWeeks(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Weekly, 3, nil)
	if !got.Equal(mustParse(t, "2026-05-22T09:00:00Z")) {
		t.Errorf("weekly+3: got %s", got)
	}
}

func TestNext_Monthly(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Monthly, 1, nil)
	if !got.Equal(mustParse(t, "2026-06-01T09:00:00Z")) {
		t.Errorf("monthly+1: got %s", got)
	}
	got, _ = Next(from, Monthly, 2, nil)
	if !got.Equal(mustParse(t, "2026-07-01T09:00:00Z")) {
		t.Errorf("every-other-month: got %s", got)
	}
}

func TestNext_Quarterly(t *testing.T) {
	from := mustParse(t, "2026-01-15T09:00:00Z")
	got, _ := Next(from, Quarterly, 1, nil)
	if !got.Equal(mustParse(t, "2026-04-15T09:00:00Z")) {
		t.Errorf("quarterly: got %s", got)
	}
}

func TestNext_Yearly(t *testing.T) {
	from := mustParse(t, "2026-01-15T09:00:00Z")
	got, _ := Next(from, Yearly, 1, nil)
	if !got.Equal(mustParse(t, "2027-01-15T09:00:00Z")) {
		t.Errorf("yearly+1: got %s", got)
	}
	got, _ = Next(from, Yearly, 5, nil)
	if !got.Equal(mustParse(t, "2031-01-15T09:00:00Z")) {
		t.Errorf("every-N-years: got %s", got)
	}
}

func TestNext_UnknownFrequency(t *testing.T) {
	if _, err := Next(time.Now(), "decadal", 1, nil); !errors.Is(err, ErrUnknownFrequency) {
		t.Errorf("expected ErrUnknownFrequency, got %v", err)
	}
}

func TestNext_IntervalDefaultsToOne(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Daily, 0, nil)
	if !got.Equal(mustParse(t, "2026-05-02T09:00:00Z")) {
		t.Errorf("interval 0 should default to 1: got %s", got)
	}
	got, _ = Next(from, Daily, -3, nil)
	if !got.Equal(mustParse(t, "2026-05-02T09:00:00Z")) {
		t.Errorf("negative interval should default to 1: got %s", got)
	}
}

// ----- Plan -----

func dailyRoot(t *testing.T, dueAt string) *models.Task {
	t.Helper()
	freq := Daily
	due := mustParse(t, dueAt)
	return &models.Task{
		TaskID:             "root",
		HouseID:            "h1",
		OwnerMemberID:      "m1",
		Title:              "Take out the trash",
		Status:             "open",
		RecurrenceFreq:     &freq,
		RecurrenceInterval: 1,
		NextRecurrenceAt:   &due,
	}
}

func TestPlan_FirstSpawnNoPriorChild(t *testing.T) {
	root := dailyRoot(t, "2026-05-01T09:00:00Z")
	now := mustParse(t, "2026-05-01T09:01:00Z")

	dec, err := Plan(now, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dec.SpawnChild == nil {
		t.Fatal("expected a SpawnChild")
	}
	if dec.SpawnChild.RecurrenceRootTaskID == nil || *dec.SpawnChild.RecurrenceRootTaskID != "root" {
		t.Errorf("child should reference root: %+v", dec.SpawnChild.RecurrenceRootTaskID)
	}
	if dec.SpawnChild.Status != "open" {
		t.Errorf("child status: %q", dec.SpawnChild.Status)
	}
	if dec.SpawnChild.Title != root.Title {
		t.Errorf("child title: %q, want %q", dec.SpawnChild.Title, root.Title)
	}
	if !dec.NextDueAt.Equal(mustParse(t, "2026-05-02T09:00:00Z")) {
		t.Errorf("next_due_at: %s", dec.NextDueAt)
	}
	if dec.MarkMissedOnID != "" {
		t.Errorf("no prior child means no missed marker: %+v", dec)
	}
}

func TestPlan_PriorChildIncomplete_MarksMissed(t *testing.T) {
	root := dailyRoot(t, "2026-05-01T09:00:00Z")
	now := mustParse(t, "2026-05-01T09:01:00Z")
	prior := &models.Task{TaskID: "child-1", Status: "open"}

	dec, err := Plan(now, root, prior)
	if err != nil {
		t.Fatal(err)
	}
	if dec.MarkMissedOnID != "child-1" {
		t.Errorf("missed marker target: %q", dec.MarkMissedOnID)
	}
	if dec.MarkMissedReply == "" {
		t.Errorf("missed reply should be non-empty")
	}
	// Even when missed, we still spawn the next occurrence.
	if dec.SpawnChild == nil {
		t.Errorf("missed branch should still spawn next")
	}
}

func TestPlan_PriorChildDone_NoMissedMarker(t *testing.T) {
	root := dailyRoot(t, "2026-05-01T09:00:00Z")
	now := mustParse(t, "2026-05-01T09:01:00Z")
	prior := &models.Task{TaskID: "child-1", Status: "done"}

	dec, err := Plan(now, root, prior)
	if err != nil {
		t.Fatal(err)
	}
	if dec.MarkMissedOnID != "" {
		t.Errorf("done child should not be marked missed: %+v", dec)
	}
}

func TestPlan_PriorChildCancelled_NoMissedMarker(t *testing.T) {
	root := dailyRoot(t, "2026-05-01T09:00:00Z")
	prior := &models.Task{TaskID: "child-1", Status: "cancelled"}

	dec, _ := Plan(time.Now(), root, prior)
	if dec.MarkMissedOnID != "" {
		t.Errorf("cancelled child should not be marked missed")
	}
}

func TestPlan_SoftDeletedRoot_Errors(t *testing.T) {
	root := dailyRoot(t, "2026-05-01T09:00:00Z")
	now := time.Now().UTC()
	root.DeletedAt = &now

	if _, err := Plan(now, root, nil); err == nil {
		t.Error("expected error for soft-deleted root")
	}
}

func TestPlan_NoFrequency_Errors(t *testing.T) {
	root := dailyRoot(t, "2026-05-01T09:00:00Z")
	root.RecurrenceFreq = nil
	if _, err := Plan(time.Now(), root, nil); err == nil {
		t.Error("expected error when RecurrenceFreq nil")
	}
}

func TestPlan_NoNextRecurrence_Errors(t *testing.T) {
	root := dailyRoot(t, "2026-05-01T09:00:00Z")
	root.NextRecurrenceAt = nil
	if _, err := Plan(time.Now(), root, nil); err == nil {
		t.Error("expected error when NextRecurrenceAt nil")
	}
}

func TestPlan_PropagatesAssignment(t *testing.T) {
	root := dailyRoot(t, "2026-05-01T09:00:00Z")
	memberID := "m-assignee"
	root.AssignedToMemberID = &memberID

	dec, err := Plan(time.Now(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dec.SpawnChild.AssignedToMemberID == nil || *dec.SpawnChild.AssignedToMemberID != memberID {
		t.Errorf("child should inherit assigned_to_member_id; got %v", dec.SpawnChild.AssignedToMemberID)
	}
}

func TestPlan_ChildIsNotItselfRecurring(t *testing.T) {
	root := dailyRoot(t, "2026-05-01T09:00:00Z")
	dec, err := Plan(time.Now(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The child must not have its own RecurrenceFreq, otherwise the next
	// tick would try to spawn from the child too — leading to runaway
	// recurrence chains.
	if dec.SpawnChild.RecurrenceFreq != nil {
		t.Errorf("spawned child should not be recurring itself; got %v", *dec.SpawnChild.RecurrenceFreq)
	}
	if dec.SpawnChild.NextRecurrenceAt != nil {
		t.Errorf("spawned child should not carry NextRecurrenceAt; got %v", *dec.SpawnChild.NextRecurrenceAt)
	}
}
