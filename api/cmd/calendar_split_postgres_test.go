//go:build integration

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilservices"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type splitTestRemote struct {
	*calendarTestRemote
	splitCalls, receiptCalls, splits int
	saved                            []byte
	receipt                          *cal.SplitSeriesReceipt
	splitErr, lookupErr              error
	failBefore, inPlace, badMapping  bool
	failBatch                        int
	none                             map[string]bool
}

func (f *splitTestRemote) ApplicationTransport() cal.Transport { return f }
func (f *splitTestRemote) Member(ctx context.Context, domain, user, grant string) (cal.Transport, error) {
	if _, err := f.calendarTestRemote.Member(ctx, domain, user, grant); err != nil {
		return nil, err
	}
	return f, nil
}
func (f *splitTestRemote) Call(ctx context.Context, service, op string, b []byte) ([]byte, error) {
	switch op {
	case "split-series-once":
		request, err := cal.DecodeSplitSeriesOnceRequest(b)
		if err != nil {
			return nil, err
		}
		f.splitCalls++
		payload := cal.EncodeEvent(request.Event)
		if f.saved != nil && !bytes.Equal(payload, f.saved) {
			return nil, &cal.ClientError{Code: 409, Message: "changed request"}
		}
		f.saved = payload
		if f.failBefore {
			return nil, errors.New("connection failed before commit")
		}
		if f.receipt == nil {
			f.splits++
			successor := request.Event
			successor.EventId = "remote-successor"
			if f.inPlace {
				successor.EventId = request.Event.EventId
			}
			original := f.event
			f.receipt = &cal.SplitSeriesReceipt{SeriesId: request.Event.EventId, IdempotencyKey: request.IdempotencyKey, SuccessorId: successor.EventId, SplitAt: request.RecurrenceId, InPlace: f.inPlace, Created: true, CreatedAt: "2026-11-07T12:00:00Z", CalendarId: request.Event.CalendarId, SeriesState: "live", Series: &original, SuccessorState: "live", Successor: &successor}
		}
		if f.splitErr != nil {
			return nil, f.splitErr
		}
		return f.mapped(request.MapRecurrenceIds)
	case "get-split-receipt":
		request, err := cal.DecodeSplitReceiptRequest(b)
		if err != nil {
			return nil, err
		}
		f.receiptCalls++
		if f.lookupErr != nil {
			return nil, f.lookupErr
		}
		if f.failBatch == f.receiptCalls {
			return nil, errors.New("lost mapping batch")
		}
		if f.receipt == nil {
			return nil, &cal.ClientError{Code: 404, Message: "no receipt"}
		}
		if request.SeriesId != f.receipt.SeriesId || request.IdempotencyKey != f.receipt.IdempotencyKey {
			return nil, &cal.ClientError{Code: 404, Message: "wrong receipt"}
		}
		return f.mapped(request.MapRecurrenceIds)
	case "get-event":
		if f.receipt == nil {
			break
		}
		id, err := cal.DecodeEventGetEventRequest(b)
		if err != nil {
			return nil, err
		}
		if id == f.receipt.SuccessorId {
			if f.receipt.SuccessorState != "live" {
				return nil, &cal.ClientError{Code: 404, Message: "successor unavailable"}
			}
			return cal.EncodeEvent(*f.receipt.Successor), nil
		}
		if id == f.receipt.SeriesId {
			return cal.EncodeEvent(*f.receipt.Series), nil
		}
	case "get-occurrence":
		if f.receipt == nil {
			break
		}
		request, err := cal.DecodeGetOccurrenceRequest(b)
		if err != nil {
			return nil, err
		}
		event := f.receipt.Series
		if request.EventId == f.receipt.SuccessorId {
			event = f.receipt.Successor
		}
		if event == nil || request.RecurrenceId == nil {
			return nil, &cal.ClientError{Code: 404, Message: "not available"}
		}
		start, err := time.Parse(time.RFC3339, string(*request.RecurrenceId))
		if err != nil {
			return nil, err
		}
		return cal.EncodeOccurrence(cal.Occurrence{Event: *event, RecurrenceId: request.RecurrenceId, Start: *request.RecurrenceId, End: cal.Timestamp(start.Add(time.Hour).Format(time.RFC3339)), Access: "owner"}), nil
	}
	return f.calendarTestRemote.Call(ctx, service, op, b)
}
func (f *splitTestRemote) mapped(ids []cal.Timestamp) ([]byte, error) {
	if len(ids) > 500 {
		return nil, errors.New("too many mapping IDs")
	}
	out := *f.receipt
	out.Created = false
	out.Mappings = nil
	boundary, _ := time.Parse(time.RFC3339, string(out.SplitAt))
	for _, rid := range ids {
		original, _ := time.Parse(time.RFC3339, string(rid))
		target := out.SeriesId
		mapped := rid
		result := cal.SplitMapResult("original")
		if !original.Before(boundary) {
			target = out.SuccessorId
			mapped = cal.Timestamp(original.Add(time.Hour).Format(time.RFC3339))
			result = "successor"
		}
		mapping := cal.SplitOccurrenceMapping{RecurrenceId: rid, Result: result, EventId: &target, MappedRecurrenceId: &mapped}
		if f.none[string(rid)] {
			mapping.Result = "none"
			mapping.EventId = nil
			mapping.MappedRecurrenceId = nil
		}
		out.Mappings = append(out.Mappings, mapping)
	}
	if f.badMapping && len(out.Mappings) > 0 {
		out.Mappings[0].RecurrenceId = "1999-01-01T00:00:00Z"
	}
	return cal.EncodeSplitSeriesReceipt(out), nil
}
func splitFixture(t *testing.T) (*calendarFixture, *splitTestRemote, []csil.Event) {
	t.Helper()
	f := newCalendarFixture(t)
	remote := &splitTestRemote{calendarTestRemote: &calendarTestRemote{}, none: map[string]bool{}}
	f.enable(t, remote.calendarTestRemote)
	f.svc.Remote = remote
	proposal := f.proposal(t)
	if _, err := f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: proposal.EventId, Revision: *proposal.Revision}); err != nil {
		t.Fatal(err)
	}
	start, end := csil.Timestamp("2026-11-01T00:00:00Z"), csil.Timestamp("2026-12-01T00:00:00Z")
	rows := []csil.Event{}
	for _, day := range []int{10, 17, 24} {
		rid := cal.Timestamp(fmt.Sprintf("2026-11-%02dT15:00:00Z", day))
		remote.occurrence = &cal.Occurrence{Event: remote.event, Start: rid, End: cal.Timestamp(fmt.Sprintf("2026-11-%02dT16:00:00Z", day)), RecurrenceId: &rid, Access: "owner"}
		found, err := f.svc.ListEvents(f.ownerCtx, csil.EventListRequest{HouseId: csil.HouseID(f.house), RangeStart: &start, RangeEnd: &end})
		if err != nil || len(found) != 1 {
			t.Fatalf("seed occurrence: %+v %v", found, err)
		}
		rows = append(rows, found[0])
	}
	return f, remote, rows
}
func splitEdit(e csil.Event) csil.Event {
	scope := "this_and_future"
	e.EditScope = &scope
	start, end := csil.Timestamp("2026-11-17T16:00:00Z"), csil.Timestamp("2026-11-17T17:00:00Z")
	e.StartsAt = &start
	e.EndsAt = &end
	e.Title = "Changed future"
	return e
}
func TestCalendar_Postgres_SplitPreservesLinksAndUnbindsMissingOccurrence(t *testing.T) {
	f, remote, rows := splitFixture(t)
	remote.none["2026-11-24T15:00:00Z"] = true
	comments := &csilservices.CommentService{Store: store.AppStore, Calendar: f.svc}
	comment, err := comments.CreateComment(f.ownerCtx, csil.Comment{TargetType: "event", TargetId: string(rows[2].EventId), Body: "Keep this discussion."})
	if err != nil {
		t.Fatal(err)
	}
	result, err := f.svc.UpdateEvent(f.ownerCtx, splitEdit(rows[1]))
	if err != nil {
		t.Fatal(err)
	}
	if result.EventId != rows[1].EventId || result.StartsAt == nil || *result.StartsAt != "2026-11-17T16:00:00Z" || result.RecurrenceRootEventId == nil || *result.RecurrenceRootEventId == *rows[0].RecurrenceRootEventId {
		t.Fatalf("wrong successor reference: %+v", result)
	}
	before, err := f.svc.GetEvent(f.ownerCtx, rows[0].EventId)
	if err != nil || *before.StartsAt != *rows[0].StartsAt {
		t.Fatalf("earlier occurrence changed: %+v %v", before, err)
	}
	gone, err := f.svc.GetEvent(f.ownerCtx, rows[2].EventId)
	if err != nil || gone.CalendarUnbound == nil || !*gone.CalendarUnbound || gone.PublicationError == nil {
		t.Fatalf("unbound reference lost: %+v %v", gone, err)
	}
	kept, err := comments.GetComment(f.ownerCtx, comment.CommentId)
	if err != nil || kept.TargetId != string(gone.EventId) {
		t.Fatal("unbound comment lost", err)
	}
	_, err = f.svc.UpdateEvent(f.ownerCtx, gone)
	calendarCode(t, err, 409)
	_, err = f.svc.DeleteEvent(f.ownerCtx, gone.EventId, false)
	calendarCode(t, err, 409)
	var retained int64
	if err = f.db.Table("calendar_splits").Where("NOT completed OR payload IS NOT NULL").Count(&retained).Error; err != nil || retained != 0 {
		t.Fatal("split did not finish", err)
	}
	if remote.splits != 1 {
		t.Fatal("wrong split count")
	}
}

func TestCalendar_Postgres_SplitLostResponseRecovery(t *testing.T) {
	f, remote, rows := splitFixture(t)
	remote.splitErr = errors.New("response lost after commit")
	_, err := f.svc.UpdateEvent(f.ownerCtx, splitEdit(rows[1]))
	calendarCode(t, err, 409)
	pending, err := f.svc.GetEvent(f.ownerCtx, rows[1].EventId)
	if err != nil || pending.EditPendingMemberId == nil || string(*pending.EditPendingMemberId) != f.owner {
		t.Fatalf("pending edit not shown: %+v %v", pending, err)
	}
	_, err = f.svc.UpdateEvent(f.ownerCtx, rows[0])
	calendarCode(t, err, 409)
	_, err = f.svc.DeleteEvent(f.ownerCtx, rows[0].EventId, true)
	calendarCode(t, err, 409)
	_, err = f.svc.RecoverEdit(f.adminCtx, rows[1].EventId)
	calendarCode(t, err, 403)
	restarted := &csilservices.CalendarService{Store: store.AppStore, DB: f.db, Remote: remote}
	result, err := restarted.RecoverEdit(f.ownerCtx, rows[1].EventId)
	if err != nil || result.EditPendingMemberId != nil || result.Title != "Changed future" {
		t.Fatalf("recovery: %+v %v", result, err)
	}
	if remote.splits != 1 || remote.splitCalls != 1 {
		t.Fatal("recovery repeated the split")
	}
	_, err = restarted.UpdateEvent(f.ownerCtx, rows[0])
	calendarCode(t, err, 409)
}

func TestCalendar_Postgres_SplitRecoveryAfterAccessLossAndNoCommit(t *testing.T) {
	f, remote, rows := splitFixture(t)
	remote.failBefore = true
	_, err := f.svc.UpdateEvent(f.ownerCtx, splitEdit(rows[1]))
	calendarCode(t, err, 409)
	remote.lookupErr = &cal.ClientError{Code: 403, Message: "access lost"}
	_, err = f.svc.RecoverEdit(f.ownerCtx, rows[1].EventId)
	calendarCode(t, err, 409)
	if remote.splitCalls != 1 {
		t.Fatal("access failure sent a new split")
	}
	remote.lookupErr = nil
	remote.failBefore = false
	_, err = f.svc.RecoverEdit(f.ownerCtx, rows[1].EventId)
	if err != nil || remote.splits != 1 || remote.splitCalls != 2 {
		t.Fatal("saved request did not recover", err)
	}
}

func TestCalendar_Postgres_SplitMappingBatchesAreAtomic(t *testing.T) {
	f, remote, rows := splitFixture(t)
	// These are local references to a long-lived series, not new remote events.
	base := time.Date(2027, 1, 1, 15, 0, 0, 0, time.UTC)
	for i := 0; i < 501; i++ {
		e := models.Event{HouseID: f.house, OwnerMemberID: f.owner, CalendarState: "remote", Revision: 1}
		if err := f.db.Create(&e).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.db.Exec("INSERT INTO calendar_refs(ref_id,house_id,remote_event_id,recurrence_id) VALUES (?,?,?,?)", e.EventID, f.house, remote.event.EventId, base.AddDate(0, 0, i*7).Format(time.RFC3339)).Error; err != nil {
			t.Fatal(err)
		}
	}
	remote.failBatch = 1
	_, err := f.svc.UpdateEvent(f.ownerCtx, splitEdit(rows[1]))
	calendarCode(t, err, 409)
	var rebound int64
	f.db.Table("calendar_refs").Where("remote_event_id='remote-successor'").Count(&rebound)
	if rebound != 0 {
		t.Fatal("part of a mapping batch committed")
	}
	remote.failBatch = 0
	_, err = f.svc.RecoverEdit(f.ownerCtx, rows[1].EventId)
	if err != nil || remote.splitCalls != 1 || remote.receiptCalls != 3 {
		t.Fatalf("batch recovery: %v calls=%d", err, remote.receiptCalls)
	}
	f.db.Table("calendar_refs").Where("remote_event_id='remote-successor' AND recurrence_id<>''").Count(&rebound)
	if rebound != 503 {
		t.Fatalf("rebound %d, want 503", rebound)
	}
}

func TestCalendar_Postgres_SplitInvalidMappingAndFailedLocalCommit(t *testing.T) {
	for _, failure := range []string{"mapping", "local_commit"} {
		t.Run(failure, func(t *testing.T) {
			f, remote, rows := splitFixture(t)
			if failure == "mapping" {
				remote.badMapping = true
			} else {
				if err := f.db.Exec("CREATE FUNCTION fail_split_ref() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'local commit failed'; END $$; CREATE TRIGGER fail_split_ref BEFORE UPDATE ON calendar_refs FOR EACH ROW EXECUTE FUNCTION fail_split_ref()").Error; err != nil {
					t.Fatal(err)
				}
			}
			_, err := f.svc.UpdateEvent(f.ownerCtx, splitEdit(rows[1]))
			if err == nil {
				t.Fatal("invalid mapping or local failure was ignored")
			}
			var changed int64
			f.db.Table("calendar_refs").Where("remote_event_id='remote-successor'").Count(&changed)
			if changed != 0 {
				t.Fatal("partial reference change committed")
			}
			// Direct store writes must also respect the pending edit.
			if err = f.db.Model(&models.Event{}).Where("event_id=?", rows[0].EventId).Update("title", "bypass").Error; err == nil {
				t.Fatal("pending edit was changed directly")
			}
			if failure == "mapping" {
				remote.badMapping = false
			} else {
				if err = f.db.Exec("DROP TRIGGER fail_split_ref ON calendar_refs; DROP FUNCTION fail_split_ref()").Error; err != nil {
					t.Fatal(err)
				}
			}
			result, err := f.svc.RecoverEdit(f.ownerCtx, rows[1].EventId)
			if err != nil || result.Title != "Changed future" || remote.splits != 1 || remote.splitCalls != 1 {
				t.Fatalf("recovery: %+v %v", result, err)
			}
		})
	}
}

func TestCalendar_Postgres_SplitTerminalAndInPlaceResults(t *testing.T) {
	for _, state := range []string{"trashed", "deleted", "hidden", "moved", "in_place"} {
		t.Run(state, func(t *testing.T) {
			f, remote, rows := splitFixture(t)
			remote.inPlace = state == "in_place"
			edited := rows[1]
			if remote.inPlace {
				edited = rows[0]
			}
			remote.splitErr = errors.New("lost reply")
			_, err := f.svc.UpdateEvent(f.ownerCtx, splitEdit(edited))
			calendarCode(t, err, 409)
			if state == "moved" {
				remote.receipt.Successor.CalendarId = "other-calendar"
			} else if state != "in_place" {
				remote.receipt.SuccessorState = cal.CreateReceiptState(state)
				remote.receipt.Successor = nil
			}
			result, err := f.svc.RecoverEdit(f.ownerCtx, edited.EventId)
			if err != nil {
				t.Fatal(err)
			}
			if result.EventId != edited.EventId || result.EditPendingMemberId != nil || remote.splitCalls != 1 {
				t.Fatalf("incorrect terminal recovery: %+v", result)
			}
			if state == "in_place" {
				if result.RecurrenceRootEventId == nil || *result.RecurrenceRootEventId != *edited.RecurrenceRootEventId {
					t.Fatal("in-place split changed the root link")
				}
			} else if result.PublicationError == nil {
				t.Fatal("unavailable successor has no notice")
			}
		})
	}
}
