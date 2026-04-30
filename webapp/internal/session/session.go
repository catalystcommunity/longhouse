// Package session issues and validates HMAC-signed cookies for webapp auth.
// The session cookie carries the verified linkkeys identity; a separate
// short-lived "login state" cookie carries the nonce between /login and
// /auth/callback.
package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	SessionCookie         = "longhouse_session"
	LoginStateCookie      = "longhouse_login_state"
	HousePickCookie       = "longhouse_house_pick"

	SessionTTL    = 24 * time.Hour
	LoginStateTTL = 10 * time.Minute
	// HousePickTTL is the window between hitting the multi-house 409 and
	// the user choosing a house. Short — the signed assertion stashed in
	// this cookie is short-lived too.
	HousePickTTL = 10 * time.Minute
)

var (
	ErrMissing = errors.New("session: cookie missing")
	ErrInvalid = errors.New("session: cookie invalid")
	ErrExpired = errors.New("session: cookie expired")
)

type Identity struct {
	Domain      string    `json:"domain"`
	UserID      string    `json:"user_id"`
	DisplayName string    `json:"display_name,omitempty"`
	// MemberID, HouseID, and APIToken are populated after the webapp
	// exchanges the verified linkkeys assertion for an api bearer token
	// at /api/v1/auth/login. Subsequent webapp→api calls send APIToken.
	MemberID  string    `json:"member_id,omitempty"`
	HouseID   string    `json:"house_id,omitempty"`
	APIToken  string    `json:"api_token,omitempty"`
	Roles     []string  `json:"roles,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

// HasRole returns true if the session's roles include name. Lower-cased
// canonical comparison; values come from the api at login time.
func (i *Identity) HasRole(name string) bool {
	for _, r := range i.Roles {
		if r == name {
			return true
		}
	}
	return false
}

type LoginState struct {
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PendingHousePick survives the round-trip between the multi-house 409 from
// /auth/login and the user picking which house to scope their token to.
type PendingHousePick struct {
	SignedAssertion string    `json:"signed_assertion"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type Manager struct {
	secret []byte
	secure bool
}

// New returns a Manager. secret must be non-empty or New panics — this is a
// startup misconfiguration, not a runtime error. secure=true sets the Secure
// flag on issued cookies (disable only for local HTTP dev).
func New(secret string, secure bool) *Manager {
	if secret == "" {
		panic("session: empty secret")
	}
	return &Manager{secret: []byte(secret), secure: secure}
}

func (m *Manager) SetIdentity(w http.ResponseWriter, id Identity) error {
	id.ExpiresAt = time.Now().Add(SessionTTL)
	return m.setSigned(w, SessionCookie, id, "/", SessionTTL)
}

func (m *Manager) GetIdentity(r *http.Request) (*Identity, error) {
	var id Identity
	if err := m.getSigned(r, SessionCookie, &id); err != nil {
		return nil, err
	}
	if time.Now().After(id.ExpiresAt) {
		return nil, ErrExpired
	}
	return &id, nil
}

func (m *Manager) ClearIdentity(w http.ResponseWriter) {
	m.clear(w, SessionCookie, "/")
}

func (m *Manager) SetLoginState(w http.ResponseWriter, state LoginState) error {
	state.ExpiresAt = time.Now().Add(LoginStateTTL)
	return m.setSigned(w, LoginStateCookie, state, "/auth/callback", LoginStateTTL)
}

func (m *Manager) SetPendingHousePick(w http.ResponseWriter, p PendingHousePick) error {
	p.ExpiresAt = time.Now().Add(HousePickTTL)
	return m.setSigned(w, HousePickCookie, p, "/", HousePickTTL)
}

func (m *Manager) ConsumePendingHousePick(w http.ResponseWriter, r *http.Request) (*PendingHousePick, error) {
	var p PendingHousePick
	if err := m.getSigned(r, HousePickCookie, &p); err != nil {
		return nil, err
	}
	m.clear(w, HousePickCookie, "/")
	if time.Now().After(p.ExpiresAt) {
		return nil, ErrExpired
	}
	return &p, nil
}

func (m *Manager) ConsumeLoginState(w http.ResponseWriter, r *http.Request) (*LoginState, error) {
	var state LoginState
	if err := m.getSigned(r, LoginStateCookie, &state); err != nil {
		return nil, err
	}
	m.clear(w, LoginStateCookie, "/auth/callback")
	if time.Now().After(state.ExpiresAt) {
		return nil, ErrExpired
	}
	return &state, nil
}

func (m *Manager) setSigned(w http.ResponseWriter, name string, v any, path string, ttl time.Duration) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("session: marshal %s: %w", name, err)
	}
	value := base64.RawURLEncoding.EncodeToString(payload) + "." + m.sign(payload)
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
	})
	return nil
}

func (m *Manager) getSigned(r *http.Request, name string, out any) error {
	c, err := r.Cookie(name)
	if err != nil {
		return ErrMissing
	}
	payload, sig, ok := splitLast(c.Value, '.')
	if !ok {
		return ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return ErrInvalid
	}
	if subtle.ConstantTimeCompare([]byte(sig), []byte(m.sign(raw))) != 1 {
		return ErrInvalid
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return ErrInvalid
	}
	return nil
}

func (m *Manager) clear(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (m *Manager) sign(payload []byte) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func splitLast(s string, sep byte) (string, string, bool) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == sep {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}
