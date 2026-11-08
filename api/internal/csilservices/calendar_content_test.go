package csilservices

import (
	cal "github.com/catalystcommunity/catalystcal/clients/go"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"testing"
)

func TestCalendarContentAllDayUsesEventZone(t *testing.T) {
	start, end := csil.Timestamp("2026-03-08T07:00:00Z"), csil.Timestamp("2026-03-09T06:00:00Z")
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
	syntheticStart, syntheticEnd := cal.Timestamp("2026-03-08T00:00:00Z"), cal.Timestamp("2026-03-09T00:00:00Z")
	out.Start = &syntheticStart
	out.End = &syntheticEnd
	a, b, err := remoteBounds(out)
	if err != nil {
		t.Fatal(err)
	}
	if a != string(start) || b != string(end) {
		t.Fatalf("round trip changed DST day: %s %s", a, b)
	}
}
func TestCalendarContentQuarterlyAndUnscheduled(t *testing.T) {
	start, end := csil.Timestamp("2026-11-05T12:00:00Z"), csil.Timestamp("2026-11-05T13:00:00Z")
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
