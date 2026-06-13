package postgres

import (
	"context"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// Soft-delete / restore / purge for the remaining first-class entities. The
// content entities (project, event, comment) live next to their other store
// methods in postgres_store.go; these governance entities are grouped here.
// All reuse the shared softDeleteFields/restoreFields helpers.
//
// Purge is a plain hard delete: every edge that referenced these rows is
// handled by the existing FK behavior (member_roles / member_skills /
// group_members / group_skills are ON DELETE CASCADE; assigned_to_skill_id and
// comment/event author columns are ON DELETE SET NULL), the same as the legacy
// hard-delete paths these soft-deletes replaced. Polymorphic grant rows whose
// grantee was a purged member/group are filtered at resolve time, as before.

// PurgeAllSoftDeletedBefore permanently deletes every Tier-1 entity soft-deleted
// before cutoff (the trash purge worker's per-tick sweep) and returns the total
// rows removed. Edge cleanup is handled inside each per-entity purge.
//
// It continues past a per-entity failure rather than aborting the whole sweep,
// returning the last error for the worker to log, so one entity's transient
// failure can't block the others' purge each tick.
func (s *PostgresStore) PurgeAllSoftDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	// Members are never purged — they deactivate (keeping record + content), so
	// they're not in this sweep.
	purgers := []func(context.Context, time.Time) (int64, error){
		s.PurgeTasksDeletedBefore, s.PurgeProjectsDeletedBefore, s.PurgeEventsDeletedBefore,
		s.PurgeCommentsDeletedBefore, s.PurgeMilestonesDeletedBefore,
		s.PurgeRolesDeletedBefore, s.PurgeSkillsDeletedBefore, s.PurgeGroupsDeletedBefore,
	}
	var total int64
	var lastErr error
	for _, purge := range purgers {
		n, err := purge(ctx, cutoff)
		if err != nil {
			lastErr = err
			continue
		}
		total += n
	}
	return total, lastErr
}

// --- Task ---
//
// Tasks already had a deleted_at column (migration 000002); these add the
// shared op-id/actor stamping and the restore/purge counterparts. Purge clears
// the task's (FK-less) dependency edges first; task_grants / task_assignees /
// project_tasks and child tasks are handled by their FK cascades.

func (s *PostgresStore) SoftDeleteTask(ctx context.Context, taskID, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Task{}).
		Where("task_id = ? AND deleted_at IS NULL", taskID).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

func (s *PostgresStore) RestoreTasksByOp(ctx context.Context, opID string) error {
	return db.WithContext(ctx).Model(&models.Task{}).
		Where("deleted_op_id = ?", opID).Updates(restoreFields).Error
}

func (s *PostgresStore) PurgeTasksDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var ids []string
	if err := db.WithContext(ctx).Model(&models.Task{}).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Pluck("task_id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	for _, id := range ids {
		if err := s.RemoveDependenciesForNode(ctx, models.DependencyTask, id); err != nil {
			return 0, err
		}
	}
	res := db.WithContext(ctx).Where("task_id IN ?", ids).Delete(&models.Task{})
	return res.RowsAffected, res.Error
}

// --- Member (deactivate/reactivate — NOT soft-delete) ---

// DeactivateMember marks a member inactive: their record and owned content stay
// put, but buildHouseRoles stops granting them this house's roles, so future
// logins/refreshes can't reach the house. Idempotent.
func (s *PostgresStore) DeactivateMember(ctx context.Context, memberID, byMemberID string) error {
	return db.WithContext(ctx).Model(&models.Member{}).
		Where("member_id = ? AND deactivated_at IS NULL", memberID).
		Updates(map[string]any{
			"deactivated_at":           time.Now().UTC(),
			"deactivated_by_member_id": byMemberID,
		}).Error
}

// ReactivateMember restores access for a previously deactivated member.
func (s *PostgresStore) ReactivateMember(ctx context.Context, memberID string) error {
	return db.WithContext(ctx).Model(&models.Member{}).
		Where("member_id = ?", memberID).
		Updates(map[string]any{
			"deactivated_at":           nil,
			"deactivated_by_member_id": nil,
		}).Error
}

// --- Role ---

func (s *PostgresStore) SoftDeleteRole(ctx context.Context, roleID, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Role{}).
		Where("role_id = ? AND deleted_at IS NULL", roleID).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

func (s *PostgresStore) RestoreRolesByOp(ctx context.Context, opID string) error {
	return db.WithContext(ctx).Model(&models.Role{}).
		Where("deleted_op_id = ?", opID).Updates(restoreFields).Error
}

func (s *PostgresStore) PurgeRolesDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := db.WithContext(ctx).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Delete(&models.Role{})
	return res.RowsAffected, res.Error
}

// --- Skill ---

func (s *PostgresStore) SoftDeleteSkill(ctx context.Context, skillID, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Skill{}).
		Where("skill_id = ? AND deleted_at IS NULL", skillID).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

func (s *PostgresStore) RestoreSkillsByOp(ctx context.Context, opID string) error {
	return db.WithContext(ctx).Model(&models.Skill{}).
		Where("deleted_op_id = ?", opID).Updates(restoreFields).Error
}

func (s *PostgresStore) PurgeSkillsDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := db.WithContext(ctx).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Delete(&models.Skill{})
	return res.RowsAffected, res.Error
}

// --- Group ---

func (s *PostgresStore) SoftDeleteGroup(ctx context.Context, groupID, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Group{}).
		Where("group_id = ? AND deleted_at IS NULL", groupID).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

func (s *PostgresStore) RestoreGroupsByOp(ctx context.Context, opID string) error {
	return db.WithContext(ctx).Model(&models.Group{}).
		Where("deleted_op_id = ?", opID).Updates(restoreFields).Error
}

func (s *PostgresStore) PurgeGroupsDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := db.WithContext(ctx).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Delete(&models.Group{})
	return res.RowsAffected, res.Error
}

// --- Milestone ---

func (s *PostgresStore) SoftDeleteMilestone(ctx context.Context, milestoneID, byMemberID, opID string) error {
	return db.WithContext(ctx).Model(&models.Milestone{}).
		Where("milestone_id = ? AND deleted_at IS NULL", milestoneID).
		Updates(softDeleteFields(byMemberID, opID)).Error
}

func (s *PostgresStore) RestoreMilestonesByOp(ctx context.Context, opID string) error {
	return db.WithContext(ctx).Model(&models.Milestone{}).
		Where("deleted_op_id = ?", opID).Updates(restoreFields).Error
}

func (s *PostgresStore) PurgeMilestonesDeletedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := db.WithContext(ctx).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Delete(&models.Milestone{})
	return res.RowsAffected, res.Error
}
