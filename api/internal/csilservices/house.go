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

// HouseService is the small surface for house creation, rename, and
// (rarely) delete. CreateHouse seeds the canonical "admin" and "member"
// roles on the new house and registers the caller as an admin member, so
// the caller can immediately do anything in their new house once they
// refresh their bearer.
//
// ListHouses is intentionally NOT restricted: returning every house in
// the DB is a footgun (and not what /me wants). We expose it for the rare
// "list houses I belong to" UI by filtering server-side to the caller's
// identity.
type HouseService struct{ Store store.Store }

func (s *HouseService) Register(d *csilrpc.Dispatcher) {
	d.Register("house", "CreateHouse", s.createHouse)
	d.Register("house", "GetHouse", s.getHouse)
	d.Register("house", "UpdateHouse", s.updateHouse)
	d.Register("house", "DeleteHouse", s.deleteHouse)
	d.Register("house", "ListHouses", s.listHouses)
}

func (s *HouseService) createHouse(ctx context.Context, body []byte) (any, error) {
	var in csil.House
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if in.Name == "" {
		return nil, csilrpc.BadRequest("name is required")
	}
	h := &models.House{
		Name:        in.Name,
		Description: derefStr(in.Description),
	}
	if err := s.Store.CreateHouse(ctx, h); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	// Seed the canonical roles + register the caller as admin so the new
	// house is immediately usable on next token refresh. Failures here
	// leave the house row dangling without an admin — surface that as a
	// 500; the caller can retry / clean up manually.
	adminRole := &models.Role{HouseID: h.HouseID, Name: models.RoleAdmin, Description: "Full administrative access"}
	if err := s.Store.CreateRole(ctx, adminRole); err != nil {
		return nil, csilrpc.Internal("could not create admin role")
	}
	memberRole := &models.Role{HouseID: h.HouseID, Name: models.RoleMember, Description: "Standard member"}
	if err := s.Store.CreateRole(ctx, memberRole); err != nil {
		return nil, csilrpc.Internal("could not create member role")
	}
	member := &models.Member{
		HouseID:        h.HouseID,
		LinkkeysDomain: id.Domain,
		LinkkeysUserID: id.UserID,
		DisplayName:    id.DisplayName,
	}
	if err := s.Store.CreateMember(ctx, member); err != nil {
		return nil, csilrpc.Internal("could not create founder member")
	}
	for _, r := range []*models.Role{adminRole, memberRole} {
		if err := s.Store.AssignRole(ctx, member.MemberID, r.RoleID); err != nil {
			return nil, csilrpc.Internal("could not assign founder role")
		}
	}
	return houseToCSIL(h), nil
}

func (s *HouseService) getHouse(ctx context.Context, body []byte) (any, error) {
	var id csil.HouseID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	h, err := s.Store.GetHouseByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, csilrpc.NotFound("house not found")
		}
		return nil, csilrpc.Internal("internal error")
	}
	if _, _, err := requireMemberForHouse(ctx, h.HouseID); err != nil {
		return nil, err
	}
	return houseToCSIL(h), nil
}

func (s *HouseService) updateHouse(ctx context.Context, body []byte) (any, error) {
	var in csil.House
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.HouseId == "" {
		return nil, csilrpc.BadRequest("house_id is required")
	}
	existing, err := s.Store.GetHouseByID(ctx, string(in.HouseId))
	if err != nil {
		return nil, csilrpc.NotFound("house not found")
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
	if err := s.Store.UpdateHouse(ctx, existing); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return houseToCSIL(existing), nil
}

func (s *HouseService) deleteHouse(ctx context.Context, body []byte) (any, error) {
	var id csil.HouseID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetHouseByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("house not found")
	}
	if _, _, err := requireRoleForHouse(ctx, existing.HouseID, "admin"); err != nil {
		return nil, err
	}
	if err := s.Store.DeleteHouse(ctx, existing.HouseID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// listHouses returns the houses the caller belongs to. The CSIL signature
// takes HouseListRequest{limit, offset} but we ignore those here — the
// identity's houses list is small (single-digit in practice).
func (s *HouseService) listHouses(ctx context.Context, body []byte) (any, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]csil.House, 0, len(id.Houses))
	for _, h := range id.Houses {
		row, err := s.Store.GetHouseByID(ctx, h.House)
		if err != nil || row == nil {
			continue
		}
		out = append(out, houseToCSIL(row))
	}
	return out, nil
}
