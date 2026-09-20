package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/transport"
	longhouseclient "github.com/catalystcommunity/longhouse/clients/go"
)

const maxCLIResponseBytes = 1 << 20

type cliCredentials struct {
	Profile          string `json:"-"`
	URL              string `json:"url"`
	HouseID          string `json:"house_id,omitempty"`
	Token            string `json:"token"`
	Domain           string `json:"domain"`
	UserID           string `json:"user_id"`
	DisplayName      string `json:"display_name,omitempty"`
	ExpiresAt        string `json:"expires_at"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresAt string `json:"refresh_expires_at"`
	SessionID        string `json:"session_id"`
}

type longhouseServiceError struct {
	Status  int
	Code    uint64
	Message string
}

func (e *longhouseServiceError) Error() string {
	return fmt.Sprintf("Longhouse error %d: %s", e.Code, e.Message)
}

func LoginCLI(flags map[string]string) error {
	profile, _ := readProfile(flags["profile"])
	baseURL, err := cliURL(flags, profile)
	if err != nil {
		return err
	}
	clientName := strings.TrimSpace(flags["client-name"])
	if clientName == "" {
		hostname, hostErr := os.Hostname()
		if hostErr != nil || hostname == "" {
			hostname = "unknown host"
		}
		clientName = hostname + " (Longhouse CLI)"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	responseBytes, err := callLonghouse(ctx, baseURL, "begin-cli-login",
		csil.EncodeBeginCliLoginRequest(csil.BeginCliLoginRequest{ClientName: clientName}), "")
	if err != nil {
		if ctx.Err() != nil {
			return cliCanceled("login canceled")
		}
		return err
	}
	login, err := csil.DecodeBeginCliLoginResponse(responseBytes)
	if err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}

	loginOutput := io.Writer(os.Stdout)
	if flags["output"] == "json" || flags["json"] == "true" {
		loginOutput = os.Stderr
	}
	fmt.Fprintf(loginOutput, "Open this URL to authorize the CLI:\n%s\n\nCode: %s\n", login.VerificationUrl, login.UserCode)
	if flags["no-browser"] != "true" {
		if err := openBrowser(login.VerificationUrl); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open a browser: %v\n", err)
		}
	}

	expiresAt, err := time.Parse(time.RFC3339, string(login.ExpiresAt))
	if err != nil {
		return fmt.Errorf("server returned an invalid login expiration: %w", err)
	}
	interval := time.Duration(login.IntervalSeconds) * time.Second
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		exchangeBytes, callErr := callLonghouse(ctx, baseURL, "exchange-cli-login",
			csil.EncodeExchangeCliLoginRequest(csil.ExchangeCliLoginRequest{DeviceCode: login.DeviceCode}), "")
		if callErr != nil {
			if ctx.Err() != nil {
				return cliCanceled("login canceled")
			}
			return callErr
		}
		exchange, decodeErr := csil.DecodeExchangeCliLoginResponse(exchangeBytes)
		if decodeErr != nil {
			return fmt.Errorf("decode login exchange: %w", decodeErr)
		}
		switch exchange.Status {
		case csil.CliLoginStatus("complete"):
			if exchange.Session == nil {
				return errors.New("server completed login without a session")
			}
			credentials := credentialsFromResponse(baseURL, *exchange.Session)
			credentials.Profile = selectedProfileName(flags["profile"])
			if profile != nil {
				credentials.HouseID = profile.HouseID
			}
			if err := writeCredentials(&credentials); err != nil {
				return err
			}
			name := credentials.DisplayName
			if name == "" {
				name = credentials.UserID + "@" + credentials.Domain
			}
			if flags["output"] == "json" || flags["json"] == "true" {
				return renderCLIResult(os.Stdout, map[string]any{"profile": credentials.Profile, "url": credentials.URL, "identity": name, "expires_at": credentials.ExpiresAt}, flags, true)
			}
			fmt.Printf("Authorized as %s.\n", name)
			return nil
		case csil.CliLoginStatus("denied"):
			return errors.New("CLI authorization was denied")
		case csil.CliLoginStatus("expired"):
			return errors.New("CLI authorization expired; run login again")
		case csil.CliLoginStatus("pending"):
			// Wait below.
		default:
			return fmt.Errorf("server returned unknown login status %q", exchange.Status)
		}

		if time.Now().UTC().After(expiresAt) {
			return errors.New("CLI authorization expired; run login again")
		}
		select {
		case <-ctx.Done():
			return cliCanceled("login canceled")
		case <-ticker.C:
		}
	}
}

func StatusCLI(flags map[string]string) error {
	credentials, err := readCredentialsForProfile(flags["profile"])
	if err != nil {
		return err
	}
	baseURL, err := cliURL(flags, credentials)
	if err != nil {
		return err
	}
	credentials.URL = baseURL
	if accessTokenNeedsRefresh(credentials) {
		if err := refreshCredentials(context.Background(), credentials); err != nil {
			return err
		}
	}
	responseBytes, err := callLonghouse(context.Background(), baseURL, "me", csil.EncodeEmptyRequest(csil.EmptyRequest{}), credentials.Token)
	if err != nil {
		return err
	}
	me, err := csil.DecodeMeResponse(responseBytes)
	if err != nil {
		return fmt.Errorf("decode account response: %w", err)
	}
	name := credentials.DisplayName
	if name == "" {
		name = me.UserId + "@" + me.Domain
	}
	if flags["output"] == "json" || flags["json"] == "true" {
		return renderCLIResult(os.Stdout, map[string]any{
			"profile": credentials.Profile, "url": baseURL, "identity": name,
			"access_expires_at": credentials.ExpiresAt, "session_expires_at": credentials.RefreshExpiresAt,
		}, flags, false)
	}
	fmt.Printf("Signed in to %s as %s\n", baseURL, name)
	fmt.Printf("Access bearer expires: %s\n", credentials.ExpiresAt)
	fmt.Printf("CLI session expires: %s\n", credentials.RefreshExpiresAt)
	return nil
}

func RefreshCLI(flags map[string]string) error {
	credentials, err := readCredentialsForProfile(flags["profile"])
	if err != nil {
		return err
	}
	baseURL, err := cliURL(flags, credentials)
	if err != nil {
		return err
	}
	credentials.URL = baseURL
	if err := refreshCredentials(context.Background(), credentials); err != nil {
		return err
	}
	if flags["output"] == "json" || flags["json"] == "true" {
		return renderCLIResult(os.Stdout, map[string]any{"profile": credentials.Profile, "expires_at": credentials.ExpiresAt}, flags, true)
	}
	fmt.Printf("Refreshed the CLI session. Access bearer expires: %s\n", credentials.ExpiresAt)
	return nil
}

func LogoutCLI(flags map[string]string) error {
	credentials, err := readCredentialsForProfile(flags["profile"])
	if err != nil {
		return err
	}
	baseURL, err := cliURL(flags, credentials)
	if err != nil {
		return err
	}
	credentials.URL = baseURL
	if accessTokenNeedsRefresh(credentials) {
		if err := refreshCredentials(context.Background(), credentials); err != nil {
			if isInvalidSessionError(err) {
				if removeErr := removeCredentialsForProfile(credentials.Profile); removeErr != nil {
					return removeErr
				}
				if flags["output"] == "json" || flags["json"] == "true" {
					return renderCLIResult(os.Stdout, map[string]any{"profile": credentials.Profile, "revoked": false, "credentials_removed": true}, flags, true)
				}
				fmt.Println("Removed local credentials. The CLI session is no longer valid.")
				return nil
			}
			return fmt.Errorf("could not refresh before revocation: %w", err)
		}
	}
	_, err = callLonghouse(context.Background(), baseURL, "revoke-session",
		csil.EncodeRevokeSessionRequest(csil.RevokeSessionRequest{SessionId: csil.CliSessionID(credentials.SessionID)}), credentials.Token)
	if err != nil {
		if isInvalidSessionError(err) {
			if removeErr := removeCredentialsForProfile(credentials.Profile); removeErr != nil {
				return removeErr
			}
			if flags["output"] == "json" || flags["json"] == "true" {
				return renderCLIResult(os.Stdout, map[string]any{"profile": credentials.Profile, "revoked": false, "credentials_removed": true}, flags, true)
			}
			fmt.Println("Removed local credentials. The CLI session is no longer valid.")
			return nil
		}
		return err
	}
	if err := removeCredentialsForProfile(credentials.Profile); err != nil {
		return err
	}
	if flags["output"] == "json" || flags["json"] == "true" {
		return renderCLIResult(os.Stdout, map[string]any{"profile": credentials.Profile, "revoked": true, "credentials_removed": true}, flags, true)
	}
	fmt.Println("Revoked the CLI session and removed local credentials.")
	return nil
}

func refreshCredentials(ctx context.Context, credentials *cliCredentials) error {
	profileName := credentials.Profile
	houseID := credentials.HouseID
	responseBytes, err := callLonghouse(ctx, credentials.URL, "refresh-session",
		csil.EncodeRefreshSessionRequest(csil.RefreshSessionRequest{RefreshToken: credentials.RefreshToken}), "")
	if err != nil {
		return err
	}
	response, err := csil.DecodeCliTokenResponse(responseBytes)
	if err != nil {
		return fmt.Errorf("decode refresh response: %w", err)
	}
	*credentials = credentialsFromResponse(credentials.URL, response)
	credentials.Profile = profileName
	credentials.HouseID = houseID
	return writeCredentials(credentials)
}

func credentialsFromResponse(baseURL string, response csil.CliTokenResponse) cliCredentials {
	displayName := ""
	if response.DisplayName != nil {
		displayName = *response.DisplayName
	}
	return cliCredentials{
		URL:              baseURL,
		Token:            response.Token,
		Domain:           response.Domain,
		UserID:           response.UserId,
		DisplayName:      displayName,
		ExpiresAt:        string(response.ExpiresAt),
		RefreshToken:     response.RefreshToken,
		RefreshExpiresAt: string(response.RefreshExpiresAt),
		SessionID:        string(response.SessionId),
	}
}

func accessTokenNeedsRefresh(credentials *cliCredentials) bool {
	expiresAt, err := time.Parse(time.RFC3339, credentials.ExpiresAt)
	return err != nil || time.Until(expiresAt) < 5*time.Minute
}

func callLonghouse(ctx context.Context, baseURL, op string, payload []byte, bearer string) ([]byte, error) {
	return callLonghouseService(ctx, baseURL, "AuthService", op, payload, bearer)
}

func callLonghouseService(ctx context.Context, baseURL, service, op string, payload []byte, bearer string) ([]byte, error) {
	endpoint, err := rpcEndpoint(baseURL)
	if err != nil {
		return nil, err
	}
	reqEnvelope := transport.NewRpcRequest(service, op, payload)
	body, err := reqEnvelope.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode request envelope: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set("Accept", "application/cbor")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Longhouse: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxCLIResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Longhouse response: %w", err)
	}
	if len(responseBody) > maxCLIResponseBytes {
		return nil, errors.New("Longhouse response is too large")
	}
	rpcResponse, err := transport.DecodeRpcResponse(responseBody)
	if err != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Longhouse returned HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("decode response envelope: %w", err)
	}
	if err := rpcResponse.AsTransportError(); err != nil {
		return nil, err
	}
	if rpcResponse.Variant != nil && *rpcResponse.Variant == "ServiceError" {
		serviceError, decodeErr := csil.DecodeServiceError(rpcResponse.Payload)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode service error: %w", decodeErr)
		}
		return nil, &longhouseServiceError{Status: resp.StatusCode, Code: serviceError.Code, Message: serviceError.Message}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Longhouse returned HTTP %d", resp.StatusCode)
	}
	return rpcResponse.Payload, nil
}

type generatedClientTransport struct {
	baseURL string
	bearer  string
}

var _ longhouseclient.Transport = (*generatedClientTransport)(nil)

func (t *generatedClientTransport) Call(ctx context.Context, service, op string, payload []byte) ([]byte, error) {
	return callLonghouseService(ctx, t.baseURL, service, op, payload, t.bearer)
}

func cliURL(flags map[string]string, credentials *cliCredentials) (string, error) {
	raw := strings.TrimSpace(flags["url"])
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("LONGHOUSE_URL"))
	}
	if raw == "" && credentials != nil {
		raw = credentials.URL
	}
	if raw == "" {
		return "", errors.New("Longhouse URL is required; use --url or LONGHOUSE_URL")
	}
	normalized, err := normalizeLonghouseURL(raw)
	if err != nil {
		return "", err
	}
	if credentials != nil {
		stored, storedErr := normalizeLonghouseURL(credentials.URL)
		if storedErr != nil {
			return "", errors.New("saved CLI session has an invalid Longhouse URL; run longhouse login")
		}
		if normalized != stored {
			return "", errors.New("the selected Longhouse URL does not match the saved CLI session; run longhouse login for that server")
		}
	}
	return normalized, nil
}

func normalizeLonghouseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("Longhouse URL must be an HTTP or HTTPS origin")
	}
	u.Path = ""
	return u.String(), nil
}

func isInvalidSessionError(err error) bool {
	var serviceErr *longhouseServiceError
	if errors.As(err, &serviceErr) && (serviceErr.Code == http.StatusUnauthorized || serviceErr.Code == http.StatusNotFound) {
		return true
	}
	var transportErr transport.StatusError
	return errors.As(err, &transportErr) && transportErr.Code == transport.StatusUnauthenticated.Code()
}

func rpcEndpoint(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/csil/v1/rpc"
	return u.String(), nil
}

func credentialsPath() (string, error) {
	base := strings.TrimSpace(os.Getenv("LONGHOUSE_CONFIG_DIR"))
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("find user configuration directory: %w", err)
		}
		base = filepath.Join(base, "longhouse")
	}
	return filepath.Join(base, "session.json"), nil
}

func readCredentials() (*cliCredentials, error) {
	return readCredentialsForProfile("")
}

func writeCredentials(credentials *cliCredentials) error {
	return saveProfile(credentials)
}

func removeCredentials() error {
	return removeCredentialsForProfile("")
}

func openBrowser(target string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
		args = []string{target}
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", target}
	default:
		command = "xdg-open"
		args = []string{target}
	}
	process := exec.Command(command, args...)
	if err := process.Start(); err != nil {
		return err
	}
	return process.Process.Release()
}
