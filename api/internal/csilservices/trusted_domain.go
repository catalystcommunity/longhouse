package csilservices

import (
	"context"
	"errors"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

// TrustedDomainService maintains the per-house trusted_domains list. Any
// verified linkkeys identity whose domain appears here is auto-provisioned
// into the house on sign-in (see auth.issueToken). Mutations are admin-
// only; reads are available to any house member so the SPA can render the
// list in Settings.
type TrustedDomainService struct{ Store store.Store }

func (s *TrustedDomainService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("trusteddomain", "AddTrustedDomain", csilrpc.Route(s.AddTrustedDomain, csil.DecodeTrustedDomainAddTrustedDomainRequest, csil.EncodeTrustedDomainAddTrustedDomainResponse))
	d.RegisterTyped("trusteddomain", "RemoveTrustedDomain", csilrpc.Route(s.RemoveTrustedDomain, csil.DecodeTrustedDomainRemoveTrustedDomainRequest, csil.EncodeTrustedDomainRemoveTrustedDomainResponse))
	d.RegisterTyped("trusteddomain", "ListTrustedDomains", csilrpc.Route(s.ListTrustedDomains, csil.DecodeTrustedDomainListTrustedDomainsRequest, csil.EncodeTrustedDomainListTrustedDomainsResponse))
	d.RegisterTyped("trusteddomain", "IsDomainTrusted", csilrpc.Route(s.IsDomainTrusted, csil.DecodeTrustedDomainIsDomainTrustedRequest, csil.EncodeTrustedDomainIsDomainTrustedResponse))
}

func (s *TrustedDomainService) AddTrustedDomain(ctx context.Context, in csil.TrustedDomain) (csil.TrustedDomain, error) {
	domain := strings.TrimSpace(strings.ToLower(in.Domain))
	if in.HouseId == "" || domain == "" {
		return csil.TrustedDomain{}, csilrpc.BadRequest("house_id and domain are required")
	}
	_, actorMemberID, err := requireRoleForHouse(ctx, string(in.HouseId), "admin")
	if err != nil {
		return csil.TrustedDomain{}, err
	}
	td := &models.TrustedDomain{
		HouseID: string(in.HouseId),
		Domain:  domain,
	}
	if err := s.Store.CreateTrustedDomain(ctx, td); err != nil {
		return csil.TrustedDomain{}, csilrpc.Internal("internal error")
	}
	// Append-only audit so admins can see who added what when reviewing
	// the member log later.
	_ = s.Store.RecordMemberAudit(ctx, &models.MemberAudit{
		HouseID:         string(in.HouseId),
		SubjectMemberID: actorMemberID,
		ActorMemberID:   &actorMemberID,
		Action:          models.AuditActionTrustedDomainAdded,
		TargetType:      strPtrCopy("trusted_domain"),
		TargetID:        &td.TrustedDomainID,
		Detail:          models.JSONMap{"domain": domain},
	})
	return trustedDomainToCSIL(td), nil
}

func (s *TrustedDomainService) RemoveTrustedDomain(ctx context.Context, id csil.TrustedDomainID) (csil.EmptyResponse, error) {
	existing, err := s.findByID(ctx, string(id))
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	_, actorMemberID, err := requireRoleForHouse(ctx, existing.HouseID, "admin")
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.DeleteTrustedDomain(ctx, existing.TrustedDomainID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	_ = s.Store.RecordMemberAudit(ctx, &models.MemberAudit{
		HouseID:         existing.HouseID,
		SubjectMemberID: actorMemberID,
		ActorMemberID:   &actorMemberID,
		Action:          models.AuditActionTrustedDomainRemoved,
		TargetType:      strPtrCopy("trusted_domain"),
		TargetID:        &existing.TrustedDomainID,
		Detail:          models.JSONMap{"domain": existing.Domain},
	})
	return csil.EmptyResponse{}, nil
}

func (s *TrustedDomainService) ListTrustedDomains(ctx context.Context, houseID csil.HouseID) ([]csil.TrustedDomain, error) {
	if _, _, err := requireMemberForHouse(ctx, string(houseID)); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListTrustedDomains(ctx, string(houseID))
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return trustedDomainsToCSIL(rows), nil
}

func (s *TrustedDomainService) IsDomainTrusted(ctx context.Context, in csil.TrustedDomain) (csil.BoolResponse, error) {
	if in.HouseId == "" || in.Domain == "" {
		return csil.BoolResponse{}, csilrpc.BadRequest("house_id and domain are required")
	}
	if _, _, err := requireMemberForHouse(ctx, string(in.HouseId)); err != nil {
		return csil.BoolResponse{}, err
	}
	ok, err := s.Store.IsDomainTrusted(ctx, string(in.HouseId), strings.ToLower(in.Domain))
	if err != nil {
		return csil.BoolResponse{}, csilrpc.Internal("internal error")
	}
	return csil.BoolResponse{Value: ok}, nil
}

// findByID locates a trusted-domain row by its id. The Store interface
// doesn't expose a direct GetTrustedDomainByID, but a row's id is unique
// across the table — we look it up by scanning the house's list.
// (Cheap: a house has at most a handful of trusted domains.)
func (s *TrustedDomainService) findByID(ctx context.Context, tdID string) (*models.TrustedDomain, error) {
	// Cross-house lookup: enumerate via raw db not exposed; instead use
	// the (admin) reasoning that the SPA already knows the row's house
	// when removing it. As a fallback for direct API calls, walk every
	// house the caller belongs to. The dispatcher's authz still runs
	// against the row's actual house, so this can't widen access.
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	for _, hr := range id.Houses {
		rows, lerr := s.Store.ListTrustedDomains(ctx, hr.House)
		if lerr != nil {
			continue
		}
		for i := range rows {
			if rows[i].TrustedDomainID == tdID {
				return &rows[i], nil
			}
		}
	}
	// Match the gorm-not-found convention for callers that errors.Is it.
	return nil, errors.Join(gorm.ErrRecordNotFound, csilrpc.NotFound("trusted domain not found"))
}
