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
	got, err := Next(from, Hourly, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := mustParse(t, "2026-05-01T09:00:00Z")
	if !got.Equal(want) {
		t.Errorf("hourly+1: got %s, want %s", got, want)
	}

	got, _ = Next(from, Hourly, 4, nil, nil)
	want = mustParse(t, "2026-05-01T12:00:00Z")
	if !got.Equal(want) {
		t.Errorf("hourly+4: got %s, want %s", got, want)
	}
}

func TestNext_Daily(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Daily, 1, nil, nil)
	if !got.Equal(mustParse(t, "2026-05-02T09:00:00Z")) {
		t.Errorf("daily+1: got %s", got)
	}
}

func TestNext_Weekly_NoByWeekday(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z") // Friday
	got, _ := Next(from, Weekly, 1, nil, nil)
	if !got.Equal(mustParse(t, "2026-05-08T09:00:00Z")) {
		t.Errorf("weekly+1: got %s", got)
	}
}

func TestNext_Weekly_Weekdays(t *testing.T) {
	// Friday → next weekday-ish occurrence with interval=1 lands seven days
	// later (also a Friday); byWeekday=[1..5] (Mon..Fri) accepts it.
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Weekly, 1, []int{1, 2, 3, 4, 5}, nil)
	if w := got.Weekday(); w < time.Monday || w > time.Friday {
		t.Errorf("weekday filter accepted non-weekday: %v", w)
	}
}

func TestNext_Weekly_Weekends(t *testing.T) {
	// Friday → next, with byWeekday = Sat/Sun, should land on Sat the
	// week after (since interval=1 first jumps 7 days, then we forward-
	// scan to the next allowed weekday).
	from := mustParse(t, "2026-05-01T09:00:00Z") // Friday
	got, _ := Next(from, Weekly, 1, []int{0, 6}, nil)
	w := got.Weekday()
	if w != time.Saturday && w != time.Sunday {
		t.Errorf("weekend filter: got %v", w)
	}
}

func TestNext_Weekly_WeekdaysPreset(t *testing.T) {
	// "Every weekday" pattern: weekly + interval=1 + byWeekday=Mon..Fri.
	// Each call should advance one weekday at a time, not jump a full
	// week between matches.
	from := mustParse(t, "2026-05-04T09:00:00Z") // Monday
	allowed := []int{1, 2, 3, 4, 5}
	want := []string{
		"2026-05-05T09:00:00Z", // Tue
		"2026-05-06T09:00:00Z", // Wed
		"2026-05-07T09:00:00Z", // Thu
		"2026-05-08T09:00:00Z", // Fri
		"2026-05-11T09:00:00Z", // skip Sat+Sun → next Mon
		"2026-05-12T09:00:00Z", // Tue
	}
	cur := from
	for i, w := range want {
		next, err := Next(cur, Weekly, 1, allowed, nil)
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if !next.Equal(mustParse(t, w)) {
			t.Errorf("step %d: got %s, want %s", i, next, w)
		}
		cur = next
	}
}

func TestNext_Weekly_EveryOtherTuesday(t *testing.T) {
	// "Every other Tuesday": interval=2 + byWeekday=[2]. Should still use
	// the "jump to active week, then forward-scan" path so we land 14
	// days out from each anchor.
	from := mustParse(t, "2026-05-05T09:00:00Z") // Tue
	got, _ := Next(from, Weekly, 2, []int{int(time.Tuesday)}, nil)
	want := mustParse(t, "2026-05-19T09:00:00Z") // 14 days later, Tue
	if !got.Equal(want) {
		t.Errorf("every-other-Tue: got %s, want %s", got, want)
	}
}

func TestNext_EveryFewWeeks(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Weekly, 3, nil, nil)
	if !got.Equal(mustParse(t, "2026-05-22T09:00:00Z")) {
		t.Errorf("weekly+3: got %s", got)
	}
}

func TestNext_Monthly(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Monthly, 1, nil, nil)
	if !got.Equal(mustParse(t, "2026-06-01T09:00:00Z")) {
		t.Errorf("monthly+1: got %s", got)
	}
	got, _ = Next(from, Monthly, 2, nil, nil)
	if !got.Equal(mustParse(t, "2026-07-01T09:00:00Z")) {
		t.Errorf("every-other-month: got %s", got)
	}
}

func TestNext_Quarterly(t *testing.T) {
	from := mustParse(t, "2026-01-15T09:00:00Z")
	got, _ := Next(from, Quarterly, 1, nil, nil)
	if !got.Equal(mustParse(t, "2026-04-15T09:00:00Z")) {
		t.Errorf("quarterly: got %s", got)
	}
}

func TestNext_Yearly(t *testing.T) {
	from := mustParse(t, "2026-01-15T09:00:00Z")
	got, _ := Next(from, Yearly, 1, nil, nil)
	if !got.Equal(mustParse(t, "2027-01-15T09:00:00Z")) {
		t.Errorf("yearly+1: got %s", got)
	}
	got, _ = Next(from, Yearly, 5, nil, nil)
	if !got.Equal(mustParse(t, "2031-01-15T09:00:00Z")) {
		t.Errorf("every-N-years: got %s", got)
	}
}

func TestNext_UnknownFrequency(t *testing.T) {
	if _, err := Next(time.Now(), "decadal", 1, nil, nil); !errors.Is(err, ErrUnknownFrequency) {
		t.Errorf("expected ErrUnknownFrequency, got %v", err)
	}
}

func TestNext_IntervalDefaultsToOne(t *testing.T) {
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Daily, 0, nil, nil)
	if !got.Equal(mustParse(t, "2026-05-02T09:00:00Z")) {
		t.Errorf("interval 0 should default to 1: got %s", got)
	}
	got, _ = Next(from, Daily, -3, nil, nil)
	if !got.Equal(mustParse(t, "2026-05-02T09:00:00Z")) {
		t.Errorf("negative interval should default to 1: got %s", got)
	}
}

// ----- Setpos + ByWeekday -----

// intPtr is a tiny helper so the test calls stay readable.
func intPtr(v int) *int { return &v }

func TestNext_Monthly_SecondThursday(t *testing.T) {
	// May 2026: Thursdays fall on 7, 14, 21, 28. From May 7 with monthly+1
	// + by_weekday=[Thu] + setpos=2 we want the second Thursday of *June*
	// (the next period), which is Jun 11. (June 2026 Thursdays: 4, 11, 18, 25.)
	from := mustParse(t, "2026-05-07T09:00:00Z")
	got, err := Next(from, Monthly, 1, []int{int(time.Thursday)}, intPtr(2))
	if err != nil {
		t.Fatal(err)
	}
	want := mustParse(t, "2026-06-11T09:00:00Z")
	if !got.Equal(want) {
		t.Errorf("second Thursday of next month: got %s, want %s", got, want)
	}
}

func TestNext_Monthly_LastFriday(t *testing.T) {
	// From May 1 with by_weekday=[Fri] + setpos=-1 → last Friday of June 2026.
	// June 2026 Fridays: 5, 12, 19, 26. Last = Jun 26.
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Monthly, 1, []int{int(time.Friday)}, intPtr(-1))
	want := mustParse(t, "2026-06-26T09:00:00Z")
	if !got.Equal(want) {
		t.Errorf("last Friday of next month: got %s, want %s", got, want)
	}
}

func TestNext_Quarterly_ThirdTuesday(t *testing.T) {
	// From May 5 (Q2 = Apr-Jun) with quarterly+1 → next quarter is Q3 (Jul-Sep).
	// Tuesdays in Jul-Sep 2026: Jul 7,14,21,28; Aug 4,11,18,25; Sep 1,8,15,22,29.
	// Third Tuesday of the quarter = Jul 21.
	from := mustParse(t, "2026-05-05T10:30:00Z")
	got, _ := Next(from, Quarterly, 1, []int{int(time.Tuesday)}, intPtr(3))
	want := mustParse(t, "2026-07-21T10:30:00Z")
	if !got.Equal(want) {
		t.Errorf("third Tuesday of next quarter: got %s, want %s", got, want)
	}
}

func TestNext_Yearly_FirstMonday(t *testing.T) {
	// From Jul 4 2026 with yearly+1 → first Monday of 2027.
	// Jan 2027: Jan 1 = Fri, Jan 4 = Mon. → Jan 4.
	from := mustParse(t, "2026-07-04T08:00:00Z")
	got, _ := Next(from, Yearly, 1, []int{int(time.Monday)}, intPtr(1))
	want := mustParse(t, "2027-01-04T08:00:00Z")
	if !got.Equal(want) {
		t.Errorf("first Monday of next year: got %s, want %s", got, want)
	}
}

func TestNext_Monthly_SetposOvershoot(t *testing.T) {
	// Asking for the 5th Sunday of June 2026 (which has only 4 Sundays:
	// 7, 14, 21, 28). The fallback is the latest matching weekday — Jun 28.
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Monthly, 1, []int{int(time.Sunday)}, intPtr(5))
	want := mustParse(t, "2026-06-28T09:00:00Z")
	if !got.Equal(want) {
		t.Errorf("setpos overshoot fallback: got %s, want %s", got, want)
	}
}

func TestNext_Monthly_NoSetpos_DefaultsToFirst(t *testing.T) {
	// With byWeekday but no setpos, behavior defaults to "first matching
	// weekday in the next period."
	from := mustParse(t, "2026-05-01T09:00:00Z")
	got, _ := Next(from, Monthly, 1, []int{int(time.Wednesday)}, nil)
	// June 2026 Wednesdays: 3, 10, 17, 24. First = Jun 3.
	want := mustParse(t, "2026-06-03T09:00:00Z")
	if !got.Equal(want) {
		t.Errorf("default first weekday: got %s, want %s", got, want)
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

// TestPlan_PropagatesAssignment is intentionally removed: with the single
// assigned_to_member_id column gone, assignment propagation now happens at
// the store level (copy task_assignees rows from root → spawned child)
// rather than as a field copy on the Task struct. A replacement test will
// live next to the new CSIL-RPC TaskService that owns the join-table writes.

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
