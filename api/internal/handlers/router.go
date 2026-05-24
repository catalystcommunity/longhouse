package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/rs/cors"
)

// RouterDeps bundles the runtime services router-level handlers need.
type RouterDeps struct {
	Auth *AuthDeps
	// DevAuth, if non-nil, registers the dev-only /api/v1/dev/* endpoints.
	// Must be left nil in production. Wiring lives in cmd/serve.go and is
	// gated by config.DevAuthAllowed().
	DevAuth *DevAuthDeps
}

// NewRouter creates the HTTP handler with all routes and CORS.
//
// Most resource routes live under /api/v1/houses/{house_id}/... and chain:
//
//	RequireBearer  → confirms the JWT
//	RequireHouseFromPath → confirms the URL's house matches the JWT
//	(RequireAdmin)  → admin-only mutations
//
// Listing/reading is open to any house member; mutations follow the rules
// in the migration plan: admin for roles/skills/groups, owner-or-admin for
// projects/tasks, anyone-in-house for create.
func NewRouter(deps *RouterDeps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", healthHandler)

	if deps == nil || deps.Auth == nil {
		return cors.AllowAll().Handler(mux)
	}

	requireBearer := auth.RequireBearer(deps.Auth.JWTSecret)
	// Member routes: valid token + membership (read from the token) in the
	// path's house.
	requireMember := func(h http.Handler) http.Handler {
		return requireBearer(auth.RequireHouseMember(h))
	}
	// Admin routes: the above + the admin role in that house.
	requireAdmin := func(h http.Handler) http.Handler {
		return requireBearer(auth.RequireHouseMember(auth.RequireAdmin(h)))
	}

	// Auth (no house in URL — token carries per-house roles; /me lists houses)
	//   start    — 302 to the IDP (browser begins the assertion flow)
	//   complete — SPA posts the sealed token; we mint the session token
	//   login    — programmatic: caller already holds a verified assertion
	//   refresh  — re-mint with a fresh roles snapshot (needs a valid token)
	mux.HandleFunc("GET /api/v1/auth/start", deps.Auth.startHandler)
	mux.HandleFunc("POST /api/v1/auth/complete", deps.Auth.completeHandler)
	mux.HandleFunc("POST /api/v1/auth/login", deps.Auth.loginHandler)
	mux.Handle("POST /api/v1/auth/refresh", requireBearer(http.HandlerFunc(deps.Auth.refreshHandler)))
	mux.Handle("GET /api/v1/me", requireBearer(http.HandlerFunc(deps.Auth.meHandler)))

	// Dev-auth endpoints — only present when explicitly enabled in a
	// non-prod environment. See devauth.go for the contract.
	if deps.DevAuth != nil {
		mux.HandleFunc("GET /api/v1/dev/users", deps.DevAuth.usersHandler)
		mux.HandleFunc("POST /api/v1/dev/login", deps.DevAuth.loginHandler)
	}

	// External share access (no auth — caller proves identity with a
	// linkkeys assertion). Stubbed; returns 501 until the verifier wires up.
	mux.HandleFunc("POST /api/v1/shared/access", sharedAccessHandler)

	// Members
	mux.Handle("GET /api/v1/houses/{house_id}/members",
		requireMember(http.HandlerFunc(listMembers)))
	mux.Handle("GET /api/v1/houses/{house_id}/members/{member_id}",
		requireMember(http.HandlerFunc(getMember)))
	mux.Handle("PATCH /api/v1/houses/{house_id}/members/{member_id}",
		requireMember(http.HandlerFunc(updateMember))) // owner-or-admin enforced inside
	mux.Handle("GET /api/v1/houses/{house_id}/members/{member_id}/audits",
		requireAdmin(http.HandlerFunc(listMemberAudits)))

	// Roles + member roles
	mux.Handle("GET /api/v1/houses/{house_id}/roles",
		requireMember(http.HandlerFunc(listRoles)))
	mux.Handle("POST /api/v1/houses/{house_id}/roles",
		requireAdmin(http.HandlerFunc(createRole)))
	mux.Handle("PATCH /api/v1/houses/{house_id}/roles/{role_id}",
		requireAdmin(http.HandlerFunc(updateRole)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/roles/{role_id}",
		requireAdmin(http.HandlerFunc(deleteRole)))
	mux.Handle("GET /api/v1/houses/{house_id}/members/{member_id}/roles",
		requireMember(http.HandlerFunc(listMemberRoles)))
	mux.Handle("POST /api/v1/houses/{house_id}/members/{member_id}/roles/{role_id}",
		requireAdmin(http.HandlerFunc(grantRole)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/members/{member_id}/roles/{role_id}",
		requireAdmin(http.HandlerFunc(revokeRole)))

	// Skills + member skills
	mux.Handle("GET /api/v1/houses/{house_id}/skills",
		requireMember(http.HandlerFunc(listSkills)))
	mux.Handle("POST /api/v1/houses/{house_id}/skills",
		requireAdmin(http.HandlerFunc(createSkill)))
	mux.Handle("PATCH /api/v1/houses/{house_id}/skills/{skill_id}",
		requireAdmin(http.HandlerFunc(updateSkill)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/skills/{skill_id}",
		requireAdmin(http.HandlerFunc(deleteSkill)))
	mux.Handle("GET /api/v1/houses/{house_id}/members/{member_id}/skills",
		requireMember(http.HandlerFunc(listMemberSkills)))
	mux.Handle("POST /api/v1/houses/{house_id}/members/{member_id}/skills/{skill_id}",
		requireMember(http.HandlerFunc(addMemberSkill))) // self-or-admin enforced inside
	mux.Handle("DELETE /api/v1/houses/{house_id}/members/{member_id}/skills/{skill_id}",
		requireMember(http.HandlerFunc(removeMemberSkill)))

	// Projects + project tasks
	mux.Handle("GET /api/v1/houses/{house_id}/projects",
		requireMember(http.HandlerFunc(listProjects)))
	mux.Handle("POST /api/v1/houses/{house_id}/projects",
		requireMember(http.HandlerFunc(createProject)))
	mux.Handle("GET /api/v1/houses/{house_id}/projects/{project_id}",
		requireMember(http.HandlerFunc(getProject)))
	mux.Handle("PATCH /api/v1/houses/{house_id}/projects/{project_id}",
		requireAdmin(http.HandlerFunc(updateProject)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/projects/{project_id}",
		requireAdmin(http.HandlerFunc(deleteProject)))
	mux.Handle("GET /api/v1/houses/{house_id}/projects/{project_id}/tasks",
		requireMember(http.HandlerFunc(listProjectTasks)))
	mux.Handle("POST /api/v1/houses/{house_id}/projects/{project_id}/tasks",
		requireMember(http.HandlerFunc(addProjectTask)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/projects/{project_id}/tasks/{task_id}",
		requireMember(http.HandlerFunc(removeProjectTask)))

	// Events
	mux.Handle("GET /api/v1/houses/{house_id}/events",
		requireMember(http.HandlerFunc(listEvents)))
	mux.Handle("POST /api/v1/houses/{house_id}/events",
		requireMember(http.HandlerFunc(createEvent)))
	mux.Handle("GET /api/v1/houses/{house_id}/events/{event_id}",
		requireMember(http.HandlerFunc(getEvent)))
	mux.Handle("PATCH /api/v1/houses/{house_id}/events/{event_id}",
		requireMember(http.HandlerFunc(updateEvent))) // owner-or-admin enforced inside
	mux.Handle("DELETE /api/v1/houses/{house_id}/events/{event_id}",
		requireMember(http.HandlerFunc(deleteEvent))) // owner-or-admin enforced inside

	// Comments (event + task threads)
	mux.Handle("GET /api/v1/houses/{house_id}/comments/{target_type}/{target_id}",
		requireMember(http.HandlerFunc(listComments)))
	mux.Handle("POST /api/v1/houses/{house_id}/comments/{target_type}/{target_id}",
		requireMember(http.HandlerFunc(createComment)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/comments/{comment_id}",
		requireMember(http.HandlerFunc(deleteComment))) // owner-or-admin enforced inside

	// Groups (admin)
	mux.Handle("GET /api/v1/houses/{house_id}/groups",
		requireMember(http.HandlerFunc(listGroups)))
	mux.Handle("POST /api/v1/houses/{house_id}/groups",
		requireAdmin(http.HandlerFunc(createGroup)))
	mux.Handle("PATCH /api/v1/houses/{house_id}/groups/{group_id}",
		requireAdmin(http.HandlerFunc(updateGroup)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/groups/{group_id}",
		requireAdmin(http.HandlerFunc(deleteGroup)))
	mux.Handle("GET /api/v1/houses/{house_id}/groups/{group_id}/members",
		requireMember(http.HandlerFunc(listGroupMembers)))
	mux.Handle("POST /api/v1/houses/{house_id}/groups/{group_id}/members/{member_id}",
		requireAdmin(http.HandlerFunc(addGroupMember)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/groups/{group_id}/members/{member_id}",
		requireAdmin(http.HandlerFunc(removeGroupMember)))

	// Trusted domains (admin)
	mux.Handle("GET /api/v1/houses/{house_id}/trusted-domains",
		requireMember(http.HandlerFunc(listTrustedDomains)))
	mux.Handle("POST /api/v1/houses/{house_id}/trusted-domains",
		requireAdmin(http.HandlerFunc(createTrustedDomain)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/trusted-domains/{trusted_domain_id}",
		requireAdmin(http.HandlerFunc(deleteTrustedDomain)))

	// Shares (admin grants per-resource external read access via linkkeys identity)
	mux.Handle("GET /api/v1/houses/{house_id}/shares",
		requireAdmin(http.HandlerFunc(listShares)))
	mux.Handle("POST /api/v1/houses/{house_id}/shares",
		requireAdmin(http.HandlerFunc(createShare)))
	mux.Handle("DELETE /api/v1/houses/{house_id}/shares/{share_id}",
		requireAdmin(http.HandlerFunc(deleteShare)))

	// Tasks (top-level: a task can exist outside any project)
	mux.Handle("GET /api/v1/houses/{house_id}/tasks",
		requireMember(http.HandlerFunc(listTasks)))
	mux.Handle("POST /api/v1/houses/{house_id}/tasks",
		requireMember(http.HandlerFunc(createTask)))
	mux.Handle("GET /api/v1/houses/{house_id}/tasks/{task_id}",
		requireMember(http.HandlerFunc(getTask)))
	mux.Handle("PATCH /api/v1/houses/{house_id}/tasks/{task_id}",
		requireMember(http.HandlerFunc(updateTask))) // owner-or-admin enforced inside
	mux.Handle("DELETE /api/v1/houses/{house_id}/tasks/{task_id}",
		requireMember(http.HandlerFunc(deleteTask))) // owner-or-admin enforced inside

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	return c.Handler(mux)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
