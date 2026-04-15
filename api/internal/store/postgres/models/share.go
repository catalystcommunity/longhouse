package models

import "time"

type Share struct {
	ShareID        string     `gorm:"column:share_id;primaryKey" json:"share_id"`
	HouseID        string     `gorm:"column:house_id;not null" json:"house_id"`
	SharedBy       string     `gorm:"column:shared_by;not null" json:"shared_by"`
	LinkkeysDomain string     `gorm:"column:linkkeys_domain;not null" json:"linkkeys_domain"`
	LinkkeysUserID string     `gorm:"column:linkkeys_user_id;not null" json:"linkkeys_user_id"`
	ResourceType   string     `gorm:"column:resource_type;not null" json:"resource_type"`
	ResourceID     string     `gorm:"column:resource_id;not null" json:"resource_id"`
	AccessLevel    string     `gorm:"column:access_level;not null;default:'read'" json:"access_level"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	ExpiresAt      *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
}

func (Share) TableName() string { return "shares" }
