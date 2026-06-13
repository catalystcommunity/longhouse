package postgres

import (
	"context"
	"database/sql"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// RecordAuditEntry appends one row to the partitioned audit_log. created_at is
// stamped by GORM's NowFunc, routing the row to the current monthly partition.
func (s *PostgresStore) RecordAuditEntry(ctx context.Context, e *models.AuditEntry) error {
	return db.WithContext(ctx).Create(e).Error
}

// GetDeleteOpDetail returns the detail JSON of the delete audit entry for a
// given deleted_op_id in a house — used by restore to recover information a
// delete recorded but the soft-delete itself didn't capture (e.g. an event
// "this & future" sweep mutes the recurrence root, and restore needs its prior
// recurrence to re-arm it). Returns nil if there's no such entry.
func (s *PostgresStore) GetDeleteOpDetail(ctx context.Context, houseID, opID string) (models.JSONMap, error) {
	const q = `SELECT detail FROM audit_log
		WHERE house_id = ? AND action = 'delete' AND detail->>'deleted_op_id' = ?
		ORDER BY created_at DESC LIMIT 1`
	var detail models.JSONMap
	err := db.WithContext(ctx).Raw(q, houseID, opID).Row().Scan(&detail)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// ListAuditEntries returns audit rows for one house, newest-first, applying the
// optional filters and keyset cursor in f. A NULL-house (global security)
// query is expressed by passing houseID == "" (matches house_id IS NULL).
func (s *PostgresStore) ListAuditEntries(ctx context.Context, houseID string, f models.AuditFilter) ([]models.AuditEntry, error) {
	q := db.WithContext(ctx).Model(&models.AuditEntry{})
	if houseID == "" {
		q = q.Where("house_id IS NULL")
	} else {
		q = q.Where("house_id = ?", houseID)
	}
	if f.ActorMemberID != nil {
		q = q.Where("actor_member_id = ?", *f.ActorMemberID)
	}
	if f.ResourceType != nil {
		q = q.Where("resource_type = ?", *f.ResourceType)
	}
	if f.ResourceID != nil {
		q = q.Where("resource_id = ?", *f.ResourceID)
	}
	if f.Action != nil {
		q = q.Where("action = ?", *f.Action)
	}
	if f.Since != nil {
		q = q.Where("created_at >= ?", *f.Since)
	}
	if f.Until != nil {
		q = q.Where("created_at < ?", *f.Until)
	}
	// Keyset pagination: rows strictly older than the cursor, in the same
	// (created_at DESC, audit_id DESC) order the page is returned in.
	if f.CursorCreatedAt != nil && f.CursorAuditID != nil {
		q = q.Where("(created_at, audit_id) < (?, ?)", *f.CursorCreatedAt, *f.CursorAuditID)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var out []models.AuditEntry
	if err := q.Order("created_at DESC, audit_id DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
