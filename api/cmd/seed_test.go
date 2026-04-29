package cmd

import (
	"context"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

const testAdminUUID = "01956bfa-1234-7abc-89de-0123456789ab"

type fakeStore struct {
	store.Store
	houses  []models.House
	members []models.Member
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
	h.HouseID = "house-1"
	f.houses = append(f.houses, *h)
	return nil
}

func (f *fakeStore) CreateMember(_ context.Context, m *models.Member) error {
	m.MemberID = "member-1"
	f.members = append(f.members, *m)
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
	hasAdmin := false
	for _, r := range m.Roles {
		if r == "admin" {
			hasAdmin = true
		}
	}
	if !hasAdmin {
		t.Errorf("member missing admin role: %v", m.Roles)
	}
}

func TestSeedInitialAdmin_NoOpWhenConfigMissing(t *testing.T) {
	fs := &fakeStore{}
	withStore(t, fs)
	withConfig(t, "", "", "Longhouse")

	if err := SeedInitialAdmin(); err != nil {
		t.Fatalf("SeedInitialAdmin: %v", err)
	}
	if len(fs.houses) != 0 || len(fs.members) != 0 {
		t.Errorf("expected no writes; got %d houses, %d members", len(fs.houses), len(fs.members))
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
			if len(fs.houses) != 0 || len(fs.members) != 0 {
				t.Errorf("expected no writes for invalid UUID %q; got %d houses, %d members", userID, len(fs.houses), len(fs.members))
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
	if len(fs.houses) != 1 || len(fs.members) != 1 {
		t.Errorf("expected no new writes; got %d houses, %d members", len(fs.houses), len(fs.members))
	}
}
