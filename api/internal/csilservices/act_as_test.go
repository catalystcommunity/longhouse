package csilservices

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/actas"
	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type delegationProvider struct {
	verified             actas.Verified
	verifyErr, revokeErr error
	revoked              []string
}

func (p *delegationProvider) SignScopeSet(context.Context, actas.ScopeSetRequest) ([]byte, error) {
	return nil, nil
}
func (p *delegationProvider) VerifyCredential(context.Context, []byte, []string) (actas.Verified, error) {
	return p.verified, p.verifyErr
}
func (p *delegationProvider) RevokedGrants(context.Context, string, []string) ([]string, error) {
	return p.revoked, p.revokeErr
}
func (p *delegationProvider) MaxPresentationAge() time.Duration { return 7 * time.Minute }

type delegationStore struct {
	fakeCliAuthStore
	nonces   map[string]bool
	members  []models.Member
	roles    []models.Role
	audits   []*models.AuditEntry
	nonceErr error
}

func (s *delegationStore) ConsumeActAsNonce(_ context.Context, domain, grant, nonce string, expires time.Time) (bool, error) {
	if s.nonceErr != nil {
		return false, s.nonceErr
	}
	if !expires.After(time.Now().Add(6 * time.Minute)) {
		return false, errors.New("nonce expires too soon")
	}
	key := domain + "/" + grant + "/" + nonce
	if s.nonces[key] {
		return false, nil
	}
	s.nonces[key] = true
	return true, nil
}
func (s *delegationStore) FindMembersByLinkkeysIdentity(context.Context, string, string) ([]models.Member, error) {
	return s.members, nil
}
func (s *delegationStore) ListRolesForMember(context.Context, string) ([]models.Role, error) {
	return s.roles, nil
}
func (s *delegationStore) RecordAuditEntry(_ context.Context, e *models.AuditEntry) error {
	s.audits = append(s.audits, e)
	return nil
}

func delegationFixture() (*AuthService, *delegationProvider, *delegationStore) {
	p := &delegationProvider{verified: actas.Verified{GrantID: "grant", SubjectDomain: "home.test", SubjectUserID: "user", Grantee: "owner@app.test/helper", Scopes: []string{"tasks.read", "tasks.write", "projects.write"}, ExpiresAt: time.Now().Add(time.Hour), Nonce: "nonce"}}
	st := &delegationStore{nonces: map[string]bool{}, members: []models.Member{{HouseID: "house", MemberID: "member"}}, roles: []models.Role{{Name: "member"}}}
	return &AuthService{ActAs: p, Store: st, JWTSecret: []byte("test")}, p, st
}

func TestActAsExchangeRetainsLimitsAndRejectsReplay(t *testing.T) {
	s, p, st := delegationFixture()
	p.verified.ExpiresAt = time.Now().Add(45 * time.Second)
	response, err := s.ExchangeActAsCredential(context.Background(), csil.ExchangeActAsCredentialRequest{Credential: []byte{1}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := auth.Verify(s.JWTSecret, response.Token)
	if err != nil {
		t.Fatal(err)
	}
	if id.Delegation == nil || id.Delegation.Grantee != p.verified.Grantee || id.Domain != "home.test" || id.UserID != "user" || len(id.Houses) != 1 {
		t.Fatalf("lost identity: %+v", id)
	}
	if id.ExpiresAt > p.verified.ExpiresAt.Unix() || id.CliSessionID != "" || id.BrowserSessionID != "" {
		t.Fatalf("authority escaped grant: %+v", id)
	}
	if !id.AllowsDelegatedMethod("project", "CreateProject") || id.AllowsDelegatedMethod("house", "DeleteHouse") {
		t.Fatal("wrong scopes")
	}
	if len(st.audits) != 1 || st.audits[0].Detail["act_as_grantee"] != p.verified.Grantee {
		t.Fatal("missing app audit identity")
	}
	if _, err = s.ExchangeActAsCredential(context.Background(), csil.ExchangeActAsCredentialRequest{}); err == nil {
		t.Fatal("replay accepted")
	}
	ctx := auth.WithIdentity(context.Background(), id)
	if _, err = s.Refresh(ctx, csil.EmptyRequest{}); err == nil {
		t.Fatal("delegation became browser session")
	}
	if _, err = s.ApproveCliLogin(ctx, csil.ApproveCliLoginRequest{}); err == nil {
		t.Fatal("delegation approved CLI")
	}
}

func TestActAsExchangeFailsClosed(t *testing.T) {
	for _, kind := range []string{"disabled", "no secret", "signature", "expired", "no scopes", "nonce store"} {
		t.Run(kind, func(t *testing.T) {
			s, p, st := delegationFixture()
			switch kind {
			case "disabled":
				s.ActAs = nil
			case "no secret":
				s.JWTSecret = nil
			case "signature":
				p.verifyErr = errors.New("bad signature")
			case "expired":
				p.verified.ExpiresAt = time.Now().Add(-time.Second)
			case "no scopes":
				p.verified.Scopes = []string{"unknown"}
			case "nonce store":
				st.nonceErr = errors.New("database down")
			}
			if _, err := s.ExchangeActAsCredential(context.Background(), csil.ExchangeActAsCredentialRequest{}); err == nil {
				t.Fatal("accepted invalid grant")
			}
		})
	}
}

func TestActAsLiveMembershipAndRevocation(t *testing.T) {
	s, p, st := delegationFixture()
	id := &auth.Identity{Domain: "home.test", UserID: "user", Houses: []auth.HouseRoles{{House: "old", Roles: []string{"admin"}}}, Delegation: &auth.Delegation{GrantID: "grant", Grantee: "app"}}
	if err := s.VerifyDelegation(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if len(id.Houses) != 1 || id.Houses[0].House != "house" || id.Houses[0].Roles[0] != "member" {
		t.Fatalf("stale roles: %+v", id.Houses)
	}
	now := time.Now()
	st.members[0].DeactivatedAt = &now
	if err := s.VerifyDelegation(context.Background(), id); err != nil || len(id.Houses) != 0 {
		t.Fatalf("deactivated membership: %+v, %v", id.Houses, err)
	}
	p.revoked = []string{"grant"}
	if err := s.VerifyDelegation(context.Background(), id); err == nil {
		t.Fatal("revocation ignored")
	}
	p.revoked = nil
	p.revokeErr = errors.New("RP unavailable")
	if err := s.VerifyDelegation(context.Background(), id); err == nil {
		t.Fatal("RP failure ignored")
	}
}
