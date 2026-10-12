package csilservices

import (
	"context"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type fakeSettingsStore struct {
	store.Store
	rows []models.HouseSetting
}

func (f *fakeSettingsStore) GetHouseSettings(context.Context, string) ([]models.HouseSetting, error) {
	return f.rows, nil
}

func (f *fakeSettingsStore) UpsertHouseSetting(_ context.Context, row *models.HouseSetting) error {
	for i := range f.rows {
		if f.rows[i].Key == row.Key {
			f.rows[i] = *row
			return nil
		}
	}
	f.rows = append(f.rows, *row)
	return nil
}

func adminContext() context.Context {
	return auth.WithIdentity(context.Background(), &auth.Identity{
		Domain: "example.test", UserID: "user-1",
		Houses: []auth.HouseRoles{{House: "house-1", Member: "member-1", Roles: []string{"admin"}}},
	})
}

func TestSettingsDefaultsIncludeLifetimeAndVisibility(t *testing.T) {
	svc := &SettingsService{Store: &fakeSettingsStore{}}
	got, err := svc.GetSettings(adminContext(), "house-1")
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.SessionLifetimeHours == nil || *got.SessionLifetimeHours != 720 {
		t.Fatalf("session lifetime default: %v", got.SessionLifetimeHours)
	}
	if got.DefaultProjectVisibility == nil || *got.DefaultProjectVisibility != "read" {
		t.Fatalf("visibility default: %v", got.DefaultProjectVisibility)
	}
}

func TestUpdateSettingsWritesLifetimeAndVisibility(t *testing.T) {
	svc := &SettingsService{Store: &fakeSettingsStore{}}
	hours := uint64(36)
	none := csil.AccessLevel("none")
	got, err := svc.UpdateSettings(adminContext(), csil.UpdateSettingsRequest{
		HouseId:  "house-1",
		Settings: csil.EffectiveSettings{SessionLifetimeHours: &hours, DefaultProjectVisibility: &none},
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if *got.SessionLifetimeHours != 36 || *got.DefaultProjectVisibility != "none" {
		t.Fatalf("settings after update: %d %s", *got.SessionLifetimeHours, *got.DefaultProjectVisibility)
	}
}

func TestUpdateSettingsRejectsOutOfRangeValues(t *testing.T) {
	svc := &SettingsService{Store: &fakeSettingsStore{}}
	for _, hours := range []uint64{0, 8761} {
		h := hours
		_, err := svc.UpdateSettings(adminContext(), csil.UpdateSettingsRequest{
			HouseId: "house-1", Settings: csil.EffectiveSettings{SessionLifetimeHours: &h},
		})
		if err == nil {
			t.Errorf("session_lifetime_hours %d was accepted", hours)
		}
	}
	bad := csil.AccessLevel("owner")
	if _, err := svc.UpdateSettings(adminContext(), csil.UpdateSettingsRequest{
		HouseId: "house-1", Settings: csil.EffectiveSettings{DefaultProjectVisibility: &bad},
	}); err == nil {
		t.Error("invalid default_project_visibility was accepted")
	}
}
