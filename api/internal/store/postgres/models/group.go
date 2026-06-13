package models

import "time"

type Group struct {
	GroupID     string    `gorm:"column:group_id;primaryKey;default:generate_ulid()" json:"group_id"`
	HouseID     string    `gorm:"column:house_id;not null" json:"house_id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description string     `gorm:"column:description;not null;default:''" json:"description"`
	DeletedAt   *time.Time `gorm:"column:deleted_at" json:"deleted_at,omitempty"`
	DeletedByMemberID *string `gorm:"column:deleted_by_member_id" json:"deleted_by_member_id,omitempty"`
	DeletedOpID *string    `gorm:"column:deleted_op_id" json:"deleted_op_id,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Group) TableName() string { return "groups" }

type GroupMember struct {
	GroupID   string    `gorm:"column:group_id;primaryKey" json:"group_id"`
	MemberID  string    `gorm:"column:member_id;primaryKey" json:"member_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (GroupMember) TableName() string { return "group_members" }
