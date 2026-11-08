//go:build integration

package cmd

import (
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilservices"
	"github.com/catalystcommunity/longhouse/api/internal/store"
)

func TestRecurrenceClear_Postgres_EventWire(t *testing.T) {
	f := newCalendarFixture(t)
	event := f.proposal(t)
	event, err := f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: event.EventId, Revision: *event.Revision})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	event.ClearRecurrence = &yes
	// Keep the previous schedule fields: the explicit clear must take priority.
	request, err := csil.DecodeEvent(csil.EncodeEvent(event))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := f.events.UpdateEvent(f.ownerCtx, request)
	if err != nil {
		t.Fatal(err)
	}
	if changed.RecurrenceFreq != nil || changed.NextRecurrenceAt != nil || len(changed.RecurrenceByWeekday) != 0 || changed.RecurrenceBySetpos != nil {
		t.Fatalf("recurrence remains after clear: %+v", changed)
	}
}

func TestRecurrenceClear_Postgres_TaskWire(t *testing.T) {
	f := newCalendarFixture(t)
	service := csilservices.TaskService{Store: store.AppStore}
	freq, due := csil.RecurrenceFreq("weekly"), csil.Timestamp("2026-11-15T16:00:00Z")
	task, err := service.CreateTask(f.ownerCtx, csil.Task{HouseId: csil.HouseID(f.house), OwnerMemberId: csil.MemberID(f.owner), Title: "Repeat", DueAt: &due, RecurrenceFreq: &freq})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	task.ClearRecurrence = &yes
	request, err := csil.DecodeTask(csil.EncodeTask(task))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := service.UpdateTask(f.ownerCtx, request)
	if err != nil {
		t.Fatal(err)
	}
	if changed.RecurrenceFreq != nil || changed.NextRecurrenceAt != nil || len(changed.RecurrenceByWeekday) != 0 || changed.RecurrenceBySetpos != nil {
		t.Fatalf("recurrence remains after clear: %+v", changed)
	}
}

func TestRecurrenceClear_Postgres_RemoteEvent(t *testing.T) {
	f := newCalendarFixture(t)
	remote := &calendarTestRemote{}
	f.enable(t, remote)
	event := f.proposal(t)
	event, err := f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: event.EventId, Revision: *event.Revision})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	event.ClearRecurrence = &yes
	changed, err := f.events.UpdateEvent(f.ownerCtx, event)
	if err != nil {
		t.Fatal(err)
	}
	if remote.event.Recurrence != nil || changed.RecurrenceRule != nil {
		t.Fatalf("remote recurrence remains: %+v", remote.event.Recurrence)
	}
}
