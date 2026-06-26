package avatar

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func serve(t *testing.T, ct string, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetch_HappyPath(t *testing.T) {
	t.Parallel()
	srv := serve(t, "application/octet-stream", pngBytes(t, 64, 48)) // lie about CT on purpose
	f := New(Options{AllowLoopback: true})
	img, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// Content type comes from the decoded format, not the (wrong) response header.
	if img.ContentType != "image/png" || img.Format != "png" {
		t.Fatalf("content type/format = %q/%q, want image/png/png", img.ContentType, img.Format)
	}
	if img.Width != 64 || img.Height != 48 {
		t.Fatalf("dims = %dx%d, want 64x48", img.Width, img.Height)
	}
}

func TestFetch_BlocksLoopbackSSRF(t *testing.T) {
	t.Parallel()
	srv := serve(t, "image/png", pngBytes(t, 64, 64))
	f := New(Options{}) // production guard: loopback denied
	_, err := f.Fetch(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("expected SSRF block reaching loopback, got %v", err)
	}
}

func TestFetch_RejectsOversize(t *testing.T) {
	t.Parallel()
	srv := serve(t, "image/png", pngBytes(t, 256, 256))
	f := New(Options{AllowLoopback: true, MaxBytes: 64}) // tiny cap
	_, err := f.Fetch(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversize rejection, got %v", err)
	}
}

func TestFetch_RejectsDisallowedScheme(t *testing.T) {
	t.Parallel()
	f := New(Options{AllowLoopback: true})
	if _, err := f.Fetch(context.Background(), "file:///etc/passwd"); err == nil {
		t.Fatal("expected scheme rejection for file://")
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	f := New(Options{MinDim: 16, MaxDim: 1024})
	t.Run("not-an-image", func(t *testing.T) {
		if _, err := f.validate([]byte("definitely not an image")); err == nil {
			t.Fatal("expected decode failure")
		}
	})
	t.Run("svg-rejected", func(t *testing.T) {
		svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
		if _, err := f.validate(svg); err == nil {
			t.Fatal("SVG must be rejected (not a raster format)")
		}
	})
	t.Run("too-small", func(t *testing.T) {
		if _, err := f.validate(pngBytes(t, 8, 8)); err == nil || !strings.Contains(err.Error(), "too small") {
			t.Fatalf("expected too-small rejection, got %v", err)
		}
	})
	t.Run("too-large", func(t *testing.T) {
		if _, err := f.validate(pngBytes(t, 2048, 2048)); err == nil || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("expected too-large rejection, got %v", err)
		}
	})
	t.Run("valid", func(t *testing.T) {
		img, err := f.validate(pngBytes(t, 64, 64))
		if err != nil || img.Width != 64 {
			t.Fatalf("valid png rejected: img=%+v err=%v", img, err)
		}
	})
}

func TestIsBlockedIP(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"8.8.8.8":              false, // public
		"1.1.1.1":              false,
		"2606:4700:4700::1111": false, // public v6
		"10.0.0.1":             true,  // RFC1918
		"192.168.1.5":          true,
		"172.16.0.1":           true,
		"127.0.0.1":            true, // loopback
		"::1":                  true,
		"169.254.169.254":      true, // link-local (cloud metadata!)
		"100.64.0.1":           true, // CGNAT
		"0.0.0.0":              true, // unspecified
		"fd00::1":              true, // ULA
	}
	for ipStr, want := range cases {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("bad test ip %q", ipStr)
		}
		if got := isBlockedIP(ip); got != want {
			t.Errorf("isBlockedIP(%s) = %v, want %v", ipStr, got, want)
		}
	}
}
