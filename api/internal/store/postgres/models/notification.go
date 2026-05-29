package models

import "time"

// NotificationEvent is the shared, self-contained snapshot of a thing that
// happened (one row per real-world event). It carries denormalized display
// data and only opaque references (TargetType/TargetID) to whatever produced
// it — there is deliberately no foreign key back to the source resource, so
// the snapshot survives deletion of that resource.
type NotificationEvent struct {
	NotificationEventID string    `gorm:"column:notification_event_id;primaryKey;default:generate_ulid()" json:"notification_event_id"`
	HouseID             string    `gorm:"column:house_id;not null" json:"house_id"`
	Kind                string    `gorm:"column:kind;not null" json:"kind"`
	ActorMemberID       *string   `gorm:"column:actor_member_id" json:"actor_member_id,omitempty"`
	ActorName           string    `gorm:"column:actor_name;not null;default:''" json:"actor_name"`
	TargetType          *string   `gorm:"column:target_type" json:"target_type,omitempty"`
	TargetID            *string   `gorm:"column:target_id" json:"target_id,omitempty"`
	TargetTitle         string    `gorm:"column:target_title;not null;default:''" json:"target_title"`
	Body                string    `gorm:"column:body;not null;default:''" json:"body"`
	CreatedAt           time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (NotificationEvent) TableName() string { return "notification_events" }

// Notification is one recipient's copy of a feed item. Fan-out-on-write: one
// row per recipient per event, each pointing at the shared snapshot and
// holding that recipient's own read state.
type Notification struct {
	NotificationID      string     `gorm:"column:notification_id;primaryKey;default:generate_ulid()" json:"notification_id"`
	NotificationEventID string     `gorm:"column:notification_event_id;not null" json:"notification_event_id"`
	HouseID             string     `gorm:"column:house_id;not null" json:"house_id"`
	MemberID            string     `gorm:"column:member_id;not null" json:"member_id"`
	ReadAt              *time.Time `gorm:"column:read_at" json:"read_at,omitempty"`
	CreatedAt           time.Time  `gorm:"column:created_at;not null" json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

// NotificationFeedItem is a read model: a notification joined to its event
// snapshot, as the feed is consumed. Not a table.
type NotificationFeedItem struct {
	NotificationID      string     `gorm:"column:notification_id"`
	NotificationEventID string     `gorm:"column:notification_event_id"`
	HouseID             string     `gorm:"column:house_id"`
	MemberID            string     `gorm:"column:member_id"`
	ReadAt              *time.Time `gorm:"column:read_at"`
	CreatedAt           time.Time  `gorm:"column:created_at"`
	Kind                string     `gorm:"column:kind"`
	ActorMemberID       *string    `gorm:"column:actor_member_id"`
	ActorName           string     `gorm:"column:actor_name"`
	TargetType          *string    `gorm:"column:target_type"`
	TargetID            *string    `gorm:"column:target_id"`
	TargetTitle         string     `gorm:"column:target_title"`
	Body                string     `gorm:"column:body"`
}
