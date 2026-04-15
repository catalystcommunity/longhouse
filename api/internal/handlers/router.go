package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/cors"
)

// NewRouter creates the HTTP handler with all routes and CORS.
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /api/health", healthHandler)

	// API v1 routes (stubs for now, will be wired to store operations)
	mux.HandleFunc("GET /api/v1/houses", notImplemented)
	mux.HandleFunc("POST /api/v1/houses", notImplemented)
	mux.HandleFunc("GET /api/v1/houses/{id}", notImplemented)
	mux.HandleFunc("PUT /api/v1/houses/{id}", notImplemented)
	mux.HandleFunc("DELETE /api/v1/houses/{id}", notImplemented)

	mux.HandleFunc("GET /api/v1/houses/{houseId}/events", notImplemented)
	mux.HandleFunc("POST /api/v1/houses/{houseId}/events", notImplemented)
	mux.HandleFunc("GET /api/v1/events/{id}", notImplemented)

	mux.HandleFunc("GET /api/v1/houses/{houseId}/tasks", notImplemented)
	mux.HandleFunc("POST /api/v1/houses/{houseId}/tasks", notImplemented)
	mux.HandleFunc("GET /api/v1/tasks/{id}", notImplemented)

	mux.HandleFunc("GET /api/v1/houses/{houseId}/members", notImplemented)
	mux.HandleFunc("GET /api/v1/comments/{targetType}/{targetId}", notImplemented)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	return c.Handler(mux)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not implemented"})
}
