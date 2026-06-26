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

// MemberService exposes house-scoped member operations. Authorization
// shifts from the URL (which used to carry house_id) into each method:
// list-by-house uses the request's HouseId; the single-id methods load
// the member, read its house, and check the caller is a member there.
type MemberService struct{ Store store.Store }

func (s *MemberService) Register(d *csilrpc.Dispatcher) {
	d.Register("member", "ListMembers", s.listMembers)
	d.Register("member", "GetMember", s.getMember)
	d.Register("member", "CreateMember", s.createMember)
	d.Register("member", "UpdateMember", s.updateMember)
	d.Register("member", "DeactivateMember", s.deactivateMember)
	d.Register("member", "ReactivateMember", s.reactivateMember)
}

func (s *MemberService) listMembers(ctx context.Context, body []byte) (any, error) {
	var req csil.HouseScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	rows, err := s.Store.ListMembersByHouse(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return membersToCSIL(rows), nil
}

func (s *MemberService) getMember(ctx context.Context, body []byte) (any, error) {
	var id csil.MemberID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	m, err := s.Store.GetMemberByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, csilrpc.NotFound("member not found")
		}
		return nil, csilrpc.Internal("internal error")
	}
	if _, _, err := requireMemberForHouse(ctx, m.HouseID); err != nil {
		return nil, err
	}
	return memberToCSIL(m), nil
}

// createMember invites a new member by linkkeys identity into the named
// house. Admin-only. The new member starts with the canonical "member"
// role only — grant additional roles via RoleService.GrantRole. If a row
// for that (house, identity) already exists we 409 — the SPA can flip to
// an "already a member, here they are" message.
func (s *MemberService) createMember(ctx context.Context, body []byte) (any, error) {
	var in csil.Member
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.HouseId == "" || in.LinkkeysDomain == "" || in.LinkkeysUserId == "" {
		return nil, csilrpc.BadRequest("house_id, linkkeys_domain, and linkkeys_user_id are required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(in.HouseId), "admin"); err != nil {
		return nil, err
	}
	existing, _ := s.Store.GetMemberByIdentity(ctx, string(in.HouseId), in.LinkkeysDomain, in.LinkkeysUserId)
	if existing != nil {
		return nil, csilrpc.Conflict("a member with that linkkeys identity already exists in this house")
	}
	m := &models.Member{
		HouseID:        string(in.HouseId),
		LinkkeysDomain: in.LinkkeysDomain,
		LinkkeysUserID: in.LinkkeysUserId,
		DisplayName:    derefStr(in.DisplayName),
	}
	if err := s.Store.CreateMember(ctx, m); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	// Auto-grant the "member" role so the invited person can immediately
	// list resources in this house on their next bearer refresh.
	if memberRole, err := s.Store.GetRoleByName(ctx, string(in.HouseId), models.RoleMember); err == nil && memberRole != nil {
		_ = s.Store.AssignRole(ctx, m.MemberID, memberRole.RoleID)
	}
	return memberToCSIL(m), nil
}

// deactivateMember denies a member future access to the house (keeping their
// record + owned content). Admin-only; idempotent.
func (s *MemberService) deactivateMember(ctx context.Context, body []byte) (any, error) {
	var id csil.MemberID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetMemberByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("member not found")
	}
	_, callerMemberID, err := requireRoleForHouse(ctx, existing.HouseID, "admin")
	if err != nil {
		return nil, err
	}
	if err := s.Store.DeactivateMember(ctx, existing.MemberID, callerMemberID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	annotateAudit(ctx, existing.HouseID, "deactivate", "member", existing.MemberID, nil)
	return csil.EmptyResponse{}, nil
}

// reactivateMember restores a deactivated member's access. Admin-only.
func (s *MemberService) reactivateMember(ctx context.Context, body []byte) (any, error) {
	var id csil.MemberID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetMemberByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("member not found")
	}
	if _, _, err := requireRoleForHouse(ctx, existing.HouseID, "admin"); err != nil {
		return nil, err
	}
	if err := s.Store.ReactivateMember(ctx, existing.MemberID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	annotateAudit(ctx, existing.HouseID, "reactivate", "member", existing.MemberID, nil)
	return csil.EmptyResponse{}, nil
}

func (s *MemberService) updateMember(ctx context.Context, body []byte) (any, error) {
	var in csil.Member
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.MemberId == "" {
		return nil, csilrpc.BadRequest("member_id is required")
	}
	existing, err := s.Store.GetMemberByID(ctx, string(in.MemberId))
	if err != nil {
		return nil, csilrpc.NotFound("member not found")
	}
	// Self-or-admin: a caller may update their own row; admins of the house
	// may update any row in it.
	id, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	if callerMemberID != existing.MemberID {
		if _, err := requireRole(id, existing.HouseID, "admin"); err != nil {
			return nil, csilrpc.Forbidden("you may only update your own profile")
		}
	}
	// Keep the mutable field list narrow on purpose so we don't accept silly
	// things like client-supplied created_at via a generic merge. email is
	// deliberately absent: it's receive-only (verified-claim territory), so a
	// client can't set it here even though the type carries it.
	if in.DisplayName != nil {
		existing.DisplayName = *in.DisplayName
	}
	if in.AvatarUrl != nil {
		existing.AvatarURL = *in.AvatarUrl
	}
	if err := s.Store.UpdateMember(ctx, existing); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return memberToCSIL(existing), nil
}
