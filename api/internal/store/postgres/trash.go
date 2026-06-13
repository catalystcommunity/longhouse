package postgres

import (
	"context"
	"fmt"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// trashUnionSQL projects every Tier-1 table's soft-deleted rows into the common
// TrashRow shape. The %s is the house predicate (parameterized). Milestones
// carry no house_id, so they join through their project. Comment "titles" are
// the first 100 chars of the body.
const trashUnionSQL = `
SELECT 'task'::text AS resource_type, t.task_id::text AS resource_id, t.house_id::text AS house_id,
       t.title AS title, t.deleted_at, t.deleted_by_member_id::text, t.deleted_op_id::text
  FROM tasks t WHERE t.house_id = ? AND t.deleted_at IS NOT NULL
UNION ALL
SELECT 'project', p.project_id::text, p.house_id::text, p.name, p.deleted_at, p.deleted_by_member_id::text, p.deleted_op_id::text
  FROM projects p WHERE p.house_id = ? AND p.deleted_at IS NOT NULL
UNION ALL
SELECT 'event', e.event_id::text, e.house_id::text, e.title, e.deleted_at, e.deleted_by_member_id::text, e.deleted_op_id::text
  FROM events e WHERE e.house_id = ? AND e.deleted_at IS NOT NULL
UNION ALL
SELECT 'comment', c.comment_id::text, c.house_id::text, left(c.body, 100), c.deleted_at, c.deleted_by_member_id::text, c.deleted_op_id::text
  FROM comments c WHERE c.house_id = ? AND c.deleted_at IS NOT NULL
UNION ALL
SELECT 'role', r.role_id::text, r.house_id::text, r.name, r.deleted_at, r.deleted_by_member_id::text, r.deleted_op_id::text
  FROM roles r WHERE r.house_id = ? AND r.deleted_at IS NOT NULL
UNION ALL
SELECT 'skill', s.skill_id::text, s.house_id::text, s.name, s.deleted_at, s.deleted_by_member_id::text, s.deleted_op_id::text
  FROM skills s WHERE s.house_id = ? AND s.deleted_at IS NOT NULL
UNION ALL
SELECT 'group', g.group_id::text, g.house_id::text, g.name, g.deleted_at, g.deleted_by_member_id::text, g.deleted_op_id::text
  FROM groups g WHERE g.house_id = ? AND g.deleted_at IS NOT NULL
UNION ALL
SELECT 'milestone', ms.milestone_id::text, pj.house_id::text, ms.label, ms.deleted_at, ms.deleted_by_member_id::text, ms.deleted_op_id::text
  FROM milestones ms JOIN projects pj ON pj.project_id = ms.project_id
  WHERE pj.house_id = ? AND ms.deleted_at IS NOT NULL
ORDER BY deleted_at DESC
LIMIT ? OFFSET ?`

// ListTrash returns every soft-deleted, restorable item in a house, newest
// deletion first.
func (s *PostgresStore) ListTrash(ctx context.Context, houseID string, limit, offset int) ([]models.TrashRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []any{houseID, houseID, houseID, houseID, houseID, houseID, houseID, houseID, limit, offset}
	var rows []models.TrashRow
	if err := db.WithContext(ctx).Raw(trashUnionSQL, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// trashTables maps a resource type to its table and primary-key column.
var trashTables = map[string]struct{ table, pk string }{
	"task":      {"tasks", "task_id"},
	"project":   {"projects", "project_id"},
	"event":     {"events", "event_id"},
	"comment":   {"comments", "comment_id"},
	"role":      {"roles", "role_id"},
	"skill":     {"skills", "skill_id"},
	"group":     {"groups", "group_id"},
	"milestone": {"milestones", "milestone_id"},
}

// FindDeletedOpID returns the deleted_op_id of a soft-deleted resource so a
// single-item restore can revert its whole delete operation. Empty string +
// nil error means "not found or not in the trash".
func (s *PostgresStore) FindDeletedOpID(ctx context.Context, resourceType, resourceID string) (string, error) {
	t, ok := trashTables[resourceType]
	if !ok {
		return "", fmt.Errorf("unknown resource type %q", resourceType)
	}
	var opID *string
	q := fmt.Sprintf("SELECT deleted_op_id::text FROM %s WHERE %s = ? AND deleted_at IS NOT NULL", t.table, t.pk)
	if err := db.WithContext(ctx).Raw(q, resourceID).Scan(&opID).Error; err != nil {
		return "", err
	}
	if opID == nil {
		return "", nil
	}
	return *opID, nil
}

// ResourceHouseID returns the house a resource belongs to (milestones resolve
// it through their project). Empty string + nil error means "no such row".
// Used to confirm a restore/purge target lives in the caller's house before
// acting on an id they supplied directly.
func (s *PostgresStore) ResourceHouseID(ctx context.Context, resourceType, resourceID string) (string, error) {
	t, ok := trashTables[resourceType]
	if !ok {
		return "", fmt.Errorf("unknown resource type %q", resourceType)
	}
	var q string
	if resourceType == "milestone" {
		q = "SELECT pj.house_id::text FROM milestones ms JOIN projects pj ON pj.project_id = ms.project_id WHERE ms.milestone_id = ?"
	} else {
		q = fmt.Sprintf("SELECT house_id::text FROM %s WHERE %s = ?", t.table, t.pk)
	}
	var house *string
	if err := db.WithContext(ctx).Raw(q, resourceID).Scan(&house).Error; err != nil {
		return "", err
	}
	if house == nil {
		return "", nil
	}
	return *house, nil
}

// PurgeResource permanently deletes one soft-deleted item now (the admin "purge
// now" action), cleaning FK-less dependency edges for tasks/projects first. It
// refuses to touch a live (non-trashed) row.
func (s *PostgresStore) PurgeResource(ctx context.Context, resourceType, resourceID string) error {
	t, ok := trashTables[resourceType]
	if !ok {
		return fmt.Errorf("unknown resource type %q", resourceType)
	}
	switch resourceType {
	case "task":
		if err := s.RemoveDependenciesForNode(ctx, models.DependencyTask, resourceID); err != nil {
			return err
		}
	case "project":
		if err := s.RemoveDependenciesForNode(ctx, models.DependencyProject, resourceID); err != nil {
			return err
		}
	}
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = ? AND deleted_at IS NOT NULL", t.table, t.pk)
	return db.WithContext(ctx).Exec(q, resourceID).Error
}
