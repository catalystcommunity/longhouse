package models

import "time"

type Comment struct {
	CommentID  string    `gorm:"column:comment_id;primaryKey" json:"comment_id"`
	HouseID    string    `gorm:"column:house_id;not null" json:"house_id"`
	MemberID   string    `gorm:"column:member_id;not null" json:"member_id"`
	TargetType string    `gorm:"column:target_type;not null" json:"target_type"`
	TargetID   string    `gorm:"column:target_id;not null" json:"target_id"`
	Body       string    `gorm:"column:body;not null" json:"body"`
	CreatedAt  time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Comment) TableName() string { return "comments" }
