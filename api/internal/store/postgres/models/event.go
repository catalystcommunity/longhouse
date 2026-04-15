package models

import "time"

type Event struct {
	EventID     string     `gorm:"column:event_id;primaryKey" json:"event_id"`
	HouseID     string     `gorm:"column:house_id;not null" json:"house_id"`
	CreatedBy   string     `gorm:"column:created_by;not null" json:"created_by"`
	Title       string     `gorm:"column:title;not null" json:"title"`
	Description string     `gorm:"column:description;not null;default:''" json:"description"`
	Location    string     `gorm:"column:location;not null;default:''" json:"location"`
	StartsAt    *time.Time `gorm:"column:starts_at" json:"starts_at,omitempty"`
	EndsAt      *time.Time `gorm:"column:ends_at" json:"ends_at,omitempty"`
	AllDay      bool       `gorm:"column:all_day;not null;default:false" json:"all_day"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Event) TableName() string { return "events" }
