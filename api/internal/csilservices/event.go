package csilservices

import (
	"context"
	"errors"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

// EventService implements calendaring on top of the new dispatcher. Owner +
// admin can mutate; any house member can read/create. Single-id methods
// look up the resource's house before checking the caller.
type EventService struct{ Store store.Store }

func (s *EventService) Register(d *csilrpc.Dispatcher) {
	d.Register("event", "ListEvents", s.listEvents)
	d.Register("event", "GetEvent", s.getEvent)
	d.Register("event", "CreateEvent", s.createEvent)
	d.Register("event", "UpdateEvent", s.updateEvent)
	d.Register("event", "DeleteEvent", s.deleteEvent)
	d.Register("event", "DeleteEventAndFuture", s.deleteEventAndFuture)
}

func (s *EventService) listEvents(ctx context.Context, body []byte) (any, error) {
	var req csil.HouseScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	rows, err := s.Store.ListEventsByHouse(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return eventsToCSIL(rows), nil
}

func (s *EventService) getEvent(ctx context.Context, body []byte) (any, error) {
	var id csil.EventID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	e, err := s.Store.GetEventByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, csilrpc.NotFound("event not found")
		}
		return nil, csilrpc.Internal("internal error")
	}
	if _, _, err := requireMemberForHouse(ctx, e.HouseID); err != nil {
		return nil, err
	}
	return eventToCSIL(e), nil
}

func (s *EventService) createEvent(ctx context.Context, body []byte) (any, error) {
	var in csil.Event
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.HouseId == "" || in.Title == "" {
		return nil, csilrpc.BadRequest("house_id and title are required")
	}
	_, callerMemberID, err := requireMemberForHouse(ctx, string(in.HouseId))
	if err != nil {
		return nil, err
	}
	owner := callerMemberID
	if in.OwnerMemberId != "" {
		owner = string(in.OwnerMemberId)
	}
	e := &models.Event{
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
		if f, ok := (*in.RecurrenceFreq).(string); ok && f != "" {
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
	if e.RecurrenceFreq != nil && e.StartsAt != nil {
		t := *e.StartsAt
		e.NextRecurrenceAt = &t
	}
	if err := s.Store.CreateEvent(ctx, e); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return eventToCSIL(e), nil
}

func (s *EventService) updateEvent(ctx context.Context, body []byte) (any, error) {
	var in csil.Event
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.EventId == "" {
		return nil, csilrpc.BadRequest("event_id is required")
	}
	existing, err := s.Store.GetEventByID(ctx, string(in.EventId))
	if err != nil {
		return nil, csilrpc.NotFound("event not found")
	}
	id, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	if callerMemberID != existing.OwnerMemberID {
		if _, err := requireRole(id, existing.HouseID, "admin"); err != nil {
			return nil, csilrpc.Forbidden("only the event owner or a house admin may edit this event")
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
	existing.StartsAt = tsToTimePtr(in.StartsAt)
	existing.EndsAt = tsToTimePtr(in.EndsAt)
	if in.AllDay != nil {
		existing.AllDay = *in.AllDay
	}
	// Recurrence updates: passing freq=nil leaves the field alone, but a
	// caller can clear an existing recurrence by sending an empty-string
	// freq (which we drop to nil). Seed next_recurrence_at if recurrence
	// was just turned on; clear it if recurrence was just turned off.
	wasRecurring := existing.RecurrenceFreq != nil
	if in.RecurrenceFreq != nil {
		if f, ok := (*in.RecurrenceFreq).(string); ok {
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
	if err := s.Store.UpdateEvent(ctx, existing); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return eventToCSIL(existing), nil
}

func (s *EventService) deleteEvent(ctx context.Context, body []byte) (any, error) {
	var id csil.EventID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetEventByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("event not found")
	}
	ident, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	if callerMemberID != existing.OwnerMemberID {
		if _, err := requireRole(ident, existing.HouseID, "admin"); err != nil {
			return nil, csilrpc.Forbidden("only the event owner or a house admin may delete this event")
		}
	}
	if err := s.Store.DeleteEvent(ctx, existing.EventID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// deleteEventAndFuture handles "delete this and every later occurrence
// of the series":
//   - If the target is the recurring root, delete it; CASCADE wipes every
//     child via the FK.
//   - If the target is a child (recurrence_root_event_id set), delete that
//     child and every later child (starts_at >=), then strip recurrence
//     from the root so the spawner won't repopulate them.
func (s *EventService) deleteEventAndFuture(ctx context.Context, body []byte) (any, error) {
	var id csil.EventID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetEventByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("event not found")
	}
	ident, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	if callerMemberID != existing.OwnerMemberID {
		if _, err := requireRole(ident, existing.HouseID, "admin"); err != nil {
			return nil, csilrpc.Forbidden("only the event owner or a house admin may delete this series")
		}
	}
	if existing.RecurrenceRootEventID == nil {
		// Root row — CASCADE drops the children.
		if err := s.Store.DeleteEvent(ctx, existing.EventID); err != nil {
			return nil, csilrpc.Internal("internal error")
		}
		return csil.EmptyResponse{}, nil
	}
	// Child row — drop it + future siblings, then mute the root.
	if existing.StartsAt == nil {
		return nil, csilrpc.BadRequest("event has no starts_at; cannot delete this-and-future")
	}
	rootID := *existing.RecurrenceRootEventID
	if err := s.Store.DeleteEventsAfter(ctx, rootID, *existing.StartsAt); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	root, err := s.Store.GetEventByID(ctx, rootID)
	if err == nil && root != nil {
		root.RecurrenceFreq = nil
		root.NextRecurrenceAt = nil
		if err := s.Store.UpdateEvent(ctx, root); err != nil {
			return nil, csilrpc.Internal("internal error")
		}
	}
	return csil.EmptyResponse{}, nil
}
