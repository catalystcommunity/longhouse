package csilservices

import (
	"context"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	log "github.com/sirupsen/logrus"
)

// DevAuthService is the local-dev sign-in shortcut. ListDevUsers enumerates
// every member across every house so the SPA's /dev-login picker can show
// a list; DevLogin mints a real identity JWT for the picked member without
// the linkkeys assertion exchange.
//
// The service is registered ONLY when config.DevAuthAllowed() is true
// (LONGHOUSE_DEV_AUTH_ENABLED=true AND LONGHOUSE_ENV ∈ {dev, nonprod}).
// In production the entire service is absent from the dispatcher's
// registry, so /api/csil/devauth/* responds 404.
type DevAuthService struct {
	Auth *AuthService // reuses Store + JWTSecret + issueToken
	Env  string       // logged on every issuance for grep-ability
}

func (s *DevAuthService) Register(d *csilrpc.Dispatcher) {
	d.RegisterPublic("devauth", "ListDevUsers", s.listDevUsers)
	d.RegisterPublic("devauth", "DevLogin", s.devLogin)
}

func (s *DevAuthService) listDevUsers(ctx context.Context, _ []byte) (any, error) {
	houses, err := s.Auth.Store.ListHouses(ctx, 1000, 0)
	if err != nil {
		log.WithError(err).Error("devauth.ListDevUsers: list houses failed")
		return nil, csilrpc.Internal("internal error")
	}
	out := []csil.DevUserEntry{}
	for _, h := range houses {
		members, err := s.Auth.Store.ListMembersByHouse(ctx, h.HouseID, 1000, 0)
		if err != nil {
			log.WithError(err).WithField("house_id", h.HouseID).Error("devauth.ListDevUsers: list members failed")
			continue
		}
		for _, m := range members {
			roles, _ := s.Auth.Store.ListRolesForMember(ctx, m.MemberID)
			roleNames := make([]string, 0, len(roles))
			for _, role := range roles {
				roleNames = append(roleNames, role.Name)
			}
			out = append(out, csil.DevUserEntry{
				MemberId:       csil.MemberID(m.MemberID),
				HouseId:        csil.HouseID(m.HouseID),
				HouseName:      h.Name,
				DisplayName:    strPtrCopy(loadMemberDisplayName(&m)),
				LinkkeysDomain: strPtrCopy(m.LinkkeysDomain),
				LinkkeysUserId: strPtrCopy(m.LinkkeysUserID),
				Roles:          roleNames,
			})
		}
	}
	return csil.DevUsersResponse{Users: out}, nil
}

func (s *DevAuthService) devLogin(ctx context.Context, body []byte) (any, error) {
	if s.Auth.JWTSecret == nil {
		return nil, csilrpc.Internal("dev-auth not configured")
	}
	var req csil.DevLoginRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	if req.MemberId == "" {
		return nil, csilrpc.BadRequest("member_id is required")
	}
	member, err := s.Auth.Store.GetMemberByID(ctx, string(req.MemberId))
	if err != nil || member == nil {
		return nil, csilrpc.BadRequest("member not found")
	}
	resp, err := s.Auth.issueToken(ctx, member.LinkkeysDomain, member.LinkkeysUserID, member.DisplayName)
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{
		"env":              s.Env,
		"linkkeys_domain":  member.LinkkeysDomain,
		"linkkeys_user_id": member.LinkkeysUserID,
	}).Warn("DEV-AUTH: minted identity JWT without assertion verification")
	return resp, nil
}
