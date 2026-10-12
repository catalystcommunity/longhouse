package csilservices

// Browser sessions keep a person signed in across app launches without a new
// IDP round trip. A browser sign-in (Complete, DevLogin, or a Refresh from a
// bearer that predates sessions) creates a row in cli_sessions with kind
// 'browser' and puts a rotating refresh token in an httpOnly cookie. The SPA
// calls ResumeSession when its bearer is missing, expired, or near expiry; the
// cookie rotates and the session expiry slides forward by the lifetime, so a
// browser in regular use stays signed in. The lifetime is the house setting
// session_lifetime_hours (the shortest one across the person's houses).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

const (
	browserSessionCookie = "longhouse_session"
	// The cookie goes only to the api. The SPA reaches the api under /api on
	// its own origin, so the browser sends it with same-origin RPC calls.
	browserSessionCookiePath = "/api"
	// Two tabs that resume at the same moment send the same cookie. The
	// second one inside this window gets a bearer instead of a reuse
	// revocation.
	browserRefreshReuseGrace = 60 * time.Second

	defaultSessionLifetimeHours = 720
	minSessionLifetimeHours     = 1
	maxSessionLifetimeHours     = 8760
)

// ResumeSession re-mints a bearer from the browser session cookie. It is
// public: the bearer may be expired or absent.
func (s *AuthService) ResumeSession(ctx context.Context, _ csil.EmptyRequest) (csil.LoginResponse, error) {
	if s.Store == nil || len(s.JWTSecret) == 0 {
		return csil.LoginResponse{}, csilrpc.Internal("auth not configured")
	}
	presented := csilrpc.RequestCookie(ctx, browserSessionCookie)
	if presented == "" {
		return csil.LoginResponse{}, csilrpc.Unauthorized("no browser session")
	}
	next, err := randomSecret("lh_bs_")
	if err != nil {
		return csil.LoginResponse{}, csilrpc.Internal("could not rotate session")
	}
	now := time.Now().UTC()
	session, err := s.Store.RotateBrowserRefreshToken(ctx, hashSecret(presented), hashSecret(next), now, browserRefreshReuseGrace)
	rotated := true
	switch {
	case err == nil:
	case errors.Is(err, store.ErrRefreshTokenRecentlyUsed):
		// Another tab rotated this cookie a moment ago, and its response set
		// the newer cookie. Keep that cookie; only mint a bearer here.
		rotated = false
	case errors.Is(err, store.ErrRefreshTokenReuse), errors.Is(err, store.ErrCliSessionInvalid):
		if errors.Is(err, store.ErrRefreshTokenReuse) {
			log.Warn("auth: browser refresh token reuse detected; session revoked")
		}
		clearBrowserSessionCookie(ctx)
		return csil.LoginResponse{}, csilrpc.Unauthorized("browser session is invalid or expired")
	default:
		log.WithError(err).Error("auth: rotate browser refresh token failed")
		return csil.LoginResponse{}, csilrpc.Internal("could not rotate session")
	}

	houses, err := s.prepareIdentity(ctx, session.SubjectDomain, session.SubjectUserID, session.DisplayName, nil)
	if err != nil {
		return csil.LoginResponse{}, err
	}
	lifetime := s.browserSessionLifetime(ctx, houses)
	if rotated {
		if err := s.Store.ExtendBrowserSession(ctx, session.SessionID, now.Add(lifetime)); err != nil {
			log.WithError(err).Error("auth: extend browser session failed")
			return csil.LoginResponse{}, csilrpc.Internal("could not rotate session")
		}
		setBrowserSessionCookie(ctx, next, lifetime)
	}
	return s.mintToken(ctx, session.SubjectDomain, session.SubjectUserID, session.DisplayName, houses,
		models.AuditActionSessionResume, min(s.bearerTTL(), lifetime), sessionRef{browser: session.SessionID})
}

// EndSession revokes the browser session named by the cookie and clears the
// cookie. It is public so sign-out works with an expired bearer. A missing or
// unknown cookie is not an error: the browser ends up signed out either way.
func (s *AuthService) EndSession(ctx context.Context, _ csil.EmptyRequest) (csil.EmptyResponse, error) {
	clearBrowserSessionCookie(ctx)
	presented := csilrpc.RequestCookie(ctx, browserSessionCookie)
	if presented == "" || s.Store == nil {
		return csil.EmptyResponse{}, nil
	}
	session, err := s.Store.RevokeBrowserSessionByToken(ctx, hashSecret(presented), time.Now().UTC())
	if err != nil {
		if !errors.Is(err, store.ErrCliSessionInvalid) {
			log.WithError(err).Warn("auth: end browser session failed")
		}
		return csil.EmptyResponse{}, nil
	}
	houses, err := buildHouseRoles(ctx, s.Store, session.SubjectDomain, session.SubjectUserID)
	if err != nil {
		houses = nil
	}
	s.recordSecurityEvent(ctx, models.AuditActionSessionEnd, session.SubjectDomain, session.SubjectUserID, houses,
		models.JSONMap{"session_id": session.SessionID})
	return csil.EmptyResponse{}, nil
}

// startBrowserSession creates a browser session for the identity and queues
// its cookie. A session cookie the browser already holds is revoked first, so
// signing in again does not leave the old session behind.
func (s *AuthService) startBrowserSession(ctx context.Context, domain, userID, displayName string, houses []auth.HouseRoles) (sessionRef, time.Duration, error) {
	lifetime := s.browserSessionLifetime(ctx, houses)
	if s.Store == nil {
		return sessionRef{}, lifetime, nil
	}
	now := time.Now().UTC()
	if old := csilrpc.RequestCookie(ctx, browserSessionCookie); old != "" {
		if _, err := s.Store.RevokeBrowserSessionByToken(ctx, hashSecret(old), now); err != nil &&
			!errors.Is(err, store.ErrCliSessionInvalid) {
			log.WithError(err).Warn("auth: revoke previous browser session failed (continuing)")
		}
	}
	token, err := randomSecret("lh_bs_")
	if err != nil {
		return sessionRef{}, 0, csilrpc.Internal("could not create session")
	}
	session := &models.CliSession{
		SubjectDomain: domain,
		SubjectUserID: userID,
		DisplayName:   displayName,
		ClientName:    browserClientName(csilrpc.RequestUserAgent(ctx)),
		CreatedAt:     now,
		LastUsedAt:    now,
		ExpiresAt:     now.Add(lifetime),
	}
	if err := s.Store.CreateBrowserSession(ctx, session, hashSecret(token)); err != nil {
		log.WithError(err).Error("auth: create browser session failed")
		return sessionRef{}, 0, csilrpc.Internal("could not create session")
	}
	setBrowserSessionCookie(ctx, token, lifetime)
	return sessionRef{browser: session.SessionID}, lifetime, nil
}

// refreshBrowserBearer serves Refresh for a bearer that names a browser
// session. The session must still be live, so revoking it stops refresh too.
// The cookie does not rotate here; ResumeSession owns that.
func (s *AuthService) refreshBrowserBearer(ctx context.Context, id *auth.Identity) (csil.LoginResponse, error) {
	session, err := s.Store.GetCliSession(ctx, id.BrowserSessionID)
	now := time.Now().UTC()
	if err != nil || session.Kind != models.SessionKindBrowser || session.RevokedAt != nil ||
		!session.ExpiresAt.After(now) || session.SubjectDomain != id.Domain || session.SubjectUserID != id.UserID {
		return csil.LoginResponse{}, csilrpc.Unauthorized("browser session is invalid or expired")
	}
	houses, err := s.prepareIdentity(ctx, id.Domain, id.UserID, id.DisplayName, nil)
	if err != nil {
		return csil.LoginResponse{}, err
	}
	return s.mintToken(ctx, id.Domain, id.UserID, id.DisplayName, houses, models.AuditActionRefresh,
		min(s.bearerTTL(), session.ExpiresAt.Sub(now)), sessionRef{browser: session.SessionID})
}

// browserSessionLifetime is the shortest session_lifetime_hours across the
// identity's houses, or the default when no house sets it.
func (s *AuthService) browserSessionLifetime(ctx context.Context, houses []auth.HouseRoles) time.Duration {
	hours := uint64(0)
	if s.Store != nil {
		for _, h := range houses {
			v, ok := houseSessionLifetimeHours(ctx, s.Store, h.House)
			if ok && (hours == 0 || v < hours) {
				hours = v
			}
		}
	}
	if hours == 0 {
		hours = defaultSessionLifetimeHours
	}
	return time.Duration(hours) * time.Hour
}

// houseSessionLifetimeHours reads one house's stored setting. A missing,
// unreadable, or out-of-range value reports ok=false.
func houseSessionLifetimeHours(ctx context.Context, st store.Store, houseID string) (uint64, bool) {
	raw, ok, err := readHouseSetting(ctx, st, houseID, settingSessionLifetimeHours)
	if err != nil || !ok {
		return 0, false
	}
	var v uint64
	if err := json.Unmarshal(raw, &v); err != nil || !validSessionLifetimeHours(v) {
		return 0, false
	}
	return v, true
}

func validSessionLifetimeHours(v uint64) bool {
	return v >= minSessionLifetimeHours && v <= maxSessionLifetimeHours
}

func setBrowserSessionCookie(ctx context.Context, token string, lifetime time.Duration) {
	csilrpc.SetCookie(ctx, &http.Cookie{
		Name:     browserSessionCookie,
		Value:    token,
		Path:     browserSessionCookiePath,
		MaxAge:   int(lifetime / time.Second),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearBrowserSessionCookie(ctx context.Context) {
	csilrpc.SetCookie(ctx, &http.Cookie{
		Name:     browserSessionCookie,
		Value:    "",
		Path:     browserSessionCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// browserClientName turns a User-Agent into a short label for the sessions
// list, such as "Safari on iPhone". It only needs to help a person recognize
// their own devices.
func browserClientName(ua string) string {
	browser := "Browser"
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "Firefox/") || strings.Contains(ua, "FxiOS/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/") || strings.Contains(ua, "CriOS/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	platform := ""
	switch {
	case strings.Contains(ua, "iPhone"):
		platform = "iPhone"
	case strings.Contains(ua, "iPad"):
		platform = "iPad"
	case strings.Contains(ua, "Android"):
		platform = "Android"
	case strings.Contains(ua, "Mac OS X") || strings.Contains(ua, "Macintosh"):
		platform = "Mac"
	case strings.Contains(ua, "Windows"):
		platform = "Windows"
	case strings.Contains(ua, "Linux"):
		platform = "Linux"
	}
	if platform == "" {
		return browser
	}
	return browser + " on " + platform
}
