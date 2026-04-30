// Package linkkeys is a thin HTTP client for the linkkeys RP PKI sidecar.
// The api uses it to verify signed assertions presented by clients at
// /api/v1/auth/login. The api never touches private keys; the sidecar
// holds them.
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

// New builds a Client. allowInvalidCerts skips TLS verification — only for
// dev clusters with self-signed RP certs.
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

// VerifyAssertion verifies a signed assertion against expectedDomain's
// published linkkeys keys (fetched via DNS by the sidecar). Returns the
// inner assertion fields when the signature checks out.
func (c *Client) VerifyAssertion(signedAssertion, expectedDomain string) (*Assertion, error) {
	var out struct {
		Assertion Assertion `json:"assertion"`
		Verified  bool      `json:"verified"`
	}
	if err := c.post("/v1alpha/verify-assertion.json",
		map[string]string{"signed_assertion": signedAssertion, "expected_domain": expectedDomain},
		&out); err != nil {
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
