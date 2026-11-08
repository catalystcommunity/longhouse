//go:build integration

package cmd

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/csilservices"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

type calendarTestRemote struct {
	mu               sync.Mutex
	creates          int
	createCalls      int
	receipt          *cal.CreateEventReceipt
	receiptErr       error
	acceptedPayload  []byte
	failBefore       bool
	occurrence       *cal.Occurrence
	actor            string
	event            cal.Event
	createErr        error
	memberErr        error
	entered, release chan struct{}
}

func (f *calendarTestRemote) ApplicationTransport() cal.Transport { return f }
func (f *calendarTestRemote) ApplicationIdentity() cal.ApplicationRef {
	return cal.ApplicationRef{SubjectDomain: "soft.test", SubjectUserId: "app", ApplicationId: "longhouse"}
}
func (f *calendarTestRemote) MemberDomain() string { return "soft.test" }
func (f *calendarTestRemote) Begin(context.Context, string) (string, error) {
	return "", errors.New("unexpected consent request")
}
func (f *calendarTestRemote) Member(_ context.Context, domain, user, grant string) (cal.Transport, error) {
	if domain != "soft.test" || user != "owner" || grant != "owner-grant" {
		return nil, errors.New("wrong effective member")
	}
	if f.memberErr != nil {
		return nil, f.memberErr
	}
	f.mu.Lock()
	f.actor = user
	f.mu.Unlock()
	return f, nil
}
func (f *calendarTestRemote) Call(_ context.Context, service, op string, b []byte) ([]byte, error) {
	switch op {
	case "create-event-once":
		req, err := cal.DecodeCreateEventOnceRequest(b)
		if err != nil {
			return nil, err
		}
		f.mu.Lock()
		f.createCalls++
		payload := cal.EncodeEvent(req.Event)
		if f.acceptedPayload != nil && !bytes.Equal(f.acceptedPayload, payload) {
			f.mu.Unlock()
			return nil, &cal.ClientError{Code: 409, Message: "changed retry"}
		}
		f.acceptedPayload = payload
		if f.failBefore {
			f.mu.Unlock()
			return nil, errors.New("request did not reach server")
		}
		if f.receipt != nil {
			r := *f.receipt
			r.Created = false
			f.mu.Unlock()
			return cal.EncodeCreateEventReceipt(r), nil
		}
		f.creates++
		e := req.Event
		e.EventId = "remote-1"
		e.CreatedBy = &cal.PersonSummary{Domain: "soft.test", Subject: f.actor}
		f.event = e
		r := cal.CreateEventReceipt{CalendarId: e.CalendarId, IdempotencyKey: req.IdempotencyKey, EventId: e.EventId, State: "live", Created: true, CreatedAt: "2026-11-07T00:00:00Z", Event: &e}
		f.receipt = &r
		f.mu.Unlock()
		if f.entered != nil {
			close(f.entered)
			<-f.release
		}
		if f.createErr != nil {
			return nil, f.createErr
		}
		return cal.EncodeCreateEventReceipt(r), nil
	case "get-create-receipt":
		req, err := cal.DecodeCreateReceiptRequest(b)
		if err != nil {
			return nil, err
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.receiptErr != nil {
			return nil, f.receiptErr
		}
		if f.receipt == nil || req.CalendarId != f.receipt.CalendarId || req.IdempotencyKey != f.receipt.IdempotencyKey {
			return nil, &cal.ClientError{Code: 404, Message: "no receipt"}
		}
		r := *f.receipt
		r.Created = false
		return cal.EncodeCreateEventReceipt(r), nil
	case "get-occurrence":
		req, err := cal.DecodeGetOccurrenceRequest(b)
		if err != nil {
			return nil, err
		}
		if f.occurrence == nil || req.RecurrenceId == nil || f.occurrence.RecurrenceId == nil || *req.RecurrenceId != *f.occurrence.RecurrenceId {
			return nil, &cal.ClientError{Code: 404, Message: "no occurrence"}
		}
		return cal.EncodeOccurrence(*f.occurrence), nil
	case "list-events":
		out := cal.OccurrenceList{}
		if f.occurrence != nil {
			out.Occurrences = []cal.Occurrence{*f.occurrence}
		}
		return cal.EncodeOccurrenceList(out), nil
	case "get-event":
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.receipt != nil && f.receipt.State != "live" {
			return nil, &cal.ClientError{Code: 404, Message: "not live"}
		}
		return cal.EncodeEvent(f.event), nil
	case "update-event":
		in, err := cal.DecodeUpdateEventRequest(b)
		if err != nil {
			return nil, err
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.event = in.Event
		return cal.EncodeEvent(f.event), nil
	default:
		return nil, errors.New("unexpected remote call: " + service + "/" + op)
	}
}

type calendarFixture struct {
	db                  *gorm.DB
	svc                 *csilservices.CalendarService
	events              *csilservices.EventService
	house, admin, owner string
	adminCtx, ownerCtx  context.Context
}

func newCalendarFixture(t *testing.T) *calendarFixture {
	t.Helper()
	freshStore(t, requireTestDB(t))
	ctx := context.Background()
	f := &calendarFixture{db: store.AppStore.(*postgres.PostgresStore).CalendarDB()}
	f.house = mkHouse(t, ctx, "Calendar acceptance")
	f.admin = mkMember(t, ctx, f.house, "admin")
	f.owner = mkMember(t, ctx, f.house, "owner")
	role := &models.Role{HouseID: f.house, Name: "admin"}
	if err := store.AppStore.CreateRole(ctx, role); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&models.MemberRole{MemberID: f.admin, RoleID: role.RoleID}).Error; err != nil {
		t.Fatal(err)
	}
	caller := func(member, user string, roles ...string) context.Context {
		return auth.WithIdentity(ctx, &auth.Identity{Domain: "soft.test", UserID: user, Houses: []auth.HouseRoles{{House: f.house, Member: member, Roles: roles}}})
	}
	f.adminCtx = caller(f.admin, "admin", "admin")
	f.ownerCtx = caller(f.owner, "owner", "member")
	f.svc = &csilservices.CalendarService{Store: store.AppStore, DB: f.db}
	f.events = &csilservices.EventService{Store: store.AppStore, Calendar: f.svc}
	return f
}
func (f *calendarFixture) proposal(t *testing.T) csil.Event {
	t.Helper()
	start, end := csil.Timestamp("2026-11-10T15:00:00Z"), csil.Timestamp("2026-11-10T16:00:00Z")
	freq := csil.RecurrenceFreq("weekly")
	e, err := f.events.CreateEvent(f.adminCtx, csil.Event{HouseId: csil.HouseID(f.house), OwnerMemberId: csil.MemberID(f.owner), Title: "Assigned meeting", StartsAt: &start, EndsAt: &end, RecurrenceFreq: &freq})
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func calendarCode(t *testing.T, err error, code int) {
	t.Helper()
	var e *csilrpc.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("got %v, want HTTP %d", err, code)
	}
}

func TestCalendar_Postgres_LocalAcceptance(t *testing.T) {
	f := newCalendarFixture(t)
	e := f.proposal(t)
	if e.CalendarState == nil || *e.CalendarState != "pending" || e.NextRecurrenceAt != nil {
		t.Fatalf("proposal published or spawned: %+v", e)
	}
	rows, err := f.events.ListEvents(f.ownerCtx, csil.EventListRequest{HouseId: csil.HouseID(f.house)})
	if err != nil || len(rows) != 0 {
		t.Fatalf("pending event on calendar: %+v %v", rows, err)
	}
	local, err := f.svc.ListLocalEvents(f.ownerCtx, csil.HouseID(f.house))
	if err != nil || len(local) != 1 {
		t.Fatalf("local inbox: %+v %v", local, err)
	}
	_, err = f.svc.AcceptEvent(f.adminCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision})
	calendarCode(t, err, 403)
	oldRevision := *e.Revision
	e.Title = "Revised proposal"
	e, err = f.events.UpdateEvent(f.adminCtx, e)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: oldRevision})
	calendarCode(t, err, 409)
	accepted, err := f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if *accepted.CalendarState != "local" || accepted.EventId != e.EventId || accepted.Title != "Revised proposal" || accepted.NextRecurrenceAt == nil {
		t.Fatalf("incorrect acceptance: %+v", accepted)
	}
	var n int64
	f.db.Table("calendar_acceptances").Where("event_id=? AND member_id=?", e.EventId, f.owner).Count(&n)
	if n != 1 {
		t.Fatalf("acceptance history=%d", n)
	}
}
func TestCalendar_Postgres_DeclineAndStaleMembership(t *testing.T) {
	f := newCalendarFixture(t)
	e := f.proposal(t)
	e, err := f.svc.DeclineEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if *e.CalendarState != "declined" {
		t.Fatal("decline did not persist")
	}
	e.Title = "Revised after decline"
	e, err = f.events.UpdateEvent(f.adminCtx, e)
	if err != nil {
		t.Fatal(err)
	}
	if *e.CalendarState != "pending" {
		t.Fatal("edit did not reopen proposal")
	}
	if err = f.db.Model(&models.Member{}).Where("member_id=?", f.owner).Update("deactivated_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision})
	calendarCode(t, err, 403)
	_, err = f.events.GetEvent(f.ownerCtx, e.EventId)
	calendarCode(t, err, 403)
	if err = f.db.Where("member_id=?", f.admin).Delete(&models.MemberRole{}).Error; err != nil {
		t.Fatal(err)
	}
	_, err = f.events.CreateEvent(f.adminCtx, csil.Event{HouseId: csil.HouseID(f.house), OwnerMemberId: csil.MemberID(f.owner), Title: "stale admin"})
	calendarCode(t, err, 403)
}
func (f *calendarFixture) enable(t *testing.T, remote *calendarTestRemote) {
	t.Helper()
	f.svc.Remote = remote
	if _, err := f.svc.ConfigureCalendar(f.adminCtx, csil.ConfigureCalendarRequest{HouseId: csil.HouseID(f.house), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("UPDATE house_calendars SET calendar_id='calendar-1',ready=true WHERE house_id=?", f.house).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("INSERT INTO calendar_connections(member_id,grant_id) VALUES (?,'owner-grant')", f.owner).Error; err != nil {
		t.Fatal(err)
	}
}
func TestCalendar_Postgres_PublishOnlyAcceptedRevision(t *testing.T) {
	f := newCalendarFixture(t)
	remote := &calendarTestRemote{entered: make(chan struct{}), release: make(chan struct{})}
	f.enable(t, remote)
	e := f.proposal(t)
	if remote.creates != 0 {
		t.Fatal("proposal contacted catalystcal")
	}
	comments := &csilservices.CommentService{Store: store.AppStore, Calendar: f.svc}
	comment, err := comments.CreateComment(f.ownerCtx, csil.Comment{TargetType: "event", TargetId: string(e.EventId), Body: "Keep this comment after acceptance."})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision})
		done <- err
	}()
	select {
	case <-remote.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("publication did not start")
	}
	e.Title = "Concurrent edit"
	_, err = f.events.UpdateEvent(f.adminCtx, e)
	calendarCode(t, err, 409)
	_, err = f.events.DeleteEvent(f.adminCtx, e.EventId)
	calendarCode(t, err, 409)
	_, err = f.events.DeleteEventAndFuture(f.adminCtx, e.EventId)
	calendarCode(t, err, 409)
	// Store-level bulk paths must also preserve the frozen proposal.
	op, err := store.AppStore.NewID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AppStore.SoftDeleteEventSeries(context.Background(), string(e.EventId), f.admin, op); err == nil {
		t.Fatal("bulk deletion bypassed publication lock")
	}
	close(remote.release)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	published, err := f.events.GetEvent(f.ownerCtx, e.EventId)
	if err != nil {
		t.Fatal(err)
	}
	if published.EventId != e.EventId || *published.CalendarState != "remote" || published.Title != "Assigned meeting" || remote.creates != 1 || remote.actor != "owner" {
		t.Fatalf("wrong publication: %+v calls=%d actor=%s", published, remote.creates, remote.actor)
	}
	stored, err := store.AppStore.GetEventByID(context.Background(), string(e.EventId))
	if err != nil {
		t.Fatal(err)
	}
	if stored.Title != "" || stored.StartsAt != nil || stored.RecurrenceFreq != nil {
		t.Fatal("published event body remains local")
	}
	edited := published
	edited.Title = "Owner edit"
	changed, err := f.events.UpdateEvent(f.ownerCtx, edited)
	if err != nil || changed.Title != "Owner edit" {
		t.Fatalf("remote owner edit: %+v %v", changed, err)
	}
	_, err = f.events.UpdateEvent(f.ownerCtx, published)
	calendarCode(t, err, 409)
	kept, err := comments.GetComment(f.ownerCtx, comment.CommentId)
	if err != nil || kept.TargetId != string(e.EventId) {
		t.Fatalf("publication lost the comment reference: %+v %v", kept, err)
	}
	if err = store.AppStore.DeleteHouse(context.Background(), f.house); err != nil {
		t.Fatal(err)
	}
	var deleted bool
	if err = f.db.Raw("SELECT deleted FROM house_calendars WHERE house_id=?", f.house).Scan(&deleted).Error; err != nil || !deleted {
		t.Fatal("house deletion did not retain a cleanup job", err)
	}
}
func TestCalendar_Postgres_UncertainPublicationRecoversAfterRestart(t *testing.T) {
	f := newCalendarFixture(t)
	remote := &calendarTestRemote{createErr: errors.New("connection closed after commit")}
	f.enable(t, remote)
	e := f.proposal(t)
	result, err := f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if *result.CalendarState != "publishing" || result.PublicationError == nil {
		t.Fatalf("uncertain result was not retained: %+v", result)
	}
	// New service instance uses only the saved database journal.
	restarted := &csilservices.CalendarService{Store: store.AppStore, DB: f.db, Remote: remote}
	result, err = restarted.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision})
	if err != nil || *result.CalendarState != "remote" || result.EventId != e.EventId {
		t.Fatalf("recovery: %+v %v", result, err)
	}
	if remote.creates != 1 || remote.createCalls != 1 {
		t.Fatal("recovery sent another create")
	}
	var retained int64
	if err = f.db.Table("calendar_publications").Where("event_id=? AND payload IS NOT NULL", e.EventId).Count(&retained).Error; err != nil || retained != 0 {
		t.Fatal("publication payload retained", err)
	}

}
func TestCalendar_Postgres_MigrationAndOutbox(t *testing.T) {
	f := newCalendarFixture(t)
	e := f.proposal(t)
	// Simulate a source event with unknown origin.
	if err := f.db.Model(&models.Event{}).Where("event_id=?", e.EventId).Updates(map[string]any{"calendar_state": "legacy", "proposed_by_member_id": nil}).Error; err != nil {
		t.Fatal(err)
	}
	report, err := f.svc.PreviewMigration(f.adminCtx, csil.HouseID(f.house))
	if err != nil || len(report) != 1 || report[0].Disposition != "owner_review" {
		t.Fatalf("migration report: %+v %v", report, err)
	}
	f.enable(t, &calendarTestRemote{})
	var n int64
	f.db.Table("calendar_legacy_backup").Where("event_id=?", e.EventId).Count(&n)
	if n != 1 {
		t.Fatal("legacy body was not backed up")
	}
	var before, after int64
	f.db.Raw("SELECT sync_generation FROM house_calendars WHERE house_id=?", f.house).Scan(&before)
	rollback := errors.New("rollback")
	err = f.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Member{}).Where("member_id=?", f.owner).Update("deactivated_at", time.Now()).Error; err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	f.db.Raw("SELECT sync_generation FROM house_calendars WHERE house_id=?", f.house).Scan(&after)
	if after != before {
		t.Fatal("rolled back membership queued remote sync")
	}
	if err = f.db.Model(&models.Member{}).Where("member_id=?", f.owner).Update("deactivated_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	f.db.Raw("SELECT sync_generation FROM house_calendars WHERE house_id=?", f.house).Scan(&after)
	if after != before+1 {
		t.Fatal("membership change did not queue sync")
	}
}

func TestCalendar_Postgres_ReceiptRecoveryStates(t *testing.T) {
	for _, state := range []string{"live", "trashed", "deleted", "hidden", "moved", "uncommitted", "denied"} {
		t.Run(state, func(t *testing.T) {
			f := newCalendarFixture(t)
			remote := &calendarTestRemote{createErr: errors.New("lost reply"), failBefore: state == "uncommitted"}
			f.enable(t, remote)
			e := f.proposal(t)
			decision := csil.EventDecision{EventId: e.EventId, Revision: *e.Revision}
			first, err := f.svc.AcceptEvent(f.ownerCtx, decision)
			if err != nil || *first.CalendarState != "publishing" {
				t.Fatalf("initial publication: %+v %v", first, err)
			}
			switch state {
			case "live":
				remote.event.Title = "Changed in catalystcal"
				remote.receipt.Event = &remote.event
			case "moved":
				remote.event.CalendarId = "other-house"
				remote.receipt.Event = &remote.event
			case "uncommitted":
				remote.failBefore = false
				remote.createErr = nil
			case "denied":
				remote.receiptErr = &cal.ClientError{Code: 403, Message: "grant removed"}
			default:
				remote.receipt.State = cal.CreateReceiptState(state)
				remote.receipt.Event = nil
			}
			result, err := f.svc.AcceptEvent(f.ownerCtx, decision)
			if state == "denied" {
				calendarCode(t, err, 409)
				stored, err := store.AppStore.GetEventByID(f.ownerCtx, string(e.EventId))
				if err != nil || stored.CalendarState != "publishing" {
					t.Fatal("authorization failure unlocked publication", err)
				}
				remote.receiptErr = nil
				result, err = f.svc.AcceptEvent(f.ownerCtx, decision)
				if err != nil {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if *result.CalendarState != "remote" || result.EventId != e.EventId || remote.creates != 1 {
				t.Fatalf("incorrect receipt recovery: %+v creates=%d", result, remote.creates)
			}
			wantCalls := 1
			if state == "uncommitted" {
				wantCalls = 2
			}
			if remote.createCalls != wantCalls {
				t.Fatal("receipt recovery issued an unexpected create")
			}
			if state == "live" && result.Title != "Changed in catalystcal" {
				t.Fatal("recovery replaced edited remote content")
			}
			if state == "trashed" || state == "deleted" || state == "hidden" || state == "moved" {
				if result.PublicationError == nil || result.Title != "Published event" {
					t.Fatal("missing safe receipt notice")
				}
				local, err := f.svc.ListLocalEvents(f.ownerCtx, csil.HouseID(f.house))
				if err != nil || len(local) != 1 {
					t.Fatal("receipt notice missing from local list", err)
				}
			}
		})
	}
}

func TestCalendar_Postgres_ReadBeforeReceiptRecoveryKeepsBothLinks(t *testing.T) {
	f := newCalendarFixture(t)
	remote := &calendarTestRemote{createErr: errors.New("lost reply")}
	f.enable(t, remote)
	e := f.proposal(t)
	decision := csil.EventDecision{EventId: e.EventId, Revision: *e.Revision}
	if _, err := f.svc.AcceptEvent(f.ownerCtx, decision); err != nil {
		t.Fatal(err)
	}
	remote.occurrence = &cal.Occurrence{Event: remote.event, Start: *remote.event.Start, End: *remote.event.End, Access: "owner"}
	start, end := csil.Timestamp("2026-11-01T00:00:00Z"), csil.Timestamp("2026-12-01T00:00:00Z")
	rows, err := f.svc.ListEvents(f.ownerCtx, csil.EventListRequest{HouseId: csil.HouseID(f.house), RangeStart: &start, RangeEnd: &end})
	if err != nil || len(rows) != 1 {
		t.Fatalf("discovery: %+v %v", rows, err)
	}
	discovered := rows[0].EventId
	if discovered == e.EventId {
		t.Fatal("test did not create an independent reference")
	}
	if _, err = f.svc.AcceptEvent(f.ownerCtx, decision); err != nil {
		t.Fatal(err)
	}
	for _, id := range []csil.EventID{e.EventId, discovered} {
		got, err := f.svc.GetEvent(f.ownerCtx, id)
		if err != nil || got.Title != remote.event.Title || got.EventId != id {
			t.Fatalf("lost alias %s: %+v %v", id, got, err)
		}
	}
	rows, err = f.svc.ListEvents(f.ownerCtx, csil.EventListRequest{HouseId: csil.HouseID(f.house), RangeStart: &start, RangeEnd: &end})
	if err != nil || len(rows) != 1 || rows[0].EventId != e.EventId {
		t.Fatalf("original reference not canonical: %+v %v", rows, err)
	}
	if remote.creates != 1 {
		t.Fatal("duplicate remote create")
	}
}

func TestCalendar_Postgres_MovedOccurrenceUsesOriginalIdentity(t *testing.T) {
	f := newCalendarFixture(t)
	remote := &calendarTestRemote{}
	f.enable(t, remote)
	e := f.proposal(t)
	if _, err := f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision}); err != nil {
		t.Fatal(err)
	}
	rid := cal.Timestamp("2026-11-17T15:00:00Z")
	moved := remote.event
	master := moved.EventId
	moved.EventId = "override-1"
	moved.MasterEventId = &master
	start, end := cal.Timestamp("2027-03-23T15:00:00Z"), cal.Timestamp("2027-03-23T16:00:00Z")
	moved.Start = &start
	moved.End = &end
	remote.occurrence = &cal.Occurrence{Event: moved, Start: start, End: end, RecurrenceId: &rid, Access: "owner"}
	a, b := csil.Timestamp("2027-03-01T00:00:00Z"), csil.Timestamp("2027-04-01T00:00:00Z")
	rows, err := f.svc.ListEvents(f.ownerCtx, csil.EventListRequest{HouseId: csil.HouseID(f.house), RangeStart: &a, RangeEnd: &b})
	if err != nil || len(rows) != 1 {
		t.Fatalf("occurrence discovery: %+v %v", rows, err)
	}
	id := rows[0].EventId
	got, err := f.svc.GetEvent(f.ownerCtx, id)
	if err != nil || string(*got.StartsAt) != string(start) || got.EventId != id || got.RecurrenceRootEventId == nil || *got.RecurrenceRootEventId != e.EventId {
		t.Fatalf("moved occurrence: %+v %v", got, err)
	}
	// All-day dates are midnight UTC on the wire, but local midnight in the UI.
	allDay, zone := true, "America/Denver"
	d1, d2 := cal.DateText("2027-03-23"), cal.DateText("2027-03-24")
	remote.occurrence.Event.AllDay = &allDay
	remote.occurrence.Event.Timezone = &zone
	remote.occurrence.Event.Start = nil
	remote.occurrence.Event.End = nil
	remote.occurrence.Event.StartDate = &d1
	remote.occurrence.Event.EndDate = &d2
	remote.occurrence.Start = "2027-03-23T00:00:00Z"
	remote.occurrence.End = "2027-03-24T00:00:00Z"
	got, err = f.svc.GetEvent(f.ownerCtx, id)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := time.Parse(time.RFC3339, string(*got.StartsAt))
	loc, _ := time.LoadLocation(zone)
	if err != nil || parsed.In(loc).Format("2006-01-02 15:04") != "2027-03-23 00:00" {
		t.Fatalf("all-day date shifted: %v %v", got.StartsAt, err)
	}
	remote.occurrence.Event.CalendarId = "foreign"
	_, err = f.svc.GetEvent(f.ownerCtx, id)
	calendarCode(t, err, 404)
	remote.occurrence = nil
	_, err = f.svc.GetEvent(f.ownerCtx, id)
	calendarCode(t, err, 404)
}

func TestCalendar_Postgres_ReceiptRecoversFailedLocalCommit(t *testing.T) {
	f := newCalendarFixture(t)
	remote := &calendarTestRemote{}
	f.enable(t, remote)
	e := f.proposal(t)
	decision := csil.EventDecision{EventId: e.EventId, Revision: *e.Revision}
	if err := f.db.Exec("CREATE FUNCTION fail_calendar_ref() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test local failure'; END $$; CREATE TRIGGER fail_calendar_ref BEFORE INSERT ON calendar_refs FOR EACH ROW EXECUTE FUNCTION fail_calendar_ref()").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptEvent(f.ownerCtx, decision); err == nil {
		t.Fatal("test did not fail local commit")
	}
	if err := f.db.Exec("DROP TRIGGER fail_calendar_ref ON calendar_refs; DROP FUNCTION fail_calendar_ref()").Error; err != nil {
		t.Fatal(err)
	}
	result, err := f.svc.AcceptEvent(f.ownerCtx, decision)
	if err != nil || result.EventId != e.EventId || *result.CalendarState != "remote" || remote.creates != 1 || remote.createCalls != 1 {
		t.Fatalf("local commit recovery: %+v %v creates=%d calls=%d", result, err, remote.creates, remote.createCalls)
	}
}

func TestCalendar_Postgres_PreReceiptPublicationRequiresReconciliation(t *testing.T) {
	f := newCalendarFixture(t)
	remote := &calendarTestRemote{createErr: errors.New("lost reply")}
	f.enable(t, remote)
	e := f.proposal(t)
	decision := csil.EventDecision{EventId: e.EventId, Revision: *e.Revision}
	if _, err := f.svc.AcceptEvent(f.ownerCtx, decision); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("DELETE FROM calendar_publications WHERE event_id=?", e.EventId).Error; err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.AcceptEvent(f.ownerCtx, decision)
	calendarCode(t, err, 409)
	if remote.createCalls != 1 {
		t.Fatal("retried a request without a durable journal")
	}
}

func TestCalendar_Postgres_MigrationAccountsForAssignmentsAndTrash(t *testing.T) {
	f := newCalendarFixture(t)
	pending := f.proposal(t)
	declined := f.proposal(t)
	if _, err := f.svc.DeclineEvent(f.ownerCtx, csil.EventDecision{EventId: declined.EventId, Revision: *declined.Revision}); err != nil {
		t.Fatal(err)
	}
	deleted := f.proposal(t)
	if _, err := f.events.DeleteEvent(f.adminCtx, deleted.EventId); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.PreviewMigration(f.adminCtx, csil.HouseID(f.house))
	if err != nil {
		t.Fatal(err)
	}
	want := map[csil.EventID]string{pending.EventId: "pending_acceptance", declined.EventId: "declined", deleted.EventId: "retained_trash"}
	if len(rows) != len(want) {
		t.Fatalf("report omitted source rows: %+v", rows)
	}
	for _, row := range rows {
		if row.Disposition != want[row.EventId] {
			t.Fatalf("wrong migration disposition: %+v", row)
		}
	}
}

func TestCalendar_Postgres_TransientAuthorizationDoesNotBreakConnection(t *testing.T) {
	f := newCalendarFixture(t)
	remote := &calendarTestRemote{}
	f.enable(t, remote)
	// Publication recovery remains independent of a temporary audience refusal.
	e := f.proposal(t)
	remote.memberErr = &cal.ClientError{Code: 401, Message: "temporary verification failure"}
	result, err := f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision})
	calendarCode(t, err, 409)
	var broken bool
	if err = f.db.Raw("SELECT broken FROM calendar_connections WHERE member_id=?", f.owner).Scan(&broken).Error; err != nil || broken {
		t.Fatal("temporary refusal broke connection", err)
	}
	remote.memberErr = nil
	result, err = f.svc.AcceptEvent(f.ownerCtx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision})
	if err != nil || *result.CalendarState != "remote" {
		t.Fatal("temporary refusal did not recover", err)
	}
}
