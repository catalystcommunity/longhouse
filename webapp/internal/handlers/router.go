package handlers

import (
	"html/template"
	"net/http"

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

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", healthCheck)
	mux.HandleFunc("GET /app/", dashboard)
	mux.HandleFunc("GET /app/dashboard", dashboard)
	mux.HandleFunc("GET /app/events", events)
	mux.HandleFunc("GET /app/tasks", tasks)

	return mux
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "dashboard.html", map[string]interface{}{
		"Title": "Dashboard",
	})
}

func events(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "events.html", map[string]interface{}{
		"Title": "Events",
	})
}

func tasks(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "tasks.html", map[string]interface{}{
		"Title": "Tasks",
	})
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.WithError(err).Errorf("Failed to render template %s", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
