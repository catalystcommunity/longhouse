package models

import "time"

type Role struct {
	RoleID      string    `gorm:"column:role_id;primaryKey;default:generate_ulid()" json:"role_id"`
	HouseID     string    `gorm:"column:house_id;not null" json:"house_id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description string    `gorm:"column:description;not null;default:''" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Role) TableName() string { return "roles" }

// Canonical role names created for every house.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

type MemberRole struct {
	MemberID  string    `gorm:"column:member_id;primaryKey" json:"member_id"`
	RoleID    string    `gorm:"column:role_id;primaryKey" json:"role_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (MemberRole) TableName() string { return "member_roles" }
