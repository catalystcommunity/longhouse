package models

import "time"

type Event struct {
	EventID               string     `gorm:"column:event_id;primaryKey;default:generate_ulid()" json:"event_id"`
	HouseID               string     `gorm:"column:house_id;not null" json:"house_id"`
	OwnerMemberID         string     `gorm:"column:owner_member_id;not null" json:"owner_member_id"`
	Title                 string     `gorm:"column:title;not null" json:"title"`
	Description           string     `gorm:"column:description;not null;default:''" json:"description"`
	Location              string     `gorm:"column:location;not null;default:''" json:"location"`
	StartsAt              *time.Time `gorm:"column:starts_at" json:"starts_at,omitempty"`
	EndsAt                *time.Time `gorm:"column:ends_at" json:"ends_at,omitempty"`
	AllDay                bool       `gorm:"column:all_day;not null;default:false" json:"all_day"`
	RecurrenceFreq        *string    `gorm:"column:recurrence_freq;type:recurrence_freq" json:"recurrence_freq,omitempty"`
	RecurrenceInterval    int        `gorm:"column:recurrence_interval;not null;default:1" json:"recurrence_interval"`
	RecurrenceByWeekday   IntList    `gorm:"column:recurrence_by_weekday;type:integer[]" json:"recurrence_by_weekday,omitempty"`
	RecurrenceBySetpos    *int       `gorm:"column:recurrence_by_setpos" json:"recurrence_by_setpos,omitempty"`
	NextRecurrenceAt      *time.Time `gorm:"column:next_recurrence_at" json:"next_recurrence_at,omitempty"`
	RecurrenceRootEventID *string    `gorm:"column:recurrence_root_event_id" json:"recurrence_root_event_id,omitempty"`
	CreatedAt             time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Event) TableName() string { return "events" }
