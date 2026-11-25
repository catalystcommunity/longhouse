// CSIL-RPC/TCP transport for the linkkeys RP service.
//
// This is the non-browser path: instead of POSTing JSON to an HTTP RP PKI
// sidecar (see client.go), we dial linkkeys' canonical CSIL-RPC endpoint over
// TCP+TLS and speak the same envelope the SPA already uses. It implements the
// same csilservices.PKIClient surface (SignRequest/DecryptToken/
// VerifyAssertion) so the auth service is transport-agnostic, plus the
// userinfo-fetch op (FetchUserInfo) the HTTP shim never exposed.
//
// Wire compatibility: the envelope is encoded by the csilgen Go transport, which wraps the request payload in CBOR tag 24
// exactly as linkkeys' server-side csilgen-transport expects (verified against
// shared conformance vectors). The inner request/response bodies are plain
// CBOR maps with snake_case keys matching the linkkeys CSIL types.
package linkkeys

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/catalystcommunity/csilgen/transports/go"
	lk "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
)

// DefaultTCPPort is linkkeys' default CSIL-RPC/TCP port (liblinkkeys::dns::
// DEFAULT_TCP_PORT). Used when a discovered or configured endpoint omits one.
const DefaultTCPPort = "4987"

const (
	rpService     = "Rp"
	opSignRequest = "sign-request"
	opUserInfo    = "userinfo-fetch"
)

// TCPConfig configures a TCPClient. Addr and Fingerprints are optional: when
// empty they are discovered from DNS off Domain (the IDP identity domain) via
// the _linkkeys_apis / _linkkeys TXT records linkkeys publishes.
type TCPConfig struct {
	Domain       string   // IDP/user identity domain (audience for userinfo, DNS discovery root)
	APIBase      string   // IDP https API base (userinfo-fetch api_base; self-call detection)
	APIKey       string   // RP API key, presented in the envelope auth field
	Addr         string   // optional host:port override (else DNS-discovered)
	Fingerprints []string // optional pinned SPKI sha256 hex (else DNS-discovered)
	Insecure     bool     // dev only: skip server-cert fingerprint pinning
	// ApplicationID is the expected application in a verified assertion.
	ApplicationID string
	// RequestedClaims is the claim request sent with Rp/sign-request. Nil: the
	// RP uses its RP_CLAIMS_CONFIG (an empty request when that is not set).
	RequestedClaims *lk.ClaimRequest
	DialTimeout     time.Duration
	IOTimeout       time.Duration
}

// TCPClient speaks CSIL-RPC to linkkeys' Rp service over TCP+TLS. It is safe
// for concurrent use: each call opens, uses, and closes its own connection
// (logins are infrequent, so a per-call dial keeps the client stateless and
// avoids managing idle/half-open pooled sockets).
type TCPClient struct {
	addr         string
	serverName   string
	fingerprints []string
	apiKey       string
	insecure     bool
	apiBase      string
	domain       string
	appID        string
	claims       *lk.ClaimRequest
	dialTimeout  time.Duration
	ioTimeout    time.Duration
}

// LonghouseClaimRequest is the claim request of the Longhouse application.
// handle is required: Longhouse shows people to each other by handle. The
// other claims are optional; a member without them signs in as usual.
func LonghouseClaimRequest() *lk.ClaimRequest {
	return &lk.ClaimRequest{
		Required: []lk.RequestedClaim{{ClaimType: "handle", Datatype: lk.ClaimValueTypeText}},
		Optional: []lk.RequestedClaim{
			{ClaimType: "email", Datatype: lk.ClaimValueTypeEmail},
			{ClaimType: "display_name", Datatype: lk.ClaimValueTypeText},
			{ClaimType: "avatar_url", Datatype: lk.ClaimValueTypeUrl},
		},
	}
}

// UserInfo is linkkeys' Rp.userinfo-fetch / Identity UserInfo response: the
// identity plus the claims released for this RP. Richer than an assertion,
// which only carries a display name.
type UserInfo struct {
	UserID      string  `cbor:"user_id"`
	Domain      string  `cbor:"domain"`
	DisplayName string  `cbor:"display_name"`
	Claims      []Claim `cbor:"claims"`
}

// Claim is one released claim. We decode the addressable fields; signatures and
// other metadata on the wire are ignored.
type Claim struct {
	ClaimID    string `cbor:"claim_id"`
	ClaimType  string `cbor:"claim_type"`
	ClaimValue []byte `cbor:"claim_value"`
}

// ClaimValues flattens the released claims into a claim_type → value map. Claim
// values are opaque bytes on the wire; the well-known identity claims
// (display_name, email, avatar_url, handle, website, ...) are UTF-8 text, so we
// expose them as strings. Returns nil when no claims were released. The linkkeys
// server already scopes Claims to the authorized set, so every entry here is one
// the user/policy actually granted.
func (u *UserInfo) ClaimValues() map[string]string {
	if u == nil || len(u.Claims) == 0 {
		return nil
	}
	out := make(map[string]string, len(u.Claims))
	for _, c := range u.Claims {
		if c.ClaimType == "" {
			continue
		}
		out[c.ClaimType] = string(c.ClaimValue)
	}
	return out
}

// NewTCPClient builds a TCPClient, resolving the endpoint and pin set from DNS
// when not supplied explicitly. DNS discovery is skipped entirely when both
// Addr and Fingerprints are provided (or Insecure is set), which is how dev
// clusters with self-signed certs and no published records still connect.
func NewTCPClient(cfg TCPConfig) (*TCPClient, error) {
	dialTimeout := cfg.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 10 * time.Second
	}
	ioTimeout := cfg.IOTimeout
	if ioTimeout <= 0 {
		ioTimeout = 15 * time.Second
	}

	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		if cfg.Domain == "" {
			return nil, fmt.Errorf("linkkeys tcp: no addr and no domain to discover from")
		}
		discovered, err := discoverEndpoint(cfg.Domain)
		if err != nil {
			return nil, fmt.Errorf("linkkeys tcp: endpoint discovery for %q: %w", cfg.Domain, err)
		}
		addr = discovered
	}
	addr = ensurePort(addr, DefaultTCPPort)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("linkkeys tcp: bad addr %q: %w", addr, err)
	}

	fingerprints := normalizeFingerprints(cfg.Fingerprints)
	if !cfg.Insecure && len(fingerprints) == 0 {
		if cfg.Domain == "" {
			return nil, fmt.Errorf("linkkeys tcp: no fingerprints and no domain to discover from (set LONGHOUSE_LINKKEYS_TCP_FINGERPRINTS or LONGHOUSE_LINKKEYS_TCP_ALLOW_INSECURE)")
		}
		discovered, err := discoverFingerprints(cfg.Domain)
		if err != nil {
			return nil, fmt.Errorf("linkkeys tcp: fingerprint discovery for %q: %w", cfg.Domain, err)
		}
		fingerprints = discovered
	}

	return &TCPClient{
		addr:         addr,
		serverName:   host,
		fingerprints: fingerprints,
		apiKey:       cfg.APIKey,
		insecure:     cfg.Insecure,
		apiBase:      cfg.APIBase,
		domain:       cfg.Domain,
		appID:        strings.TrimSpace(cfg.ApplicationID),
		claims:       cfg.RequestedClaims,
		dialTimeout:  dialTimeout,
		ioTimeout:    ioTimeout,
	}, nil
}

// SignRequest uses the application account associated with the API key.
func (c *TCPClient) SignRequest(callbackURL, nonce string) (string, error) {
	out, err := lk.NewRpClient(c).SignRequest(context.Background(), lk.RpSignRequest{CallbackUrl: callbackURL, Nonce: nonce, RequestedClaims: c.claims})
	return out.SignedRequest, err
}

func (c *TCPClient) DecryptToken(encryptedToken string) (string, error) {
	out, err := lk.NewRpClient(c).DecryptToken(context.Background(), lk.RpDecryptRequest{EncryptedToken: encryptedToken})
	return out.SignedAssertion, err
}

func (c *TCPClient) VerifyAssertion(signedAssertion, expectedDomain string) (*Assertion, error) {
	out, err := lk.NewRpClient(c).VerifyAssertion(context.Background(), lk.RpVerifyRequest{SignedAssertion: signedAssertion, ExpectedDomain: expectedDomain})
	if err != nil {
		return nil, err
	}
	if !out.Verified {
		return nil, fmt.Errorf("linkkeys: assertion rejected by RP")
	}
	a := out.Assertion
	if c.appID != "" && a.Application.ApplicationId != c.appID {
		return nil, fmt.Errorf("linkkeys: assertion names another application")
	}
	result := &Assertion{UserID: a.UserId, Domain: a.Domain, Audience: a.Audience, Nonce: a.Nonce, IssuedAt: a.IssuedAt.Format(time.RFC3339Nano), ExpiresAt: a.ExpiresAt.Format(time.RFC3339Nano), AuthorizedClaims: a.AuthorizedClaims}
	if a.DisplayName != nil {
		result.DisplayName = *a.DisplayName
	}
	return result, nil
}

func (c *TCPClient) FetchUserInfo(token, domain string) (*UserInfo, error) {
	if domain == "" {
		domain = c.domain
	}
	out, err := lk.NewRpClient(c).UserinfoFetch(context.Background(), lk.RpUserInfoRequest{Token: token, Domain: domain, ApiBase: c.apiBase})
	if err != nil {
		return nil, err
	}
	result := &UserInfo{UserID: out.UserId, Domain: out.Domain, DisplayName: out.DisplayName}
	for _, claim := range out.Claims {
		result.Claims = append(result.Claims, Claim{ClaimID: claim.ClaimId, ClaimType: claim.ClaimType, ClaimValue: claim.ClaimValue})
	}
	return result, nil
}

func (c *TCPClient) Call(ctx context.Context, service, op string, payload []byte) ([]byte, error) {
	reply, err := c.CallReply(ctx, service, op, payload)
	return reply.Payload, err
}

func (c *TCPClient) CallReply(ctx context.Context, service, op string, payload []byte) (lk.WireReply, error) {
	dialer := net.Dialer{Timeout: c.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return lk.WireReply{}, fmt.Errorf("linkkeys tcp: dial %s: %w", c.addr, err)
	}
	defer conn.Close()
	if c.ioTimeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(c.ioTimeout))
	}

	tlsConn := tls.Client(conn, c.tlsConfig())
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return lk.WireReply{}, fmt.Errorf("linkkeys tcp: tls handshake with %s: %w", c.addr, err)
	}

	client := transport.NewRpcClient(transport.NewStreamCarrier(tlsConn), false)
	var auth *string
	if c.apiKey != "" {
		auth = &c.apiKey
	}
	resp, err := client.Call(service, op, payload, auth)
	if err != nil {
		return lk.WireReply{}, fmt.Errorf("linkkeys tcp: %s/%s: %w", service, op, err)
	}
	return lk.WireReply{Variant: resp.Variant, Payload: resp.Payload}, nil
}

// tlsConfig pins the server cert by SPKI fingerprint rather than trusting a CA
// chain (linkkeys derives its cert from a domain key and publishes the
// fingerprints over DNS). InsecureSkipVerify disables Go's chain validation;
// our VerifyConnection hook re-imposes the pin. In dev (Insecure) we skip the
// pin too and accept any cert.
func (c *TCPClient) tlsConfig() *tls.Config {
	cfg := &tls.Config{
		ServerName:         c.serverName,
		InsecureSkipVerify: true, //nolint:gosec // pin enforced in VerifyConnection below
	}
	if c.insecure {
		return cfg
	}
	pins := c.fingerprints
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return fmt.Errorf("linkkeys tcp: server presented no certificate")
		}
		got, err := spkiFingerprint(cs.PeerCertificates[0])
		if err != nil {
			return fmt.Errorf("linkkeys tcp: fingerprint server cert: %w", err)
		}
		for _, want := range pins {
			if got == want {
				return nil
			}
		}
		return fmt.Errorf("linkkeys tcp: server cert fingerprint %s matches no pinned fingerprint", got)
	}
	return cfg
}

// spkiFingerprint computes linkkeys' cert fingerprint: lowercase hex of
// SHA-256 over the SubjectPublicKey bit-string bytes (the raw public key, not
// the full SubjectPublicKeyInfo) — matching liblinkkeys::crypto::fingerprint.
func spkiFingerprint(cert *x509.Certificate) (string, error) {
	var spki struct {
		Algorithm asn1.RawValue
		PublicKey asn1.BitString
	}
	if _, err := asn1.Unmarshal(cert.RawSubjectPublicKeyInfo, &spki); err != nil {
		return "", err
	}
	sum := sha256.Sum256(spki.PublicKey.Bytes)
	return hex.EncodeToString(sum[:]), nil
}

// discoverEndpoint reads _linkkeys_apis.<domain> TXT and returns the tcp=
// host[:port] linkkeys advertises for CSIL-RPC clients.
func discoverEndpoint(domain string) (string, error) {
	txts, err := net.LookupTXT("_linkkeys_apis." + domain)
	if err != nil {
		return "", err
	}
	for _, txt := range txts {
		fields := parseLinkkeysTXT(txt)
		if fields == nil {
			continue
		}
		if tcp := fields["tcp"]; tcp != "" {
			return tcp, nil
		}
	}
	return "", fmt.Errorf("no tcp= endpoint in _linkkeys_apis.%s TXT", domain)
}

// discoverFingerprints reads _linkkeys.<domain> TXT and returns the published
// fp= server-cert fingerprints (the trust anchor).
func discoverFingerprints(domain string) ([]string, error) {
	txts, err := net.LookupTXT("_linkkeys." + domain)
	if err != nil {
		return nil, err
	}
	for _, txt := range txts {
		if !strings.Contains(txt, "v=lk1") {
			continue
		}
		var fps []string
		for _, tok := range strings.Fields(txt) {
			if v, ok := strings.CutPrefix(tok, "fp="); ok && v != "" {
				fps = append(fps, strings.ToLower(v))
			}
		}
		if len(fps) > 0 {
			return fps, nil
		}
	}
	return nil, fmt.Errorf("no fp= fingerprints in _linkkeys.%s TXT", domain)
}

// parseLinkkeysTXT parses a `v=lk1 k=v k=v` linkkeys TXT record into a map,
// returning nil for records that aren't lk1. Repeated keys keep the first.
func parseLinkkeysTXT(txt string) map[string]string {
	if !strings.Contains(txt, "v=lk1") {
		return nil
	}
	out := map[string]string{}
	for _, tok := range strings.Fields(txt) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			continue
		}
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	return out
}

// ensurePort appends :defaultPort to a host that lacks one.
func ensurePort(addr, defaultPort string) string {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	return net.JoinHostPort(addr, defaultPort)
}

func normalizeFingerprints(in []string) []string {
	var out []string
	for _, fp := range in {
		fp = strings.ToLower(strings.TrimSpace(fp))
		if fp != "" {
			out = append(out, fp)
		}
	}
	return out
}
