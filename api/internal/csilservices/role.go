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
	d.RegisterTyped("role", "CreateRole", csilrpc.Route(s.CreateRole, csil.DecodeRoleCreateRoleRequest, csil.EncodeRoleCreateRoleResponse))
	d.RegisterTyped("role", "UpdateRole", csilrpc.Route(s.UpdateRole, csil.DecodeRoleUpdateRoleRequest, csil.EncodeRoleUpdateRoleResponse))
	d.RegisterTyped("role", "DeleteRole", csilrpc.Route(s.DeleteRole, csil.DecodeRoleDeleteRoleRequest, csil.EncodeRoleDeleteRoleResponse))
	d.RegisterTyped("role", "ListRoles", csilrpc.Route(s.ListRoles, csil.DecodeRoleListRolesRequest, csil.EncodeRoleListRolesResponse))
	d.RegisterTyped("role", "GrantRole", csilrpc.Route(s.GrantRole, csil.DecodeRoleGrantRoleRequest, csil.EncodeRoleGrantRoleResponse))
	d.RegisterTyped("role", "RevokeRole", csilrpc.Route(s.RevokeRole, csil.DecodeRoleRevokeRoleRequest, csil.EncodeRoleRevokeRoleResponse))
	d.RegisterTyped("role", "ListMemberRoles", csilrpc.Route(s.ListMemberRoles, csil.DecodeRoleListMemberRolesRequest, csil.EncodeRoleListMemberRolesResponse))
}

func (s *RoleService) CreateRole(ctx context.Context, in csil.Role) (csil.Role, error) {
	if in.HouseId == "" || in.Name == "" {
		return csil.Role{}, csilrpc.BadRequest("house_id and name are required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(in.HouseId), "admin"); err != nil {
		return csil.Role{}, err
	}
	r := &models.Role{
		HouseID:     string(in.HouseId),
		Name:        in.Name,
		Description: derefStr(in.Description),
	}
	if err := s.Store.CreateRole(ctx, r); err != nil {
		return csil.Role{}, csilrpc.Internal("internal error")
	}
	return roleToCSIL(r), nil
}

func (s *RoleService) UpdateRole(ctx context.Context, in csil.Role) (csil.Role, error) {
	if in.RoleId == "" {
		return csil.Role{}, csilrpc.BadRequest("role_id is required")
	}
	existing, err := s.Store.GetRoleByID(ctx, string(in.RoleId))
	if err != nil {
		return csil.Role{}, csilrpc.NotFound("role not found")
	}
	if _, _, err := requireRoleForHouse(ctx, existing.HouseID, "admin"); err != nil {
		return csil.Role{}, err
	}
	if in.Name != "" {
		existing.Name = in.Name
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	if err := s.Store.UpdateRole(ctx, existing); err != nil {
		return csil.Role{}, csilrpc.Internal("internal error")
	}
	return roleToCSIL(existing), nil
}

// DeleteRole refuses to delete the canonical roles (admin, member) — the
// house seed relies on them, and admin-recovery becomes painful without
// them. Custom roles delete freely.
func (s *RoleService) DeleteRole(ctx context.Context, id csil.RoleID) (csil.EmptyResponse, error) {
	existing, err := s.Store.GetRoleByID(ctx, string(id))
	if err != nil || existing.DeletedAt != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("role not found")
	}
	_, callerMemberID, err := requireRoleForHouse(ctx, existing.HouseID, "admin")
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if existing.Name == models.RoleAdmin || existing.Name == models.RoleMember {
		return csil.EmptyResponse{}, csilrpc.Conflict("the admin and member roles are reserved and cannot be deleted")
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteRole(ctx, existing.RoleID, callerMemberID, opID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, existing.HouseID, "role", existing.RoleID, opID, existing)
	return csil.EmptyResponse{}, nil
}

func (s *RoleService) ListRoles(ctx context.Context, req csil.HouseScopedListRequest) ([]csil.Role, error) {
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

func (s *RoleService) GrantRole(ctx context.Context, ref csil.MemberRoleRef) (csil.EmptyResponse, error) {
	role, err := s.Store.GetRoleByID(ctx, string(ref.RoleId))
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("role not found")
	}
	if _, _, err := requireRoleForHouse(ctx, role.HouseID, "admin"); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.AssignRole(ctx, string(ref.MemberId), role.RoleID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *RoleService) RevokeRole(ctx context.Context, ref csil.MemberRoleRef) (csil.EmptyResponse, error) {
	role, err := s.Store.GetRoleByID(ctx, string(ref.RoleId))
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("role not found")
	}
	if _, _, err := requireRoleForHouse(ctx, role.HouseID, "admin"); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.RevokeRole(ctx, string(ref.MemberId), role.RoleID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *RoleService) ListMemberRoles(ctx context.Context, req csil.MemberScopedListRequest) ([]csil.Role, error) {
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListRolesForMember(ctx, string(req.MemberId))
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return rolesToCSIL(rows), nil
}
