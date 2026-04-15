package models

import "time"

type Task struct {
	TaskID      string     `gorm:"column:task_id;primaryKey" json:"task_id"`
	HouseID     string     `gorm:"column:house_id;not null" json:"house_id"`
	CreatedBy   string     `gorm:"column:created_by;not null" json:"created_by"`
	AssignedTo  *string    `gorm:"column:assigned_to" json:"assigned_to,omitempty"`
	Title       string     `gorm:"column:title;not null" json:"title"`
	Description string     `gorm:"column:description;not null;default:''" json:"description"`
	Status      string     `gorm:"column:status;type:task_status;not null;default:'open'" json:"status"`
	DueAt       *time.Time `gorm:"column:due_at" json:"due_at,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Task) TableName() string { return "tasks" }
