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

type deadlineRemote struct {
	calendarTestRemote
	calendars                            []cal.Calendar
	events                               map[cal.EventID]cal.Event
	receipts                             map[string]cal.CreateEventReceipt
	requests                             map[string][]byte
	count                                int
	loseCreate, failBefore, loseCalendar bool
	onCreate                             func()
}

func (r *deadlineRemote) ApplicationTransport() cal.Transport { return r }
func (r *deadlineRemote) Member(context.Context, string, string, string) (cal.Transport, error) {
	return nil, errors.New("deadline sync must use the app identity")
}
func (r *deadlineRemote) Call(_ context.Context, service, op string, b []byte) ([]byte, error) {
	missing := &cal.ClientError{Code: 404, Message: "not found"}
	switch op {
	case "list-calendars":
		return cal.EncodeCalendarList(cal.CalendarList{Calendars: r.calendars}), nil
	case "create-calendar":
		req, err := cal.DecodeCreateCalendarRequest(b)
		if err != nil {
			return nil, err
		}
		app := r.ApplicationIdentity().ApplicationId
		c := cal.Calendar{CalendarId: cal.CalendarID(fmt.Sprintf("category-%d", len(r.calendars)+1)), Name: req.Name, Description: req.Description, Kind: "local", Access: "owner", AccessPath: "owner", Owner: cal.PersonSummary{Domain: "soft.test", Subject: "app", ApplicationId: &app}}
		r.calendars = append(r.calendars, c)
		if r.loseCalendar {
			r.loseCalendar = false
			return nil, errors.New("lost calendar response")
		}
		return cal.EncodeCalendar(c), nil
	case "delete-calendar":
		id, err := cal.DecodeCalendarDeleteCalendarRequest(b)
		if err != nil {
			return nil, err
		}
		kept := []cal.Calendar{}
		for _, c := range r.calendars {
			if c.CalendarId != id {
				kept = append(kept, c)
			}
		}
		r.calendars = kept
		for eid, event := range r.events {
			if event.CalendarId == id {
				delete(r.events, eid)
			}
		}
		return cal.EncodeEmptyResponse(cal.EmptyResponse{}), nil
	case "get-create-receipt":
		req, err := cal.DecodeCreateReceiptRequest(b)
		if err != nil {
			return nil, err
		}
		receipt, ok := r.receipts[req.IdempotencyKey]
		if !ok {
			return nil, missing
		}
		if event, ok := r.events[receipt.EventId]; ok {
			receipt.Event = &event
		} else {
			receipt.State = "deleted"
			receipt.Event = nil
		}
		return cal.EncodeCreateEventReceipt(receipt), nil
	case "create-event-once":
		req, err := cal.DecodeCreateEventOnceRequest(b)
		if err != nil {
			return nil, err
		}
		body := cal.EncodeEvent(req.Event)
		if previous, ok := r.requests[req.IdempotencyKey]; ok && !bytes.Equal(previous, body) {
			return nil, errors.New("retry changed saved request")
		}
		r.requests[req.IdempotencyKey] = body
		if r.failBefore {
			r.failBefore = false
			return nil, errors.New("request did not reach server")
		}
		if receipt, ok := r.receipts[req.IdempotencyKey]; ok {
			return cal.EncodeCreateEventReceipt(receipt), nil
		}
		if len(req.Event.Reminders) != 0 || len(req.Event.Attendees) != 0 || req.Event.Recurrence != nil || req.Event.Transparency == nil || *req.Event.Transparency != "transparent" {
			return nil, errors.New("deadline must be silent and transparent")
		}
		r.count++
		event := req.Event
		event.EventId = cal.EventID(fmt.Sprintf("deadline-%d", r.count))
		r.events[event.EventId] = event
		receipt := cal.CreateEventReceipt{CalendarId: event.CalendarId, EventId: event.EventId, IdempotencyKey: req.IdempotencyKey, State: "live", Created: true, Event: &event}
		r.receipts[req.IdempotencyKey] = receipt
		if r.onCreate != nil {
			fn := r.onCreate
			r.onCreate = nil
			fn()
		}
		if r.loseCreate {
			r.loseCreate = false
			return nil, errors.New("lost create response")
		}
		return cal.EncodeCreateEventReceipt(receipt), nil
	case "get-event":
		req, err := cal.DecodeEventGetEventRequest(b)
		if err != nil {
			return nil, err
		}
		event, ok := r.events[req]
		if !ok {
			return nil, missing
		}
		return cal.EncodeEvent(event), nil
	case "update-event":
		req, err := cal.DecodeUpdateEventRequest(b)
		if err != nil {
			return nil, err
		}
		if _, ok := r.events[req.Event.EventId]; !ok {
			return nil, missing
		}
		r.events[req.Event.EventId] = req.Event
		return cal.EncodeEvent(req.Event), nil
	case "delete-event":
		req, err := cal.DecodeDeleteEventRequest(b)
		if err != nil {
			return nil, err
		}
		delete(r.events, req.EventId)
		return cal.EncodeDeleteEventResponse(cal.DeleteEventResponse{}), nil
	case "list-events":
		req, err := cal.DecodeListEventsRequest(b)
		if err != nil {
			return nil, err
		}
		out := cal.OccurrenceList{Occurrences: []cal.Occurrence{}}
		for _, event := range r.events {
			if len(req.CalendarIds) == 1 && event.CalendarId == req.CalendarIds[0] {
				out.Occurrences = append(out.Occurrences, cal.Occurrence{Event: event, Start: *event.Start, End: *event.End, Access: "owner"})
			}
		}
		return cal.EncodeOccurrenceList(out), nil
	default:
		return nil, fmt.Errorf("unexpected call %s/%s", service, op)
	}
}
func deadlineFixture(t *testing.T) (*calendarFixture, *deadlineRemote) {
	t.Helper()
	f := newCalendarFixture(t)
	r := &deadlineRemote{events: map[cal.EventID]cal.Event{}, receipts: map[string]cal.CreateEventReceipt{}, requests: map[string][]byte{}}
	f.svc.Remote = r
	if _, err := f.svc.ConfigureCalendar(f.adminCtx, csil.ConfigureCalendarRequest{HouseId: csil.HouseID(f.house), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("UPDATE house_calendars SET calendar_id='house-calendar',ready=true WHERE house_id=?", f.house).Error; err != nil {
		t.Fatal(err)
	}
	return f, r
}
func deadlineProject(t *testing.T, f *calendarFixture, category string) *models.Project {
	t.Helper()
	p := &models.Project{HouseID: f.house, Name: "Project", Category: &category, Visibility: "none", CreatedByMemberID: &f.owner}
	if err := store.AppStore.CreateProject(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return p
}
func deadlineLink(t *testing.T, f *calendarFixture, p *models.Project, title string) string {
	t.Helper()
	due := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	id := mkTask(t, context.Background(), f.house, f.owner, title, "open", &due)
	if err := store.AppStore.AddProjectTask(context.Background(), p.ProjectID, id, 0); err != nil {
		t.Fatal(err)
	}
	return id
}
func deadlineSyncNow(t *testing.T, f *calendarFixture) error {
	t.Helper()
	if err := f.db.Exec("UPDATE calendar_deadline_sync SET next_attempt_at=now() WHERE house_id=?", f.house).Error; err != nil {
		t.Fatal(err)
	}
	return f.svc.SyncTaskDeadlines(context.Background())
}
func deadlineRequest(p *models.Project) csil.ProjectDeadlineRequest {
	return csil.ProjectDeadlineRequest{ProjectId: csil.ProjectID(p.ProjectID), RangeStart: "2026-11-01T00:00:00Z", RangeEnd: "2026-12-01T00:00:00Z"}
}

func TestDeadlineCalendars_Postgres_LifecycleAndAccess(t *testing.T) {
	f, r := deadlineFixture(t)
	p := deadlineProject(t, f, "Garden")
	p2 := deadlineProject(t, f, "Garden")
	p3 := deadlineProject(t, f, "House")
	task := deadlineLink(t, f, p, "Plant trees")
	for _, project := range []*models.Project{p2, p3} {
		if err := store.AppStore.AddProjectTask(context.Background(), project.ProjectID, task, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	if len(r.calendars) != 2 || len(r.events) != 2 {
		t.Fatalf("want two category calendars and two entries: %d, %d", len(r.calendars), len(r.events))
	}
	view, err := f.svc.ListProjectDeadlines(f.ownerCtx, deadlineRequest(p))
	if err != nil || len(view.Deadlines) != 1 || view.Pending {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	// A demoted admin bearer cannot read the owner's private project.
	if err = f.db.Where("member_id=?", f.admin).Delete(&models.MemberRole{}).Error; err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.ListProjectDeadlines(f.adminCtx, deadlineRequest(p))
	calendarCode(t, err, 404)
	// Category moves remove the old event if no other project still uses it.
	if err = f.db.Model(p).Update("category", "House").Error; err != nil {
		t.Fatal(err)
	}
	if err = store.AppStore.RemoveProjectTask(context.Background(), p2.ProjectID, task); err != nil {
		t.Fatal(err)
	}
	if err = deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	if len(r.events) != 1 {
		t.Fatalf("old category retained: %+v", r.events)
	}
	// The view must not return the old date while the new date waits for sync.
	due := time.Date(2026, 11, 12, 8, 30, 0, 0, time.UTC)
	if err = f.db.Model(&models.Task{}).Where("task_id=?", task).Updates(map[string]any{"title": "New title", "due_at": due}).Error; err != nil {
		t.Fatal(err)
	}
	view, err = f.svc.ListProjectDeadlines(f.ownerCtx, deadlineRequest(p))
	if err != nil || !view.Pending || len(view.Deadlines) != 0 {
		t.Fatalf("stale date visible: %+v %v", view, err)
	}
	if err = deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	view, err = f.svc.ListProjectDeadlines(f.ownerCtx, deadlineRequest(p))
	if err != nil || len(view.Deadlines) != 1 || view.Deadlines[0].Title != "New title" {
		t.Fatalf("updated view: %+v %v", view, err)
	}
	if err = f.db.Model(&models.Task{}).Where("task_id=?", task).Update("due_at", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err = deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	if len(r.events) != 0 {
		t.Fatal("removed deadline retained")
	}
	if err = f.db.Model(&models.Task{}).Where("task_id=?", task).Update("due_at", due).Error; err != nil {
		t.Fatal(err)
	}
	if err = deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	if len(r.events) != 1 {
		t.Fatal("deadline not recreated")
	}
	if err = f.db.Model(&models.Task{}).Where("task_id=?", task).Update("deleted_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if err = deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	if len(r.events) != 0 {
		t.Fatal("deleted task retained")
	}
	if err = f.db.Model(&models.Task{}).Where("task_id=?", task).Update("deleted_at", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err = deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	if len(r.events) != 1 {
		t.Fatal("restored task missing")
	}
}

func TestDeadlineCalendars_Postgres_Recovery(t *testing.T) {
	for _, failBefore := range []bool{false, true} {
		t.Run(fmt.Sprint(failBefore), func(t *testing.T) {
			f, r := deadlineFixture(t)
			p := deadlineProject(t, f, "")
			task := deadlineLink(t, f, p, "First version")
			r.loseCalendar = true
			if err := deadlineSyncNow(t, f); err == nil {
				t.Fatal("want uncertain calendar error")
			}
			r.loseCreate = !failBefore
			r.failBefore = failBefore
			if err := deadlineSyncNow(t, f); err == nil {
				t.Fatal("want uncertain create error")
			}
			// A new service instance must recover the exact journal, then apply current data.
			f.svc = &csilservices.CalendarService{Store: store.AppStore, DB: f.db, Remote: r}
			if err := f.db.Model(&models.Task{}).Where("task_id=?", task).Update("title", "Second version").Error; err != nil {
				t.Fatal(err)
			}
			if err := deadlineSyncNow(t, f); err != nil {
				t.Fatal(err)
			}
			if r.count != 1 || len(r.events) != 1 || len(r.calendars) != 1 {
				t.Fatalf("retry duplicated data: events=%d creates=%d calendars=%d", len(r.events), r.count, len(r.calendars))
			}
			for _, e := range r.events {
				if e.Title != "Second version" {
					t.Fatalf("wrong final title: %s", e.Title)
				}
			}
			var n int64
			if err := f.db.Table("calendar_deadlines").Where("octet_length(create_request)>0").Count(&n).Error; err != nil || n != 0 {
				t.Fatalf("journal not cleared: %d %v", n, err)
			}
		})
	}
}

func TestDeadlineCalendars_Postgres_DescendantsAndConcurrentChange(t *testing.T) {
	f, r := deadlineFixture(t)
	p := deadlineProject(t, f, "Work")
	root := deadlineLink(t, f, p, "Root")
	due := time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC)
	child := mkTask(t, context.Background(), f.house, f.owner, "Child", "open", &due)
	occurrence := mkTask(t, context.Background(), f.house, f.owner, "Occurrence", "open", &due)
	if err := f.db.Model(&models.Task{}).Where("task_id=?", child).Update("parent_task_id", root).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&models.Task{}).Where("task_id=?", occurrence).Update("recurrence_root_task_id", root).Error; err != nil {
		t.Fatal(err)
	}
	r.onCreate = func() {
		if err := f.db.Model(&models.Task{}).Where("task_id=?", root).Update("title", "Changed during sync").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	if len(r.events) != 3 {
		t.Fatalf("missing inherited deadlines: %d", len(r.events))
	}
	view, err := f.svc.ListProjectDeadlines(f.ownerCtx, deadlineRequest(p))
	if err != nil || !view.Pending || len(view.Deadlines) != 3 {
		t.Fatalf("concurrent generation lost: %+v %v", view, err)
	}
	if err = deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	view, err = f.svc.ListProjectDeadlines(f.ownerCtx, deadlineRequest(p))
	if err != nil || view.Pending {
		t.Fatalf("repair failed: %+v %v", view, err)
	}
	// Deactivated membership must fail even with a previously valid bearer.
	if err = f.db.Model(&models.Member{}).Where("member_id=?", f.owner).Update("deactivated_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.ListProjectDeadlines(f.ownerCtx, deadlineRequest(p))
	calendarCode(t, err, 403)
}

func TestDeadlineCalendars_Postgres_EmptyCategoryIsolationAndRemovedProposal(t *testing.T) {
	f, r := deadlineFixture(t)
	blank := deadlineProject(t, f, "")
	named := deadlineProject(t, f, "Other")
	task := deadlineLink(t, f, blank, "Both categories")
	if err := store.AppStore.AddProjectTask(context.Background(), named.ProjectID, task, 0); err != nil {
		t.Fatal(err)
	}
	if err := deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	if err := store.AppStore.RemoveProjectTask(context.Background(), blank.ProjectID, task); err != nil {
		t.Fatal(err)
	}
	if err := deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := f.db.Table("calendar_deadlines").Where("house_id=?", f.house).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("blank category deletion changed named mapping: %d %v", count, err)
	}
	view, err := f.svc.ListProjectDeadlines(f.ownerCtx, deadlineRequest(named))
	if err != nil || len(view.Deadlines) != 1 {
		t.Fatalf("named deadline lost: %+v %v", view, err)
	}
	// A deadline removed after an uncertain create must be recovered and removed.
	r.loseCreate = true
	pending := deadlineLink(t, f, blank, "Removed during retry")
	if err = deadlineSyncNow(t, f); err == nil {
		t.Fatal("want uncertain create")
	}
	if err = f.db.Model(&models.Task{}).Where("task_id=?", pending).Update("due_at", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err = deadlineSyncNow(t, f); err != nil {
		t.Fatal(err)
	}
	if len(r.events) != 1 {
		t.Fatalf("removed proposal leaked: %d", len(r.events))
	}
}

func TestDeadlineCalendars_Postgres_DeletedReceiptAndHouseCleanup(t *testing.T) {
	f, r := deadlineFixture(t)
	p := deadlineProject(t, f, "Garden")
	deadlineLink(t, f, p, "Deadline")
	r.loseCreate = true
	if err := deadlineSyncNow(t, f); err == nil {
		t.Fatal("want lost response")
	}
	for id := range r.events {
		delete(r.events, id)
	}
	if err := deadlineSyncNow(t, f); err == nil {
		t.Fatal("deleted receipt must require review")
	}
	if r.count != 1 {
		t.Fatal("deleted event was recreated")
	}
	// Include a calendar whose remote create response was lost.
	deadlineProject(t, f, "Another category")
	r.loseCalendar = true
	if err := deadlineSyncNow(t, f); err == nil {
		t.Fatal("want lost calendar response")
	}
	if len(r.calendars) != 2 {
		t.Fatal("missing uncertain calendar")
	}
	if err := f.db.Exec("DELETE FROM houses WHERE house_id=?", f.house).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	f.svc.Run(ctx)
	if len(r.calendars) != 0 {
		t.Fatalf("house cleanup left category calendars: %d", len(r.calendars))
	}
	var count int64
	if err := f.db.Table("house_calendars").Where("house_id=?", f.house).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("cleanup failed: %d %v", count, err)
	}
}
