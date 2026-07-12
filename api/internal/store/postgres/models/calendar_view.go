package models

import "time"

// CalendarSubscription is one entry in a member's calendar view: another
// member's calendar they've added, plus whether it's currently enabled (shown).
// Both states matter — a subject's events render only when a subscription
// exists AND enabled is true. Serialized as JSON inside the subscriptions
// column of member_calendar_views.
type CalendarSubscription struct {
	SubjectMemberID string `json:"subject_member_id"`
	Enabled         bool   `json:"enabled"`
}

// MemberCalendarView is a viewer's persisted set of calendar subscriptions.
// One row per viewer (member_id is already house-unique). Purely a display
// preference; see migration 000019.
type MemberCalendarView struct {
	ViewerMemberID string `gorm:"column:viewer_member_id;primaryKey" json:"viewer_member_id"`
	HouseID        string `gorm:"column:house_id;not null" json:"house_id"`
	// GORM's built-in json serializer marshals this slice to/from the jsonb
	// column, so no hand-rolled Scan/Value is needed.
	Subscriptions []CalendarSubscription `gorm:"column:subscriptions;type:jsonb;serializer:json;not null" json:"subscriptions"`
	CreatedAt     time.Time              `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt     time.Time              `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (MemberCalendarView) TableName() string { return "member_calendar_views" }
