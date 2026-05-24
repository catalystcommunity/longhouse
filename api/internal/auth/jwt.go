// Package auth issues and validates the api's bearer tokens.
//
// The token is our own format, not JWT/JOSE/CWT: three base64url parts
// `header.payload.sig`, signature is HMAC-SHA256 over `header.payload`.
// Both header and payload are CBOR (RFC 8949) — CBOR is just the binary
// encoding; the payload's *shape* is a Longhouse identity structure
// (mirrors a CSIL-defined type; not COSE/CWT-compatible by design).
//
// The payload embeds the caller's per-house membership + roles, so
// authorization needs no per-request DB lookup. The cost is that roles are
// a mint-time snapshot — see Mint/refresh for the freshness story.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
)

const (
	DefaultTTL = 12 * time.Hour
)

var (
	ErrTokenMalformed = errors.New("auth: token is malformed")
	ErrBadSignature   = errors.New("auth: token signature is invalid")
	ErrTokenExpired   = errors.New("auth: token has expired")
)

// HouseRoles is the caller's membership in one house: the member row id
// they are in that house, plus the roles they hold there. The CBOR keys are
// short and stable — this is wire structure, not a Go-only detail.
type HouseRoles struct {
	House  string   `cbor:"house"`
	Member string   `cbor:"member"`
	Roles  []string `cbor:"roles"`
}

// Identity is the CBOR payload of a bearer token. It identifies the person
// (a linkkeys identity) and carries their full per-house membership + roles
// so authorization is self-contained. ExpiresAt is Unix seconds.
type Identity struct {
	Domain      string       `cbor:"domain"`
	UserID      string       `cbor:"user_id"`
	DisplayName string       `cbor:"display_name,omitempty"`
	Houses      []HouseRoles `cbor:"houses"`
	IssuedAt    int64        `cbor:"iat"`
	ExpiresAt   int64        `cbor:"exp"`
}

// House returns the caller's membership entry for houseID, or nil if the
// token grants no membership there.
func (id *Identity) House(houseID string) *HouseRoles {
	for i := range id.Houses {
		if id.Houses[i].House == houseID {
			return &id.Houses[i]
		}
	}
	return nil
}

// tokenHeader is the CBOR header. typ marks our format; alg leaves room for
// future algorithm agility. Not a JOSE header — just a small CBOR map.
type tokenHeader struct {
	Alg string `cbor:"alg"`
	Typ string `cbor:"typ"`
}

// MemberContext is the per-request authorization context for one house —
// which member the caller is and their roles there. RequireHouseMember
// derives it from the token's House entry for the path's house_id, then
// handlers read it via FromContext. The field surface (MemberID/HouseID/
// Roles/HasRole) is stable so handlers didn't change across token reworks.
type MemberContext struct {
	MemberID string
	HouseID  string
	Roles    []string
}

// HasRole returns true if the caller holds the named role in this house.
func (m *MemberContext) HasRole(name string) bool {
	for _, r := range m.Roles {
		if r == name {
			return true
		}
	}
	return false
}

// Mint signs a new identity token. ttl is clamped to DefaultTTL when zero.
// Negative values are preserved so callers (tests, expiry flows) can mint
// already-expired tokens deliberately.
//
// Roles embedded here are a mint-time snapshot: a revoked role keeps working
// until the token expires or is refreshed. Keep TTLs short in production and
// re-mint via /auth/refresh to bound the staleness window.
func Mint(secret []byte, id Identity, ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = DefaultTTL
	}
	now := time.Now().UTC()
	id.IssuedAt = now.Unix()
	id.ExpiresAt = now.Add(ttl).Unix()

	header, err := cbor.Marshal(tokenHeader{Alg: "HS256", Typ: "longhouse-id"})
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	payload, err := cbor.Marshal(id)
	if err != nil {
		return "", fmt.Errorf("marshal identity: %w", err)
	}

	hb := base64.RawURLEncoding.EncodeToString(header)
	pb := base64.RawURLEncoding.EncodeToString(payload)
	signing := hb + "." + pb
	sig := sign(secret, signing)
	return signing + "." + sig, nil
}

// Verify checks a token's signature and expiry, returning the identity.
func Verify(secret []byte, token string) (*Identity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrTokenMalformed
	}
	signing := parts[0] + "." + parts[1]
	expected := sign(secret, signing)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[2])) != 1 {
		return nil, ErrBadSignature
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenMalformed
	}
	var id Identity
	if err := cbor.Unmarshal(payload, &id); err != nil {
		return nil, ErrTokenMalformed
	}
	if id.ExpiresAt > 0 && time.Now().UTC().Unix() >= id.ExpiresAt {
		return nil, ErrTokenExpired
	}
	return &id, nil
}

func sign(secret []byte, signing string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signing))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
