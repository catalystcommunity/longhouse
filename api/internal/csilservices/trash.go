package csilservices

import (
	"context"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// TrashService is the admin-only trash bin: list soft-deleted items, restore a
// delete operation (or a single item's operation), or purge an item now. Every
// method requires the caller's admin role in the target house.
type TrashService struct {
	Store store.Store
}

func (s *TrashService) Register(d *csilrpc.Dispatcher) {
	d.Register("trash", "ListTrash", s.listTrash)
	d.Register("trash", "Restore", s.restore)
	d.Register("trash", "Purge", s.purge)
}

func (s *TrashService) listTrash(ctx context.Context, body []byte) (any, error) {
	var req csil.HouseScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if req.HouseId == "" {
		return nil, csilrpc.BadRequest("house_id is required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(req.HouseId), models.RoleAdmin); err != nil {
		return nil, err
	}
	limit, offset := 100, 0
	if req.Limit != nil {
		limit = int(*req.Limit)
	}
	if req.Offset != nil {
		offset = int(*req.Offset)
	}
	rows, err := s.Store.ListTrash(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	items := make([]csil.TrashItem, 0, len(rows))
	for i := range rows {
		items = append(items, trashRowToCSIL(&rows[i]))
	}
	return csil.TrashPage{Items: items}, nil
}

func (s *TrashService) restore(ctx context.Context, body []byte) (any, error) {
	var req csil.RestoreRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if req.HouseId == "" {
		return nil, csilrpc.BadRequest("house_id is required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(req.HouseId), models.RoleAdmin); err != nil {
		return nil, err
	}

	opID := ""
	resType, resID := "", ""
	switch {
	case req.DeletedOpId != nil && *req.DeletedOpId != "":
		opID = *req.DeletedOpId
	case req.ResourceType != nil && req.ResourceId != nil:
		resType, resID = *req.ResourceType, *req.ResourceId
		// Confirm the item lives in the caller's house before resolving its op.
		house, err := s.Store.ResourceHouseID(ctx, resType, resID)
		if err != nil {
			return nil, csilrpc.BadRequest("unknown resource type")
		}
		if house == "" {
			return nil, csilrpc.NotFound("item not found in trash")
		}
		if house != string(req.HouseId) {
			return nil, csilrpc.Forbidden("item belongs to another house")
		}
		op, err := s.Store.FindDeletedOpID(ctx, resType, resID)
		if err != nil {
			return nil, csilrpc.Internal("internal error")
		}
		if op == "" {
			return nil, csilrpc.NotFound("item not found in trash")
		}
		opID = op
	default:
		return nil, csilrpc.BadRequest("provide deleted_op_id, or resource_type and resource_id")
	}

	// A delete op touches exactly one entity type; restoring across all tables
	// for the op id is simplest and only the matching table has rows. opIDs are
	// only ever exposed to a house's own admins (trash/audit are house-scoped),
	// so this can't cross houses.
	restorers := []func(context.Context, string) error{
		s.Store.RestoreTasksByOp, s.Store.RestoreProjectsByOp, s.Store.RestoreEventsByOp,
		s.Store.RestoreCommentsByOp, s.Store.RestoreRolesByOp,
		s.Store.RestoreSkillsByOp, s.Store.RestoreGroupsByOp, s.Store.RestoreMilestonesByOp,
	}
	for _, restore := range restorers {
		if err := restore(ctx, opID); err != nil {
			return nil, csilrpc.Internal("internal error")
		}
	}
	s.reArmEventSeries(ctx, string(req.HouseId), opID)
	annotateAudit(ctx, string(req.HouseId), models.AuditActionRestore, resType, resID,
		map[string]any{"deleted_op_id": opID})
	return csil.EmptyResponse{}, nil
}

// reArmEventSeries restores the recurrence on a root that a "delete this &
// future" op muted. The soft-delete only brought the deleted child rows back;
// the root's recurrence_freq/next_recurrence_at were cleared on the root (a
// live row), and the prior values were recorded in the delete audit entry's
// detail. Best-effort: if the audit detail is missing (audit write failed) the
// children still restore, just without future spawning resuming.
func (s *TrashService) reArmEventSeries(ctx context.Context, houseID, opID string) {
	detail, err := s.Store.GetDeleteOpDetail(ctx, houseID, opID)
	if err != nil || detail == nil {
		return
	}
	rootID, _ := detail["root_event_id"].(string)
	if rootID == "" {
		return
	}
	root, err := s.Store.GetEventByID(ctx, rootID)
	if err != nil || root == nil {
		return
	}
	if freq, ok := detail["root_prior_recurrence_freq"].(string); ok && freq != "" {
		f := freq
		root.RecurrenceFreq = &f
	}
	if nextStr, ok := detail["root_prior_next_recurrence_at"].(string); ok && nextStr != "" {
		if t, perr := time.Parse(time.RFC3339, nextStr); perr == nil {
			tt := t
			root.NextRecurrenceAt = &tt
		}
	}
	_ = s.Store.UpdateEvent(ctx, root)
}

func (s *TrashService) purge(ctx context.Context, body []byte) (any, error) {
	var req csil.PurgeRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if req.HouseId == "" || req.ResourceType == "" || req.ResourceId == "" {
		return nil, csilrpc.BadRequest("house_id, resource_type and resource_id are required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(req.HouseId), models.RoleAdmin); err != nil {
		return nil, err
	}
	house, err := s.Store.ResourceHouseID(ctx, req.ResourceType, req.ResourceId)
	if err != nil {
		return nil, csilrpc.BadRequest("unknown resource type")
	}
	if house == "" {
		return nil, csilrpc.NotFound("item not found in trash")
	}
	if house != string(req.HouseId) {
		return nil, csilrpc.Forbidden("item belongs to another house")
	}
	if err := s.Store.PurgeResource(ctx, req.ResourceType, req.ResourceId); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	annotateAudit(ctx, string(req.HouseId), models.AuditActionPurge, req.ResourceType, req.ResourceId, nil)
	return csil.EmptyResponse{}, nil
}

func trashRowToCSIL(r *models.TrashRow) csil.TrashItem {
	out := csil.TrashItem{
		ResourceType: r.ResourceType,
		ResourceId:   r.ResourceID,
		HouseId:      csil.HouseID(r.HouseID),
		Title:        strPtrCopy(r.Title),
		DeletedAt:    ts(r.DeletedAt),
		DeletedOpId:  "",
	}
	if r.DeletedOpID != nil {
		out.DeletedOpId = *r.DeletedOpID
	}
	if r.DeletedByMemberID != nil {
		m := csil.MemberID(*r.DeletedByMemberID)
		out.DeletedByMemberId = &m
	}
	return out
}
