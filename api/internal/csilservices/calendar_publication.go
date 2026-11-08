package csilservices

import (
	"context"
	"errors"
	"fmt"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

type calendarPublication struct {
	EventID                             string `gorm:"primaryKey"`
	Revision                            uint64 `gorm:"primaryKey"`
	CalendarID                          string
	IdempotencyKey                      string
	Payload                             []byte
	RemoteEventID, RemoteState, TrashID string
}

func (s *CalendarService) publishAccepted(ctx context.Context, conn *gorm.DB, e *models.Event, m *models.Member, writer cal.Transport, calendarID string, recovering bool) error {
	var publication calendarPublication
	if err := conn.First(&publication, "event_id=? AND revision=?", e.EventID, e.Revision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return csilrpc.Conflict("this publication predates durable receipts; an operator must reconcile it before any retry")
		}
		return err
	}
	request, err := cal.DecodeEvent(publication.Payload)
	if err != nil {
		return fmt.Errorf("decode accepted calendar request: %w", err)
	}
	if string(request.CalendarId) != publication.CalendarID || publication.CalendarID != calendarID {
		return errors.New("accepted calendar request does not match its receipt scope")
	}
	client := cal.NewEventClient(writer)
	var receipt cal.CreateEventReceipt
	if recovering {
		receipt, err = client.GetCreateReceipt(ctx, cal.CreateReceiptRequest{CalendarId: cal.CalendarID(publication.CalendarID), IdempotencyKey: publication.IdempotencyKey})
		var remote *cal.ClientError
		if err != nil && !(errors.As(err, &remote) && remote.Code == 404) {
			return s.recordPublicationFailure(ctx, conn, e, err)
		}
	}
	if !recovering || err != nil {
		receipt, err = client.CreateEventOnce(ctx, cal.CreateEventOnceRequest{IdempotencyKey: publication.IdempotencyKey, Event: request})
		if err != nil {
			return s.recordPublicationFailure(ctx, conn, e, err)
		}
	}
	// Even an authorization failure on a retry cannot prove the first call
	// failed. Keep the accepted revision locked until a receipt resolves it.
	if string(receipt.CalendarId) != publication.CalendarID || receipt.IdempotencyKey != publication.IdempotencyKey || receipt.EventId == "" || (publication.RemoteEventID != "" && publication.RemoteEventID != string(receipt.EventId)) {
		return errors.New("calendar create receipt does not match the accepted request")
	}
	message := ""
	switch receipt.State {
	case "live":
		if receipt.Event == nil || receipt.Event.EventId != receipt.EventId {
			return errors.New("calendar live receipt has no matching event")
		}
		creator := receipt.Event.CreatedBy
		if creator == nil || creator.Subject != m.LinkkeysUserID || creator.Domain != m.LinkkeysDomain || (creator.ApplicationId != nil && *creator.ApplicationId != "") {
			return errors.New("published event creator does not match the accepting member")
		}
		if string(receipt.Event.CalendarId) != publication.CalendarID {
			message = "This event was published, then moved out of this house calendar. It was not created again."
		}
	case "trashed":
		message = "This event was published, then moved to calendar trash. Restore it from calendar trash if needed."
	case "deleted":
		message = "This event was published, then permanently deleted. It was not created again."
	case "hidden":
		message = "This event was published, but is no longer accessible. It was not created again."
	default:
		return errors.New("unknown calendar receipt state")
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		ref := calendarRef{RefID: e.EventID, HouseID: e.HouseID, RemoteEventID: string(receipt.EventId)}
		// A calendar read may have discovered this event while the create reply
		// was lost. Keep both local IDs as aliases so neither set of links is lost.
		if err := tx.Create(&ref).Error; err != nil {
			return err
		}
		trash := ""
		if receipt.TrashId != nil {
			trash = string(*receipt.TrashId)
		}
		if err := tx.Model(&calendarPublication{}).Where("event_id=? AND revision=?", e.EventID, e.Revision).Updates(map[string]any{"remote_event_id": string(receipt.EventId), "remote_state": string(receipt.State), "trash_id": trash, "payload": nil}).Error; err != nil {
			return err
		}
		return tx.Model(&models.Event{}).Where("event_id=? AND calendar_state='publishing'", e.EventID).Updates(map[string]any{"calendar_state": "remote", "title": "", "description": "", "location": "", "starts_at": nil, "ends_at": nil, "recurrence_freq": nil, "recurrence_rule": "", "recurrence_by_weekday": nil, "recurrence_by_setpos": nil, "all_day": false, "next_recurrence_at": nil, "publication_error": message, "revision": gorm.Expr("revision+1")}).Error
	})
}

func (s *CalendarService) recordPublicationFailure(ctx context.Context, conn *gorm.DB, e *models.Event, cause error) error {
	// The error text from the service is not persisted because it can contain
	// request data. The pending item gives the owner a safe retry operation.
	if err := conn.Model(&models.Event{}).Where("event_id=? AND calendar_state='publishing'", e.EventID).Update("publication_error", "Publication is not confirmed. Retry publication to recover the saved request; do not create a replacement.").Error; err != nil {
		return err
	}
	var remote *cal.ClientError
	if errors.As(cause, &remote) && (remote.Code == 401 || remote.Code == 403) {
		return csilrpc.Conflict("calendar authorization failed; reconnect if needed, then retry the saved publication")
	}
	return nil
}
