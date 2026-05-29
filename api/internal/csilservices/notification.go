package csilservices

import (
	"context"
	"errors"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"gorm.io/gorm"
)

// NotificationService serves each member's personal feed. Notifications are
// fanned out on write (see CommentService); this service only reads them and
// flips per-recipient read state. A notification always belongs to exactly
// one member, so every op is scoped to the caller's own feed — there is no
// admin view of someone else's notifications.
type NotificationService struct{ Store store.Store }

func (s *NotificationService) Register(d *csilrpc.Dispatcher) {
	d.Register("notification", "ListNotifications", s.listNotifications)
	d.Register("notification", "UnreadCount", s.unreadCount)
	d.Register("notification", "MarkRead", s.markRead)
	d.Register("notification", "MarkAllRead", s.markAllRead)
}

func (s *NotificationService) listNotifications(ctx context.Context, body []byte) (any, error) {
	var req csil.NotificationListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	_, memberID, err := requireMemberForHouse(ctx, string(req.HouseId))
	if err != nil {
		return nil, err
	}
	unreadOnly := req.UnreadOnly != nil && *req.UnreadOnly
	limit, offset := normalizePaging(req.Limit, req.Offset)
	items, err := s.Store.ListNotificationsByMember(ctx, string(req.HouseId), memberID, unreadOnly, limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return notificationsToCSIL(items), nil
}

func (s *NotificationService) unreadCount(ctx context.Context, body []byte) (any, error) {
	var houseID csil.HouseID
	if err := csilrpc.Decode(body, &houseID); err != nil {
		return nil, err
	}
	_, memberID, err := requireMemberForHouse(ctx, string(houseID))
	if err != nil {
		return nil, err
	}
	count, err := s.Store.CountUnreadNotifications(ctx, string(houseID), memberID)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.NotificationUnreadCount{Count: uint64(count)}, nil
}

func (s *NotificationService) markRead(ctx context.Context, body []byte) (any, error) {
	var id csil.NotificationID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	item, err := s.Store.GetNotificationFeedItem(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, csilrpc.NotFound("notification not found")
		}
		return nil, csilrpc.Internal("internal error")
	}
	_, memberID, err := requireMemberForHouse(ctx, item.HouseID)
	if err != nil {
		return nil, err
	}
	if item.MemberID != memberID {
		return nil, csilrpc.Forbidden("not your notification")
	}
	if item.ReadAt == nil {
		if err := s.Store.MarkNotificationRead(ctx, string(id), time.Now().UTC()); err != nil {
			return nil, csilrpc.Internal("internal error")
		}
		if updated, err := s.Store.GetNotificationFeedItem(ctx, string(id)); err == nil {
			item = updated
		}
	}
	return notificationToCSIL(item), nil
}

func (s *NotificationService) markAllRead(ctx context.Context, body []byte) (any, error) {
	var houseID csil.HouseID
	if err := csilrpc.Decode(body, &houseID); err != nil {
		return nil, err
	}
	_, memberID, err := requireMemberForHouse(ctx, string(houseID))
	if err != nil {
		return nil, err
	}
	if err := s.Store.MarkAllNotificationsRead(ctx, string(houseID), memberID, time.Now().UTC()); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}
