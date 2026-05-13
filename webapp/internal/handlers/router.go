package handlers

import (
	"context"
	"html/template"
	"net/http"
	"strings"

	"github.com/catalystcommunity/longhouse/webapp/internal/api"
	"github.com/catalystcommunity/longhouse/webapp/internal/config"
	"github.com/catalystcommunity/longhouse/webapp/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/webapp/internal/session"
	"github.com/catalystcommunity/longhouse/webapp/internal/templates"
	log "github.com/sirupsen/logrus"
)

// APIClient is the subset of api.Client the webapp's auth flow needs.
type APIClient interface {
	Login(signedAssertion string, houseID *string) (*api.LoginResponse, error)
}

// PKIClient is the subset of linkkeys.Client that handlers need.
type PKIClient interface {
	SignRequest(callbackURL, nonce string) (string, error)
	DecryptToken(encryptedToken string) (string, error)
	VerifyAssertion(signedAssertion, expectedDomain string) (*linkkeys.Assertion, error)
}

// SessionClientFactory builds a per-request, token-bearing api client.
// Production wires this to api.New(BaseURL).WithToken(...); tests override
// to return a fake satisfying the SessionAPI interface.
type SessionClientFactory func(token string) SessionAPI

type Deps struct {
	Sessions      *session.Manager
	PKI           PKIClient
	API           APIClient
	NewSessionAPI SessionClientFactory

	IDPURL      string
	IDPDomain   string
	CallbackURL string
}

// pages maps each page template's filename to its own parsed template
// tree. Each tree is layout + that one page only — preventing the Go
// template parser from collapsing every `{{define "content"}}` into a
// single (last-wins) definition. Standalone templates that don't use the
// layout (login.html, error.html) live in `standalones`.
var (
	pages       map[string]*template.Template
	standalones *template.Template
)

func init() {
	pages = map[string]*template.Template{}
	pageFiles := []string{
		"dashboard.html",
		"events.html",
		"event.html",
		"members.html",
		"roles.html",
		"skills.html",
		"projects.html",
		"project.html",
		"tasks.html",
		"task.html",
		"trusted_domains.html",
		"groups.html",
		"member_audits.html",
		"shares.html",
	}
	for _, name := range pageFiles {
		t, err := template.ParseFS(templates.FS, "layout.html", name)
		if err != nil {
			log.WithError(err).Fatalf("Failed to parse template %s", name)
		}
		pages[name] = t
	}
	var err error
	standalones, err = template.ParseFS(templates.FS, "login.html", "error.html", "house_picker.html")
	if err != nil {
		log.WithError(err).Fatal("Failed to parse standalone templates")
	}
}

// NewRouter builds the webapp router. Pass nil deps to use defaults from
// package config.
func NewRouter(deps *Deps) http.Handler {
	if deps == nil {
		base := api.New(config.APIURL)
		deps = &Deps{
			Sessions: session.New(config.SessionSecret, true),
			PKI: linkkeys.New(
				config.LinkkeysPKIURL,
				config.LinkkeysPKIAPIKey,
				config.LinkkeysPKIAllowInvalid,
			),
			API:           base,
			NewSessionAPI: func(token string) SessionAPI { return base.WithToken(token) },
			IDPURL:        config.LinkkeysIDPURL,
			IDPDomain:     config.LinkkeysIDPDomain,
			CallbackURL:   config.RPCallbackURL,
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthCheck)
	mux.HandleFunc("GET /", rootRedirect)
	mux.HandleFunc("GET /login", deps.login)
	mux.HandleFunc("GET /auth/callback", deps.callback)
	mux.HandleFunc("POST /logout", deps.logout)
	mux.HandleFunc("POST /auth/pick-house", deps.pickHouse)

	app := deps.requireAuth
	admin := deps.requireAdmin

	// Existing app routes. /app/{$} is an exact-match for /app/ — without
	// the {$} anchor, Go's mux would treat /app/ as a subtree and silently
	// route every unregistered /app/* path to the dashboard handler (which
	// caused HTMX-style fragment URLs to recursively render the layout
	// inside itself).
	mux.Handle("GET /app", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusFound)
	}))
	mux.Handle("GET /app/{$}", app(http.HandlerFunc(dashboard)))
	mux.Handle("GET /app/dashboard", app(http.HandlerFunc(dashboard)))

	// Dashboard fragments (JSON for vanilla-JS widgets).
	mux.Handle("GET /app/fragments/priority-tasks",
		app(http.HandlerFunc(deps.fragmentPriorityTasks)))
	mux.Handle("GET /app/fragments/calendar",
		app(http.HandlerFunc(deps.fragmentCalendar)))

	// Members
	mux.Handle("GET /app/members", app(http.HandlerFunc(deps.viewMembers)))
	mux.Handle("POST /app/members/{member_id}", app(http.HandlerFunc(deps.viewUpdateMember)))
	mux.Handle("GET /app/admin/members/{member_id}/audits",
		admin(http.HandlerFunc(deps.viewMemberAudits)))

	// Roles (admin)
	mux.Handle("GET /app/admin/roles", admin(http.HandlerFunc(deps.viewRoles)))
	mux.Handle("POST /app/admin/roles", admin(http.HandlerFunc(deps.viewCreateRole)))
	mux.Handle("POST /app/admin/roles/{role_id}/delete", admin(http.HandlerFunc(deps.viewDeleteRole)))

	// Member-role grants/revokes (admin)
	mux.Handle("POST /app/admin/members/{member_id}/roles/{role_id}",
		admin(http.HandlerFunc(deps.viewGrantRole)))
	mux.Handle("POST /app/admin/members/{member_id}/roles/{role_id}/revoke",
		admin(http.HandlerFunc(deps.viewRevokeRole)))

	// Groups (admin)
	mux.Handle("GET /app/admin/groups", admin(http.HandlerFunc(deps.viewGroups)))
	mux.Handle("POST /app/admin/groups", admin(http.HandlerFunc(deps.viewCreateGroup)))
	mux.Handle("POST /app/admin/groups/{group_id}/delete",
		admin(http.HandlerFunc(deps.viewDeleteGroup)))
	mux.Handle("POST /app/admin/groups/{group_id}/members",
		admin(http.HandlerFunc(deps.viewAddGroupMember)))
	mux.Handle("POST /app/admin/groups/{group_id}/members/{member_id}/remove",
		admin(http.HandlerFunc(deps.viewRemoveGroupMember)))

	// Trusted domains (admin)
	mux.Handle("GET /app/admin/trusted-domains", admin(http.HandlerFunc(deps.viewTrustedDomains)))
	mux.Handle("POST /app/admin/trusted-domains", admin(http.HandlerFunc(deps.viewCreateTrustedDomain)))
	mux.Handle("POST /app/admin/trusted-domains/{trusted_domain_id}/delete",
		admin(http.HandlerFunc(deps.viewDeleteTrustedDomain)))

	// Shares (admin)
	mux.Handle("GET /app/admin/shares", admin(http.HandlerFunc(deps.viewShares)))
	mux.Handle("POST /app/admin/shares", admin(http.HandlerFunc(deps.viewCreateShare)))
	mux.Handle("POST /app/admin/shares/{share_id}/delete",
		admin(http.HandlerFunc(deps.viewDeleteShare)))

	// Skills
	mux.Handle("GET /app/skills", app(http.HandlerFunc(deps.viewSkills)))
	mux.Handle("POST /app/admin/skills", admin(http.HandlerFunc(deps.viewCreateSkill)))
	mux.Handle("POST /app/admin/skills/{skill_id}/delete", admin(http.HandlerFunc(deps.viewDeleteSkill)))
	mux.Handle("POST /app/skills/{skill_id}/add", app(http.HandlerFunc(deps.viewAddOwnSkill)))
	mux.Handle("POST /app/skills/{skill_id}/remove", app(http.HandlerFunc(deps.viewRemoveOwnSkill)))

	// Projects
	mux.Handle("GET /app/projects", app(http.HandlerFunc(deps.viewProjects)))
	mux.Handle("POST /app/projects", app(http.HandlerFunc(deps.viewCreateProject)))
	mux.Handle("GET /app/projects/{project_id}", app(http.HandlerFunc(deps.viewProject)))

	// Comments (posted from a task or event detail page)
	mux.Handle("POST /app/comments/{target_type}/{target_id}",
		app(http.HandlerFunc(deps.viewCreateComment)))
	mux.Handle("POST /app/comments/{comment_id}/delete",
		app(http.HandlerFunc(deps.viewDeleteComment)))

	// Tasks
	mux.Handle("GET /app/tasks", app(http.HandlerFunc(deps.viewTasks)))
	mux.Handle("POST /app/tasks", app(http.HandlerFunc(deps.viewCreateTask)))
	mux.Handle("GET /app/tasks/{task_id}", app(http.HandlerFunc(deps.viewTask)))
	mux.Handle("POST /app/tasks/{task_id}", app(http.HandlerFunc(deps.viewUpdateTask)))

	// Events
	mux.Handle("GET /app/events", app(http.HandlerFunc(deps.viewEvents)))
	mux.Handle("POST /app/events", app(http.HandlerFunc(deps.viewCreateEvent)))
	mux.Handle("GET /app/events/{event_id}", app(http.HandlerFunc(deps.viewEvent)))

	return mux
}

// ----- Page handlers -----

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// rootRedirect bounces "/" to the app, which the auth middleware will
// itself redirect to /login if the caller has no session.
func rootRedirect(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/app/", http.StatusFound)
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	renderTemplate(w, "dashboard.html", map[string]any{
		"Title":    "Dashboard",
		"Identity": id,
	})
}

func renderTemplate(w http.ResponseWriter, name string, data any) {
	tpl, ok := pages[name]
	if !ok {
		log.Errorf("template not registered: %s", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.WithError(err).Errorf("Failed to render template %s", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func renderLogin(w http.ResponseWriter, status int, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := standalones.ExecuteTemplate(w, "login.html", map[string]any{"Error": errMsg}); err != nil {
		log.WithError(err).Error("Failed to render login template")
	}
}

// ----- Middleware + context helpers -----

type ctxKey int

const identityCtxKey ctxKey = iota

func identityFrom(ctx context.Context) *session.Identity {
	id, _ := ctx.Value(identityCtxKey).(*session.Identity)
	return id
}

func withIdentity(ctx context.Context, id *session.Identity) context.Context {
	return context.WithValue(ctx, identityCtxKey, id)
}

// requireAuth redirects unauthenticated requests to /login and stashes the
// identity in context for downstream handlers.
func (d *Deps) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := d.Sessions.GetIdentity(r)
		if err != nil {
			if strings.HasPrefix(r.URL.Path, "/app/fragments/") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), id)))
	})
}

// requireAdmin layers on top of requireAuth, returning 403 for non-admins.
func (d *Deps) requireAdmin(next http.Handler) http.Handler {
	return d.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := identityFrom(r.Context())
		if id == nil || !id.HasRole("admin") {
			http.Error(w, "admin role required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// sessionAPI builds a SessionAPI carrying the caller's bearer token.
func (d *Deps) sessionAPI(id *session.Identity) SessionAPI {
	if d.NewSessionAPI != nil {
		return d.NewSessionAPI(id.APIToken)
	}
	return api.New(config.APIURL).WithToken(id.APIToken)
}
