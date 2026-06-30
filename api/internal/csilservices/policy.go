package csilservices

import (
	"context"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// policy resolves effective resource access per docs/rbac.md. The model:
//
//   - Effective access is the MAX over every SURFACE that reaches the caller:
//     the resource's own house-at-large visibility (caller is a house
//     member), each project the task belongs to (directly or via an ancestor
//     task), explicit grants (the caller + the caller's groups), and
//     owner/admin (always full).
//   - The umbrella guardrail is a WRITE-time rule (a resource's own
//     visibility can't exceed the MIN of its containers); maxAllowed* below
//     computes that ceiling for the set-visibility handlers.
//
// First cut: resolution does per-resource lookups (no materialized path).
// At house scale this is fine; see docs/rbac.md §8 for the scaling plan.
type policy struct {
	store store.Store
}

func newPolicy(st store.Store) *policy { return &policy{store: st} }

// grantee identifies the caller for grant matching: their member id plus the
// ids of every group they belong to, all within one house.
type grantee struct {
	memberID string
	groupIDs map[string]bool
	isAdmin  bool
}

// granteeFor builds the grant-matching identity for the caller in a house.
// Group membership isn't in the bearer token (it carries house + roles
// only), so we look it up once here.
func (p *policy) granteeFor(ctx context.Context, id *auth.Identity, houseID, memberID string) grantee {
	g := grantee{memberID: memberID, groupIDs: map[string]bool{}}
	if hr := id.House(houseID); hr != nil {
		for _, r := range hr.Roles {
			if r == "admin" {
				g.isAdmin = true
			}
		}
	}
	if gids, err := p.store.ListGroupIDsForMember(ctx, memberID); err == nil {
		for _, gid := range gids {
			g.groupIDs[gid] = true
		}
	}
	return g
}

// maxGrant returns the highest access level any of the given grants confers
// on the grantee (matching their member id or one of their groups).
func maxGrantForProject(grants []models.ProjectGrant, g grantee) string {
	level := models.AccessNone
	for _, gr := range grants {
		if grantMatches(gr.GranteeType, gr.GranteeID, g) {
			level = models.MaxAccess(level, gr.AccessLevel)
		}
	}
	return level
}

func maxGrantForTask(grants []models.TaskGrant, g grantee) string {
	level := models.AccessNone
	for _, gr := range grants {
		if grantMatches(gr.GranteeType, gr.GranteeID, g) {
			level = models.MaxAccess(level, gr.AccessLevel)
		}
	}
	return level
}

func grantMatches(granteeType, granteeID string, g grantee) bool {
	switch granteeType {
	case models.GranteeMember:
		return granteeID == g.memberID
	case models.GranteeGroup:
		return g.groupIDs[granteeID]
	}
	return false
}

// projectAccess resolves the caller's effective access to a single project:
// MAX(house-at-large visibility, grants, owner/creator-fallback, admin).
func (p *policy) projectAccess(ctx context.Context, proj *models.Project, g grantee) string {
	if g.isAdmin {
		return models.AccessFull
	}
	level := models.AccessNone

	grants, _ := p.store.ListProjectGrants(ctx, proj.ProjectID)

	// Owner = a member grantee at full. Creator falls back to owner when no
	// owner grants exist.
	hasOwner := false
	for _, gr := range grants {
		if gr.GranteeType == models.GranteeMember && gr.AccessLevel == models.AccessFull {
			hasOwner = true
			break
		}
	}
	if !hasOwner && proj.CreatedByMemberID != nil && *proj.CreatedByMemberID == g.memberID {
		return models.AccessFull
	}

	// Surface 1: house-at-large (the caller is a house member by the time we
	// resolve, so they always get the project's own visibility).
	level = models.MaxAccess(level, visibilityOf(proj.Visibility))

	// Surface 3: explicit grants (member + groups).
	level = models.MaxAccess(level, maxGrantForProject(grants, g))

	return level
}

// taskAccess resolves the caller's effective access to a task: MAX over the
// task's own visibility, every project containing the task or an ancestor
// task, grants along the ancestor chain, and owner/admin.
func (p *policy) taskAccess(ctx context.Context, task *models.Task, g grantee) string {
	if g.isAdmin {
		return models.AccessFull
	}
	if task.OwnerMemberID == g.memberID {
		return models.AccessFull
	}
	level := models.AccessNone

	// The ancestor chain (nearest-first) plus the task itself drive every
	// surface.
	chain := []models.Task{*task}
	if ancestors, err := p.store.GetTaskAncestors(ctx, task.TaskID); err == nil {
		chain = append(chain, ancestors...)
	}

	// Surface 1 — house-at-large — is the task's own visibility, but CAPPED by
	// the umbrella: the MIN visibility across every container (ancestor tasks
	// AND containing projects). This is what makes "a task in a private
	// project is private to the house" hold even if the task's own visibility
	// was never explicitly lowered. Project membership is still additive
	// below, so a member of a containing project keeps their access.
	own := visibilityOf(task.Visibility)
	for i := 1; i < len(chain); i++ {
		own = models.MinAccess(own, visibilityOf(chain[i].Visibility))
	}

	for i := range chain {
		t := &chain[i]
		// Owner of the task or any ancestor → they manage the subtree.
		if t.OwnerMemberID == g.memberID {
			return models.AccessFull
		}
		// Surface 3: task grants on the task or any ancestor (inherited down).
		if grants, err := p.store.ListTaskGrants(ctx, t.TaskID); err == nil {
			level = models.MaxAccess(level, maxGrantForTask(grants, g))
		}
		// Containing projects: additive surface (member of any project sees
		// the task at that project's level) AND a floor on the house-at-large
		// surface (a private container hides it from non-members).
		if projects, err := p.store.ListProjectsForTask(ctx, t.TaskID); err == nil {
			for j := range projects {
				own = models.MinAccess(own, visibilityOf(projects[j].Visibility))
				level = models.MaxAccess(level, p.projectAccess(ctx, &projects[j], g))
			}
		}
	}

	// Fold the (capped) house-at-large surface in last.
	level = models.MaxAccess(level, own)
	return level
}

// maxAllowedTaskVisibility is the umbrella ceiling for a task's own
// visibility: the MIN visibility across its parent task and its containing
// projects. A task with no container is capped at read (no free-floating
// private tasks).
func (p *policy) maxAllowedTaskVisibility(ctx context.Context, task *models.Task) string {
	ceil := ""
	if task.ParentTaskID != nil {
		if parent, err := p.store.GetTaskByID(ctx, *task.ParentTaskID); err == nil {
			ceil = minCeil(ceil, visibilityOf(parent.Visibility))
		}
	}
	if projects, err := p.store.ListProjectsForTask(ctx, task.TaskID); err == nil {
		for i := range projects {
			ceil = minCeil(ceil, visibilityOf(projects[i].Visibility))
		}
	}
	if ceil == "" {
		// No container: free-floating tasks stay house-visible, never private.
		return models.AccessRead
	}
	return ceil
}

// isFreeFloating reports whether a task has no container at all: no parent
// task and no containing project. Such tasks are pinned to read (no
// free-floating private tasks). See docs/rbac.md.
func (p *policy) isFreeFloating(ctx context.Context, task *models.Task) bool {
	if task.ParentTaskID != nil {
		return false
	}
	projects, err := p.store.ListProjectsForTask(ctx, task.TaskID)
	if err != nil {
		// Fail closed toward "contained" so we never accidentally allow a
		// private free-floating task on a transient error.
		return false
	}
	return len(projects) == 0
}

// minCeil folds a new level into a running MIN, treating "" as unset.
func minCeil(running, next string) string {
	if running == "" {
		return next
	}
	return models.MinAccess(running, next)
}

// visibilityOf normalizes a possibly-empty stored visibility to a real level
// (empty → read, the column default).
func visibilityOf(v string) string {
	if v == "" {
		return models.AccessRead
	}
	return v
}

// accessLevelPtr extracts a stored access-level string from an optional CSIL
// AccessLevel (interface{} alias). nil/empty/non-string → fallback.
func accessLevelPtr(p *csil.AccessLevel, fallback string) string {
	if p == nil {
		return fallback
	}
	if s := string(*p); s != "" {
		return s
	}
	return fallback
}

// accessLevelVal is accessLevelPtr for a non-pointer AccessLevel.
func accessLevelVal(v csil.AccessLevel, fallback string) string {
	if s := string(v); s != "" {
		return s
	}
	return fallback
}

// granteeTypeVal extracts a grantee_type string. nil/empty → "".
func granteeTypeVal(v csil.GranteeType) string {
	return string(v)
}

// validAccessLevel reports whether s is one of the four known levels.
func validAccessLevel(s string) bool {
	switch s {
	case models.AccessNone, models.AccessRead, models.AccessEdit, models.AccessFull:
		return true
	}
	return false
}

// canRead / canEdit / canFull are the call-site predicates.
func canRead(level string) bool {
	return models.AccessRank(level) >= models.AccessRank(models.AccessRead)
}
func canEdit(level string) bool {
	return models.AccessRank(level) >= models.AccessRank(models.AccessEdit)
}
func canFull(level string) bool { return level == models.AccessFull }
