package csilservices

import (
	"context"
	"errors"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

// EventService implements calendaring on top of the new dispatcher. Owner +
// admin can mutate; any house member can read/create. Single-id methods
// look up the resource's house before checking the caller.
type EventService struct {
	Store    store.Store
	Calendar *CalendarService
}

func (s *EventService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("event", "ListEvents", csilrpc.Route(s.ListEvents, csil.DecodeEventListRequest, csil.EncodeEventListEventsResponse))
	d.RegisterTyped("event", "GetEvent", csilrpc.Route(s.GetEvent, csil.DecodeEventGetEventRequest, csil.EncodeEvent))
	d.RegisterTyped("event", "CreateEvent", csilrpc.Route(s.CreateEvent, csil.DecodeEvent, csil.EncodeEvent))
	d.RegisterTyped("event", "UpdateEvent", csilrpc.Route(s.UpdateEvent, csil.DecodeEvent, csil.EncodeEvent))
	d.RegisterTyped("event", "DeleteEvent", csilrpc.Route(s.DeleteEvent, csil.DecodeEventDeleteEventRequest, csil.EncodeEmptyResponse))
	d.RegisterTyped("event", "DeleteEventAndFuture", csilrpc.Route(s.DeleteEventAndFuture, csil.DecodeEventDeleteEventAndFutureRequest, csil.EncodeEmptyResponse))
	d.RegisterTyped("event", "GetCalendarView", csilrpc.Route(s.GetCalendarView, csil.DecodeEventGetCalendarViewRequest, csil.EncodeCalendarView))
	d.RegisterTyped("event", "SetCalendarView", csilrpc.Route(s.SetCalendarView, csil.DecodeCalendarView, csil.EncodeCalendarView))
	if s.Calendar != nil {
		d.RegisterTyped("event", "RecoverEdit", csilrpc.Route(s.Calendar.RecoverEdit, csil.DecodeEventRecoverEditRequest, csil.EncodeEvent))
		d.RegisterTyped("event", "AcceptEvent", csilrpc.Route(s.Calendar.AcceptEvent, csil.DecodeEventDecision, csil.EncodeEvent))
		d.RegisterTyped("event", "DeclineEvent", csilrpc.Route(s.Calendar.DeclineEvent, csil.DecodeEventDecision, csil.EncodeEvent))
		d.RegisterTyped("event", "ListLocalEvents", csilrpc.Route(s.Calendar.ListLocalEvents, csil.DecodeEventListLocalEventsRequest, csil.EncodeEventListLocalEventsResponse))
	}
}

// GetCalendarView returns the caller's saved calendar view for a house (which
// other members' calendars they've added + enabled). Empty when never set.
func (s *EventService) GetCalendarView(ctx context.Context, houseID csil.HouseID) (csil.CalendarView, error) {
	if s.Calendar != nil {
		if _, err := s.Calendar.member(ctx, string(houseID), false); err != nil {
			return csil.CalendarView{}, err
		}
	}
	_, viewerMemberID, err := requireMemberForHouse(ctx, string(houseID))
	if err != nil {
		return csil.CalendarView{}, err
	}
	v, err := s.Store.GetCalendarView(ctx, viewerMemberID)
	if err != nil {
		return csil.CalendarView{}, csilrpc.Internal("internal error")
	}
	return calendarViewToCSIL(string(houseID), viewerMemberID, v), nil
}

// SetCalendarView replaces the caller's subscription list for a house. The
// viewer is always the caller (bearer-derived); any client-sent
// viewer_member_id is ignored. Subjects are recorded as-is — this is a display
// preference, not an access grant, so no per-subject read check is needed
// (events are already house-readable).
func (s *EventService) SetCalendarView(ctx context.Context, in csil.CalendarView) (csil.CalendarView, error) {
	if s.Calendar != nil {
		if _, err := s.Calendar.member(ctx, string(in.HouseId), false); err != nil {
			return csil.CalendarView{}, err
		}
	}
	if in.HouseId == "" {
		return csil.CalendarView{}, csilrpc.BadRequest("house_id is required")
	}
	_, viewerMemberID, err := requireMemberForHouse(ctx, string(in.HouseId))
	if err != nil {
		return csil.CalendarView{}, err
	}
	// De-dupe by subject (last one wins) and drop self / empty ids — a viewer
	// never subscribes to their own calendar (it's always shown) and empty ids
	// are noise.
	seen := make(map[string]int, len(in.Subscriptions))
	subs := make([]models.CalendarSubscription, 0, len(in.Subscriptions))
	for _, sub := range in.Subscriptions {
		sid := string(sub.SubjectMemberId)
		if sid == "" || sid == viewerMemberID {
			continue
		}
		if idx, ok := seen[sid]; ok {
			subs[idx].Enabled = sub.Enabled
			continue
		}
		seen[sid] = len(subs)
		subs = append(subs, models.CalendarSubscription{SubjectMemberID: sid, Enabled: sub.Enabled})
	}
	view := &models.MemberCalendarView{
		ViewerMemberID: viewerMemberID,
		HouseID:        string(in.HouseId),
		Subscriptions:  subs,
	}
	if err := s.Store.UpsertCalendarView(ctx, view); err != nil {
		return csil.CalendarView{}, csilrpc.Internal("internal error")
	}
	return calendarViewToCSIL(string(in.HouseId), viewerMemberID, view), nil
}

func (s *EventService) ListEvents(ctx context.Context, req csil.EventListRequest) ([]csil.Event, error) {
	if s.Calendar != nil {
		return s.Calendar.ListEvents(ctx, req)
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	f := store.EventFilter{HouseID: string(req.HouseId), RangeStart: tsToTimePtr(req.RangeStart), RangeEnd: tsToTimePtr(req.RangeEnd)}
	if req.RangeStart != nil && f.RangeStart == nil {
		return nil, csilrpc.BadRequest("range_start must be an RFC3339 timestamp")
	}
	if req.RangeEnd != nil && f.RangeEnd == nil {
		return nil, csilrpc.BadRequest("range_end must be an RFC3339 timestamp")
	}
	if f.RangeStart != nil && f.RangeEnd != nil && !f.RangeEnd.After(*f.RangeStart) {
		return nil, csilrpc.BadRequest("range_end must be after range_start")
	}
	rows, err := s.Store.ListEvents(ctx, f, limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return eventsToCSIL(rows), nil
}

func (s *EventService) GetEvent(ctx context.Context, id csil.EventID) (csil.Event, error) {
	if s.Calendar != nil {
		return s.Calendar.GetEvent(ctx, id)
	}
	e, err := s.Store.GetEventByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return csil.Event{}, csilrpc.NotFound("event not found")
		}
		return csil.Event{}, csilrpc.Internal("internal error")
	}
	if e.DeletedAt != nil {
		return csil.Event{}, csilrpc.NotFound("event not found")
	}
	if _, _, err := requireMemberForHouse(ctx, e.HouseID); err != nil {
		return csil.Event{}, err
	}
	return eventToCSIL(e), nil
}

func (s *EventService) CreateEvent(ctx context.Context, in csil.Event) (csil.Event, error) {
	if s.Calendar != nil {
		return s.Calendar.CreateEvent(ctx, in)
	}
	return s.createLocalEvent(ctx, in, "")
}

func (s *EventService) createLocalEvent(ctx context.Context, in csil.Event, state string) (csil.Event, error) {
	if in.HouseId == "" || in.Title == "" {
		return csil.Event{}, csilrpc.BadRequest("house_id and title are required")
	}
	ident, callerMemberID, err := requireMemberForHouse(ctx, string(in.HouseId))
	if err != nil {
		return csil.Event{}, err
	}
	owner := callerMemberID
	if state == "" {
		state = "local"
	}
	if in.OwnerMemberId != "" && string(in.OwnerMemberId) != callerMemberID {
		if _, err := requireRole(ident, string(in.HouseId), "admin"); err != nil {
			return csil.Event{}, csilrpc.Forbidden("only a house admin may create an event for another member")
		}
		member, err := s.Store.GetMemberByID(ctx, string(in.OwnerMemberId))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return csil.Event{}, csilrpc.BadRequest("owner must be an active member of this house")
		}
		if err != nil {
			return csil.Event{}, csilrpc.Internal("internal error")
		}
		if member.HouseID != string(in.HouseId) || member.DeactivatedAt != nil {
			return csil.Event{}, csilrpc.BadRequest("owner must be an active member of this house")
		}
		owner = string(in.OwnerMemberId)
		state = "pending"
	}
	e := &models.Event{
		Timezone:           derefStr(in.Timezone),
		RecurrenceRule:     derefStr(in.RecurrenceRule),
		CalendarState:      state,
		Revision:           1,
		ProposedByMemberID: &callerMemberID,
		HouseID:            string(in.HouseId),
		OwnerMemberID:      owner,
		Title:              in.Title,
		Description:        derefStr(in.Description),
		Location:           derefStr(in.Location),
		StartsAt:           tsToTimePtr(in.StartsAt),
		EndsAt:             tsToTimePtr(in.EndsAt),
		AllDay:             in.AllDay != nil && *in.AllDay,
		RecurrenceInterval: 1,
	}
	if in.RecurrenceFreq != nil {
		if f := string(*in.RecurrenceFreq); f != "" {
			e.RecurrenceFreq = &f
		}
	}
	if in.RecurrenceInterval != nil && *in.RecurrenceInterval > 0 {
		e.RecurrenceInterval = int(*in.RecurrenceInterval)
	}
	if len(in.RecurrenceByWeekday) > 0 {
		w := make(models.IntList, len(in.RecurrenceByWeekday))
		for i, d := range in.RecurrenceByWeekday {
			w[i] = int(d)
		}
		e.RecurrenceByWeekday = w
	}
	if in.RecurrenceBySetpos != nil && *in.RecurrenceBySetpos != 0 {
		v := int(*in.RecurrenceBySetpos)
		e.RecurrenceBySetpos = &v
	}
	// Seed next_recurrence_at so the worker spawns occurrences from the
	// root's starts_at. The root row itself stays at its own time; the
	// worker creates children at the next interval and bumps this forward.
	if e.CalendarState == "local" && e.RecurrenceFreq != nil && e.StartsAt != nil {
		t := *e.StartsAt
		e.NextRecurrenceAt = &t
	}
	if err := s.Store.CreateEvent(ctx, e); err != nil {
		return csil.Event{}, csilrpc.Internal("internal error")
	}
	if e.CalendarState == "pending" {
		annotateAudit(ctx, e.HouseID, models.AuditActionCreate, "event", e.EventID, map[string]any{"assignment": "proposed", "owner_member_id": e.OwnerMemberID, "revision": e.Revision})
	}
	return eventToCSIL(e), nil
}

func (s *EventService) UpdateEvent(ctx context.Context, in csil.Event) (csil.Event, error) {
	if s.Calendar != nil {
		return s.Calendar.UpdateEvent(ctx, in)
	}
	if in.EventId == "" {
		return csil.Event{}, csilrpc.BadRequest("event_id is required")
	}
	existing, err := s.Store.GetEventByID(ctx, string(in.EventId))
	if err != nil {
		return csil.Event{}, csilrpc.NotFound("event not found")
	}
	if in.Revision != nil && *in.Revision != existing.Revision {
		return csil.Event{}, csilrpc.Conflict("event changed; reload before saving")
	}
	id, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return csil.Event{}, err
	}
	if callerMemberID != existing.OwnerMemberID {
		if _, err := requireRole(id, existing.HouseID, "admin"); err != nil {
			return csil.Event{}, csilrpc.Forbidden("only the event owner or a house admin may edit this event")
		}
	}
	if in.Title != "" {
		existing.Title = in.Title
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	if in.Location != nil {
		existing.Location = *in.Location
	}
	if in.Timezone != nil {
		existing.Timezone = *in.Timezone
	}
	if in.RecurrenceRule != nil {
		existing.RecurrenceRule = *in.RecurrenceRule
	}
	if existing.CalendarState == "declined" {
		existing.CalendarState = "pending"
	}
	existing.StartsAt = tsToTimePtr(in.StartsAt)
	existing.EndsAt = tsToTimePtr(in.EndsAt)
	if in.AllDay != nil {
		existing.AllDay = *in.AllDay
	}
	// Recurrence updates: passing freq=nil leaves the field alone, but a
	// caller uses clear_recurrence to remove the schedule. Seed
	// next_recurrence_at when recurrence starts; clear it when recurrence stops.
	wasRecurring := existing.RecurrenceFreq != nil
	if in.RecurrenceFreq != nil {
		f := string(*in.RecurrenceFreq)
		if f == "" {
			existing.RecurrenceFreq = nil
			existing.NextRecurrenceAt = nil
			existing.RecurrenceByWeekday = nil
			existing.RecurrenceBySetpos = nil
		} else {
			existing.RecurrenceFreq = &f
			if !wasRecurring && existing.StartsAt != nil {
				t := *existing.StartsAt
				existing.NextRecurrenceAt = &t
			}
		}
	}
	if in.RecurrenceInterval != nil && *in.RecurrenceInterval > 0 {
		existing.RecurrenceInterval = int(*in.RecurrenceInterval)
	}
	// nil means "leave alone"; empty slice means "clear". Same convention
	// as task assignees so the UI can repaint without losing state.
	if in.RecurrenceByWeekday != nil {
		if len(in.RecurrenceByWeekday) == 0 {
			existing.RecurrenceByWeekday = nil
		} else {
			w := make(models.IntList, len(in.RecurrenceByWeekday))
			for i, d := range in.RecurrenceByWeekday {
				w[i] = int(d)
			}
			existing.RecurrenceByWeekday = w
		}
	}
	if in.RecurrenceBySetpos != nil {
		if *in.RecurrenceBySetpos == 0 {
			existing.RecurrenceBySetpos = nil
		} else {
			v := int(*in.RecurrenceBySetpos)
			existing.RecurrenceBySetpos = &v
		}
	}
	if in.ClearRecurrence != nil && *in.ClearRecurrence {
		existing.RecurrenceFreq = nil
		existing.NextRecurrenceAt = nil
		existing.RecurrenceByWeekday = nil
		existing.RecurrenceBySetpos = nil
	}
	if existing.CalendarState == "pending" {
		existing.NextRecurrenceAt = nil
	}
	if err := s.Store.UpdateEvent(ctx, existing); err != nil {
		return csil.Event{}, csilrpc.Internal("internal error")
	}
	return eventToCSIL(existing), nil
}

func (s *EventService) DeleteEvent(ctx context.Context, id csil.EventID) (csil.EmptyResponse, error) {
	if s.Calendar != nil {
		return s.Calendar.DeleteEvent(ctx, id, false)
	}
	existing, err := s.Store.GetEventByID(ctx, string(id))
	if err != nil || existing.DeletedAt != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("event not found")
	}
	ident, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if callerMemberID != existing.OwnerMemberID {
		if _, err := requireRole(ident, existing.HouseID, "admin"); err != nil {
			return csil.EmptyResponse{}, csilrpc.Forbidden("only the event owner or a house admin may delete this event")
		}
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteEvent(ctx, existing.EventID, callerMemberID, opID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, existing.HouseID, "event", existing.EventID, opID, existing)
	return csil.EmptyResponse{}, nil
}

// deleteEventAndFuture handles "delete this and every later occurrence
// of the series":
//   - If the target is the recurring root, delete it; CASCADE wipes every
//     child via the FK.
//   - If the target is a child (recurrence_root_event_id set), delete that
//     child and every later child (starts_at >=), then strip recurrence
//     from the root so the spawner won't repopulate them.
func (s *EventService) DeleteEventAndFuture(ctx context.Context, id csil.EventID) (csil.EmptyResponse, error) {
	if s.Calendar != nil {
		return s.Calendar.DeleteEvent(ctx, id, true)
	}
	existing, err := s.Store.GetEventByID(ctx, string(id))
	if err != nil || existing.DeletedAt != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("event not found")
	}
	ident, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if callerMemberID != existing.OwnerMemberID {
		if _, err := requireRole(ident, existing.HouseID, "admin"); err != nil {
			return csil.EmptyResponse{}, csilrpc.Forbidden("only the event owner or a house admin may delete this series")
		}
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if existing.RecurrenceRootEventID == nil {
		// Root row — soft-delete the whole series (root + every child) under
		// one opID. The recurrence worker skips soft-deleted roots, so no need
		// to mute recurrence_freq; restore is a clean un-delete that resumes
		// spawning.
		if err := s.Store.SoftDeleteEventSeries(ctx, existing.EventID, callerMemberID, opID); err != nil {
			return csil.EmptyResponse{}, csilrpc.Internal("internal error")
		}
		annotateDeleteWithDetail(ctx, existing.HouseID, "event", existing.EventID, opID, existing,
			map[string]any{"mode": "series"})
		return csil.EmptyResponse{}, nil
	}
	// Child row — soft-delete it + future siblings, then mute the root so the
	// spawner stops repopulating future occurrences. (Re-arming the root's
	// recurrence on restore is layered in once the audit before-snapshot
	// exists; the soft-deleted children themselves restore via their opID.)
	if existing.StartsAt == nil {
		return csil.EmptyResponse{}, csilrpc.BadRequest("event has no starts_at; cannot delete this-and-future")
	}
	rootID := *existing.RecurrenceRootEventID
	if err := s.Store.SoftDeleteEventsAfter(ctx, rootID, *existing.StartsAt, callerMemberID, opID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	var priorFreq *string
	var priorNext *time.Time
	root, err := s.Store.GetEventByID(ctx, rootID)
	if err == nil && root != nil {
		priorFreq = root.RecurrenceFreq
		priorNext = root.NextRecurrenceAt
		root.RecurrenceFreq = nil
		root.NextRecurrenceAt = nil
		if err := s.Store.UpdateEvent(ctx, root); err != nil {
			return csil.EmptyResponse{}, csilrpc.Internal("internal error")
		}
	}
	// Record enough to re-arm the root's recurrence on restore (the soft-delete
	// of the children alone doesn't capture the root mutation).
	annotateDeleteWithDetail(ctx, existing.HouseID, "event", existing.EventID, opID, existing,
		map[string]any{
			"mode":                          "this_and_future",
			"root_event_id":                 rootID,
			"root_prior_recurrence_freq":    priorFreq,
			"root_prior_next_recurrence_at": priorNext,
		})
	return csil.EmptyResponse{}, nil
}
