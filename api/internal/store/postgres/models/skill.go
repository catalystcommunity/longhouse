package models

import "time"

type Skill struct {
	SkillID     string    `gorm:"column:skill_id;primaryKey;default:generate_ulid()" json:"skill_id"`
	HouseID     string    `gorm:"column:house_id;not null" json:"house_id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description string    `gorm:"column:description;not null;default:''" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Skill) TableName() string { return "skills" }

type MemberSkill struct {
	MemberID  string    `gorm:"column:member_id;primaryKey" json:"member_id"`
	SkillID   string    `gorm:"column:skill_id;primaryKey" json:"skill_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (MemberSkill) TableName() string { return "member_skills" }
