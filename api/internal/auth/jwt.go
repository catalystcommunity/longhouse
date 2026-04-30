// Package auth issues and validates the api's bearer tokens.
//
// We implement a small subset of JWT — the parts that matter to us — rather
// than pulling in a full library. The header is always {"alg":"HS256"}; the
// payload is a Claims struct; the signature is HMAC-SHA256 over
// `header.payload`. base64url, no padding, three dot-separated parts.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultTTL = 12 * time.Hour
)

var (
	ErrTokenMalformed = errors.New("auth: token is malformed")
	ErrBadSignature   = errors.New("auth: token signature is invalid")
	ErrTokenExpired   = errors.New("auth: token has expired")
)

// Claims is the payload of an api bearer token. ExpiresAt is a Unix
// timestamp (seconds since epoch) for compactness — JWT convention.
type Claims struct {
	MemberID  string   `json:"member_id"`
	HouseID   string   `json:"house_id"`
	Roles     []string `json:"roles"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
}

// HasRole returns true if the token grants the named role.
func (c *Claims) HasRole(name string) bool {
	for _, r := range c.Roles {
		if r == name {
			return true
		}
	}
	return false
}

// Mint signs a new token. ttl is clamped to DefaultTTL when zero. Negative
// values are preserved so callers (tests, expiry-rotation flows) can mint
// already-expired tokens deliberately.
func Mint(secret []byte, c Claims, ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = DefaultTTL
	}
	now := time.Now().UTC()
	c.IssuedAt = now.Unix()
	c.ExpiresAt = now.Add(ttl).Unix()

	header := []byte(`{"alg":"HS256","typ":"JWT"}`)
	payload, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	hb := base64.RawURLEncoding.EncodeToString(header)
	pb := base64.RawURLEncoding.EncodeToString(payload)
	signing := hb + "." + pb
	sig := sign(secret, signing)
	return signing + "." + sig, nil
}

// Verify checks a token's signature and expiry, returning the claims.
func Verify(secret []byte, token string) (*Claims, error) {
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
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, ErrTokenMalformed
	}
	if c.ExpiresAt > 0 && time.Now().UTC().Unix() >= c.ExpiresAt {
		return nil, ErrTokenExpired
	}
	return &c, nil
}

func sign(secret []byte, signing string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signing))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
