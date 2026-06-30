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

// GroupService surfaces the per-house group join (a set of members the
// admins curate as one named bucket — "House team", "Garden crew", etc.).
// Reading is open to any house member; mutating membership and group
// metadata is admin-only.
type GroupService struct{ Store store.Store }

func (s *GroupService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("group", "CreateGroup", csilrpc.Route(s.CreateGroup, csil.DecodeGroupCreateGroupRequest, csil.EncodeGroupCreateGroupResponse))
	d.RegisterTyped("group", "UpdateGroup", csilrpc.Route(s.UpdateGroup, csil.DecodeGroupUpdateGroupRequest, csil.EncodeGroupUpdateGroupResponse))
	d.RegisterTyped("group", "DeleteGroup", csilrpc.Route(s.DeleteGroup, csil.DecodeGroupDeleteGroupRequest, csil.EncodeGroupDeleteGroupResponse))
	d.RegisterTyped("group", "ListGroups", csilrpc.Route(s.ListGroups, csil.DecodeGroupListGroupsRequest, csil.EncodeGroupListGroupsResponse))
	d.RegisterTyped("group", "AddGroupMember", csilrpc.Route(s.AddGroupMember, csil.DecodeGroupAddGroupMemberRequest, csil.EncodeGroupAddGroupMemberResponse))
	d.RegisterTyped("group", "RemoveGroupMember", csilrpc.Route(s.RemoveGroupMember, csil.DecodeGroupRemoveGroupMemberRequest, csil.EncodeGroupRemoveGroupMemberResponse))
	d.RegisterTyped("group", "ListGroupMembers", csilrpc.Route(s.ListGroupMembers, csil.DecodeGroupListGroupMembersRequest, csil.EncodeGroupListGroupMembersResponse))
}

func (s *GroupService) CreateGroup(ctx context.Context, in csil.Group) (csil.Group, error) {
	if in.HouseId == "" || in.Name == "" {
		return csil.Group{}, csilrpc.BadRequest("house_id and name are required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(in.HouseId), "admin"); err != nil {
		return csil.Group{}, err
	}
	g := &models.Group{
		HouseID:     string(in.HouseId),
		Name:        in.Name,
		Description: derefStr(in.Description),
	}
	if err := s.Store.CreateGroup(ctx, g); err != nil {
		return csil.Group{}, csilrpc.Internal("internal error")
	}
	return groupToCSIL(g), nil
}

func (s *GroupService) UpdateGroup(ctx context.Context, in csil.Group) (csil.Group, error) {
	if in.GroupId == "" {
		return csil.Group{}, csilrpc.BadRequest("group_id is required")
	}
	existing, err := s.Store.GetGroupByID(ctx, string(in.GroupId))
	if err != nil {
		return csil.Group{}, csilrpc.NotFound("group not found")
	}
	if _, _, err := requireRoleForHouse(ctx, existing.HouseID, "admin"); err != nil {
		return csil.Group{}, err
	}
	if in.Name != "" {
		existing.Name = in.Name
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	if err := s.Store.UpdateGroup(ctx, existing); err != nil {
		return csil.Group{}, csilrpc.Internal("internal error")
	}
	return groupToCSIL(existing), nil
}

func (s *GroupService) DeleteGroup(ctx context.Context, id csil.GroupID) (csil.EmptyResponse, error) {
	existing, err := s.Store.GetGroupByID(ctx, string(id))
	if err != nil || existing.DeletedAt != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("group not found")
	}
	_, callerMemberID, err := requireRoleForHouse(ctx, existing.HouseID, "admin")
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteGroup(ctx, existing.GroupID, callerMemberID, opID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, existing.HouseID, "group", existing.GroupID, opID, existing)
	return csil.EmptyResponse{}, nil
}

func (s *GroupService) ListGroups(ctx context.Context, req csil.HouseScopedListRequest) ([]csil.Group, error) {
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	rows, err := s.Store.ListGroupsByHouse(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return groupsToCSIL(rows), nil
}

func (s *GroupService) AddGroupMember(ctx context.Context, ref csil.GroupMemberRef) (csil.EmptyResponse, error) {
	if err := s.adminAuthzGroup(ctx, string(ref.GroupId)); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.AddGroupMember(ctx, string(ref.GroupId), string(ref.MemberId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *GroupService) RemoveGroupMember(ctx context.Context, ref csil.GroupMemberRef) (csil.EmptyResponse, error) {
	if err := s.adminAuthzGroup(ctx, string(ref.GroupId)); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.RemoveGroupMember(ctx, string(ref.GroupId), string(ref.MemberId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// ListGroupMembers's CSIL request is MemberScopedListRequest, but only
// the group_id-equivalent is meaningful here. We accept either shape via
// the existing decoder; the house_id field is ignored (the group's own
// house gates the authz).
func (s *GroupService) ListGroupMembers(ctx context.Context, req csil.MemberScopedListRequest) ([]csil.Member, error) {
	// In this service the "member id" slot carries the group id (the CSIL
	// signature is shared with the per-member listings on other services).
	groupID := string(req.MemberId)
	if groupID == "" {
		return nil, csilrpc.BadRequest("group id is required (passed in member_id)")
	}
	g, err := s.Store.GetGroupByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, csilrpc.NotFound("group not found")
		}
		return nil, csilrpc.Internal("internal error")
	}
	if _, _, err := requireMemberForHouse(ctx, g.HouseID); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListGroupMembers(ctx, g.GroupID)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return membersToCSIL(rows), nil
}

// adminAuthzGroup loads the group's house and gates on the admin role.
func (s *GroupService) adminAuthzGroup(ctx context.Context, groupID string) error {
	g, err := s.Store.GetGroupByID(ctx, groupID)
	if err != nil {
		return csilrpc.NotFound("group not found")
	}
	if _, _, err := requireRoleForHouse(ctx, g.HouseID, "admin"); err != nil {
		return err
	}
	return nil
}
