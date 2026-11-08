package csilservices

import (
	"context"
	"errors"
	"fmt"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

const splitPendingMessage = "Calendar edit is not confirmed. The member who saved it must retry the saved edit before further changes."

type calendarSplit struct {
	EventID                                                               string `gorm:"primaryKey"`
	Revision                                                              uint64 `gorm:"primaryKey"`
	HouseID, CalendarID, SeriesID, MemberID, IdempotencyKey, RecurrenceID string
	Payload                                                               []byte
	Completed                                                             bool
	SuccessorID                                                           string
}

func (s *CalendarService) requireNoSplit(ctx context.Context, house, series string) error {
	var count int64
	if err := s.DB.WithContext(ctx).Model(&calendarSplit{}).Where("house_id=? AND series_id=? AND NOT completed", house, series).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return csilrpc.Conflict(splitPendingMessage)
	}
	return nil
}

func (s *CalendarService) splitSeries(ctx context.Context, e *models.Event, m *models.Member, h houseCalendar, ref calendarRef, content cal.Event, rid cal.Timestamp, writer cal.Transport) (csil.Event, error) {
	if content.Recurrence == nil {
		return csil.Event{}, csilrpc.BadRequest("this event is not a recurring series")
	}
	operation := calendarSplit{EventID: e.EventID, Revision: e.Revision, HouseID: e.HouseID, CalendarID: h.CalendarID, SeriesID: ref.RemoteEventID, MemberID: m.MemberID, RecurrenceID: string(rid), IdempotencyKey: fmt.Sprintf("longhouse-split:%s:%d", e.EventID, e.Revision), Payload: cal.EncodeEvent(content)}
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&operation).Error; err != nil {
			return err
		}
		return tx.Model(&models.Event{}).Where("event_id IN (SELECT ref_id FROM calendar_refs WHERE house_id=? AND remote_event_id=?) AND NOT calendar_unbound", e.HouseID, ref.RemoteEventID).Updates(map[string]any{"edit_pending_member_id": m.MemberID, "publication_error": splitPendingMessage}).Error
	}); err != nil {
		return csil.Event{}, err
	}
	annotateAudit(ctx, e.HouseID, models.AuditActionUpdate, "event", e.EventID, map[string]any{"calendar_edit": "split", "revision": e.Revision})
	if err := s.finishSplit(ctx, &operation, writer, false); err != nil {
		return csil.Event{}, err
	}
	return s.GetEvent(ctx, csil.EventID(e.EventID))
}

func (s *CalendarService) RecoverEdit(ctx context.Context, id csil.EventID) (out csil.Event, err error) {
	e, m, err := s.event(ctx, id, true)
	if err != nil {
		return out, err
	}
	err = s.calendarLocked(ctx, e.HouseID, func(c *CalendarService) error {
		var ref calendarRef
		if err := c.DB.First(&ref, "ref_id=?", id).Error; err != nil {
			return err
		}
		var operation calendarSplit
		if err := c.DB.Where("house_id=? AND series_id=? AND NOT completed", e.HouseID, ref.RemoteEventID).First(&operation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				out, err = c.GetEvent(ctx, id)
				return err
			}
			return err
		}
		if operation.MemberID != m.MemberID {
			return csilrpc.Forbidden("only the member who saved this edit can recover it")
		}
		h, err := c.house(ctx, e.HouseID)
		if err != nil {
			return err
		}
		if !h.Ready || !h.Enabled || h.CalendarID != operation.CalendarID {
			return csilrpc.Conflict("the original house calendar is not available")
		}
		writer, err := c.writer(ctx, m)
		if err != nil {
			return err
		}
		annotateAudit(ctx, e.HouseID, models.AuditActionUpdate, "event", operation.EventID, map[string]any{"calendar_edit": "split_recovery", "revision": operation.Revision})
		if err = c.finishSplit(ctx, &operation, writer, true); err != nil {
			return err
		}
		out, err = c.GetEvent(ctx, csil.EventID(operation.EventID))
		return err
	})
	return
}

func (s *CalendarService) finishSplit(ctx context.Context, operation *calendarSplit, writer cal.Transport, recovering bool) error {
	content, err := cal.DecodeEvent(operation.Payload)
	if err != nil {
		return err
	}
	if string(content.EventId) != operation.SeriesID || string(content.CalendarId) != operation.CalendarID {
		return errors.New("saved split does not match its scope")
	}
	var refs []calendarRef
	if err = s.DB.Where("house_id=? AND remote_event_id=? AND recurrence_id<>'' AND ref_id IN (SELECT event_id FROM events WHERE NOT calendar_unbound)", operation.HouseID, operation.SeriesID).Order("recurrence_id, ref_id").Find(&refs).Error; err != nil {
		return err
	}
	ids := []cal.Timestamp{}
	seen := map[string]bool{}
	for _, ref := range refs {
		if !seen[ref.RecurrenceID] {
			seen[ref.RecurrenceID] = true
			ids = append(ids, cal.Timestamp(ref.RecurrenceID))
		}
	}
	client := cal.NewEventClient(writer)
	mappings := map[string]cal.SplitOccurrenceMapping{}
	var receipt cal.SplitSeriesReceipt
	// Every response maps at most 500 IDs. All batches use the same durable
	// snapshot; no reference is changed until every batch has been checked.
	for offset := 0; offset < len(ids) || offset == 0; offset += 500 {
		end := offset + 500
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[offset:end]
		var current cal.SplitSeriesReceipt
		if offset > 0 || recovering {
			current, err = client.GetSplitReceipt(ctx, cal.SplitReceiptRequest{SeriesId: cal.EventID(operation.SeriesID), IdempotencyKey: operation.IdempotencyKey, MapRecurrenceIds: batch})
		}
		if offset == 0 && (!recovering || isRemoteNotFound(err)) {
			current, err = client.SplitSeriesOnce(ctx, cal.SplitSeriesOnceRequest{IdempotencyKey: operation.IdempotencyKey, Event: content, RecurrenceId: cal.Timestamp(operation.RecurrenceID), MapRecurrenceIds: batch})
		}
		if err != nil {
			return csilrpc.Conflict("calendar edit is not confirmed; reconnect if needed, then retry the saved edit")
		}
		if err = validateSplitReceipt(operation, current, batch); err != nil {
			return err
		}
		if offset == 0 {
			receipt = current
		} else if current.SuccessorId != receipt.SuccessorId || current.SplitAt != receipt.SplitAt || current.InPlace != receipt.InPlace {
			return errors.New("calendar split receipt changed between mapping batches")
		}
		for _, mapping := range current.Mappings {
			mappings[string(mapping.RecurrenceId)] = mapping
		}
		if end == len(ids) {
			break
		}
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, ref := range refs {
			mapping := mappings[ref.RecurrenceID]
			notice := ""
			if mapping.Result == "none" {
				notice = "This occurrence has no match after the series edit. Its link and comments are kept."
				if err := tx.Model(&models.Event{}).Where("event_id=?", ref.RefID).Update("calendar_unbound", true).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Model(&calendarRef{}).Where("ref_id=?", ref.RefID).Updates(map[string]any{"remote_event_id": string(*mapping.EventId), "recurrence_id": string(*mapping.MappedRecurrenceId)}).Error; err != nil {
					return err
				}
				state, body := receipt.SeriesState, receipt.Series
				if mapping.Result == "successor" {
					state, body = receipt.SuccessorState, receipt.Successor
				}
				notice = splitResultNotice(state, body, operation.CalendarID)
			}
			if err := tx.Model(&models.Event{}).Where("event_id=?", ref.RefID).Updates(map[string]any{"edit_pending_member_id": nil, "publication_error": notice, "revision": gorm.Expr("revision+1")}).Error; err != nil {
				return err
			}
		}
		rootNotice := splitResultNotice(receipt.SeriesState, receipt.Series, operation.CalendarID)
		if receipt.InPlace {
			rootNotice = splitResultNotice(receipt.SuccessorState, receipt.Successor, operation.CalendarID)
		}
		if err := tx.Model(&models.Event{}).Where("event_id IN (SELECT ref_id FROM calendar_refs WHERE house_id=? AND remote_event_id=? AND recurrence_id='')", operation.HouseID, operation.SeriesID).Updates(map[string]any{"edit_pending_member_id": nil, "publication_error": rootNotice, "revision": gorm.Expr("revision+1")}).Error; err != nil {
			return err
		}
		// A successor root lets an old occurrence link open its current series.
		var root calendarRef
		err := tx.Order("ref_id ASC").First(&root, "house_id=? AND remote_event_id=? AND recurrence_id=''", operation.HouseID, receipt.SuccessorId).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var source models.Event
			if err = tx.First(&source, "event_id=?", operation.EventID).Error; err != nil {
				return err
			}
			shell := models.Event{HouseID: operation.HouseID, OwnerMemberID: source.OwnerMemberID, CalendarState: "remote", Revision: 1, PublicationError: splitResultNotice(receipt.SuccessorState, receipt.Successor, operation.CalendarID)}
			if err = tx.Create(&shell).Error; err != nil {
				return err
			}
			if err = tx.Create(&calendarRef{RefID: shell.EventID, HouseID: operation.HouseID, RemoteEventID: string(receipt.SuccessorId)}).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		return tx.Model(operation).Updates(map[string]any{"completed": true, "successor_id": string(receipt.SuccessorId), "payload": nil}).Error
	})
}
func isRemoteNotFound(err error) bool {
	var remote *cal.ClientError
	return errors.As(err, &remote) && remote.Code == 404
}
func splitResultNotice(state cal.CreateReceiptState, event *cal.Event, calendar string) string {
	if state == "live" && event != nil && string(event.CalendarId) == calendar {
		return ""
	}
	return "The series edit completed, but this event is no longer available in this house calendar. Its link and comments are kept."
}
func validateSplitReceipt(operation *calendarSplit, receipt cal.SplitSeriesReceipt, ids []cal.Timestamp) error {
	if string(receipt.SeriesId) != operation.SeriesID || receipt.IdempotencyKey != operation.IdempotencyKey || string(receipt.CalendarId) != operation.CalendarID || receipt.SuccessorId == "" || (receipt.InPlace && receipt.SuccessorId != receipt.SeriesId) {
		return errors.New("calendar split receipt does not match the saved request")
	}
	boundary, err := time.Parse(time.RFC3339, string(receipt.SplitAt))
	if err != nil {
		return errors.New("invalid calendar split boundary")
	}
	requested, err := time.Parse(time.RFC3339, operation.RecurrenceID)
	if err != nil || (!receipt.InPlace && (!boundary.Equal(requested) || receipt.SuccessorId == receipt.SeriesId)) {
		return errors.New("calendar split boundary does not match the request")
	}

	if len(receipt.Mappings) != len(ids) {
		return errors.New("calendar split mapping is incomplete")
	}
	for _, state := range []cal.CreateReceiptState{receipt.SeriesState, receipt.SuccessorState} {
		if state != "live" && state != "trashed" && state != "hidden" && state != "deleted" {
			return errors.New("unknown calendar split state")
		}
	}
	if receipt.SeriesState == "live" && (receipt.Series == nil || receipt.Series.EventId != receipt.SeriesId) {
		return errors.New("calendar split has no matching source")
	}
	if receipt.SuccessorState == "live" && (receipt.Successor == nil || receipt.Successor.EventId != receipt.SuccessorId) {
		return errors.New("calendar split has no matching successor")
	}
	for i, mapping := range receipt.Mappings {
		if mapping.RecurrenceId != ids[i] {
			return errors.New("calendar split mapping order mismatch")
		}
		if mapping.Result == "none" {
			continue
		}
		if mapping.EventId == nil || mapping.MappedRecurrenceId == nil {
			return errors.New("calendar split mapping has no target")
		}
		if _, err := time.Parse(time.RFC3339, string(*mapping.MappedRecurrenceId)); err != nil {
			return errors.New("invalid mapped recurrence ID")
		}
		switch mapping.Result {
		case "original":
			if *mapping.EventId != receipt.SeriesId || *mapping.MappedRecurrenceId != mapping.RecurrenceId {
				return errors.New("calendar split changed a retained reference")
			}
		case "successor":
			if *mapping.EventId != receipt.SuccessorId {
				return errors.New("calendar split mapping names another series")
			}
		default:
			return errors.New("unknown calendar split mapping")
		}
	}
	return nil
}
