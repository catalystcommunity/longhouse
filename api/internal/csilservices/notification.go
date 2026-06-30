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
	d.RegisterTyped("notification", "ListNotifications", csilrpc.Route(s.ListNotifications, csil.DecodeNotificationListNotificationsRequest, csil.EncodeNotificationListNotificationsResponse))
	d.RegisterTyped("notification", "UnreadCount", csilrpc.Route(s.UnreadCount, csil.DecodeNotificationUnreadCountRequest, csil.EncodeNotificationUnreadCountResponse))
	d.RegisterTyped("notification", "MarkRead", csilrpc.Route(s.MarkRead, csil.DecodeNotificationMarkReadRequest, csil.EncodeNotificationMarkReadResponse))
	d.RegisterTyped("notification", "MarkAllRead", csilrpc.Route(s.MarkAllRead, csil.DecodeNotificationMarkAllReadRequest, csil.EncodeNotificationMarkAllReadResponse))
}

func (s *NotificationService) ListNotifications(ctx context.Context, req csil.NotificationListRequest) ([]csil.Notification, error) {
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

func (s *NotificationService) UnreadCount(ctx context.Context, houseID csil.HouseID) (csil.NotificationUnreadCount, error) {
	_, memberID, err := requireMemberForHouse(ctx, string(houseID))
	if err != nil {
		return csil.NotificationUnreadCount{}, err
	}
	count, err := s.Store.CountUnreadNotifications(ctx, string(houseID), memberID)
	if err != nil {
		return csil.NotificationUnreadCount{}, csilrpc.Internal("internal error")
	}
	return csil.NotificationUnreadCount{Count: uint64(count)}, nil
}

func (s *NotificationService) MarkRead(ctx context.Context, id csil.NotificationID) (csil.Notification, error) {
	item, err := s.Store.GetNotificationFeedItem(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return csil.Notification{}, csilrpc.NotFound("notification not found")
		}
		return csil.Notification{}, csilrpc.Internal("internal error")
	}
	_, memberID, err := requireMemberForHouse(ctx, item.HouseID)
	if err != nil {
		return csil.Notification{}, err
	}
	if item.MemberID != memberID {
		return csil.Notification{}, csilrpc.Forbidden("not your notification")
	}
	if item.ReadAt == nil {
		if err := s.Store.MarkNotificationRead(ctx, string(id), time.Now().UTC()); err != nil {
			return csil.Notification{}, csilrpc.Internal("internal error")
		}
		if updated, err := s.Store.GetNotificationFeedItem(ctx, string(id)); err == nil {
			item = updated
		}
	}
	return notificationToCSIL(item), nil
}

func (s *NotificationService) MarkAllRead(ctx context.Context, houseID csil.HouseID) (csil.EmptyResponse, error) {
	_, memberID, err := requireMemberForHouse(ctx, string(houseID))
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.MarkAllNotificationsRead(ctx, string(houseID), memberID, time.Now().UTC()); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}
