// Package avatar fetches and validates the image a linkkeys avatar_url claim
// points at, so Longhouse can cache it (bytea) and stop depending on the remote
// URL staying alive. The fetch is deliberately untrusting: the URL came from an
// IDP-released claim, so we (1) refuse to connect to private/loopback/link-local
// addresses (SSRF), (2) bound time + bytes, and (3) prove the payload decodes as
// a real raster image within a sane size window — never trusting Content-Type
// and never accepting SVG (which can carry script).
package avatar

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	// Register the raster decoders we accept. SVG is intentionally absent.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Image is a validated, cacheable avatar.
type Image struct {
	Bytes       []byte
	ContentType string
	Format      string
	Width       int
	Height      int
}

// Options tune the fetcher. The zero value is filled with safe defaults by New.
type Options struct {
	Timeout      time.Duration
	MaxBytes     int64
	MinDim       int
	MaxDim       int
	MaxRedirects int
	// AllowLoopback disables the loopback/private-IP guard. TESTS ONLY — never
	// set in production, it reopens SSRF.
	AllowLoopback bool
}

// Fetcher fetches+validates avatar images. Safe for concurrent use.
type Fetcher struct {
	client   *http.Client
	maxBytes int64
	minDim   int
	maxDim   int
}

var formatContentType = map[string]string{
	"png":  "image/png",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
}

// New builds a Fetcher. Defaults: 8s timeout, 5 MiB cap, 16–4096 px per side,
// 3 redirects — generous enough for real avatars, bounded against abuse.
func New(o Options) *Fetcher {
	if o.Timeout <= 0 {
		o.Timeout = 8 * time.Second
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = 5 << 20
	}
	if o.MinDim <= 0 {
		o.MinDim = 16
	}
	if o.MaxDim <= 0 {
		o.MaxDim = 4096
	}
	if o.MaxRedirects <= 0 {
		o.MaxRedirects = 3
	}
	allowLoopback := o.AllowLoopback
	dialer := &net.Dialer{
		Timeout: o.Timeout,
		// Control runs after DNS resolution with the concrete remote IP, before
		// connect — so it also defeats DNS-rebinding, and it re-runs for every
		// redirect hop's dial.
		Control: func(_, address string, _ syscall.RawConn) error {
			return guardAddress(address, allowLoopback)
		},
	}
	maxRedirects := o.MaxRedirects
	client := &http.Client{
		Timeout:   o.Timeout,
		Transport: &http.Transport{DialContext: dialer.DialContext},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("avatar: too many redirects (>%d)", maxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("avatar: redirect to disallowed scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
	return &Fetcher{client: client, maxBytes: o.MaxBytes, minDim: o.MinDim, maxDim: o.MaxDim}
}

// Fetch retrieves rawURL and returns the validated image, or an error if it
// can't be reached safely, is too big, or isn't a real image in range.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Image, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("avatar: bad url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("avatar: disallowed scheme %q", u.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("avatar: build request: %w", err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("avatar: fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("avatar: fetch: status %d", resp.StatusCode)
	}

	// Read at most maxBytes+1 so we can tell "exactly at the cap" from "over".
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("avatar: read body: %w", err)
	}
	if int64(len(body)) > f.maxBytes {
		return nil, fmt.Errorf("avatar: image exceeds %d bytes", f.maxBytes)
	}

	return f.validate(body)
}

// validate decodes body to prove it's a real raster image within the dimension
// window, deriving the content type from the actual format (not the response
// header). Exposed-by-package-internals so tests can check the decode logic
// without a network round-trip.
func (f *Fetcher) validate(body []byte) (*Image, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("avatar: not a decodable image: %w", err)
	}
	ct, ok := formatContentType[format]
	if !ok {
		return nil, fmt.Errorf("avatar: unsupported image format %q", format)
	}
	if cfg.Width < f.minDim || cfg.Height < f.minDim {
		return nil, fmt.Errorf("avatar: image too small (%dx%d, min %d)", cfg.Width, cfg.Height, f.minDim)
	}
	if cfg.Width > f.maxDim || cfg.Height > f.maxDim {
		return nil, fmt.Errorf("avatar: image too large (%dx%d, max %d)", cfg.Width, cfg.Height, f.maxDim)
	}
	// Full decode confirms the bytes aren't a truncated/corrupt header that
	// passed DecodeConfig. Bounded by the byte cap already enforced.
	if _, _, err := image.Decode(bytes.NewReader(body)); err != nil {
		return nil, fmt.Errorf("avatar: image failed full decode: %w", err)
	}
	return &Image{Bytes: body, ContentType: ct, Format: format, Width: cfg.Width, Height: cfg.Height}, nil
}

// guardAddress rejects connections to addresses we must never reach from the
// api pod. address is host:port with host already resolved to an IP.
func guardAddress(address string, allowLoopback bool) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("avatar: bad dial address %q", address)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("avatar: unresolved dial host %q", host)
	}
	if allowLoopback && ip.IsLoopback() {
		return nil
	}
	if isBlockedIP(ip) {
		return fmt.Errorf("avatar: refusing to connect to non-public address %s", ip)
	}
	return nil
}

// isBlockedIP reports whether ip is in a range we won't fetch from: loopback,
// RFC1918/ULA private, link-local, unspecified, multicast, or CGNAT.
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	// 100.64.0.0/10 (CGNAT) — not covered by IsPrivate.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}
