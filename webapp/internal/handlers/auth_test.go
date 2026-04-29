package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/catalystcommunity/longhouse/webapp/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/webapp/internal/session"
)

type fakePKI struct {
	signRequest     func(callbackURL, nonce string) (string, error)
	decryptToken    func(encryptedToken string) (string, error)
	verifyAssertion func(signedAssertion, expectedDomain string) (*linkkeys.Assertion, error)
}

func (f *fakePKI) SignRequest(cb, n string) (string, error) { return f.signRequest(cb, n) }
func (f *fakePKI) DecryptToken(tok string) (string, error)  { return f.decryptToken(tok) }
func (f *fakePKI) VerifyAssertion(sa, d string) (*linkkeys.Assertion, error) {
	return f.verifyAssertion(sa, d)
}

func testDeps(pki PKIClient) *Deps {
	return &Deps{
		Sessions:    session.New("test-secret", false),
		PKI:         pki,
		IDPURL:      "https://idp.example",
		IDPDomain:   "idp.example",
		RPDomain:    "longhouse.example",
		CallbackURL: "https://longhouse.example/auth/callback",
	}
}

func TestLogin_RedirectsToIDPWithNonceAndSignedRequest(t *testing.T) {
	var capturedNonce string
	pki := &fakePKI{
		signRequest: func(cb, n string) (string, error) {
			capturedNonce = n
			if cb != "https://longhouse.example/auth/callback" {
				t.Errorf("callback_url: %q", cb)
			}
			return "SIGNED-" + n, nil
		},
	}
	d := testDeps(pki)
	router := NewRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status: got %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	if u.Host != "idp.example" || u.Path != "/auth/authorize" {
		t.Errorf("redirect target: %q", loc)
	}
	q := u.Query()
	if q.Get("nonce") != capturedNonce {
		t.Errorf("nonce mismatch: url=%q captured=%q", q.Get("nonce"), capturedNonce)
	}
	if q.Get("signed_request") != "SIGNED-"+capturedNonce {
		t.Errorf("signed_request: %q", q.Get("signed_request"))
	}
	if q.Get("relying_party") != "longhouse.example" {
		t.Errorf("relying_party: %q", q.Get("relying_party"))
	}

	sawState := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.LoginStateCookie {
			sawState = true
		}
	}
	if !sawState {
		t.Error("expected login state cookie")
	}
}

func TestLogin_PKIError(t *testing.T) {
	pki := &fakePKI{signRequest: func(cb, n string) (string, error) { return "", errors.New("boom") }}
	router := NewRouter(testDeps(pki))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status: got %d, want 502", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "identity service") {
		t.Errorf("expected error in body: %q", rec.Body.String())
	}
}

// fullLoginFlow runs /login, captures the nonce + cookies, then hits
// /auth/callback with a matching assertion and returns the final recorder.
func fullLoginFlow(t *testing.T, d *Deps, assertion *linkkeys.Assertion, overrideNonce string) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(d)

	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rec1.Code != http.StatusFound {
		t.Fatalf("login: got %d", rec1.Code)
	}
	u, _ := url.Parse(rec1.Header().Get("Location"))
	nonce := u.Query().Get("nonce")
	if overrideNonce != "" {
		assertion.Nonce = overrideNonce
	} else {
		assertion.Nonce = nonce
	}

	// Re-point the fake verifier now that we know the nonce. The fake is
	// shared across the two requests via the Deps struct.
	f := d.PKI.(*fakePKI)
	f.verifyAssertion = func(sa, dom string) (*linkkeys.Assertion, error) {
		return assertion, nil
	}

	req2 := httptest.NewRequest(http.MethodGet, "/auth/callback?encrypted_token=ENC", nil)
	for _, c := range rec1.Result().Cookies() {
		req2.AddCookie(c)
	}
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	return rec2
}

func TestCallback_HappyPath(t *testing.T) {
	pki := &fakePKI{
		signRequest:  func(cb, n string) (string, error) { return "SIGNED", nil },
		decryptToken: func(tok string) (string, error) { return "ASSERT", nil },
	}
	d := testDeps(pki)

	rec := fullLoginFlow(t, d, &linkkeys.Assertion{
		UserID: "u1", Domain: "idp.example", Audience: "longhouse.example", DisplayName: "Tod",
	}, "")

	if rec.Code != http.StatusFound {
		t.Fatalf("callback status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/app/dashboard" {
		t.Errorf("redirect: %q", rec.Header().Get("Location"))
	}
	sawSession := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.SessionCookie && c.Value != "" && c.MaxAge >= 0 {
			sawSession = true
		}
	}
	if !sawSession {
		t.Error("expected session cookie")
	}
}

func TestCallback_NonceMismatch(t *testing.T) {
	pki := &fakePKI{
		signRequest:  func(cb, n string) (string, error) { return "SIGNED", nil },
		decryptToken: func(tok string) (string, error) { return "ASSERT", nil },
	}
	d := testDeps(pki)

	rec := fullLoginFlow(t, d, &linkkeys.Assertion{
		UserID: "u1", Domain: "idp.example", Audience: "longhouse.example",
	}, "bogus-nonce")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "nonce") {
		t.Errorf("expected nonce mismatch msg: %q", rec.Body.String())
	}
}

func TestCallback_WrongDomain(t *testing.T) {
	pki := &fakePKI{
		signRequest:  func(cb, n string) (string, error) { return "SIGNED", nil },
		decryptToken: func(tok string) (string, error) { return "ASSERT", nil },
	}
	d := testDeps(pki)

	rec := fullLoginFlow(t, d, &linkkeys.Assertion{
		UserID: "u1", Domain: "evil.example", Audience: "longhouse.example",
	}, "")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestCallback_AudienceMismatch(t *testing.T) {
	pki := &fakePKI{
		signRequest:  func(cb, n string) (string, error) { return "SIGNED", nil },
		decryptToken: func(tok string) (string, error) { return "ASSERT", nil },
	}
	d := testDeps(pki)

	rec := fullLoginFlow(t, d, &linkkeys.Assertion{
		UserID: "u1", Domain: "idp.example", Audience: "someone-else.example",
	}, "")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestCallback_MissingLoginState(t *testing.T) {
	pki := &fakePKI{
		decryptToken:    func(tok string) (string, error) { return "ASSERT", nil },
		verifyAssertion: func(sa, d string) (*linkkeys.Assertion, error) { return nil, nil },
	}
	router := NewRouter(testDeps(pki))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/callback?encrypted_token=ENC", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestRequireAuth_RedirectsAnonToLogin(t *testing.T) {
	router := NewRouter(testDeps(&fakePKI{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/dashboard", nil))

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
		t.Errorf("expected redirect to /login, got %d → %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRequireAuth_AllowsValidSession(t *testing.T) {
	d := testDeps(&fakePKI{})
	router := NewRouter(d)

	authRec := httptest.NewRecorder()
	if err := d.Sessions.SetIdentity(authRec, session.Identity{Domain: "idp.example", UserID: "u1"}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/app/dashboard", nil)
	for _, c := range authRec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestLogout_ClearsCookieAndRedirects(t *testing.T) {
	d := testDeps(&fakePKI{})
	router := NewRouter(d)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/logout", nil))

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
		t.Errorf("expected redirect to /login, got %d → %q", rec.Code, rec.Header().Get("Location"))
	}
	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.SessionCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("expected session cookie to be cleared")
	}
}
