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
		HouseID:       string(in.HouseId),
		OwnerMemberID: owner,
		Title:         in.Title,
		Description:   derefStr(in.Description),
		Location:      derefStr(in.Location),
		StartsAt:      tsToTimePtr(in.StartsAt),
		EndsAt:        tsToTimePtr(in.EndsAt),
		AllDay:        in.AllDay != nil && *in.AllDay,
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
