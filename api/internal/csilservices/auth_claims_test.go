package csilservices

import (
	"context"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// TestReconcileField covers the per-field reconciliation rule: seed when empty,
// track upstream while untouched, preserve a user override, and always advance
// the mirror.
func TestReconcileField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                  string
		field, mirror, claim  string
		claimAbsent           bool
		wantField, wantMirror string
		wantChanged           bool
	}{
		{name: "seed-empty", field: "", mirror: "", claim: "a@x", wantField: "a@x", wantMirror: "a@x", wantChanged: true},
		{name: "track-upstream", field: "a@x", mirror: "a@x", claim: "b@x", wantField: "b@x", wantMirror: "b@x", wantChanged: true},
		{name: "user-override-preserved", field: "mine@x", mirror: "a@x", claim: "b@x", wantField: "mine@x", wantMirror: "b@x", wantChanged: true},
		{name: "noop-when-equal", field: "a@x", mirror: "a@x", claim: "a@x", wantField: "a@x", wantMirror: "a@x", wantChanged: false},
		{name: "claim-not-released", field: "a@x", mirror: "a@x", claimAbsent: true, wantField: "a@x", wantMirror: "a@x", wantChanged: false},
		{name: "cleared-override-resyncs", field: "", mirror: "old@x", claim: "new@x", wantField: "new@x", wantMirror: "new@x", wantChanged: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims := map[string]string{}
			if !tc.claimAbsent {
				claims["email"] = tc.claim
			}
			field, mirror := tc.field, tc.mirror
			got := reconcileField(&field, &mirror, claims, "email")
			if got != tc.wantChanged || field != tc.wantField || mirror != tc.wantMirror {
				t.Fatalf("reconcileField = (field=%q mirror=%q changed=%v), want (field=%q mirror=%q changed=%v)",
					field, mirror, got, tc.wantField, tc.wantMirror, tc.wantChanged)
			}
		})
	}
}

// reconcileStore is a Store double that serves member rows for one identity and
// captures UpdateMember writes.
type reconcileStore struct {
	store.Store
	members []models.Member
	updated []models.Member
}

func (s *reconcileStore) FindMembersByLinkkeysIdentity(context.Context, string, string) ([]models.Member, error) {
	return s.members, nil
}

func (s *reconcileStore) UpdateMember(_ context.Context, m *models.Member) error {
	s.updated = append(s.updated, *m)
	return nil
}

// TestReconcileMemberClaims_AcrossHousesAndOverride checks the full pass: a
// granted claim seeds one house's row and tracks-then-respects an override in
// another, and a nil claim set is a no-op.
func TestReconcileMemberClaims_AcrossHousesAndOverride(t *testing.T) {
	t.Parallel()
	st := &reconcileStore{members: []models.Member{
		// House A: never touched — should seed email + avatar.
		{MemberID: "mA", HouseID: "hA"},
		// House B: user overrode display_name (field != mirror); claim must not
		// clobber it, but the mirror must advance.
		{MemberID: "mB", HouseID: "hB", DisplayName: "Mine", DisplayNameClaimed: "Old"},
	}}
	claims := map[string]string{
		"display_name": "Ada",
		"email":        "ada@example.com",
		"avatar_url":   "https://idp/ada.png",
	}
	if err := reconcileMemberClaims(context.Background(), st, "d", "u", claims); err != nil {
		t.Fatalf("reconcileMemberClaims: %v", err)
	}
	if len(st.updated) != 2 {
		t.Fatalf("expected both rows updated, got %d", len(st.updated))
	}
	byID := map[string]models.Member{}
	for _, m := range st.updated {
		byID[m.MemberID] = m
	}
	a := byID["mA"]
	if a.DisplayName != "Ada" || a.Email != "ada@example.com" || a.AvatarURL != "https://idp/ada.png" {
		t.Fatalf("house A not seeded from claims: %+v", a)
	}
	if a.DisplayNameClaimed != "Ada" || a.EmailClaimed != "ada@example.com" {
		t.Fatalf("house A mirrors not set: %+v", a)
	}
	b := byID["mB"]
	if b.DisplayName != "Mine" {
		t.Fatalf("house B clobbered a user override: display_name=%q", b.DisplayName)
	}
	if b.DisplayNameClaimed != "Ada" {
		t.Fatalf("house B mirror should advance to upstream, got %q", b.DisplayNameClaimed)
	}
}

func TestReconcileMemberClaims_NilClaimsNoop(t *testing.T) {
	t.Parallel()
	st := &reconcileStore{members: []models.Member{{MemberID: "m1"}}}
	if err := reconcileMemberClaims(context.Background(), st, "d", "u", nil); err != nil {
		t.Fatal(err)
	}
	if len(st.updated) != 0 {
		t.Fatalf("nil claims must not write, got %d updates", len(st.updated))
	}
}

// fakeFetcherPKI implements PKIClient + UserInfoFetcher to drive fetchClaims.
type fakeFetcherPKI struct {
	info *linkkeys.UserInfo
	err  error
}

func (fakeFetcherPKI) SignRequest(string, string) (string, error) { return "", nil }
func (fakeFetcherPKI) DecryptToken(string) (string, error)        { return "", nil }
func (fakeFetcherPKI) VerifyAssertion(string, string) (*linkkeys.Assertion, error) {
	return &linkkeys.Assertion{}, nil
}
func (p fakeFetcherPKI) FetchUserInfo(string, string) (*linkkeys.UserInfo, error) {
	return p.info, p.err
}

func TestFetchClaims_FoldsDisplayNameAndResolves(t *testing.T) {
	t.Parallel()
	svc := &AuthService{PKI: fakeFetcherPKI{info: &linkkeys.UserInfo{
		DisplayName: "Ada Lovelace", // arrives out of band, not as a claim
		Claims:      []linkkeys.Claim{{ClaimType: "email", ClaimValue: []byte("ada@x")}},
	}}}
	a := &linkkeys.Assertion{Domain: "d", DisplayName: "fallback"}
	claims := svc.fetchClaims("tok", a)
	if claims["email"] != "ada@x" {
		t.Fatalf("email claim missing: %v", claims)
	}
	if claims["display_name"] != "Ada Lovelace" {
		t.Fatalf("display_name should be folded from UserInfo envelope: %v", claims)
	}
	if got := resolveDisplayName(claims, a); got != "Ada Lovelace" {
		t.Fatalf("resolveDisplayName = %q, want claim value", got)
	}
}

func TestFetchClaims_FallsBackWhenNoFetcher(t *testing.T) {
	t.Parallel()
	// fakeCompletePKI (auth_complete_test.go) is a PKIClient but NOT a fetcher.
	svc := &AuthService{PKI: fakeCompletePKI{}}
	a := &linkkeys.Assertion{DisplayName: "assertion-name"}
	if claims := svc.fetchClaims("tok", a); claims != nil {
		t.Fatalf("expected nil claims without a fetcher, got %v", claims)
	}
	if got := resolveDisplayName(nil, a); got != "assertion-name" {
		t.Fatalf("resolveDisplayName fallback = %q", got)
	}
}
