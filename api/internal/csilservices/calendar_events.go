package csilservices

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	bridge "github.com/catalystcommunity/longhouse/api/internal/calendar"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

func (s *CalendarService) event(ctx context.Context, id csil.EventID, edit bool) (*models.Event, *models.Member, error) {
	e, err := s.Store.GetEventByID(ctx, string(id))
	if err != nil {
		return nil, nil, csilrpc.NotFound("event not found")
	}
	m, err := s.member(ctx, e.HouseID, false)
	if err != nil {
		return nil, nil, err
	}
	if edit && m.MemberID != e.OwnerMemberID {
		if _, err = s.member(ctx, e.HouseID, true); err != nil {
			return nil, nil, csilrpc.Forbidden("only the event owner or a house admin may change this event")
		}
	}
	return e, m, nil
}
func (s *CalendarService) CreateEvent(ctx context.Context, in csil.Event) (csil.Event, error) {
	m, err := s.member(ctx, string(in.HouseId), false)
	if err != nil {
		return csil.Event{}, err
	}
	assigned := in.OwnerMemberId != "" && string(in.OwnerMemberId) != m.MemberID
	if assigned {
		if _, err = s.member(ctx, string(in.HouseId), true); err != nil {
			return csil.Event{}, err
		}
	}
	if err = validateEventTimes(in, false); err != nil {
		return csil.Event{}, err
	}
	h, err := s.house(ctx, string(in.HouseId))
	if err != nil {
		return csil.Event{}, err
	}
	if !h.Enabled {
		return s.native().CreateEvent(ctx, in)
	}
	// Assignment and unscheduled items need no remote authorization.
	if assigned || in.StartsAt == nil {
		return s.native().createLocalEvent(ctx, in, "pending")
	}
	if !h.Ready {
		return csil.Event{}, csilrpc.Conflict("house calendar setup is pending")
	}
	writer, err := s.writer(ctx, m)
	if err != nil {
		return csil.Event{}, err
	}
	if _, err = calendarContent(in, h.CalendarID, ""); err != nil {
		return csil.Event{}, err
	}
	e, err := s.native().createLocalEvent(ctx, in, "pending")
	if err != nil {
		return csil.Event{}, err
	}
	// A self-create is the owner's explicit approval of this revision.
	return s.acceptEventWithWriter(ctx, csil.EventDecision{EventId: e.EventId, Revision: *e.Revision}, writer)
}

func (s *CalendarService) ListLocalEvents(ctx context.Context, id csil.HouseID) ([]csil.Event, error) {
	if _, err := s.member(ctx, string(id), false); err != nil {
		return nil, err
	}
	h, err := s.house(ctx, string(id))
	if err != nil {
		return nil, err
	}
	q := s.DB.WithContext(ctx).Where("house_id = ? AND deleted_at IS NULL AND (calendar_state <> 'remote' OR publication_error <> '')", id)
	if !h.Enabled {
		q = q.Where("calendar_state IN ('pending','declined','publishing') OR starts_at IS NULL")
	}
	var rows []models.Event
	err = q.Order("created_at DESC, event_id").Find(&rows).Error
	return eventsToCSIL(rows), err
}
func (s *CalendarService) listEvents(ctx context.Context, in csil.EventListRequest) ([]csil.Event, error) {
	if _, err := s.member(ctx, string(in.HouseId), false); err != nil {
		return nil, err
	}
	h, err := s.house(ctx, string(in.HouseId))
	if err != nil {
		return nil, err
	}
	if !h.Enabled {
		rows, err := s.native().ListEvents(ctx, in)
		if err != nil {
			return nil, err
		}
		// Pending assignments are shown in the local list, not as accepted events.
		out := []csil.Event{}
		for _, e := range rows {
			if e.CalendarState == nil || (*e.CalendarState != "pending" && *e.CalendarState != "declined" && *e.CalendarState != "publishing") {
				out = append(out, e)
			}
		}
		return out, nil
	}
	if s.Remote == nil || !h.Ready {
		return nil, csilrpc.Conflict("house calendar is not ready")
	}
	start, end := tsToTimePtr(in.RangeStart), tsToTimePtr(in.RangeEnd)
	if start == nil || end == nil || !end.After(*start) {
		return nil, csilrpc.BadRequest("catalystcal reads require range_start and range_end")
	}
	occurrences, err := bridge.Occurrences(ctx, s.Remote.ApplicationTransport(), h.CalendarID, *start, *end)
	if err != nil {
		return nil, remoteError(err)
	}
	out := []csil.Event{}
	for _, occ := range occurrences {
		e, err := s.reference(ctx, h.HouseID, occ)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if *out[i].StartsAt == *out[j].StartsAt {
			return out[i].EventId < out[j].EventId
		}
		return *out[i].StartsAt < *out[j].StartsAt
	})
	limit, offset := normalizePaging(in.Limit, in.Offset)
	if offset >= len(out) {
		return []csil.Event{}, nil
	}
	last := offset + limit
	if last > len(out) {
		last = len(out)
	}
	return out[offset:last], nil
}

func (s *CalendarService) getEvent(ctx context.Context, id csil.EventID) (csil.Event, error) {
	e, _, err := s.event(ctx, id, false)
	if err != nil {
		return csil.Event{}, err
	}
	if e.CalendarState != "remote" || e.CalendarUnbound || e.EditPendingMemberID != nil {
		return eventToCSIL(e), nil
	}
	h, ref, remote, err := s.loadRemote(ctx, e)
	if err != nil {
		var missing *csilrpc.Error
		if e.PublicationError != "" && errors.As(err, &missing) && missing.Code == 404 {
			return eventToCSIL(e), nil
		}
		return csil.Event{}, err
	}
	if ref.RecurrenceID != "" {
		rid := cal.Timestamp(ref.RecurrenceID)
		occ, err := cal.NewEventClient(s.Remote.ApplicationTransport()).GetOccurrence(ctx, cal.GetOccurrenceRequest{EventId: cal.EventID(ref.RemoteEventID), RecurrenceId: &rid})
		if err != nil {
			return csil.Event{}, remoteError(err)
		}
		master := occ.Event.EventId
		if occ.Event.MasterEventId != nil {
			master = *occ.Event.MasterEventId
		}
		if string(occ.Event.CalendarId) != h.CalendarID || string(master) != ref.RemoteEventID || occ.RecurrenceId == nil || string(*occ.RecurrenceId) != ref.RecurrenceID {
			return csil.Event{}, csilrpc.NotFound("occurrence is no longer in this house calendar")
		}
		return s.remoteView(e, ref, occ, h), nil
	}
	if e.PublicationError != "" {
		if err := s.DB.WithContext(ctx).Model(&models.Event{}).Where("event_id=?", e.EventID).Update("publication_error", "").Error; err != nil {
			return csil.Event{}, err
		}
		e.PublicationError = ""
	}
	start, end, err := remoteBounds(remote)
	if err != nil {
		return csil.Event{}, err
	}
	return s.remoteView(e, ref, cal.Occurrence{Event: remote, Start: cal.Timestamp(start), End: cal.Timestamp(end)}, h), nil
}
func (s *CalendarService) loadRemote(ctx context.Context, e *models.Event) (houseCalendar, calendarRef, cal.Event, error) {
	h, err := s.house(ctx, e.HouseID)
	var ref calendarRef
	if err != nil {
		return h, ref, cal.Event{}, err
	}
	if s.Remote == nil || !h.Enabled || !h.Ready {
		return h, ref, cal.Event{}, csilrpc.Conflict("house calendar is not ready")
	}
	if err = s.DB.WithContext(ctx).First(&ref, "ref_id = ? AND house_id = ?", e.EventID, e.HouseID).Error; err != nil {
		return h, ref, cal.Event{}, err
	}
	remote, err := cal.NewEventClient(s.Remote.ApplicationTransport()).GetEvent(ctx, cal.EventID(ref.RemoteEventID))
	if err != nil {
		return h, ref, remote, remoteError(err)
	}
	if string(remote.CalendarId) != h.CalendarID {
		return h, ref, remote, csilrpc.NotFound("event is no longer in this house calendar")
	}
	return h, ref, remote, nil
}

func (s *CalendarService) updateEvent(ctx context.Context, in csil.Event) (csil.Event, error) {
	e, m, err := s.event(ctx, in.EventId, true)
	if err != nil {
		return csil.Event{}, err
	}
	if in.Revision == nil || *in.Revision != e.Revision {
		return csil.Event{}, csilrpc.Conflict("event changed; reload before saving")
	}
	if e.CalendarUnbound || e.EditPendingMemberID != nil {
		return csil.Event{}, csilrpc.Conflict("recover the saved edit before changing this event")
	}
	if e.CalendarState == "publishing" {
		return csil.Event{}, csilrpc.Conflict("publication is in progress; the accepted revision cannot change")
	}
	if err = validateEventTimes(in, false); err != nil {
		return csil.Event{}, err
	}
	if e.CalendarState != "remote" {
		return s.native().UpdateEvent(ctx, in)
	}
	h, ref, previous, err := s.loadRemote(ctx, e)
	if err != nil {
		return csil.Event{}, err
	}
	if err := s.requireNoSplit(ctx, e.HouseID, ref.RemoteEventID); err != nil {
		return csil.Event{}, err
	}
	writer, err := s.writer(ctx, m)
	if err != nil {
		return csil.Event{}, err
	}
	content, err := calendarContent(in, h.CalendarID, "")
	if err != nil {
		return csil.Event{}, err
	}
	// Keep fields that the Longhouse editor does not own.
	previous.Title = content.Title
	previous.Description = content.Description
	previous.Location = content.Location
	previous.Start = content.Start
	previous.End = content.End
	previous.AllDay = content.AllDay
	previous.StartDate = content.StartDate
	previous.EndDate = content.EndDate
	previous.Timezone = content.Timezone
	if in.RecurrenceFreq != nil || in.RecurrenceRule != nil || (in.ClearRecurrence != nil && *in.ClearRecurrence) {
		previous.Recurrence = content.Recurrence
	}
	scope := cal.RecurrenceScope("all")
	var rid *cal.Timestamp
	if ref.RecurrenceID != "" {
		scope = "this"
		v := cal.Timestamp(ref.RecurrenceID)
		rid = &v
	}
	if in.EditScope != nil {
		scope = cal.RecurrenceScope(*in.EditScope)
	}
	if scope != "all" && scope != "this" && scope != "this_and_future" {
		return csil.Event{}, csilrpc.BadRequest("invalid edit_scope")
	}
	if scope == "this_and_future" {
		if rid == nil {
			return csil.Event{}, csilrpc.BadRequest("open an occurrence to edit this and future instances")
		}
		return s.splitSeries(ctx, e, m, h, ref, previous, *rid, writer)
	}
	if scope == "all" && rid != nil {
		return csil.Event{}, csilrpc.BadRequest("open the whole series before editing all occurrences")
	}
	var result cal.Event
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "calendar-series:"+e.HouseID+":"+ref.RemoteEventID).Error; err != nil {
			return err
		}
		var current models.Event
		if err := tx.First(&current, "event_id=?", e.EventID).Error; err != nil {
			return err
		}
		if current.Revision != *in.Revision {
			return csilrpc.Conflict("event changed; reload before saving")
		}
		var err error
		result, err = cal.NewEventClient(writer).UpdateEvent(ctx, cal.UpdateEventRequest{Event: previous, Scope: &scope, RecurrenceId: rid})
		if err != nil {
			return remoteError(err)
		}
		e.Revision++
		return tx.Exec("UPDATE events SET revision=revision+1 WHERE event_id IN (SELECT ref_id FROM calendar_refs WHERE house_id=? AND remote_event_id=?)", e.HouseID, ref.RemoteEventID).Error
	})
	if err != nil {
		return csil.Event{}, err
	}
	start, end, err := remoteBounds(result)
	if err != nil {
		return csil.Event{}, err
	}
	return s.remoteView(e, ref, cal.Occurrence{Event: result, Start: cal.Timestamp(start), End: cal.Timestamp(end), RecurrenceId: rid}, h), nil
}

func (s *CalendarService) AcceptEvent(ctx context.Context, in csil.EventDecision) (csil.Event, error) {
	return s.acceptEventWithWriter(ctx, in, nil)
}
func (s *CalendarService) acceptEventWithWriter(ctx context.Context, in csil.EventDecision, writer cal.Transport) (csil.Event, error) {
	e, m, err := s.event(ctx, in.EventId, false)
	if err != nil {
		return csil.Event{}, err
	}
	if m.MemberID != e.OwnerMemberID {
		return csil.Event{}, csilrpc.Forbidden("only the assigned owner can accept this event")
	}
	if in.Revision != e.Revision {
		return csil.Event{}, csilrpc.Conflict("event changed; review the current revision before accepting")
	}
	action := "accepted"
	if e.CalendarState == "publishing" {
		action = "publication_recovery"
	}
	annotateAudit(ctx, e.HouseID, models.AuditActionUpdate, "event", e.EventID, map[string]any{"assignment": action, "revision": in.Revision})
	if e.CalendarState == "remote" {
		return s.GetEvent(ctx, in.EventId)
	}
	h, err := s.house(ctx, e.HouseID)
	if err != nil {
		return csil.Event{}, err
	}
	if !h.Enabled {
		if e.CalendarState != "pending" {
			return csil.Event{}, csilrpc.Conflict("event is not pending acceptance")
		}
		now := time.Now()
		err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&models.Event{}).Where("event_id=? AND revision=? AND calendar_state='pending' AND deleted_at IS NULL", e.EventID, in.Revision).Updates(map[string]any{"calendar_state": "local", "accepted_at": now, "accepted_revision": in.Revision, "revision": gorm.Expr("revision+1"), "next_recurrence_at": e.StartsAt})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return csilrpc.Conflict("event changed before acceptance")
			}
			return tx.Exec("INSERT INTO calendar_acceptances(event_id,revision,member_id) VALUES (?,?,?) ON CONFLICT DO NOTHING", e.EventID, in.Revision, m.MemberID).Error
		})
		if err != nil {
			return csil.Event{}, err
		}
		return s.GetEvent(ctx, in.EventId)
	}
	if !h.Ready {
		return csil.Event{}, csilrpc.Conflict("house calendar setup is pending")
	}
	if e.CalendarState == "declined" {
		return csil.Event{}, csilrpc.Conflict("edit the declined proposal before accepting it")
	}
	if writer == nil {
		writer, err = s.writer(ctx, m)
		if err != nil {
			return csil.Event{}, err
		}
	}
	var remote cal.Event
	if e.CalendarState != "publishing" {
		content := eventToCSIL(e)
		// Legacy recurrence cannot be reconstructed from changed or missing rows.
		// Publish the stored occurrence as a single event after explicit approval.
		var backup int64
		if err = s.DB.WithContext(ctx).Table("calendar_legacy_backup").Where("event_id=? AND body->>'calendar_state' IN ('legacy','local')", e.EventID).Count(&backup).Error; err != nil {
			return csil.Event{}, err
		}
		if backup > 0 {
			content.RecurrenceFreq = nil
			content.RecurrenceRule = nil
		}
		uid := "longhouse-" + e.EventID + "@" + s.Remote.ApplicationIdentity().SubjectDomain
		remote, err = calendarContent(content, h.CalendarID, uid)
		if err != nil {
			return csil.Event{}, err
		}
	}
	// Serialize acceptance across API replicas. Commit the accepted revision
	// before any remote mutation. No worker publishes a proposal on its own.
	err = s.DB.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		conn = conn.Session(&gorm.Session{NewDB: true})
		if err := conn.Exec("SELECT pg_advisory_lock(hashtextextended(?,0))", "calendar-event:"+e.EventID).Error; err != nil {
			return err
		}
		defer conn.WithContext(context.Background()).Exec("SELECT pg_advisory_unlock(hashtextextended(?,0))", "calendar-event:"+e.EventID)
		if err := conn.First(e, "event_id=? AND deleted_at IS NULL", e.EventID).Error; err != nil {
			return err
		}
		if e.CalendarState == "remote" {
			return nil
		}
		if e.Revision != in.Revision {
			return csilrpc.Conflict("event changed before acceptance")
		}
		if e.CalendarState == "publishing" {
			return s.publishAccepted(ctx, conn, e, m, writer, h.CalendarID, true)
		}
		if err := conn.Transaction(func(tx *gorm.DB) error {
			r := tx.Model(&models.Event{}).Where("event_id=? AND revision=? AND calendar_state IN ('pending','legacy','local') AND deleted_at IS NULL", e.EventID, in.Revision).Updates(map[string]any{"calendar_state": "publishing", "accepted_at": time.Now(), "accepted_revision": in.Revision, "publication_error": ""})
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return csilrpc.Conflict("event changed before acceptance")
			}
			publication := calendarPublication{EventID: e.EventID, Revision: in.Revision, CalendarID: h.CalendarID, IdempotencyKey: fmt.Sprintf("longhouse:%s:%d", e.EventID, in.Revision), Payload: cal.EncodeEvent(remote)}
			if err := tx.Create(&publication).Error; err != nil {
				return err
			}
			return tx.Exec("INSERT INTO calendar_acceptances(event_id,revision,member_id) VALUES (?,?,?) ON CONFLICT DO NOTHING", e.EventID, in.Revision, m.MemberID).Error
		}); err != nil {
			return err
		}
		return s.publishAccepted(ctx, conn, e, m, writer, h.CalendarID, false)
	})
	if err != nil {
		return csil.Event{}, err
	}
	return s.GetEvent(ctx, in.EventId)
}
func (s *CalendarService) DeclineEvent(ctx context.Context, in csil.EventDecision) (csil.Event, error) {
	e, m, err := s.event(ctx, in.EventId, false)
	if err != nil {
		return csil.Event{}, err
	}
	if m.MemberID != e.OwnerMemberID {
		return csil.Event{}, csilrpc.Forbidden("only the assigned owner can decline this event")
	}
	h, err := s.house(ctx, e.HouseID)
	if err != nil {
		return csil.Event{}, err
	}
	states := []string{"pending"}
	if h.Enabled {
		states = append(states, "legacy", "local")
	}
	annotateAudit(ctx, e.HouseID, models.AuditActionUpdate, "event", e.EventID, map[string]any{"assignment": "declined", "revision": in.Revision})
	r := s.DB.WithContext(ctx).Model(&models.Event{}).Where("event_id=? AND revision=? AND calendar_state IN ? AND deleted_at IS NULL", e.EventID, in.Revision, states).Updates(map[string]any{"calendar_state": "declined", "revision": gorm.Expr("revision+1")})
	if r.Error != nil {
		return csil.Event{}, r.Error
	}
	if r.RowsAffected != 1 {
		return csil.Event{}, csilrpc.Conflict("event changed or is no longer pending")
	}
	return s.GetEvent(ctx, in.EventId)
}

func (s *CalendarService) deleteEvent(ctx context.Context, id csil.EventID, future bool) (csil.EmptyResponse, error) {
	e, m, err := s.event(ctx, id, true)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if e.CalendarUnbound || e.EditPendingMemberID != nil {
		return csil.EmptyResponse{}, csilrpc.Conflict("recover the saved edit before deleting this event")
	}
	if e.CalendarState == "publishing" {
		return csil.EmptyResponse{}, csilrpc.Conflict("publication is in progress; this event cannot be cancelled")
	}
	if e.CalendarState != "remote" {
		if future {
			return s.native().DeleteEventAndFuture(ctx, id)
		}
		return s.native().DeleteEvent(ctx, id)
	}
	_, ref, _, err := s.loadRemote(ctx, e)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.requireNoSplit(ctx, e.HouseID, ref.RemoteEventID); err != nil {
		return csil.EmptyResponse{}, err
	}
	writer, err := s.writer(ctx, m)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	scope := cal.RecurrenceScope("all")
	var rid *cal.Timestamp
	if ref.RecurrenceID != "" {
		scope = "this"
		v := cal.Timestamp(ref.RecurrenceID)
		rid = &v
		if future {
			scope = "this_and_future"
		}
	}
	_, err = cal.NewEventClient(writer).DeleteEvent(ctx, cal.DeleteEventRequest{EventId: cal.EventID(ref.RemoteEventID), RecurrenceId: rid, Scope: &scope})
	return csil.EmptyResponse{}, remoteError(err)
}

func (s *CalendarService) reference(ctx context.Context, house string, occ cal.Occurrence) (csil.Event, error) {
	remoteID := string(occ.Event.EventId)
	if occ.Event.MasterEventId != nil {
		remoteID = string(*occ.Event.MasterEventId)
	}
	rid := ""
	if occ.RecurrenceId != nil {
		rid = string(*occ.RecurrenceId)
	}
	var ref calendarRef
	var e models.Event
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "calendar-ref:"+house+":"+remoteID+":"+rid).Error; err != nil {
			return err
		}
		err := tx.Order("ref_id ASC").Where("ref_id IN (SELECT event_id FROM events WHERE NOT calendar_unbound)").First(&ref, "house_id=? AND remote_event_id=? AND recurrence_id=?", house, remoteID, rid).Error
		if err == nil {
			return tx.First(&e, "event_id=?", ref.RefID).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if occ.Event.CreatedBy == nil {
			return errors.New("calendar event has no creator")
		}
		var member models.Member
		if err = tx.First(&member, "house_id=? AND linkkeys_domain=? AND linkkeys_user_id=?", house, occ.Event.CreatedBy.Domain, occ.Event.CreatedBy.Subject).Error; err != nil {
			return err
		}
		e = models.Event{HouseID: house, OwnerMemberID: member.MemberID, CalendarState: "remote", Revision: 1}
		var pending calendarSplit
		if err := tx.Where("house_id=? AND series_id=? AND NOT completed", house, remoteID).First(&pending).Error; err == nil {
			e.EditPendingMemberID = &pending.MemberID
			e.PublicationError = splitPendingMessage
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err = tx.Create(&e).Error; err != nil {
			return err
		}
		ref = calendarRef{RefID: e.EventID, HouseID: house, RemoteEventID: remoteID, RecurrenceID: rid}
		return tx.Create(&ref).Error
	})
	if err != nil {
		return csil.Event{}, err
	}
	out := s.remoteView(&e, ref, occ, houseCalendar{HouseID: house})
	if ref.RecurrenceID != "" {
		rootOccurrence := occ
		rootOccurrence.RecurrenceId = nil
		root, err := s.reference(ctx, house, rootOccurrence)
		if err != nil {
			return csil.Event{}, err
		}
		out.RecurrenceRootEventId = &root.EventId
	}
	return out, nil
}
func (s *CalendarService) remoteView(e *models.Event, ref calendarRef, occ cal.Occurrence, h houseCalendar) csil.Event {
	out := eventToCSIL(e)
	out.Title = occ.Event.Title
	out.Description = occ.Event.Description
	out.Location = occ.Event.Location
	out.AllDay = occ.Event.AllDay
	if e.EditPendingMemberID == nil {
		out.PublicationError = nil
	}
	start, end := csil.Timestamp(occ.Start), csil.Timestamp(occ.End)
	// catalystcal represents all-day occurrence dates as midnight UTC. Use
	// the event's calendar dates in its zone for the local date-based UI.
	if occ.Event.AllDay != nil && *occ.Event.AllDay {
		if a, b, err := remoteBounds(occ.Event); err == nil {
			start, end = csil.Timestamp(a), csil.Timestamp(b)
		}
	}
	out.StartsAt = &start
	out.EndsAt = &end
	out.Timezone = occ.Event.Timezone
	if occ.Event.Recurrence != nil {
		out.RecurrenceRule = &occ.Event.Recurrence.Rrule
	}
	out.CreatedAt = csil.Timestamp(occ.Event.CreatedAt)
	out.UpdatedAt = csil.Timestamp(occ.Event.UpdatedAt)
	if ref.RecurrenceID != "" {
		var root calendarRef
		if err := s.DB.Order("ref_id ASC").First(&root, "house_id=? AND remote_event_id=? AND recurrence_id=''", e.HouseID, ref.RemoteEventID).Error; err == nil {
			v := csil.EventID(root.RefID)
			out.RecurrenceRootEventId = &v
		}
	}
	return out
}
func validateEventTimes(in csil.Event, required bool) error {
	start, end := tsToTimePtr(in.StartsAt), tsToTimePtr(in.EndsAt)
	if in.StartsAt != nil && start == nil || in.EndsAt != nil && end == nil {
		return csilrpc.BadRequest("event times must use RFC3339")
	}
	if required && (start == nil || end == nil) {
		return csilrpc.BadRequest("set the event start and end before publication")
	}
	if start != nil && end != nil && !end.After(*start) {
		return csilrpc.BadRequest("event end must follow its start")
	}
	return nil
}
func calendarContent(in csil.Event, calendarID, uid string) (cal.Event, error) {
	if in.ClearRecurrence != nil && *in.ClearRecurrence {
		in.RecurrenceFreq = nil
		in.RecurrenceRule = nil
	}
	if err := validateEventTimes(in, true); err != nil {
		return cal.Event{}, err
	}
	start, end := tsToTimePtr(in.StartsAt), tsToTimePtr(in.EndsAt)
	a, b := cal.Timestamp(start.UTC().Format(time.RFC3339)), cal.Timestamp(end.UTC().Format(time.RFC3339))
	zone := "UTC"
	if in.Timezone != nil && *in.Timezone != "" {
		zone = *in.Timezone
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return cal.Event{}, csilrpc.BadRequest("invalid event time zone")
	}
	out := cal.Event{CalendarId: cal.CalendarID(calendarID), Title: in.Title, Description: in.Description, Location: in.Location, Start: &a, End: &b, AllDay: in.AllDay, Timezone: &zone, Uid: strPtrCopy(uid)}
	if in.AllDay != nil && *in.AllDay {
		x, y := cal.DateText(start.In(loc).Format("2006-01-02")), cal.DateText(end.In(loc).Format("2006-01-02"))
		if x >= y {
			return out, csilrpc.BadRequest("all-day events need an exclusive end date after the start date")
		}
		out.Start = nil
		out.End = nil
		out.StartDate = &x
		out.EndDate = &y
	}
	if in.RecurrenceFreq != nil && *in.RecurrenceFreq != "" {
		interval := int64(1)
		if in.RecurrenceInterval != nil {
			interval = *in.RecurrenceInterval
		}
		if interval < 1 {
			return out, csilrpc.BadRequest("recurrence interval must be positive")
		}
		freq := strings.ToUpper(string(*in.RecurrenceFreq))
		if freq == "QUARTERLY" {
			freq = "MONTHLY"
			interval *= 3
		}
		switch freq {
		case "HOURLY", "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
		default:
			return out, csilrpc.BadRequest("unsupported recurrence frequency")
		}
		rule := fmt.Sprintf("FREQ=%s;INTERVAL=%d", freq, interval)
		days := []string{}
		names := []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}
		for _, day := range in.RecurrenceByWeekday {
			if day < 0 || day > 6 {
				return out, csilrpc.BadRequest("invalid recurrence weekday")
			}
			days = append(days, names[day])
		}
		if len(days) > 0 {
			rule += ";BYDAY=" + strings.Join(days, ",")
			if in.RecurrenceBySetpos != nil && *in.RecurrenceBySetpos != 0 && (freq == "MONTHLY" || freq == "YEARLY") {
				rule += fmt.Sprintf(";BYSETPOS=%d", *in.RecurrenceBySetpos)
			}
		}
		out.Recurrence = &cal.Recurrence{Rrule: rule}
	}
	if in.RecurrenceRule != nil {
		out.Recurrence = nil
		if *in.RecurrenceRule != "" {
			out.Recurrence = &cal.Recurrence{Rrule: *in.RecurrenceRule}
		}
	}
	return out, nil
}
func remoteBounds(e cal.Event) (string, string, error) {
	if e.Start != nil && e.End != nil && (e.AllDay == nil || !*e.AllDay) {
		return string(*e.Start), string(*e.End), nil
	}
	if e.StartDate != nil && e.EndDate != nil {
		zone := "UTC"
		if e.Timezone != nil {
			zone = *e.Timezone
		}
		loc, err := time.LoadLocation(zone)
		if err != nil {
			return "", "", err
		}
		a, err := time.ParseInLocation("2006-01-02", string(*e.StartDate), loc)
		if err != nil {
			return "", "", err
		}
		b, err := time.ParseInLocation("2006-01-02", string(*e.EndDate), loc)
		return a.UTC().Format(time.RFC3339), b.UTC().Format(time.RFC3339), err
	}
	return "", "", errors.New("calendar event has no time bounds")
}

func (s *CalendarService) listTrash(ctx context.Context, id csil.HouseID) ([]csil.CalendarTrashEntry, error) {
	m, err := s.member(ctx, string(id), false)
	if err != nil {
		return nil, err
	}
	h, err := s.house(ctx, string(id))
	if err != nil {
		return nil, err
	}
	if !h.Enabled {
		return []csil.CalendarTrashEntry{}, nil
	}
	transport, err := s.writer(ctx, m)
	if err != nil {
		return nil, err
	}
	limit := uint64(500)
	list, err := cal.NewEventClient(transport).ListTrash(ctx, cal.ListTrashRequest{CalendarId: cal.CalendarID(h.CalendarID), Limit: &limit})
	if err != nil {
		return nil, remoteError(err)
	}
	out := []csil.CalendarTrashEntry{}
	for _, entry := range list.Entries {
		if string(entry.CalendarId) != h.CalendarID {
			return nil, errors.New("trash belongs to another calendar")
		}
		start, end, err := remoteBounds(entry.Event)
		if err != nil {
			return nil, err
		}
		event, err := s.reference(ctx, h.HouseID, cal.Occurrence{Event: entry.Event, Start: cal.Timestamp(start), End: cal.Timestamp(end), RecurrenceId: entry.RecurrenceId})
		if err != nil {
			return nil, err
		}
		out = append(out, csil.CalendarTrashEntry{TrashId: string(entry.TrashId), Event: event, DeletedAt: csil.Timestamp(entry.DeletedAt), ExpiresAt: csil.Timestamp(entry.ExpiresAt)})
	}
	return out, nil
}
func (s *CalendarService) restoreEvent(ctx context.Context, in csil.RestoreCalendarEventRequest) (csil.Event, error) {
	var pending int64
	if err := s.DB.WithContext(ctx).Model(&calendarSplit{}).Where("house_id=? AND NOT completed", in.HouseId).Count(&pending).Error; err != nil {
		return csil.Event{}, err
	}
	if pending > 0 {
		return csil.Event{}, csilrpc.Conflict(splitPendingMessage)
	}
	entries, err := s.ListTrash(ctx, in.HouseId)
	if err != nil {
		return csil.Event{}, err
	}
	found := false
	for _, entry := range entries {
		if entry.TrashId == in.TrashId {
			found = true
		}
	}
	if !found {
		return csil.Event{}, csilrpc.NotFound("calendar trash entry not found")
	}
	m, err := s.member(ctx, string(in.HouseId), false)
	if err != nil {
		return csil.Event{}, err
	}
	writer, err := s.writer(ctx, m)
	if err != nil {
		return csil.Event{}, err
	}
	restored, err := cal.NewEventClient(writer).RestoreEvent(ctx, cal.RestoreEventRequest{TrashId: cal.TrashID(in.TrashId)})
	if err != nil {
		return csil.Event{}, remoteError(err)
	}
	start, end, err := remoteBounds(restored)
	if err != nil {
		return csil.Event{}, err
	}
	return s.reference(ctx, string(in.HouseId), cal.Occurrence{Event: restored, Start: cal.Timestamp(start), End: cal.Timestamp(end)})
}
