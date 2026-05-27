package models

import "time"

// HouseSetting is one (house_id, key) row backing the SettingsService.
// Value is raw JSON; the service layer decodes it according to the key
// the CSIL spec documents.
type HouseSetting struct {
	HouseID   string    `gorm:"column:house_id;primaryKey" json:"house_id"`
	Key       string    `gorm:"column:key;primaryKey" json:"key"`
	Value     []byte    `gorm:"column:value;type:jsonb;not null" json:"value"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
	UpdatedBy *string   `gorm:"column:updated_by" json:"updated_by,omitempty"`
}

func (HouseSetting) TableName() string { return "house_settings" }
