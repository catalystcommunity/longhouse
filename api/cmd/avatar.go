package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/avatar"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// avatarHandler serves a member's cached avatar image. It's a plain
// (non-CSIL-RPC) authenticated GET so the SPA can fetch the bytes with its
// bearer and render them. On a cache miss it fetches the source avatar_url once
// (SSRF-guarded, validated), caches it (bytea), and serves it; thereafter we no
// longer depend on the remote URL. A fetch failure is a 404 — the SPA's Avatar
// component already falls back to initials.
type avatarHandler struct {
	store     store.Store
	fetcher   *avatar.Fetcher
	jwtSecret []byte
}

func newAvatarHandler(st store.Store, jwtSecret []byte) *avatarHandler {
	return &avatarHandler{store: st, fetcher: avatar.New(avatar.Options{}), jwtSecret: jwtSecret}
}

func (h *avatarHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := bearerIdentity(h.jwtSecret, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	memberID := r.PathValue("member")
	m, err := h.store.GetMemberByID(r.Context(), memberID)
	// 404 (not 403) when the member is missing, has no avatar, or the caller
	// isn't in that member's house — so we never reveal which member_ids exist
	// to someone outside the house.
	if err != nil || m == nil || m.AvatarURL == "" || !identityInHouse(id, m.HouseID) {
		http.NotFound(w, r)
		return
	}

	sum := sha256.Sum256([]byte(m.AvatarURL))
	urlHash := hex.EncodeToString(sum[:])

	cached, err := h.store.GetCachedAvatar(r.Context(), urlHash)
	if err != nil {
		log.WithError(err).Warn("avatar: cache lookup failed")
	}
	if cached == nil {
		img, ferr := h.fetcher.Fetch(r.Context(), m.AvatarURL)
		if ferr != nil {
			log.WithError(ferr).WithField("member_id", memberID).Debug("avatar: fetch failed; 404 to initials fallback")
			http.NotFound(w, r)
			return
		}
		cached = &models.CachedAvatar{
			URLHash:     urlHash,
			SourceURL:   m.AvatarURL,
			ContentType: img.ContentType,
			Bytes:       img.Bytes,
			Width:       img.Width,
			Height:      img.Height,
			ByteSize:    len(img.Bytes),
			FetchedAt:   time.Now().UTC(),
		}
		if perr := h.store.PutCachedAvatar(r.Context(), cached); perr != nil {
			log.WithError(perr).Warn("avatar: cache store failed (serving anyway)")
		}
	}
	serveAvatar(w, r, cached, urlHash)
}

func serveAvatar(w http.ResponseWriter, r *http.Request, a *models.CachedAvatar, etag string) {
	quoted := `"` + etag + `"`
	if r.Header.Get("If-None-Match") == quoted {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("ETag", quoted)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(a.Bytes)
}

var errMissingBearer = errors.New("missing bearer token")

func bearerIdentity(secret []byte, r *http.Request) (*auth.Identity, error) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return nil, errMissingBearer
	}
	return auth.Verify(secret, h[len(prefix):])
}

// identityInHouse reports whether the bearer carries membership in houseID,
// matching the resource-based authz convention (resource → house_id → caller
// has a role in that house).
func identityInHouse(id *auth.Identity, houseID string) bool {
	for _, h := range id.Houses {
		if h.House == houseID {
			return true
		}
	}
	return false
}
