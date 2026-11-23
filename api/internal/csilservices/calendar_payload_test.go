package csilservices

import (
	"bytes"
	"testing"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	"github.com/fxamacker/cbor/v2"
)

func TestRecoverOldCalendarPayload(t *testing.T) {
	at := time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC)
	start, end := "2026-03-08", "2026-03-09"
	for _, created := range []string{"", at.Format(time.RFC3339)} {
		// Publications and deadline creates used empty receive-only times.
		// A split saved a copy of the series, including its original times.
		fields := map[string]any{
			"event_id": "series", "calendar_id": "house", "title": "Meeting", "sequence": 3,
			"start": at.Format(time.RFC3339), "start_date": start, "end_date": end,
			"created_at": created, "updated_at": created, "recurrence_id": at.Format(time.RFC3339),
			"recurrence": map[string]any{"rrule": "FREQ=DAILY", "exdates": []string{at.Format(time.RFC3339)}, "rdates": []string{at.AddDate(0, 0, 1).Format(time.RFC3339)}},
		}
		payload, err := cbor.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cal.DecodeEvent(payload); err == nil {
			t.Fatal("old payload passed the wire decoder")
		}
		event, err := decodeSavedEvent(payload)
		if err != nil {
			t.Fatal(err)
		}
		want := cal.Event{EventId: "series", CalendarId: "house", Title: "Meeting", Sequence: 3, Start: &at,
			StartDate: &start, EndDate: &end, RecurrenceId: &at, Recurrence: &cal.Recurrence{
				Rrule: "FREQ=DAILY", Exdates: []time.Time{at}, Rdates: []time.Time{at.AddDate(0, 0, 1)},
			}}
		if created != "" {
			want.CreatedAt, want.UpdatedAt = at, at
		}
		if !bytes.Equal(cal.EncodeEvent(event), cal.EncodeEvent(want)) {
			t.Fatalf("saved request changed: %+v", event)
		}
	}
}
