package csilservices

import (
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
)

func TestCalendarContentAllDayUsesEventZone(t *testing.T) {
	start, end := time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC), time.Date(2026, 3, 9, 6, 0, 0, 0, time.UTC)
	zone := "America/Denver"
	all := true
	out, err := calendarContent(csil.Event{Title: "DST day", StartsAt: &start, EndsAt: &end, Timezone: &zone, AllDay: &all}, "calendar", "uid")
	if err != nil {
		t.Fatal(err)
	}
	if out.Start != nil || out.End != nil || out.StartDate == nil || *out.StartDate != "2026-03-08" || out.EndDate == nil || *out.EndDate != "2026-03-09" {
		t.Fatalf("wrong all-day dates: %+v", out)
	}
	// catalystcal also returns synthetic midnight-UTC timestamps for all-day
	// events. The calendar dates, not these timestamps, determine local bounds.
	syntheticStart, syntheticEnd := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	out.Start = &syntheticStart
	out.End = &syntheticEnd
	a, b, err := remoteBounds(out)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Equal(start) || !b.Equal(end) {
		t.Fatalf("round trip changed DST day: %s %s", a, b)
	}
}
func TestCalendarContentQuarterlyAndUnscheduled(t *testing.T) {
	start, end := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC), time.Date(2026, 11, 5, 13, 0, 0, 0, time.UTC)
	freq := csil.RecurrenceFreq("quarterly")
	interval := int64(2)
	out, err := calendarContent(csil.Event{Title: "Review", StartsAt: &start, EndsAt: &end, RecurrenceFreq: &freq, RecurrenceInterval: &interval}, "calendar", "uid")
	if err != nil {
		t.Fatal(err)
	}
	if out.Recurrence == nil || out.Recurrence.Rrule != "FREQ=MONTHLY;INTERVAL=6" {
		t.Fatalf("wrong quarterly interval: %+v", out.Recurrence)
	}
	if _, err = calendarContent(csil.Event{Title: "Unscheduled"}, "calendar", "uid"); err == nil {
		t.Fatal("published unscheduled item")
	}
}
