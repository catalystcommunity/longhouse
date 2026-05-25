package csilservices

import (
	"context"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// SkillService handles the skills catalog plus the two attachment surfaces
// (member skills + group skills). Admin-only to create/edit/delete a
// skill or attach it to a group; a member can attach a skill to
// themselves (and admins can attach a skill to anyone).
type SkillService struct{ Store store.Store }

func (s *SkillService) Register(d *csilrpc.Dispatcher) {
	d.Register("skill", "CreateSkill", s.createSkill)
	d.Register("skill", "UpdateSkill", s.updateSkill)
	d.Register("skill", "DeleteSkill", s.deleteSkill)
	d.Register("skill", "ListSkills", s.listSkills)
	d.Register("skill", "AddMemberSkill", s.addMemberSkill)
	d.Register("skill", "RemoveMemberSkill", s.removeMemberSkill)
	d.Register("skill", "ListMemberSkills", s.listMemberSkills)
	d.Register("skill", "AddGroupSkill", s.addGroupSkill)
	d.Register("skill", "RemoveGroupSkill", s.removeGroupSkill)
	d.Register("skill", "ListGroupSkills", s.listGroupSkills)
}

func (s *SkillService) createSkill(ctx context.Context, body []byte) (any, error) {
	var in csil.Skill
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.HouseId == "" || in.Name == "" {
		return nil, csilrpc.BadRequest("house_id and name are required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(in.HouseId), "admin"); err != nil {
		return nil, err
	}
	sk := &models.Skill{
		HouseID:     string(in.HouseId),
		Name:        in.Name,
		Description: derefStr(in.Description),
	}
	if err := s.Store.CreateSkill(ctx, sk); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return skillToCSIL(sk), nil
}

func (s *SkillService) updateSkill(ctx context.Context, body []byte) (any, error) {
	var in csil.Skill
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.SkillId == "" {
		return nil, csilrpc.BadRequest("skill_id is required")
	}
	existing, err := s.Store.GetSkillByID(ctx, string(in.SkillId))
	if err != nil {
		return nil, csilrpc.NotFound("skill not found")
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
	if err := s.Store.UpdateSkill(ctx, existing); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return skillToCSIL(existing), nil
}

func (s *SkillService) deleteSkill(ctx context.Context, body []byte) (any, error) {
	var id csil.SkillID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetSkillByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("skill not found")
	}
	if _, _, err := requireRoleForHouse(ctx, existing.HouseID, "admin"); err != nil {
		return nil, err
	}
	if err := s.Store.DeleteSkill(ctx, existing.SkillID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) listSkills(ctx context.Context, body []byte) (any, error) {
	var req csil.HouseScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	rows, err := s.Store.ListSkillsByHouse(ctx, string(req.HouseId), limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return skillsToCSIL(rows), nil
}

func (s *SkillService) addMemberSkill(ctx context.Context, body []byte) (any, error) {
	var ref csil.MemberSkillRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	if err := s.memberMutationAuthz(ctx, string(ref.MemberId)); err != nil {
		return nil, err
	}
	if err := s.Store.AssignSkill(ctx, string(ref.MemberId), string(ref.SkillId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) removeMemberSkill(ctx context.Context, body []byte) (any, error) {
	var ref csil.MemberSkillRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	if err := s.memberMutationAuthz(ctx, string(ref.MemberId)); err != nil {
		return nil, err
	}
	if err := s.Store.UnassignSkill(ctx, string(ref.MemberId), string(ref.SkillId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) listMemberSkills(ctx context.Context, body []byte) (any, error) {
	var req csil.MemberScopedListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListSkillsForMember(ctx, string(req.MemberId))
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return skillsToCSIL(rows), nil
}

func (s *SkillService) addGroupSkill(ctx context.Context, body []byte) (any, error) {
	var ref csil.GroupSkillRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	if err := s.groupMutationAuthz(ctx, string(ref.GroupId)); err != nil {
		return nil, err
	}
	if err := s.Store.AssignGroupSkill(ctx, string(ref.GroupId), string(ref.SkillId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) removeGroupSkill(ctx context.Context, body []byte) (any, error) {
	var ref csil.GroupSkillRef
	if err := csilrpc.Decode(body, &ref); err != nil {
		return nil, err
	}
	if err := s.groupMutationAuthz(ctx, string(ref.GroupId)); err != nil {
		return nil, err
	}
	if err := s.Store.UnassignGroupSkill(ctx, string(ref.GroupId), string(ref.SkillId)); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) listGroupSkills(ctx context.Context, body []byte) (any, error) {
	var id csil.GroupID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	g, err := s.Store.GetGroupByID(ctx, string(id))
	if err != nil {
		return nil, csilrpc.NotFound("group not found")
	}
	if _, _, err := requireMemberForHouse(ctx, g.HouseID); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListSkillsForGroup(ctx, g.GroupID)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return skillsToCSIL(rows), nil
}

// memberMutationAuthz lets a caller mutate their own skills, or any
// admin-of-the-house caller mutate anyone's.
func (s *SkillService) memberMutationAuthz(ctx context.Context, targetMemberID string) error {
	target, err := s.Store.GetMemberByID(ctx, targetMemberID)
	if err != nil {
		return csilrpc.NotFound("member not found")
	}
	id, callerMemberID, err := requireMemberForHouse(ctx, target.HouseID)
	if err != nil {
		return err
	}
	if callerMemberID == target.MemberID {
		return nil
	}
	if _, err := requireRole(id, target.HouseID, "admin"); err != nil {
		return csilrpc.Forbidden("only the member or a house admin may change a member's skills")
	}
	return nil
}

// groupMutationAuthz: admin-only on the group's house.
func (s *SkillService) groupMutationAuthz(ctx context.Context, groupID string) error {
	g, err := s.Store.GetGroupByID(ctx, groupID)
	if err != nil {
		return csilrpc.NotFound("group not found")
	}
	if _, _, err := requireRoleForHouse(ctx, g.HouseID, "admin"); err != nil {
		return err
	}
	return nil
}
