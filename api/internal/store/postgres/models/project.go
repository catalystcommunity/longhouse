package models

import "time"

type Project struct {
	ProjectID   string    `gorm:"column:project_id;primaryKey;default:generate_ulid()" json:"project_id"`
	HouseID     string    `gorm:"column:house_id;not null" json:"house_id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description string    `gorm:"column:description;not null;default:''" json:"description"`
	Status      string    `gorm:"column:status;not null;default:'active'" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Project) TableName() string { return "projects" }

type ProjectTask struct {
	ProjectID string    `gorm:"column:project_id;primaryKey" json:"project_id"`
	TaskID    string    `gorm:"column:task_id;primaryKey" json:"task_id"`
	Position  int       `gorm:"column:position;not null" json:"position"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (ProjectTask) TableName() string { return "project_tasks" }
