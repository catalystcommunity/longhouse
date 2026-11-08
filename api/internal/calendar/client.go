// Package calendar contains the catalystcal and LinkKeys clients.
package calendar

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	rp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	lk "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
)

// Keys is stored in a mounted Secret. Seeds are base64-encoded Ed25519 seeds.
type Keys struct {
	Application cal.ApplicationRef `json:"application"`
	InstanceID  string             `json:"instance_id"`
	Keys        []Key              `json:"keys"`
}
type Key struct {
	KeyID string `json:"key_id"`
	Seed  string `json:"seed"`
}
type Config struct {
	URL, KeysFile, RPAddress, RPAPIKey, BrowserURL, SubjectDomain, CallbackURL, HomeAddress string
	RPFingerprints                                                                          []string
	HomeFingerprints                                                                        []string
}
type Client struct {
	sessionsMu sync.Mutex
	sessions   map[string]*cal.ActAsSession
	Config     Config
	App        cal.Transport
	RP         lk.Transport
	Home       lk.Transport
	Keys       Keys
	signer     rp.GranteeSigner
	resolver   *rp.CachedResolver
}

var ErrGrantUnavailable = errors.New("calendar grant is not usable")

func (c *Client) ApplicationTransport() cal.Transport     { return c.App }
func (c *Client) ApplicationIdentity() cal.ApplicationRef { return c.Keys.Application }
func (c *Client) MemberDomain() string                    { return c.Config.SubjectDomain }

func New(cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, nil
	}
	for _, raw := range []string{cfg.URL, cfg.BrowserURL, cfg.CallbackURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
			return nil, fmt.Errorf("calendar URLs must use HTTPS (HTTP is allowed on loopback)")
		}
	}
	for name, pins := range map[string][]string{"RP": cfg.RPFingerprints, "home domain": cfg.HomeFingerprints} {
		valid := false
		for _, pin := range pins {
			if strings.TrimSpace(pin) != "" {
				valid = true
			}
		}
		if !valid {
			return nil, fmt.Errorf("calendar %s TLS fingerprints are required", name)
		}
	}
	keys, err := ReadKeys(cfg.KeysFile)
	if err != nil {
		return nil, err
	}
	seed, _ := base64.StdEncoding.DecodeString(keys.Keys[0].Seed)
	private := ed25519.NewKeyFromSeed(seed)
	home, err := rp.NewPinnedRpcTransport(rp.PinnedRpcTransportOptions{TCPAddress: cfg.HomeAddress, Fingerprints: cfg.HomeFingerprints, APIKey: "application-signature"})
	if err != nil {
		return nil, fmt.Errorf("calendar home domain: %w", err)
	}
	transport, err := rp.NewPinnedRpcTransport(rp.PinnedRpcTransportOptions{TCPAddress: cfg.RPAddress, Fingerprints: cfg.RPFingerprints, APIKey: cfg.RPAPIKey})
	if err != nil {
		return nil, err
	}
	app := cal.NewAppSession(cfg.URL, &cal.AppSigner{Application: keys.Application, InstanceID: keys.InstanceID, KeyID: keys.Keys[0].KeyID, Key: private})
	return &Client{Config: cfg, Keys: keys, RP: transport, Home: home, App: cal.NewHTTPTransport(cfg.URL, app), signer: rp.NewApplicationGranteeSigner(keys.InstanceID, rp.ApplicationSigner{KeyID: keys.Keys[0].KeyID, Algorithm: "ed25519", PrivateKeyBytes: private.Seed()}), resolver: rp.NewCachedResolver(transport, rp.CachedResolverOptions{})}, nil
}
func (c *Client) grantee() lk.GranteeRef {
	a := c.Keys.Application
	return lk.GranteeRef{Application: &lk.ApplicationRef{SubjectDomain: a.SubjectDomain, SubjectUserId: a.SubjectUserId, ApplicationId: a.ApplicationId}}
}
func Nonce() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (c *Client) audience(ctx context.Context) (lk.ApplicationRef, error) {
	a, err := cal.NewAuthClient(c.App).GetApplicationIdentity(ctx, cal.EmptyRequest{})
	return lk.ApplicationRef{SubjectDomain: a.Audience.SubjectDomain, SubjectUserId: a.Audience.SubjectUserId, ApplicationId: a.Audience.ApplicationId}, err
}
func (c *Client) Begin(ctx context.Context, nonce string) (string, error) {
	a := c.Keys.Application
	response, err := cal.NewAuthClient(c.App).GetActAsScopeSet(ctx, cal.ScopeSetRequest{GranteeDomain: a.SubjectDomain, GranteeUserId: a.SubjectUserId, GranteeApplicationId: a.ApplicationId, Scopes: []string{"calendars.read", "events.write"}})
	if err != nil {
		return "", err
	}
	signed, err := lk.DecodeSignedActAsScopeSet(response.SignedScopeSet)
	if err != nil {
		return "", err
	}
	audience, err := c.audience(ctx)
	if err != nil {
		return "", err
	}
	keys, err := c.resolver.Resolve(ctx, rp.InstanceRef{SubjectDomain: audience.SubjectDomain, SubjectUserID: audience.SubjectUserId, ApplicationID: audience.ApplicationId, InstanceID: signed.SignerInstanceId}, nil)
	if err != nil {
		return "", err
	}
	if keys.Freshness == rp.FreshnessStale {
		return "", errors.New("calendar audience keys are stale")
	}
	scope, err := rp.VerifyScopeSet(signed, audience, keys.Keys.AttestedKeyRefs(), rp.AcceptBeforeRevocation)
	if err != nil {
		return "", err
	}
	if err = rp.CheckScopeSetCurrent(scope, time.Now(), 300); err != nil {
		return "", err
	}
	if scope.Grantee.Application == nil || *scope.Grantee.Application != *c.grantee().Application {
		return "", errors.New("calendar scope grantee mismatch")
	}
	// The consent screen allows reduction. Require only the scopes this client uses.
	if len(scope.Entries) != 2 {
		return "", errors.New("calendar did not offer exactly the requested scopes")
	}
	for _, entry := range scope.Entries {
		if entry.Scope != "calendars.read" && entry.Scope != "events.write" {
			return "", errors.New("calendar offered unexpected scopes")
		}
	}
	now := time.Now().UTC()
	request, err := rp.SignGrantRequest(lk.ActAsGrantRequest{Grantee: c.grantee(), ScopeSet: signed, CallbackUrl: c.Config.CallbackURL, Nonce: nonce, RequestedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339)}, c.signer)
	if err != nil {
		return "", err
	}
	return rp.GrantRequestURL(c.Config.BrowserURL, request)
}

// Member checks the grant at its home domain before every write. Reuse the
// short catalystcal bearer to avoid repeating its expensive credential exchange.
func (c *Client) Member(ctx context.Context, domain, userID, grantID string) (cal.Transport, error) {
	if domain != c.Config.SubjectDomain || grantID == "" {
		return nil, ErrGrantUnavailable
	}
	audience, err := c.audience(ctx)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s/%s/%s/%s/%s/%s", domain, userID, grantID, audience.SubjectDomain, audience.SubjectUserId, audience.ApplicationId)
	c.sessionsMu.Lock()
	session, known := c.sessions[key]
	if !known {
		if c.sessions == nil || len(c.sessions) >= 1024 {
			c.sessions = make(map[string]*cal.ActAsSession)
		}
		session = cal.NewActAsSession(c.Config.URL, func(ctx context.Context) ([]byte, error) {
			result, err := c.refreshMemberGrant(ctx, domain, userID, grantID, audience)
			if err != nil {
				return nil, err
			}
			nonce := make([]byte, 32)
			if _, err = rand.Read(nonce); err != nil {
				return nil, err
			}
			credential, err := rp.Present(result.Grant, audience, cal.Digest(cal.ActAsDigestInput), time.Now().UTC(), nonce, c.signer)
			if err != nil {
				return nil, err
			}
			return lk.EncodeActAsCredential(credential), nil
		})
		c.sessions[key] = session
	}
	c.sessionsMu.Unlock()
	if known {
		if _, err = c.refreshMemberGrant(ctx, domain, userID, grantID, audience); err != nil {
			if errors.Is(err, ErrGrantUnavailable) {
				c.sessionsMu.Lock()
				delete(c.sessions, key)
				c.sessionsMu.Unlock()
			}
			return nil, err
		}
	}
	if _, err = session.Token(ctx); err != nil {
		return nil, err
	}
	return cal.NewHTTPTransport(c.Config.URL, session), nil
}
func (c *Client) refreshMemberGrant(ctx context.Context, domain, userID, grantID string, audience lk.ApplicationRef) (rp.RefreshedGrant, error) {
	now := time.Now().UTC()
	request, err := rp.SignRefreshRequest(lk.ActAsRefreshRequest{GrantId: grantID, Grantee: c.grantee(), RequestedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(5 * time.Minute).Format(time.RFC3339), Nonce: Nonce()}, c.signer)
	if err != nil {
		return rp.RefreshedGrant{}, err
	}
	result, err := rp.RefreshGrant(ctx, c.RP, domain, request)
	if err != nil {
		// LinkKeys also uses an authentication error for rate limiting. Only an
		// explicit grant revocation or expiry makes the saved connection unusable.
		if strings.Contains(err.Error(), "the grant is revoked") || strings.Contains(err.Error(), "the grant has expired") {
			return result, ErrGrantUnavailable
		}
		return result, err
	}
	grant := result.Decoded
	if grant.UserId != userID || grant.SubjectDomain != domain || grant.Audience != audience || len(grant.ApprovedScope) != 2 || !slices.Contains(grant.ApprovedScope, "events.write") || !slices.Contains(grant.ApprovedScope, "calendars.read") {
		return result, ErrGrantUnavailable
	}
	expires, err := time.Parse(time.RFC3339, grant.ExpiresAt)
	if err != nil || !expires.After(now) {
		return result, ErrGrantUnavailable
	}
	return result, nil
}

// Occurrences splits both the server's range bound and truncated responses.
// It fails closed if more than 5000 events occupy the same instant.
func Occurrences(ctx context.Context, transport cal.Transport, calendarID string, start, end time.Time) ([]cal.Occurrence, error) {
	if !end.After(start) {
		return nil, errors.New("calendar range end must follow start")
	}
	result := []cal.Occurrence{}
	seen := map[string]bool{}
	calls := 0
	var read func(time.Time, time.Time) error
	read = func(a, b time.Time) error {
		if b.Sub(a) > 399*24*time.Hour {
			mid := a.Add(399 * 24 * time.Hour)
			if err := read(a, mid); err != nil {
				return err
			}
			return read(mid, b)
		}
		calls++
		if calls > 2048 {
			return errors.New("calendar range is too dense; use a smaller range")
		}
		no := false
		rows, err := cal.NewEventClient(transport).ListEvents(ctx, cal.ListEventsRequest{RangeStart: cal.Timestamp(a.UTC().Format(time.RFC3339Nano)), RangeEnd: cal.Timestamp(b.UTC().Format(time.RFC3339Nano)), CalendarIds: []cal.CalendarID{cal.CalendarID(calendarID)}, IncludeInvitations: &no})
		if err != nil {
			return err
		}
		if rows.Truncated {
			if b.Sub(a) <= time.Second {
				return errors.New("calendar cannot return every event in this range")
			}
			mid := a.Add(b.Sub(a) / 2)
			if err = read(a, mid); err != nil {
				return err
			}
			return read(mid, b)
		}
		for _, row := range rows.Occurrences {
			if string(row.Event.CalendarId) != calendarID {
				return errors.New("calendar returned an event from another house")
			}
			key := string(row.Event.EventId)
			if row.RecurrenceId != nil {
				key += "/" + string(*row.RecurrenceId)
			}
			if !seen[key] {
				seen[key] = true
				result = append(result, row)
			}
		}
		return nil
	}
	err := read(start, end)
	return result, err
}
