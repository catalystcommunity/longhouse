package handlers

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/catalystcommunity/longhouse/webapp/internal/config"
	"github.com/catalystcommunity/longhouse/webapp/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/webapp/internal/session"
	"github.com/catalystcommunity/longhouse/webapp/internal/templates"
	log "github.com/sirupsen/logrus"
)

var tmpl *template.Template

func init() {
	var err error
	tmpl, err = template.ParseFS(templates.FS, "*.html")
	if err != nil {
		log.WithError(err).Fatal("Failed to parse templates")
	}
}

// PKIClient is the subset of linkkeys.Client that handlers need. Defined here
// so tests can swap in a fake.
type PKIClient interface {
	SignRequest(callbackURL, nonce string) (string, error)
	DecryptToken(encryptedToken string) (string, error)
	VerifyAssertion(signedAssertion, expectedDomain string) (*linkkeys.Assertion, error)
}

type Deps struct {
	Sessions *session.Manager
	PKI      PKIClient
	// IDP config — copied from config so tests can override.
	IDPURL      string
	IDPDomain   string
	RPDomain    string
	CallbackURL string
}

// NewRouter builds the webapp router. Pass nil deps to use defaults from
// package config; deps is threaded through tests.
func NewRouter(deps *Deps) http.Handler {
	if deps == nil {
		deps = &Deps{
			Sessions: session.New(config.SessionSecret, true),
			PKI: linkkeys.New(
				config.LinkkeysPKIURL,
				config.LinkkeysPKIAPIKey,
				config.LinkkeysPKIAllowInvalid,
			),
			IDPURL:      config.LinkkeysIDPURL,
			IDPDomain:   config.LinkkeysIDPDomain,
			RPDomain:    config.RPDomain,
			CallbackURL: config.RPCallbackURL,
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", healthCheck)
	mux.HandleFunc("GET /login", deps.login)
	mux.HandleFunc("GET /auth/callback", deps.callback)
	mux.HandleFunc("POST /logout", deps.logout)
	mux.Handle("GET /app/", deps.requireAuth(http.HandlerFunc(dashboard)))
	mux.Handle("GET /app/dashboard", deps.requireAuth(http.HandlerFunc(dashboard)))
	mux.Handle("GET /app/events", deps.requireAuth(http.HandlerFunc(events)))
	mux.Handle("GET /app/tasks", deps.requireAuth(http.HandlerFunc(tasks)))

	return mux
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "dashboard.html", map[string]interface{}{"Title": "Dashboard"})
}

func events(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "events.html", map[string]interface{}{"Title": "Events"})
}

func tasks(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "tasks.html", map[string]interface{}{"Title": "Tasks"})
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.WithError(err).Errorf("Failed to render template %s", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func renderLogin(w http.ResponseWriter, status int, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "login.html", map[string]interface{}{"Error": errMsg}); err != nil {
		log.WithError(err).Error("Failed to render login template")
	}
}

// requireAuth redirects unauthenticated requests to the login page. It
// deliberately swallows the underlying reason — the user doesn't need to know
// the difference between "missing cookie" and "tampered cookie".
func (d *Deps) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := d.Sessions.GetIdentity(r); err != nil {
			if strings.HasPrefix(r.URL.Path, "/app/fragments/") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
