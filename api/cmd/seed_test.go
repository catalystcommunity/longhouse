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
	houses       []models.House
	members      []models.Member
	roles        []models.Role
	memberRoles  []models.MemberRole
	memberAudits []models.MemberAudit
	idSeq        int
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

	if len(fs.memberAudits) != 2 {
		t.Errorf("want 2 audit entries (one per role grant), got %d", len(fs.memberAudits))
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
