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
	d.RegisterTyped("member", "ListMembers", csilrpc.Route(s.ListMembers, csil.DecodeMemberListMembersRequest, csil.EncodeMemberListMembersResponse))
	d.RegisterTyped("member", "GetMember", csilrpc.Route(s.GetMember, csil.DecodeMemberGetMemberRequest, csil.EncodeMemberGetMemberResponse))
	d.RegisterTyped("member", "CreateMember", csilrpc.Route(s.CreateMember, csil.DecodeMemberCreateMemberRequest, csil.EncodeMemberCreateMemberResponse))
	d.RegisterTyped("member", "UpdateMember", csilrpc.Route(s.UpdateMember, csil.DecodeMemberUpdateMemberRequest, csil.EncodeMemberUpdateMemberResponse))
	d.RegisterTyped("member", "DeactivateMember", csilrpc.Route(s.DeactivateMember, csil.DecodeMemberDeactivateMemberRequest, csil.EncodeMemberDeactivateMemberResponse))
	d.RegisterTyped("member", "ReactivateMember", csilrpc.Route(s.ReactivateMember, csil.DecodeMemberReactivateMemberRequest, csil.EncodeMemberReactivateMemberResponse))
}

func (s *MemberService) ListMembers(ctx context.Context, req csil.HouseScopedListRequest) ([]csil.Member, error) {
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

func (s *MemberService) GetMember(ctx context.Context, id csil.MemberID) (csil.Member, error) {
	m, err := s.Store.GetMemberByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return csil.Member{}, csilrpc.NotFound("member not found")
		}
		return csil.Member{}, csilrpc.Internal("internal error")
	}
	if _, _, err := requireMemberForHouse(ctx, m.HouseID); err != nil {
		return csil.Member{}, err
	}
	return memberToCSIL(m), nil
}

// CreateMember invites a new member by linkkeys identity into the named
// house. Admin-only. The new member starts with the canonical "member"
// role only — grant additional roles via RoleService.GrantRole. If a row
// for that (house, identity) already exists we 409 — the SPA can flip to
// an "already a member, here they are" message.
func (s *MemberService) CreateMember(ctx context.Context, in csil.Member) (csil.Member, error) {
	if in.HouseId == "" || in.LinkkeysDomain == "" || in.LinkkeysUserId == "" {
		return csil.Member{}, csilrpc.BadRequest("house_id, linkkeys_domain, and linkkeys_user_id are required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(in.HouseId), "admin"); err != nil {
		return csil.Member{}, err
	}
	existing, _ := s.Store.GetMemberByIdentity(ctx, string(in.HouseId), in.LinkkeysDomain, in.LinkkeysUserId)
	if existing != nil {
		return csil.Member{}, csilrpc.Conflict("a member with that linkkeys identity already exists in this house")
	}
	m := &models.Member{
		HouseID:        string(in.HouseId),
		LinkkeysDomain: in.LinkkeysDomain,
		LinkkeysUserID: in.LinkkeysUserId,
		DisplayName:    derefStr(in.DisplayName),
	}
	if err := s.Store.CreateMember(ctx, m); err != nil {
		return csil.Member{}, csilrpc.Internal("internal error")
	}
	// Auto-grant the "member" role so the invited person can immediately
	// list resources in this house on their next bearer refresh.
	if memberRole, err := s.Store.GetRoleByName(ctx, string(in.HouseId), models.RoleMember); err == nil && memberRole != nil {
		_ = s.Store.AssignRole(ctx, m.MemberID, memberRole.RoleID)
	}
	return memberToCSIL(m), nil
}

// DeactivateMember denies a member future access to the house (keeping their
// record + owned content). Admin-only; idempotent.
func (s *MemberService) DeactivateMember(ctx context.Context, id csil.MemberID) (csil.EmptyResponse, error) {
	existing, err := s.Store.GetMemberByID(ctx, string(id))
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("member not found")
	}
	_, callerMemberID, err := requireRoleForHouse(ctx, existing.HouseID, "admin")
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.DeactivateMember(ctx, existing.MemberID, callerMemberID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	annotateAudit(ctx, existing.HouseID, "deactivate", "member", existing.MemberID, nil)
	return csil.EmptyResponse{}, nil
}

// ReactivateMember restores a deactivated member's access. Admin-only.
func (s *MemberService) ReactivateMember(ctx context.Context, id csil.MemberID) (csil.EmptyResponse, error) {
	existing, err := s.Store.GetMemberByID(ctx, string(id))
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("member not found")
	}
	if _, _, err := requireRoleForHouse(ctx, existing.HouseID, "admin"); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.ReactivateMember(ctx, existing.MemberID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	annotateAudit(ctx, existing.HouseID, "reactivate", "member", existing.MemberID, nil)
	return csil.EmptyResponse{}, nil
}

func (s *MemberService) UpdateMember(ctx context.Context, in csil.Member) (csil.Member, error) {
	if in.MemberId == "" {
		return csil.Member{}, csilrpc.BadRequest("member_id is required")
	}
	existing, err := s.Store.GetMemberByID(ctx, string(in.MemberId))
	if err != nil {
		return csil.Member{}, csilrpc.NotFound("member not found")
	}
	// Self-or-admin: a caller may update their own row; admins of the house
	// may update any row in it.
	id, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return csil.Member{}, err
	}
	if callerMemberID != existing.MemberID {
		if _, err := requireRole(id, existing.HouseID, "admin"); err != nil {
			return csil.Member{}, csilrpc.Forbidden("you may only update your own profile")
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
		return csil.Member{}, csilrpc.Internal("internal error")
	}
	return memberToCSIL(existing), nil
}
