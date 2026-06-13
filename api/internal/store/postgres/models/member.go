package models

import "time"

type Member struct {
	MemberID       string     `gorm:"column:member_id;primaryKey;default:generate_ulid()" json:"member_id"`
	HouseID        string     `gorm:"column:house_id;not null" json:"house_id"`
	LinkkeysDomain string     `gorm:"column:linkkeys_domain;not null" json:"linkkeys_domain"`
	LinkkeysUserID string     `gorm:"column:linkkeys_user_id;not null" json:"linkkeys_user_id"`
	DisplayName    string     `gorm:"column:display_name;not null;default:''" json:"display_name"`
	CachedPubKey   []byte     `gorm:"column:cached_public_key" json:"cached_public_key,omitempty"`
	// Deactivated members keep their record + owned content but are denied
	// future login (buildHouseRoles skips a deactivated membership). Not the
	// same as soft-delete: members are never trashed or purged.
	DeactivatedAt         *time.Time `gorm:"column:deactivated_at" json:"deactivated_at,omitempty"`
	DeactivatedByMemberID *string    `gorm:"column:deactivated_by_member_id" json:"deactivated_by_member_id,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
	LastSeenAt     *time.Time `gorm:"column:last_seen_at" json:"last_seen_at,omitempty"`
}

func (Member) TableName() string { return "members" }
