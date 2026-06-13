package models

import "time"

// TrashRow is a unified view of one soft-deleted, restorable item across all
// Tier-1 tables. It is a query projection (not a table) returned by the trash
// listing, so the admin UI can show everything in the bin in one list.
type TrashRow struct {
	ResourceType      string     `gorm:"column:resource_type"`
	ResourceID        string     `gorm:"column:resource_id"`
	HouseID           string     `gorm:"column:house_id"`
	Title             string     `gorm:"column:title"`
	DeletedAt         time.Time  `gorm:"column:deleted_at"`
	DeletedByMemberID *string    `gorm:"column:deleted_by_member_id"`
	DeletedOpID       *string    `gorm:"column:deleted_op_id"`
}
