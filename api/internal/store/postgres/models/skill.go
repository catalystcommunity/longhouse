package models

import "time"

type Skill struct {
	SkillID     string    `gorm:"column:skill_id;primaryKey;default:generate_ulid()" json:"skill_id"`
	HouseID     string    `gorm:"column:house_id;not null" json:"house_id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description string     `gorm:"column:description;not null;default:''" json:"description"`
	DeletedAt   *time.Time `gorm:"column:deleted_at" json:"deleted_at,omitempty"`
	DeletedByMemberID *string `gorm:"column:deleted_by_member_id" json:"deleted_by_member_id,omitempty"`
	DeletedOpID *string    `gorm:"column:deleted_op_id" json:"deleted_op_id,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Skill) TableName() string { return "skills" }

type MemberSkill struct {
	MemberID  string    `gorm:"column:member_id;primaryKey" json:"member_id"`
	SkillID   string    `gorm:"column:skill_id;primaryKey" json:"skill_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (MemberSkill) TableName() string { return "member_skills" }

// GroupSkill is the join row in group_skills. Independent of MemberSkill —
// the store doesn't transitively merge group skills into a member's list.
type GroupSkill struct {
	GroupID   string    `gorm:"column:group_id;primaryKey" json:"group_id"`
	SkillID   string    `gorm:"column:skill_id;primaryKey" json:"skill_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (GroupSkill) TableName() string { return "group_skills" }
