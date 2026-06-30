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
	d.RegisterTyped("skill", "CreateSkill", csilrpc.Route(s.CreateSkill, csil.DecodeSkillCreateSkillRequest, csil.EncodeSkillCreateSkillResponse))
	d.RegisterTyped("skill", "UpdateSkill", csilrpc.Route(s.UpdateSkill, csil.DecodeSkillUpdateSkillRequest, csil.EncodeSkillUpdateSkillResponse))
	d.RegisterTyped("skill", "DeleteSkill", csilrpc.Route(s.DeleteSkill, csil.DecodeSkillDeleteSkillRequest, csil.EncodeSkillDeleteSkillResponse))
	d.RegisterTyped("skill", "ListSkills", csilrpc.Route(s.ListSkills, csil.DecodeSkillListSkillsRequest, csil.EncodeSkillListSkillsResponse))
	d.RegisterTyped("skill", "AddMemberSkill", csilrpc.Route(s.AddMemberSkill, csil.DecodeSkillAddMemberSkillRequest, csil.EncodeSkillAddMemberSkillResponse))
	d.RegisterTyped("skill", "RemoveMemberSkill", csilrpc.Route(s.RemoveMemberSkill, csil.DecodeSkillRemoveMemberSkillRequest, csil.EncodeSkillRemoveMemberSkillResponse))
	d.RegisterTyped("skill", "ListMemberSkills", csilrpc.Route(s.ListMemberSkills, csil.DecodeSkillListMemberSkillsRequest, csil.EncodeSkillListMemberSkillsResponse))
	d.RegisterTyped("skill", "AddGroupSkill", csilrpc.Route(s.AddGroupSkill, csil.DecodeSkillAddGroupSkillRequest, csil.EncodeSkillAddGroupSkillResponse))
	d.RegisterTyped("skill", "RemoveGroupSkill", csilrpc.Route(s.RemoveGroupSkill, csil.DecodeSkillRemoveGroupSkillRequest, csil.EncodeSkillRemoveGroupSkillResponse))
	d.RegisterTyped("skill", "ListGroupSkills", csilrpc.Route(s.ListGroupSkills, csil.DecodeSkillListGroupSkillsRequest, csil.EncodeSkillListGroupSkillsResponse))
}

func (s *SkillService) CreateSkill(ctx context.Context, in csil.Skill) (csil.Skill, error) {
	if in.HouseId == "" || in.Name == "" {
		return csil.Skill{}, csilrpc.BadRequest("house_id and name are required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(in.HouseId), "admin"); err != nil {
		return csil.Skill{}, err
	}
	sk := &models.Skill{
		HouseID:     string(in.HouseId),
		Name:        in.Name,
		Description: derefStr(in.Description),
	}
	if err := s.Store.CreateSkill(ctx, sk); err != nil {
		return csil.Skill{}, csilrpc.Internal("internal error")
	}
	return skillToCSIL(sk), nil
}

func (s *SkillService) UpdateSkill(ctx context.Context, in csil.Skill) (csil.Skill, error) {
	if in.SkillId == "" {
		return csil.Skill{}, csilrpc.BadRequest("skill_id is required")
	}
	existing, err := s.Store.GetSkillByID(ctx, string(in.SkillId))
	if err != nil {
		return csil.Skill{}, csilrpc.NotFound("skill not found")
	}
	if _, _, err := requireRoleForHouse(ctx, existing.HouseID, "admin"); err != nil {
		return csil.Skill{}, err
	}
	if in.Name != "" {
		existing.Name = in.Name
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	if err := s.Store.UpdateSkill(ctx, existing); err != nil {
		return csil.Skill{}, csilrpc.Internal("internal error")
	}
	return skillToCSIL(existing), nil
}

func (s *SkillService) DeleteSkill(ctx context.Context, id csil.SkillID) (csil.EmptyResponse, error) {
	existing, err := s.Store.GetSkillByID(ctx, string(id))
	if err != nil || existing.DeletedAt != nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("skill not found")
	}
	_, callerMemberID, err := requireRoleForHouse(ctx, existing.HouseID, "admin")
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteSkill(ctx, existing.SkillID, callerMemberID, opID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, existing.HouseID, "skill", existing.SkillID, opID, existing)
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) ListSkills(ctx context.Context, req csil.HouseScopedListRequest) ([]csil.Skill, error) {
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

func (s *SkillService) AddMemberSkill(ctx context.Context, ref csil.MemberSkillRef) (csil.EmptyResponse, error) {
	if err := s.memberMutationAuthz(ctx, string(ref.MemberId)); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.AssignSkill(ctx, string(ref.MemberId), string(ref.SkillId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) RemoveMemberSkill(ctx context.Context, ref csil.MemberSkillRef) (csil.EmptyResponse, error) {
	if err := s.memberMutationAuthz(ctx, string(ref.MemberId)); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.UnassignSkill(ctx, string(ref.MemberId), string(ref.SkillId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) ListMemberSkills(ctx context.Context, req csil.MemberScopedListRequest) ([]csil.Skill, error) {
	if _, _, err := requireMemberForHouse(ctx, string(req.HouseId)); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListSkillsForMember(ctx, string(req.MemberId))
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return skillsToCSIL(rows), nil
}

func (s *SkillService) AddGroupSkill(ctx context.Context, ref csil.GroupSkillRef) (csil.EmptyResponse, error) {
	if err := s.groupMutationAuthz(ctx, string(ref.GroupId)); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.AssignGroupSkill(ctx, string(ref.GroupId), string(ref.SkillId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) RemoveGroupSkill(ctx context.Context, ref csil.GroupSkillRef) (csil.EmptyResponse, error) {
	if err := s.groupMutationAuthz(ctx, string(ref.GroupId)); err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.UnassignGroupSkill(ctx, string(ref.GroupId), string(ref.SkillId)); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

func (s *SkillService) ListGroupSkills(ctx context.Context, id csil.GroupID) ([]csil.Skill, error) {
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
