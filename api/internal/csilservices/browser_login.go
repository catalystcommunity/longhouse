package csilservices

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	log "github.com/sirupsen/logrus"
)

const browserLoginCookie = "longhouse_login"

func (s *AuthService) checkLoginDomain(ctx context.Context, domain string) error {
	if strings.EqualFold(domain, s.IDPDomain) {
		return nil
	}
	if s.Store == nil {
		return csilrpc.Internal("could not check domain access")
	}
	houses, err := s.Store.HousesTrustingDomain(ctx, domain)
	if err != nil {
		return csilrpc.Internal("could not check domain access; try again")
	}
	if len(houses) == 0 {
		return csilrpc.Forbidden("can't sign in with that domain")
	}
	return nil
}

func (s *AuthService) BeginBrowserLogin(ctx context.Context, req csil.BeginBrowserLoginRequest) (csil.BeginBrowserLoginResponse, error) {
	var empty csil.BeginBrowserLoginResponse
	if s.PKI == nil || len(s.JWTSecret) == 0 || s.CallbackURL == "" || s.IDPDomain == "" {
		return empty, csilrpc.Internal("browser login is not configured on this server")
	}
	domain, username, endpoint := strings.ToLower(s.IDPDomain), "", s.IDPURL
	if req.Login != nil {
		var err error
		username, domain, err = linkkeys.ParseBrowserLogin(*req.Login)
		if err != nil {
			return empty, csilrpc.BadRequest(err.Error())
		}
	}
	// Check trust before DNS discovery or RP calls. No house is selected here.
	if err := s.checkLoginDomain(ctx, domain); err != nil {
		return empty, err
	}
	if !strings.EqualFold(domain, s.IDPDomain) || endpoint == "" {
		resolver := s.LoginResolver
		if resolver == nil {
			resolver = net.DefaultResolver
		}
		discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		var err error
		endpoint, err = linkkeys.DiscoverBrowserEndpoint(discoveryCtx, resolver, domain)
		if err != nil {
			switch {
			case errors.Is(err, linkkeys.ErrNoBrowserEndpoint):
				return empty, csilrpc.BadRequest("that domain has no public LinkKeys sign-in service")
			case errors.Is(err, linkkeys.ErrInvalidBrowserEndpoint):
				return empty, csilrpc.BadRequest("that domain's public LinkKeys sign-in address is invalid")
			default:
				return empty, csilrpc.NewError(502, "could not look up that domain's LinkKeys service; try again")
			}
		}
	}
	nonce := auth.MintNonce(s.JWTSecret)
	signed, err := s.PKI.SignRequest(s.CallbackURL, nonce)
	if err != nil {
		log.WithError(err).Error("auth: browser sign-request failed")
		return empty, csilrpc.NewError(502, "could not reach the identity service; try again")
	}
	q := url.Values{"signed_request": {signed}}
	if username != "" {
		q.Set("username", username)
	}
	s.setLoginCookie(ctx, auth.BrowserLoginState(s.JWTSecret, domain, nonce), int(auth.NonceTTL/time.Second))
	return csil.BeginBrowserLoginResponse{RedirectUrl: strings.TrimRight(endpoint, "/") + "/auth/authorize?" + q.Encode()}, nil
}

func (s *AuthService) setLoginCookie(ctx context.Context, state string, maxAge int) {
	csilrpc.SetCookie(ctx, &http.Cookie{
		Name: browserLoginCookie, Value: state, Path: "/api", MaxAge: maxAge,
		HttpOnly: true, Secure: strings.HasPrefix(s.CallbackURL, "https://"), SameSite: http.SameSiteLaxMode,
	})
}
