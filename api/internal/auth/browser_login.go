package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// BrowserLoginState binds the selected domain and nonce to the browser.
// Its signature has a separate purpose from an API bearer signature.
func BrowserLoginState(secret []byte, domain, nonce string) string {
	body := base64.RawURLEncoding.EncodeToString([]byte(domain)) + "." + nonce
	return body + "." + base64.RawURLEncoding.EncodeToString(browserLoginMAC(secret, body))
}

func VerifyBrowserLoginState(secret []byte, state string) (domain, nonce string, err error) {
	parts := strings.Split(state, ".")
	if len(parts) != 3 {
		return "", "", ErrNonceInvalid
	}
	mac, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(mac, browserLoginMAC(secret, parts[0]+"."+parts[1])) {
		return "", "", ErrNonceInvalid
	}
	d, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(d) == 0 {
		return "", "", ErrNonceInvalid
	}
	if err := VerifyNonce(secret, parts[1]); err != nil {
		return "", "", err
	}
	return string(d), parts[1], nil
}

func browserLoginMAC(secret []byte, body string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("longhouse-browser-login\x00"))
	m.Write([]byte(body))
	return m.Sum(nil)
}
