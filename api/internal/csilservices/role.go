package csilservices

import (
	"context"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// RoleService covers the per-house role catalog and the grant/revoke
// relations. The canonical "admin" and "member" roles are seeded with
// every house and cannot be deleted; custom roles can be added freely
// (admin-only on every write).
type RoleService struct{ Store store.Store }

func (s *RoleService) Register(d *csilrpc.Dispatcher) {
	d.Register("role", "CreateRole", s.createRole)
	d.Register("role", "UpdateRole", s.updateRole)
	d.Register("role", "DeleteRole", s.deleteRole)
	d.Register("role", "ListRoles", s.listRoles)
	d.Register("role", "GrantRole", s.grantRole)
	d.Register("role", "RevokeRole", s.revokeRole)
	d.Register("role", "ListMemberRoles", s.listMemberRoles)
}

func (s *RoleService) createRole(ctx context.Context, body []byte) (any, error) {
	var in csil.Role
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.HouseId == "" || in.Name == "" {
		return nil, csilrpc.BadRequest("house_id and name are required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(in.HouseId), "admin"); err != nil {
		return nil, err
	}
	r := &models.Role{
		HouseID:     string(in.HouseId),
		Name:        in.Name,
		Description: derefStr(in.Description),
	}
	if err := s.Store.CreateRole(ctx, r); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return roleToCSIL(r), nil
}

func (s *RoleService) updateRole(ctx context.Context, body []byte) (any, error) {
	var in csil.Role
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.RoleId == "" {
		return nil, csilrpc.BadRequest("role_id is required")
	}
	existing, err := s.Store.GetRoleByID(ctx, string(in.RoleId))
	if err != nil {
		return nil, csilrpc.NotFound("role not found")
	}
	if _, _, err := requireRoleForHouse(ctx, existing.HouseID, "admin"); err != nil {
		return nil, err
	}
	if in.Name != "" {
		existing.Name = in.Name
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	if err := s.Store.UpdateRole(ctx, existing); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return roleToCSIL(existing), nil
}

// deleteRole refuses to delete the canonical roles (admin, member) — the
// house seed relies on them, and admin-recovery becomes painful without
// them. Custom roles delete freely.
func (s *RoleService) deleteRole(ctx context.Context, body []byte) (any, error) {
	var id csil.RoleID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetRoleByID(ctx, string(id))
	if err != nil || existing.DeletedAt != nil {
		return nil, csilrpc.NotFound("role not found")
	}
	_, callerMemberID, err := requireRoleForHouse(ctx, existing.HouseID, "admin")
	if err != nil {
		return nil, err
	}
	if existing.Name == models.RoleAdmin || existing.Name == models.RoleMember {
		return nil, csilrpc.Conflict("the admin and member roles are reserved and cannot be deleted")
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteRole(ctx, existing.RoleID, callerMemberID, opID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, existing.HouseID, "role", existing.RoleID, opID, existing)
	return csil.EmptyResponse{}, nil
}

func (s *RoleService) listRoles(ctx context.Context, body []byte) (any, error) {
	var req csil.HouseScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	rows, err := s.Store.ListRolesByHouse(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return rolesToCSIL(rows), nil
}

func (s *RoleService) grantRole(ctx context.Context, body []byte) (any, error) {
	var ref csil.MemberRoleRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	role, err := s.Store.GetRoleByID(ctx, string(ref.RoleId))
	if err != nil {
		return nil, csilrpc.NotFound("role not found")
	}
	if _, _, err := requireRoleForHouse(ctx, role.HouseID, "admin"); err != nil {
		return nil, err
	}
	if err := s.Store.AssignRole(ctx, string(ref.MemberId), role.RoleID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *RoleService) revokeRole(ctx context.Context, body []byte) (any, error) {
	var ref csil.MemberRoleRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	role, err := s.Store.GetRoleByID(ctx, string(ref.RoleId))
	if err != nil {
		return nil, csilrpc.NotFound("role not found")
	}
	if _, _, err := requireRoleForHouse(ctx, role.HouseID, "admin"); err != nil {
		return nil, err
	}
	if err := s.Store.RevokeRole(ctx, string(ref.MemberId), role.RoleID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *RoleService) listMemberRoles(ctx context.Context, body []byte) (any, error) {
	var req csil.MemberScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListRolesForMember(ctx, string(req.MemberId))
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return rolesToCSIL(rows), nil
}
