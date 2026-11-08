package csilservices

import (
	"context"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"gorm.io/gorm"
)

// Hold the lock across remote reads and local reference changes. Otherwise a
// list response from before a split could recreate an obsolete reference after
// the receipt has rebound it. A clone keeps nested calls on the same connection.
func (s *CalendarService) calendarLocked(ctx context.Context, house string, fn func(*CalendarService) error) error {
	if s.lockedHouse == house {
		return fn(s)
	}
	return s.DB.WithContext(ctx).Connection(func(db *gorm.DB) error {
		db = db.Session(&gorm.Session{NewDB: true})
		key := "calendar-house:" + house
		if err := db.Exec("SELECT pg_advisory_lock(hashtextextended(?,0))", key).Error; err != nil {
			return err
		}
		defer db.WithContext(context.Background()).Exec("SELECT pg_advisory_unlock(hashtextextended(?,0))", key)
		clone := *s
		clone.DB = db
		clone.lockedHouse = house
		return fn(&clone)
	})
}
func (s *CalendarService) ListEvents(ctx context.Context, in csil.EventListRequest) (out []csil.Event, err error) {
	if _, err = s.member(ctx, string(in.HouseId), false); err != nil {
		return out, err
	}
	err = s.calendarLocked(ctx, string(in.HouseId), func(c *CalendarService) error { out, err = c.listEvents(ctx, in); return err })
	return
}
func (s *CalendarService) GetEvent(ctx context.Context, id csil.EventID) (out csil.Event, err error) {
	e, _, err := s.event(ctx, id, false)
	if err != nil {
		return out, err
	}
	err = s.calendarLocked(ctx, e.HouseID, func(c *CalendarService) error { out, err = c.getEvent(ctx, id); return err })
	return
}
func (s *CalendarService) UpdateEvent(ctx context.Context, in csil.Event) (out csil.Event, err error) {
	e, _, err := s.event(ctx, in.EventId, true)
	if err != nil {
		return out, err
	}
	if e.EditPendingMemberID != nil {
		return out, csilrpc.Conflict(splitPendingMessage)
	}
	err = s.calendarLocked(ctx, e.HouseID, func(c *CalendarService) error { out, err = c.updateEvent(ctx, in); return err })
	return
}
func (s *CalendarService) DeleteEvent(ctx context.Context, id csil.EventID, future bool) (out csil.EmptyResponse, err error) {
	e, _, err := s.event(ctx, id, true)
	if err != nil {
		return out, err
	}
	if e.EditPendingMemberID != nil {
		return out, csilrpc.Conflict(splitPendingMessage)
	}
	err = s.calendarLocked(ctx, e.HouseID, func(c *CalendarService) error { out, err = c.deleteEvent(ctx, id, future); return err })
	return
}
func (s *CalendarService) RestoreEvent(ctx context.Context, in csil.RestoreCalendarEventRequest) (out csil.Event, err error) {
	if _, err = s.member(ctx, string(in.HouseId), false); err != nil {
		return out, err
	}
	err = s.calendarLocked(ctx, string(in.HouseId), func(c *CalendarService) error { out, err = c.restoreEvent(ctx, in); return err })
	return
}

func (s *CalendarService) ListTrash(ctx context.Context, id csil.HouseID) (out []csil.CalendarTrashEntry, err error) {
	if _, err = s.member(ctx, string(id), false); err != nil {
		return out, err
	}
	err = s.calendarLocked(ctx, string(id), func(c *CalendarService) error { out, err = c.listTrash(ctx, id); return err })
	return
}
