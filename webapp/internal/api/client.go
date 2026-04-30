// Package api is a thin HTTP client for the longhouse api. The wire types
// are intentionally duplicated from api/internal/csil (which is internal to
// the api Go module) — when we have a third client we'll lift them into a
// shared module. For one client, hand-mirroring is the smaller diff.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrMultipleHouses is returned when the caller authenticates with an
// identity that's a member of more than one house and didn't pick which.
// The HTTP body is preserved on the error for the eventual house picker UI.
var ErrMultipleHouses = errors.New("api: caller is a member of multiple houses")

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	// Token is sent as Bearer on subsequent calls. Set after Login.
	Token string
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type LoginRequest struct {
	SignedAssertion string  `json:"signed_assertion"`
	HouseID         *string `json:"house_id,omitempty"`
}

type LoginResponse struct {
	Token     string   `json:"token"`
	MemberID  string   `json:"member_id"`
	HouseID   string   `json:"house_id"`
	Roles     []string `json:"roles"`
	ExpiresAt string   `json:"expires_at"`
}

type houseSummary struct {
	HouseID string `json:"house_id"`
	Name    string `json:"name"`
}

// MultiHouseError carries the list of houses the caller belongs to when
// Login fails with ErrMultipleHouses. UI layers unwrap to find it.
type MultiHouseError struct {
	Houses []HouseChoice
}

type HouseChoice struct {
	HouseID string
	Name    string
}

func (e *MultiHouseError) Error() string {
	return fmt.Sprintf("api: caller is a member of %d houses; pick one", len(e.Houses))
}

func (e *MultiHouseError) Unwrap() error { return ErrMultipleHouses }

// Login exchanges a verified linkkeys assertion for an api bearer token.
// houseID is optional; when nil and the caller belongs to one house, the
// api picks it; when nil and the caller has multiple houses, returns
// MultiHouseError so the UI can show a picker.
func (c *Client) Login(signedAssertion string, houseID *string) (*LoginResponse, error) {
	body, err := json.Marshal(LoginRequest{SignedAssertion: signedAssertion, HouseID: houseID})
	if err != nil {
		return nil, fmt.Errorf("marshal login: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build login: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post login: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		var out LoginResponse
		if err := json.Unmarshal(respBody, &out); err != nil {
			return nil, fmt.Errorf("decode login response: %w", err)
		}
		return &out, nil
	case http.StatusConflict:
		var multi struct {
			Error  string         `json:"error"`
			Houses []houseSummary `json:"houses"`
		}
		_ = json.Unmarshal(respBody, &multi)
		choices := make([]HouseChoice, 0, len(multi.Houses))
		for _, h := range multi.Houses {
			choices = append(choices, HouseChoice{HouseID: h.HouseID, Name: h.Name})
		}
		return nil, &MultiHouseError{Houses: choices}
	default:
		return nil, fmt.Errorf("login: %s: %s", resp.Status, string(respBody))
	}
}
