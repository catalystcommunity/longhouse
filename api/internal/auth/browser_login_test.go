package auth

import (
	"strings"
	"testing"
)

func TestBrowserLoginState(t *testing.T) {
	secret := []byte("login-secret")
	nonce := MintNonce(secret)
	state := BrowserLoginState(secret, "example.com", nonce)
	d, n, err := VerifyBrowserLoginState(secret, state)
	if err != nil || d != "example.com" || n != nonce {
		t.Fatalf("%q %q %v", d, n, err)
	}
	parts := strings.Split(state, ".")
	for _, invalid := range []string{
		"", "not.state", "ZXZpbC5jb20." + parts[1] + "." + parts[2],
		parts[0] + "." + MintNonce(secret) + "." + parts[2],
		BrowserLoginState([]byte("other-key"), "example.com", nonce),
		BrowserLoginState(secret, "example.com", mintNonceAt(secret, -1)),
	} {
		if _, _, err := VerifyBrowserLoginState(secret, invalid); err == nil {
			t.Fatal("accepted invalid or expired state")
		}
	}
	if _, err := Verify(secret, state); err == nil {
		t.Fatal("login state accepted as API bearer")
	}
}
