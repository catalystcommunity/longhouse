package csilservices

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type loginStore struct {
	fakeAuthStore
	trusted bool
	err     error
}

func (s *loginStore) HousesTrustingDomain(context.Context, string) ([]models.House, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.trusted {
		return []models.House{{HouseID: "one"}, {HouseID: "two"}}, nil
	}
	return nil, nil
}
func (*loginStore) FindMembersByLinkkeysIdentity(context.Context, string, string) ([]models.Member, error) {
	return []models.Member{{HouseID: "one", MemberID: "m1"}, {HouseID: "two", MemberID: "m2"}}, nil
}
func (*loginStore) ListRolesForMember(context.Context, string) ([]models.Role, error) {
	return []models.Role{{Name: models.RoleMember}}, nil
}

func (*loginStore) GetHouseSettings(context.Context, string) ([]models.HouseSetting, error) {
	return nil, nil
}

type loginResolver struct {
	calls   int
	records []string
	err     error
}

func (r *loginResolver) LookupTXT(context.Context, string) ([]string, error) {
	r.calls++
	return r.records, r.err
}

type loginPKI struct {
	nonce, expected string
	assertion       *linkkeys.Assertion
	calls           int
	err             error
}

func (p *loginPKI) SignRequest(_ string, nonce string) (string, error) {
	p.nonce = nonce
	p.calls++
	return "signed", p.err
}
func (p *loginPKI) DecryptToken(string) (string, error) { return "assertion", nil }
func (p *loginPKI) VerifyAssertion(_ string, domain string) (*linkkeys.Assertion, error) {
	p.expected = domain
	return p.assertion, nil
}

func loginService() (*AuthService, *loginStore, *loginResolver, *loginPKI) {
	st := &loginStore{trusted: true}
	r := &loginResolver{records: []string{"v=lk1 https=https://idp.example.com/prefix"}}
	p := &loginPKI{}
	return &AuthService{Store: st, PKI: p, LoginResolver: r, JWTSecret: []byte("test-secret"), IDPDomain: "home.example", RPDomain: "home.example", IDPURL: "https://home.example", CallbackURL: "https://app.example/auth/callback"}, st, r, p
}
func loginContext(cookie *http.Cookie) context.Context {
	r := httptest.NewRequest(http.MethodPost, "https://app.example/api/csil/v1/rpc", nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	return csilrpc.WithHTTPRequest(context.Background(), r)
}
func beginLogin(t *testing.T, s *AuthService, login *string) (csil.BeginBrowserLoginResponse, *http.Cookie) {
	t.Helper()
	ctx := loginContext(nil)
	resp, err := s.BeginBrowserLogin(ctx, csil.BeginBrowserLoginRequest{Login: login})
	if err != nil {
		t.Fatal(err)
	}
	cookies := csilrpc.QueuedCookies(ctx)
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].Path != "/api" {
		t.Fatalf("bad login cookie: %#v", cookies)
	}
	return resp, cookies[0]
}

func TestBrowserLoginDefaultAndExternal(t *testing.T) {
	for _, external := range []bool{false, true} {
		s, _, r, p := loginService()
		var login *string
		if external {
			v := "Foo+bar@EXAMPLE.com"
			login = &v
		}
		resp, cookie := beginLogin(t, s, login)
		u, err := url.Parse(resp.RedirectUrl)
		if err != nil {
			t.Fatal(err)
		}
		domain := "home.example"
		if external {
			domain = "example.com"
			if r.calls != 1 || u.Host != "idp.example.com" || u.Path != "/prefix/auth/authorize" || u.Query().Get("username") != "Foo+bar" {
				t.Fatal(resp.RedirectUrl)
			}
		} else if r.calls != 0 || u.Host != "home.example" || u.Query().Has("username") {
			t.Fatal(resp.RedirectUrl)
		}
		p.assertion = &linkkeys.Assertion{Domain: domain, Nonce: p.nonce, UserID: "user", Audience: s.RPDomain}
		ctx := loginContext(cookie)
		result, err := s.Complete(ctx, csil.CompleteRequest{EncryptedToken: "sealed"})
		if err != nil {
			t.Fatal(err)
		}
		id, err := auth.Verify(s.JWTSecret, result.Token)
		if err != nil || id.Domain != domain || len(id.Houses) != 2 || p.expected != domain {
			t.Fatalf("identity=%+v err=%v expected=%s", id, err, p.expected)
		}
		if csilrpc.QueuedCookies(ctx)[0].MaxAge != -1 {
			t.Fatal("login cookie was not cleared")
		}
	}
}

func TestBrowserLoginRejectsBeforeRedirect(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*loginStore, *loginResolver, *loginPKI)
		message   string
		dns       int
	}{
		{"untrusted", func(st *loginStore, _ *loginResolver, _ *loginPKI) { st.trusted = false }, "can't sign in with that domain", 0},
		{"store error", func(st *loginStore, _ *loginResolver, _ *loginPKI) { st.err = errors.New("db") }, "could not check domain", 0},
		{"no endpoint", func(_ *loginStore, r *loginResolver, _ *loginPKI) { r.records = nil }, "no public LinkKeys", 1},
		{"DNS error", func(_ *loginStore, r *loginResolver, _ *loginPKI) { r.err = context.DeadlineExceeded }, "could not look up", 1},
		{"RP error", func(_ *loginStore, _ *loginResolver, p *loginPKI) { p.err = errors.New("rp") }, "could not reach", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, st, r, p := loginService()
			tc.configure(st, r, p)
			login := "foo@example.com"
			ctx := loginContext(nil)
			_, err := s.BeginBrowserLogin(ctx, csil.BeginBrowserLoginRequest{Login: &login})
			if err == nil || !strings.Contains(err.Error(), tc.message) || r.calls != tc.dns || len(csilrpc.QueuedCookies(ctx)) != 0 {
				t.Fatalf("err=%v dns=%d", err, r.calls)
			}
			if tc.dns == 0 && p.calls != 0 {
				t.Fatal("called RP before trust check")
			}
		})
	}
}

func TestBrowserCompleteRejectsChangedLogin(t *testing.T) {
	for _, tc := range []string{"missing cookie", "tampered cookie", "wrong domain", "wrong nonce", "wrong audience", "trust removed"} {
		t.Run(tc, func(t *testing.T) {
			s, st, _, p := loginService()
			login := "foo@example.com"
			_, cookie := beginLogin(t, s, &login)
			p.assertion = &linkkeys.Assertion{Domain: "example.com", Nonce: p.nonce, UserID: "user", Audience: s.RPDomain}
			switch tc {
			case "missing cookie":
				cookie = nil
			case "tampered cookie":
				cookie.Value = "changed" + cookie.Value
			case "wrong domain":
				p.assertion.Domain = "other.example"
			case "wrong nonce":
				p.assertion.Nonce = auth.MintNonce(s.JWTSecret)
			case "wrong audience":
				p.assertion.Audience = "other.example"
			case "trust removed":
				st.trusted = false
			}
			if _, err := s.Complete(loginContext(cookie), csil.CompleteRequest{EncryptedToken: "sealed"}); err == nil {
				t.Fatal("accepted changed login")
			}
		})
	}
}
