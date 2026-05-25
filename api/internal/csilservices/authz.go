package csilservices

import (
	"context"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
)

// Authorization helpers used by every service. These check the caller's
// identity (set by the dispatcher) against the resource's house, then
// against any required role names.

// requireIdentity is the single line every authenticated handler runs first.
// Returns the verified identity from context or an Unauthorized error if
// the dispatcher didn't attach one (which would mean a public registration
// was used by mistake — fail closed).
func requireIdentity(ctx context.Context) (*auth.Identity, error) {
	id := auth.IdentityFromContext(ctx)
	if id == nil {
		return nil, csilrpc.Unauthorized("missing identity")
	}
	return id, nil
}

// requireMember checks the caller is a member of houseID with any role.
// Returns the member id (in that house) for convenience — handlers that
// need to write rows tagged with the caller's member id can use it
// without a second store call.
func requireMember(id *auth.Identity, houseID string) (memberID string, err error) {
	for _, hr := range id.Houses {
		if hr.House == houseID {
			return hr.Member, nil
		}
	}
	return "", csilrpc.Forbidden("not a member of this house")
}

// requireRole is requireMember + "and they hold at least one of these
// roles". Passing zero role names is identical to requireMember.
func requireRole(id *auth.Identity, houseID string, anyOf ...string) (memberID string, err error) {
	for _, hr := range id.Houses {
		if hr.House != houseID {
			continue
		}
		if len(anyOf) == 0 {
			return hr.Member, nil
		}
		for _, want := range anyOf {
			for _, has := range hr.Roles {
				if has == want {
					return hr.Member, nil
				}
			}
		}
		return "", csilrpc.Forbidden("missing required role: " + strings.Join(anyOf, " or "))
	}
	return "", csilrpc.Forbidden("not a member of this house")
}

// requireMemberForHouse is the (ctx → identity → house) one-liner most
// read handlers want. Splits identity lookup + membership check into one
// step that returns the (identity, memberID, error) triple.
func requireMemberForHouse(ctx context.Context, houseID string) (*auth.Identity, string, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, "", err
	}
	memberID, err := requireMember(id, houseID)
	if err != nil {
		return nil, "", err
	}
	return id, memberID, nil
}

// houseOfResource is a small helper used by services whose method signatures
// don't carry house_id (e.g. GetTask(TaskID), UpdateProject(Project)). It
// fetches the resource's house indirectly so the authz check can run.
//
// Implemented as a thin wrapper over the store with explicit not-found
// handling — the caller-visible error is "not found" so we don't leak the
// existence of a resource the caller has no right to see.
type resourceLoader func(ctx context.Context, store store.Store) (houseID string, err error)

func authzForResource(ctx context.Context, st store.Store, load resourceLoader, anyOf ...string) (*auth.Identity, string, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, "", err
	}
	houseID, err := load(ctx, st)
	if err != nil {
		return nil, "", csilrpc.NotFound("not found")
	}
	memberID, err := requireRole(id, houseID, anyOf...)
	if err != nil {
		return nil, "", err
	}
	return id, memberID, nil
}

// normalizePaging clamps and defaults the (limit, offset) pair from
// HouseScopedListRequest / similar. Limits default to 50 and are capped at
// 500 so a single call can't drag the entire table.
func normalizePaging(limit, offset *uint64) (int, int) {
	l := 50
	if limit != nil {
		l = int(*limit)
	}
	if l <= 0 {
		l = 50
	}
	if l > 500 {
		l = 500
	}
	o := 0
	if offset != nil && *offset > 0 {
		o = int(*offset)
	}
	return l, o
}
