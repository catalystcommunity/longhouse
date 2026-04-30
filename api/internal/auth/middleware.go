package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type ctxKey struct{}

var claimsKey = ctxKey{}

// FromContext returns the Claims attached to a request by RequireBearer.
// Handlers behind the middleware can rely on this returning non-nil.
func FromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// RequireBearer returns a middleware that rejects unauthenticated requests
// and attaches the verified Claims to the request context. Tokens are read
// from the Authorization header as `Bearer <jwt>`.
func RequireBearer(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := bearerToken(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			claims, err := Verify(secret, tok)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token: "+err.Error())
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin wraps a handler so that only callers whose token grants the
// canonical admin role may proceed. Composes after RequireBearer.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := FromContext(r.Context())
		if c == nil {
			writeError(w, http.StatusUnauthorized, "missing token context")
			return
		}
		if !c.HasRole(models.RoleAdmin) {
			writeError(w, http.StatusForbidden, "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireHouseFromPath ensures the {house_id} path value matches the
// caller's token. Composes after RequireBearer. Use on routes nested under
// /api/v1/houses/{house_id}/...
func RequireHouseFromPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := FromContext(r.Context())
		if c == nil {
			writeError(w, http.StatusUnauthorized, "missing token context")
			return
		}
		urlHouse := r.PathValue("house_id")
		if urlHouse == "" {
			writeError(w, http.StatusBadRequest, "missing house_id in path")
			return
		}
		if urlHouse != c.HouseID {
			writeError(w, http.StatusForbidden, "token house does not match path")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	return tok, tok != ""
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
