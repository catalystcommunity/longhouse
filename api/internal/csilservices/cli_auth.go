package csilservices

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

const (
	cliLoginLifetime = 10 * time.Minute
	cliPollInterval  = 2
)

const userCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func (s *AuthService) BeginCliLogin(ctx context.Context, req csil.BeginCliLoginRequest) (csil.BeginCliLoginResponse, error) {
	if s.Store == nil || len(s.JWTSecret) == 0 {
		return csil.BeginCliLoginResponse{}, csilrpc.Internal("auth not configured")
	}
	clientName := strings.TrimSpace(req.ClientName)
	if clientName == "" || len(clientName) > 128 {
		return csil.BeginCliLoginResponse{}, csilrpc.BadRequest("client_name must contain 1 through 128 characters")
	}

	now := time.Now().UTC()
	expires := now.Add(cliLoginLifetime)
	for attempt := 0; attempt < 5; attempt++ {
		deviceCode, err := randomSecret("lh_dc_")
		if err != nil {
			return csil.BeginCliLoginResponse{}, csilrpc.Internal("could not create login request")
		}
		userCode, err := randomUserCode()
		if err != nil {
			return csil.BeginCliLoginResponse{}, csilrpc.Internal("could not create login request")
		}
		verificationURL, err := s.cliVerificationURL(userCode)
		if err != nil {
			log.WithError(err).Error("auth: CLI verification URL is not configured")
			return csil.BeginCliLoginResponse{}, csilrpc.Internal("CLI login is not configured")
		}
		login := &models.CliLoginRequest{
			DeviceCodeHash: hashSecret(deviceCode),
			UserCodeHash:   hashSecret(normalizeUserCode(userCode)),
			ClientName:     clientName,
			Status:         "pending",
			CreatedAt:      now,
			ExpiresAt:      expires,
		}
		if err := s.Store.CreateCliLogin(ctx, login); err != nil {
			if attempt < 4 {
				continue
			}
			log.WithError(err).Error("auth: create CLI login request failed")
			return csil.BeginCliLoginResponse{}, csilrpc.Internal("could not create login request")
		}
		return csil.BeginCliLoginResponse{
			DeviceCode:      deviceCode,
			UserCode:        userCode,
			VerificationUrl: verificationURL,
			ExpiresAt:       timestamp(expires),
			IntervalSeconds: cliPollInterval,
		}, nil
	}
	return csil.BeginCliLoginResponse{}, csilrpc.Internal("could not create login request")
}

func (s *AuthService) ApproveCliLogin(ctx context.Context, req csil.ApproveCliLoginRequest) (csil.EmptyResponse, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	code, err := checkedUserCode(req.UserCode)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	err = s.Store.ApproveCliLogin(ctx, hashSecret(code), id.Domain, id.UserID, id.DisplayName, time.Now().UTC())
	if err != nil {
		return csil.EmptyResponse{}, cliLoginMutationError(err)
	}
	return csil.EmptyResponse{}, nil
}

func (s *AuthService) InspectCliLogin(ctx context.Context, req csil.ApproveCliLoginRequest) (csil.CliLoginRequestInfo, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return csil.CliLoginRequestInfo{}, err
	}
	code, err := checkedUserCode(req.UserCode)
	if err != nil {
		return csil.CliLoginRequestInfo{}, err
	}
	login, err := s.Store.GetCliLoginByUserCode(ctx, hashSecret(code), time.Now().UTC())
	if err != nil {
		return csil.CliLoginRequestInfo{}, cliLoginMutationError(err)
	}
	return csil.CliLoginRequestInfo{
		UserCode:   formatUserCode(code),
		ClientName: login.ClientName,
		ExpiresAt:  timestamp(login.ExpiresAt),
	}, nil
}

func (s *AuthService) DenyCliLogin(ctx context.Context, req csil.DenyCliLoginRequest) (csil.EmptyResponse, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return csil.EmptyResponse{}, err
	}
	code, err := checkedUserCode(req.UserCode)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.DenyCliLogin(ctx, hashSecret(code), time.Now().UTC()); err != nil {
		return csil.EmptyResponse{}, cliLoginMutationError(err)
	}
	return csil.EmptyResponse{}, nil
}

func (s *AuthService) ExchangeCliLogin(ctx context.Context, req csil.ExchangeCliLoginRequest) (csil.ExchangeCliLoginResponse, error) {
	if req.DeviceCode == "" {
		return csil.ExchangeCliLoginResponse{}, csilrpc.BadRequest("device_code is required")
	}
	refreshToken, err := randomSecret("lh_rt_")
	if err != nil {
		return csil.ExchangeCliLoginResponse{}, csilrpc.Internal("could not create session")
	}
	now := time.Now().UTC()
	session, err := s.Store.ExchangeCliLogin(
		ctx, hashSecret(req.DeviceCode), hashSecret(refreshToken), now, now.Add(s.refreshTokenTTL()),
	)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrCliLoginPending):
			return csil.ExchangeCliLoginResponse{Status: csil.CliLoginStatus("pending")}, nil
		case errors.Is(err, store.ErrCliLoginDenied):
			return csil.ExchangeCliLoginResponse{Status: csil.CliLoginStatus("denied")}, nil
		case errors.Is(err, store.ErrCliLoginExpired), errors.Is(err, store.ErrCliLoginNotFound), errors.Is(err, store.ErrCliLoginUsed):
			return csil.ExchangeCliLoginResponse{Status: csil.CliLoginStatus("expired")}, nil
		default:
			log.WithError(err).Error("auth: exchange CLI login failed")
			return csil.ExchangeCliLoginResponse{}, csilrpc.Internal("could not exchange login request")
		}
	}
	token, err := s.cliToken(ctx, session, refreshToken, models.AuditActionCliLogin)
	if err != nil {
		return csil.ExchangeCliLoginResponse{}, err
	}
	return csil.ExchangeCliLoginResponse{
		Status:  csil.CliLoginStatus("complete"),
		Session: &token,
	}, nil
}

func (s *AuthService) RefreshSession(ctx context.Context, req csil.RefreshSessionRequest) (csil.CliTokenResponse, error) {
	if req.RefreshToken == "" {
		return csil.CliTokenResponse{}, csilrpc.BadRequest("refresh_token is required")
	}
	nextToken, err := randomSecret("lh_rt_")
	if err != nil {
		return csil.CliTokenResponse{}, csilrpc.Internal("could not rotate session")
	}
	session, err := s.Store.RotateCliRefreshToken(
		ctx, hashSecret(req.RefreshToken), hashSecret(nextToken), time.Now().UTC(),
	)
	if err != nil {
		if errors.Is(err, store.ErrRefreshTokenReuse) {
			log.Warn("auth: CLI refresh token reuse detected; session revoked")
		}
		if errors.Is(err, store.ErrRefreshTokenReuse) || errors.Is(err, store.ErrCliSessionInvalid) {
			return csil.CliTokenResponse{}, csilrpc.Unauthorized("refresh token is invalid or expired")
		}
		log.WithError(err).Error("auth: rotate CLI refresh token failed")
		return csil.CliTokenResponse{}, csilrpc.Internal("could not rotate session")
	}
	return s.cliToken(ctx, session, nextToken, models.AuditActionCliRefresh)
}

func (s *AuthService) ListSessions(ctx context.Context, _ csil.EmptyRequest) (csil.CliSessionsResponse, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.CliSessionsResponse{}, err
	}
	sessions, err := s.Store.ListCliSessions(ctx, id.Domain, id.UserID)
	if err != nil {
		log.WithError(err).Error("auth: list CLI sessions failed")
		return csil.CliSessionsResponse{}, csilrpc.Internal("could not list sessions")
	}
	out := make([]csil.CliSessionSummary, 0, len(sessions))
	for _, session := range sessions {
		var revokedAt *csil.Timestamp
		if session.RevokedAt != nil {
			t := timestamp(*session.RevokedAt)
			revokedAt = &t
		}
		out = append(out, csil.CliSessionSummary{
			SessionId:  csil.CliSessionID(session.SessionID),
			ClientName: session.ClientName,
			CreatedAt:  timestamp(session.CreatedAt),
			LastUsedAt: timestamp(session.LastUsedAt),
			ExpiresAt:  timestamp(session.ExpiresAt),
			RevokedAt:  revokedAt,
		})
	}
	return csil.CliSessionsResponse{Sessions: out}, nil
}

func (s *AuthService) RevokeSession(ctx context.Context, req csil.RevokeSessionRequest) (csil.EmptyResponse, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if req.SessionId == "" {
		return csil.EmptyResponse{}, csilrpc.BadRequest("session_id is required")
	}
	now := time.Now().UTC()
	if err := s.Store.RevokeCliSession(ctx, string(req.SessionId), id.Domain, id.UserID, now); err != nil {
		if errors.Is(err, store.ErrCliSessionInvalid) {
			return csil.EmptyResponse{}, csilrpc.NotFound("session not found")
		}
		log.WithError(err).Error("auth: revoke CLI session failed")
		return csil.EmptyResponse{}, csilrpc.Internal("could not revoke session")
	}
	s.recordSecurityEvent(ctx, models.AuditActionCliRevoke, id.Domain, id.UserID, id.Houses,
		models.JSONMap{"session_id": string(req.SessionId)})
	return csil.EmptyResponse{}, nil
}

func (s *AuthService) cliToken(ctx context.Context, session *models.CliSession, refreshToken, action string) (csil.CliTokenResponse, error) {
	ttl := s.bearerTTL()
	if remaining := time.Until(session.ExpiresAt); remaining < ttl {
		ttl = remaining
	}
	if ttl <= 0 {
		return csil.CliTokenResponse{}, csilrpc.Unauthorized("CLI session is expired")
	}
	login, err := s.issueTokenWithTTL(ctx, session.SubjectDomain, session.SubjectUserID, session.DisplayName, nil, action, ttl, session.SessionID)
	if err != nil {
		return csil.CliTokenResponse{}, err
	}
	return csil.CliTokenResponse{
		Token:            login.Token,
		Domain:           login.Domain,
		UserId:           login.UserId,
		DisplayName:      login.DisplayName,
		ExpiresAt:        login.ExpiresAt,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: timestamp(session.ExpiresAt),
		SessionId:        csil.CliSessionID(session.SessionID),
	}, nil
}

func (s *AuthService) cliVerificationURL(userCode string) (string, error) {
	u, err := url.Parse(s.CallbackURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("app callback URL is not an absolute URL")
	}
	u.Path = "/cli/authorize"
	u.RawPath = ""
	u.RawQuery = url.Values{"code": []string{userCode}}.Encode()
	u.Fragment = ""
	return u.String(), nil
}

func (s *AuthService) refreshTokenTTL() time.Duration {
	if s.RefreshTokenTTL <= 0 {
		return 30 * 24 * time.Hour
	}
	return s.RefreshTokenTTL
}

func checkedUserCode(raw string) (string, error) {
	code := normalizeUserCode(raw)
	if len(code) != 8 {
		return "", csilrpc.BadRequest("user_code is invalid")
	}
	for _, ch := range code {
		if !strings.ContainsRune(userCodeAlphabet, ch) {
			return "", csilrpc.BadRequest("user_code is invalid")
		}
	}
	return code, nil
}

func normalizeUserCode(raw string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), "-", ""))
}

func formatUserCode(code string) string {
	if len(code) != 8 {
		return code
	}
	return code[:4] + "-" + code[4:]
}

func cliLoginMutationError(err error) error {
	switch {
	case errors.Is(err, store.ErrCliLoginNotFound):
		return csilrpc.NotFound("login request not found")
	case errors.Is(err, store.ErrCliLoginExpired):
		return csilrpc.Conflict("login request expired")
	case errors.Is(err, store.ErrCliLoginUsed):
		return csilrpc.Conflict("login request was already handled")
	default:
		log.WithError(err).Error("auth: update CLI login request failed")
		return csilrpc.Internal("could not update login request")
	}
}

func randomSecret(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func randomUserCode() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	code := make([]byte, 8)
	for i, b := range raw {
		code[i] = userCodeAlphabet[int(b)&31]
	}
	return string(code[:4]) + "-" + string(code[4:]), nil
}

func hashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func timestamp(t time.Time) csil.Timestamp {
	return csil.Timestamp(t.UTC().Format(time.RFC3339))
}
