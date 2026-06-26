package models

import "time"

type Member struct {
	MemberID       string     `gorm:"column:member_id;primaryKey;default:generate_ulid()" json:"member_id"`
	HouseID        string     `gorm:"column:house_id;not null" json:"house_id"`
	LinkkeysDomain string     `gorm:"column:linkkeys_domain;not null" json:"linkkeys_domain"`
	LinkkeysUserID string     `gorm:"column:linkkeys_user_id;not null" json:"linkkeys_user_id"`
	DisplayName    string     `gorm:"column:display_name;not null;default:''" json:"display_name"`
	// Email and AvatarURL are Longhouse-owned, seeded/reconciled from linkkeys
	// claims at login but user-editable thereafter (see migration 000016 and
	// auth.reconcileMemberClaims). Empty when no claim was ever released and the
	// user set nothing.
	Email     string `gorm:"column:email;not null;default:''" json:"email,omitempty"`
	AvatarURL string `gorm:"column:avatar_url;not null;default:''" json:"avatar_url,omitempty"`
	// *Claimed columns mirror the last value linkkeys released for each field,
	// so reconciliation can tell "still tracking upstream" from "user overrode".
	// Internal bookkeeping — never serialized out.
	DisplayNameClaimed string `gorm:"column:display_name_claimed;not null;default:''" json:"-"`
	EmailClaimed       string `gorm:"column:email_claimed;not null;default:''" json:"-"`
	AvatarURLClaimed   string `gorm:"column:avatar_url_claimed;not null;default:''" json:"-"`
	CachedPubKey       []byte `gorm:"column:cached_public_key" json:"cached_public_key,omitempty"`
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
