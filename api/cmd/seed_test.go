package cmd

import (
	"context"
	"strconv"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

const testAdminUUID = "01956bfa-1234-7abc-89de-0123456789ab"

type fakeStore struct {
	store.Store
	houses         []models.House
	members        []models.Member
	roles          []models.Role
	memberRoles    []models.MemberRole
	memberAudits   []models.MemberAudit
	trustedDomains []models.TrustedDomain
	idSeq          int
}

func (f *fakeStore) nextID(prefix string) string {
	f.idSeq++
	return prefix + strconv.Itoa(f.idSeq)
}

func (f *fakeStore) ListHouses(_ context.Context, limit, _ int) ([]models.House, error) {
	out := f.houses
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) ListMembersByHouse(_ context.Context, houseID string, limit, _ int) ([]models.Member, error) {
	var out []models.Member
	for _, m := range f.members {
		if m.HouseID == houseID {
			out = append(out, m)
		}
	}
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) CreateHouse(_ context.Context, h *models.House) error {
	h.HouseID = f.nextID("house-")
	f.houses = append(f.houses, *h)
	return nil
}

func (f *fakeStore) CreateMember(_ context.Context, m *models.Member) error {
	m.MemberID = f.nextID("member-")
	f.members = append(f.members, *m)
	return nil
}

func (f *fakeStore) CreateRole(_ context.Context, r *models.Role) error {
	r.RoleID = f.nextID("role-")
	f.roles = append(f.roles, *r)
	return nil
}

func (f *fakeStore) AssignRole(_ context.Context, memberID, roleID string) error {
	f.memberRoles = append(f.memberRoles, models.MemberRole{MemberID: memberID, RoleID: roleID})
	return nil
}

func (f *fakeStore) RecordMemberAudit(_ context.Context, a *models.MemberAudit) error {
	a.AuditID = f.nextID("audit-")
	f.memberAudits = append(f.memberAudits, *a)
	return nil
}

func (f *fakeStore) CreateTrustedDomain(_ context.Context, td *models.TrustedDomain) error {
	td.TrustedDomainID = f.nextID("td-")
	f.trustedDomains = append(f.trustedDomains, *td)
	return nil
}

func (f *fakeStore) FindMembersByLinkkeysIdentity(_ context.Context, domain, userID string) ([]models.Member, error) {
	var out []models.Member
	for _, m := range f.members {
		if m.LinkkeysDomain == domain && m.LinkkeysUserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeStore) ListTrustedDomains(_ context.Context, houseID string) ([]models.TrustedDomain, error) {
	var out []models.TrustedDomain
	for _, td := range f.trustedDomains {
		if td.HouseID == houseID {
			out = append(out, td)
		}
	}
	return out, nil
}

func (f *fakeStore) ListRolesForMember(_ context.Context, memberID string) ([]models.Role, error) {
	var out []models.Role
	for _, mr := range f.memberRoles {
		if mr.MemberID != memberID {
			continue
		}
		if r := f.roleByID(mr.RoleID); r != nil {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeStore) roleByID(id string) *models.Role {
	for i := range f.roles {
		if f.roles[i].RoleID == id {
			return &f.roles[i]
		}
	}
	return nil
}

func withStore(t *testing.T, s store.Store) {
	t.Helper()
	prev := store.AppStore
	store.AppStore = s
	t.Cleanup(func() { store.AppStore = prev })
}

func withConfig(t *testing.T, domain, userID, houseName string) {
	t.Helper()
	prevD, prevU, prevH := config.InitialAdminDomain, config.InitialAdminUserID, config.InitialHouseName
	config.InitialAdminDomain = domain
	config.InitialAdminUserID = userID
	config.InitialHouseName = houseName
	t.Cleanup(func() {
		config.InitialAdminDomain = prevD
		config.InitialAdminUserID = prevU
		config.InitialHouseName = prevH
	})
}

func TestSeedInitialAdmin_CreatesHouseAndAdmin(t *testing.T) {
	fs := &fakeStore{}
	withStore(t, fs)
	withConfig(t, "todandlorna.com", testAdminUUID, "Test House")

	if err := SeedInitialAdmin(); err != nil {
		t.Fatalf("SeedInitialAdmin: %v", err)
	}
	if len(fs.houses) != 1 {
		t.Fatalf("want 1 house, got %d", len(fs.houses))
	}
	if fs.houses[0].Name != "Test House" {
		t.Errorf("house name: got %q, want %q", fs.houses[0].Name, "Test House")
	}
	if len(fs.members) != 1 {
		t.Fatalf("want 1 member, got %d", len(fs.members))
	}
	m := fs.members[0]
	if m.LinkkeysDomain != "todandlorna.com" || m.LinkkeysUserID != testAdminUUID {
		t.Errorf("identity mismatch: got %q / %q", m.LinkkeysDomain, m.LinkkeysUserID)
	}

	gotRoles := map[string]bool{}
	for _, r := range fs.roles {
		gotRoles[r.Name] = true
	}
	if !gotRoles[models.RoleAdmin] || !gotRoles[models.RoleMember] {
		t.Errorf("want both admin and member roles created; got %+v", gotRoles)
	}

	assignedNames := map[string]bool{}
	for _, mr := range fs.memberRoles {
		if mr.MemberID != m.MemberID {
			t.Errorf("role assigned to unexpected member %q", mr.MemberID)
		}
		if r := fs.roleByID(mr.RoleID); r != nil {
			assignedNames[r.Name] = true
		}
	}
	if !assignedNames[models.RoleAdmin] || !assignedNames[models.RoleMember] {
		t.Errorf("want admin and member assigned to the bootstrap member; got %+v", assignedNames)
	}

	// 2 role-grant audits + 1 trusted-domain-added audit (the bootstrap
	// seeds the admin's own linkkeys domain into trusted_domains so any
	// additional identities from that domain auto-join on first sign-in).
	if len(fs.memberAudits) != 3 {
		t.Errorf("want 3 audit entries (2 role grants + 1 trusted domain seed), got %d", len(fs.memberAudits))
	}
	if len(fs.trustedDomains) != 1 || fs.trustedDomains[0].Domain != "todandlorna.com" {
		t.Errorf("want a trusted_domain row for todandlorna.com; got %+v", fs.trustedDomains)
	}
}

func TestSeedInitialAdmin_NoOpWhenConfigMissing(t *testing.T) {
	fs := &fakeStore{}
	withStore(t, fs)
	withConfig(t, "", "", "Longhouse")

	if err := SeedInitialAdmin(); err != nil {
		t.Fatalf("SeedInitialAdmin: %v", err)
	}
	if len(fs.houses) != 0 || len(fs.members) != 0 || len(fs.roles) != 0 || len(fs.memberRoles) != 0 {
		t.Errorf("expected no writes; got %d houses, %d members, %d roles, %d assignments",
			len(fs.houses), len(fs.members), len(fs.roles), len(fs.memberRoles))
	}
}

func TestSeedInitialAdmin_NoOpOnInvalidUUID(t *testing.T) {
	cases := []string{
		"tod",
		"tod@todandlorna.com",
		"not-a-uuid",
		"01956bfa-1234-7abc-89de",
		"01956bfa12347abc89de0123456789ab",
	}
	for _, userID := range cases {
		t.Run(userID, func(t *testing.T) {
			fs := &fakeStore{}
			withStore(t, fs)
			withConfig(t, "todandlorna.com", userID, "Longhouse")

			if err := SeedInitialAdmin(); err != nil {
				t.Fatalf("SeedInitialAdmin: %v", err)
			}
			if len(fs.houses) != 0 || len(fs.members) != 0 || len(fs.roles) != 0 {
				t.Errorf("expected no writes for invalid UUID %q; got %d houses, %d members, %d roles",
					userID, len(fs.houses), len(fs.members), len(fs.roles))
			}
		})
	}
}

func TestSeedInitialAdmin_NoOpWhenAlreadyBootstrapped(t *testing.T) {
	fs := &fakeStore{
		houses:  []models.House{{HouseID: "existing-house", Name: "Existing"}},
		members: []models.Member{{MemberID: "existing-member", HouseID: "existing-house", LinkkeysDomain: "other.com", LinkkeysUserID: "someone"}},
	}
	withStore(t, fs)
	withConfig(t, "todandlorna.com", testAdminUUID, "Longhouse")

	if err := SeedInitialAdmin(); err != nil {
		t.Fatalf("SeedInitialAdmin: %v", err)
	}
	if len(fs.houses) != 1 || len(fs.members) != 1 || len(fs.roles) != 0 {
		t.Errorf("expected no new writes; got %d houses, %d members, %d roles",
			len(fs.houses), len(fs.members), len(fs.roles))
	}
}

// adminFakeStore builds a fakeStore where the configured bootstrap admin
// already has a member row + admin role in a house, but (by default) no
// trusted_domains row — the exact state of a house bootstrapped before the
// trusted-domain seeding existed.
func adminFakeStore() *fakeStore {
	adminRole := models.Role{RoleID: "role-admin", HouseID: "founding-house", Name: models.RoleAdmin}
	memberRole := models.Role{RoleID: "role-member", HouseID: "founding-house", Name: models.RoleMember}
	return &fakeStore{
		houses:  []models.House{{HouseID: "founding-house", Name: "Founding"}},
		members: []models.Member{{MemberID: "admin-member", HouseID: "founding-house", LinkkeysDomain: "todandlorna.com", LinkkeysUserID: testAdminUUID}},
		roles:   []models.Role{adminRole, memberRole},
		memberRoles: []models.MemberRole{
			{MemberID: "admin-member", RoleID: "role-admin"},
			{MemberID: "admin-member", RoleID: "role-member"},
		},
		idSeq: 100,
	}
}

func TestEnsureInitialTrustedDomain_BackfillsMissingRow(t *testing.T) {
	fs := adminFakeStore()
	withStore(t, fs)
	withConfig(t, "todandlorna.com", testAdminUUID, "Founding")

	if err := EnsureInitialTrustedDomain(); err != nil {
		t.Fatalf("EnsureInitialTrustedDomain: %v", err)
	}
	if len(fs.trustedDomains) != 1 {
		t.Fatalf("want 1 trusted domain, got %d", len(fs.trustedDomains))
	}
	if fs.trustedDomains[0].Domain != "todandlorna.com" || fs.trustedDomains[0].HouseID != "founding-house" {
		t.Errorf("unexpected trusted domain row: %+v", fs.trustedDomains[0])
	}
	// One trusted-domain-added audit recorded.
	if len(fs.memberAudits) != 1 || fs.memberAudits[0].Action != models.AuditActionTrustedDomainAdded {
		t.Errorf("want a single trusted_domain_added audit, got %+v", fs.memberAudits)
	}
}

func TestEnsureInitialTrustedDomain_IdempotentWhenPresent(t *testing.T) {
	fs := adminFakeStore()
	fs.trustedDomains = []models.TrustedDomain{{TrustedDomainID: "td-existing", HouseID: "founding-house", Domain: "todandlorna.com"}}
	withStore(t, fs)
	withConfig(t, "todandlorna.com", testAdminUUID, "Founding")

	if err := EnsureInitialTrustedDomain(); err != nil {
		t.Fatalf("EnsureInitialTrustedDomain: %v", err)
	}
	if len(fs.trustedDomains) != 1 {
		t.Errorf("want no new trusted-domain rows, got %d", len(fs.trustedDomains))
	}
	if len(fs.memberAudits) != 0 {
		t.Errorf("want no audits on a no-op, got %d", len(fs.memberAudits))
	}
}

func TestEnsureInitialTrustedDomain_SkipsNonAdminHouse(t *testing.T) {
	// Admin identity is only a plain member here — we must not auto-trust
	// the whole domain on a house they merely joined.
	fs := adminFakeStore()
	fs.memberRoles = []models.MemberRole{{MemberID: "admin-member", RoleID: "role-member"}}
	withStore(t, fs)
	withConfig(t, "todandlorna.com", testAdminUUID, "Founding")

	if err := EnsureInitialTrustedDomain(); err != nil {
		t.Fatalf("EnsureInitialTrustedDomain: %v", err)
	}
	if len(fs.trustedDomains) != 0 {
		t.Errorf("want no trusted-domain rows for a non-admin membership, got %d", len(fs.trustedDomains))
	}
}

func TestEnsureInitialTrustedDomain_NoOpWhenConfigMissing(t *testing.T) {
	fs := adminFakeStore()
	withStore(t, fs)
	withConfig(t, "", "", "Founding")

	if err := EnsureInitialTrustedDomain(); err != nil {
		t.Fatalf("EnsureInitialTrustedDomain: %v", err)
	}
	if len(fs.trustedDomains) != 0 {
		t.Errorf("want no writes when config missing, got %d", len(fs.trustedDomains))
	}
}
