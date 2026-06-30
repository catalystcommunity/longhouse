package csilservices

import (
	"context"
	"encoding/json"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// SettingsService backs the SPA's per-house settings dictionary. Every
// supported key is enumerated below with its write-policy (house-admin or
// self) and its value shape; the CSIL spec is the source of truth, this
// file is the policy mirror.
type SettingsService struct{ Store store.Store }

const (
	settingBugReportsEnabled        = "bug_reports_enabled"
	settingBugReportsProjectID      = "bug_reports_project_id"
	settingDefaultProjectVisibility = "default_project_visibility"
)

func (s *SettingsService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("settings", "GetSettings", csilrpc.Route(s.GetSettings, csil.DecodeSettingsGetSettingsRequest, csil.EncodeSettingsGetSettingsResponse))
	d.RegisterTyped("settings", "UpdateSettings", csilrpc.Route(s.UpdateSettings, csil.DecodeSettingsUpdateSettingsRequest, csil.EncodeSettingsUpdateSettingsResponse))
}

// GetSettings returns the merged effective settings for a house: defaults
// filled in for any key not present in house_settings. Any member of the
// house may read — settings drive client UI so every authenticated member
// needs the merged dictionary.
func (s *SettingsService) GetSettings(ctx context.Context, houseID csil.HouseID) (csil.EffectiveSettings, error) {
	if _, _, err := requireMemberForHouse(ctx, string(houseID)); err != nil {
		return csil.EffectiveSettings{}, err
	}
	return s.loadEffective(ctx, string(houseID))
}

// UpdateSettings applies a partial update. Only fields present in the
// inbound EffectiveSettings (non-nil) are written. Every house-layer key
// requires the admin role on house_id; the per-key check is centralized
// here so callers can't bypass it by stuffing a field into the partial.
func (s *SettingsService) UpdateSettings(ctx context.Context, req csil.UpdateSettingsRequest) (csil.EffectiveSettings, error) {
	if req.HouseId == "" {
		return csil.EffectiveSettings{}, csilrpc.BadRequest("house_id is required")
	}
	_, memberID, err := requireRoleForHouse(ctx, string(req.HouseId), "admin")
	if err != nil {
		return csil.EffectiveSettings{}, err
	}

	if req.Settings.BugReportsEnabled != nil {
		if err := s.writeKey(ctx, string(req.HouseId), settingBugReportsEnabled, *req.Settings.BugReportsEnabled, memberID); err != nil {
			return csil.EffectiveSettings{}, err
		}
	}
	if req.Settings.BugReportsProjectId != nil {
		if err := s.writeKey(ctx, string(req.HouseId), settingBugReportsProjectID, string(*req.Settings.BugReportsProjectId), memberID); err != nil {
			return csil.EffectiveSettings{}, err
		}
	}
	return s.loadEffective(ctx, string(req.HouseId))
}

// loadEffective reads every stored setting for the house and decodes the
// known keys into an EffectiveSettings response. Unknown keys (rows from
// an older or newer schema) are ignored — forward/backward compatible.
func (s *SettingsService) loadEffective(ctx context.Context, houseID string) (csil.EffectiveSettings, error) {
	rows, err := s.Store.GetHouseSettings(ctx, houseID)
	if err != nil {
		return csil.EffectiveSettings{}, csilrpc.Internal("internal error")
	}
	out := csil.EffectiveSettings{}
	defaultFalse := false
	out.BugReportsEnabled = &defaultFalse
	for _, r := range rows {
		switch r.Key {
		case settingBugReportsEnabled:
			var v bool
			if err := json.Unmarshal(r.Value, &v); err == nil {
				bv := v
				out.BugReportsEnabled = &bv
			}
		case settingBugReportsProjectID:
			var v string
			if err := json.Unmarshal(r.Value, &v); err == nil && v != "" {
				pid := csil.ProjectID(v)
				out.BugReportsProjectId = &pid
			}
		}
	}
	return out, nil
}

// writeKey JSON-encodes the value and upserts the row.
func (s *SettingsService) writeKey(ctx context.Context, houseID, key string, value any, updatedBy string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return csilrpc.Internal("internal error")
	}
	row := &models.HouseSetting{
		HouseID: houseID,
		Key:     key,
		Value:   raw,
	}
	if updatedBy != "" {
		row.UpdatedBy = &updatedBy
	}
	if err := s.Store.UpsertHouseSetting(ctx, row); err != nil {
		return csilrpc.Internal("internal error")
	}
	return nil
}

// readHouseSetting is a small helper for other services (BugService) that
// need to consult a single key without rebuilding the EffectiveSettings
// response. Returns (rawValue, present).
func readHouseSetting(ctx context.Context, st store.Store, houseID, key string) ([]byte, bool, error) {
	rows, err := st.GetHouseSettings(ctx, houseID)
	if err != nil {
		return nil, false, err
	}
	for _, r := range rows {
		if r.Key == key {
			return r.Value, true, nil
		}
	}
	return nil, false, nil
}
