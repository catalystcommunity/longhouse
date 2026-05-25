package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type identityKeyT struct{}
type memberKeyT struct{}

var (
	identityKey = identityKeyT{}
	memberKey   = memberKeyT{}
)

// IdentityFromContext returns the token Identity attached by RequireBearer.
// Available on any authenticated route (house-scoped or not).
func IdentityFromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(identityKey).(*Identity)
	return id
}

// WithIdentity attaches an Identity to ctx under the same key
// IdentityFromContext reads. Exported so packages that do their own
// bearer verification (e.g. the CSIL-RPC dispatcher) can stash identity
// without re-using the http.Handler middleware.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// FromContext returns the per-request MemberContext attached by
// RequireHouseMember. Handlers behind that middleware can rely on this
// returning non-nil. (Named FromContext for source compatibility with the
// pre-identity-scoped handlers, which read .MemberID/.HouseID/.Roles.)
func FromContext(ctx context.Context) *MemberContext {
	m, _ := ctx.Value(memberKey).(*MemberContext)
	return m
}

// RequireBearer verifies the bearer token and attaches its Identity to the
// context. Tokens are read from `Authorization: Bearer <jwt>`.
func RequireBearer(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := bearerToken(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			id, err := Verify(secret, tok)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token: "+err.Error())
				return
			}
			ctx := context.WithValue(r.Context(), identityKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireHouseMember authorizes the caller for the {house_id} in the path by
// reading their membership straight from the token — no DB lookup. If the
// token grants no membership in that house, the request is rejected with 403
// (trusted-domain joins are a deliberate, separate action). On success it
// attaches a MemberContext (member_id + roles for THIS house) that handlers
// read via FromContext. Composes after RequireBearer; use on
// /api/v1/houses/{house_id}/...
func RequireHouseMember(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := IdentityFromContext(r.Context())
		if id == nil {
			writeError(w, http.StatusUnauthorized, "missing token context")
			return
		}
		houseID := r.PathValue("house_id")
		if houseID == "" {
			writeError(w, http.StatusBadRequest, "missing house_id in path")
			return
		}

		hr := id.House(houseID)
		if hr == nil {
			writeError(w, http.StatusForbidden, "not a member of this house")
			return
		}

		ctx := context.WithValue(r.Context(), memberKey, &MemberContext{
			MemberID: hr.Member,
			HouseID:  houseID,
			Roles:    hr.Roles,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin wraps a handler so only callers with the canonical admin role
// in the resolved house may proceed. Composes after RequireHouseMember.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := FromContext(r.Context())
		if c == nil {
			writeError(w, http.StatusUnauthorized, "missing member context")
			return
		}
		if !c.HasRole(models.RoleAdmin) {
			writeError(w, http.StatusForbidden, "admin role required")
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
