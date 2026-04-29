// Package linkkeys is a thin HTTP client for the linkkeys RP PKI sidecar.
// The sidecar holds this longhouse instance's Ed25519 keypair and performs all
// crypto on our behalf; this package never touches private keys.
package linkkeys

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Assertion mirrors the relevant fields of linkkeys' IdentityAssertion.
type Assertion struct {
	UserID      string `json:"user_id"`
	Domain      string `json:"domain"`
	Audience    string `json:"audience"`
	Nonce       string `json:"nonce"`
	IssuedAt    string `json:"issued_at"`
	ExpiresAt   string `json:"expires_at"`
	DisplayName string `json:"display_name,omitempty"`
}

// New builds a Client. If allowInvalidCerts is true, TLS verification is
// skipped — only use this for dev clusters with self-signed RP certs.
func New(baseURL, apiKey string, allowInvalidCerts bool) *Client {
	tr := &http.Transport{}
	if allowInvalidCerts {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Transport: tr, Timeout: 15 * time.Second},
	}
}

// SignRequest asks the PKI sidecar to sign an AuthRequest. The returned
// signed_request is base64url and is appended to the IDP redirect URL.
func (c *Client) SignRequest(callbackURL, nonce string) (string, error) {
	var out struct {
		SignedRequest string `json:"signed_request"`
	}
	err := c.post("/v1alpha/sign-request.json",
		map[string]string{"callback_url": callbackURL, "nonce": nonce},
		&out)
	if err != nil {
		return "", err
	}
	return out.SignedRequest, nil
}

// DecryptToken decrypts the encrypted_token returned to /auth/callback. The
// result is still only signed — callers must then VerifyAssertion it.
func (c *Client) DecryptToken(encryptedToken string) (string, error) {
	var out struct {
		SignedAssertion string `json:"signed_assertion"`
	}
	err := c.post("/v1alpha/decrypt-token.json",
		map[string]string{"encrypted_token": encryptedToken},
		&out)
	if err != nil {
		return "", err
	}
	return out.SignedAssertion, nil
}

// VerifyAssertion checks the signed assertion against the expected IDP's
// published domain keys (fetched via DNS by the sidecar).
func (c *Client) VerifyAssertion(signedAssertion, expectedDomain string) (*Assertion, error) {
	var out struct {
		Assertion Assertion `json:"assertion"`
		Verified  bool      `json:"verified"`
	}
	err := c.post("/v1alpha/verify-assertion.json",
		map[string]string{"signed_assertion": signedAssertion, "expected_domain": expectedDomain},
		&out)
	if err != nil {
		return nil, err
	}
	if !out.Verified {
		return nil, fmt.Errorf("linkkeys: assertion rejected by RP")
	}
	return &out.Assertion, nil
}

func (c *Client) post(path string, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("do %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("linkkeys %s: %s: %s", path, resp.Status, string(respBody))
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
