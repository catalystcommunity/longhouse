package postgres

import (
	"context"
	"fmt"
	"time"
)

// CreateAuditPartitionsAhead ensures monthly audit_log partitions exist for the
// current month and the next `ahead` months. Idempotent (CREATE TABLE IF NOT
// EXISTS), so it's safe to run every worker tick. Creating partitions ahead of
// time keeps the DEFAULT partition empty in steady state. Identifiers/bounds
// are computed (not user input), so the formatted DDL is safe.
func (s *PostgresStore) CreateAuditPartitionsAhead(ctx context.Context, now time.Time, ahead int) error {
	base := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i <= ahead; i++ {
		start := base.AddDate(0, i, 0)
		end := start.AddDate(0, 1, 0)
		name := fmt.Sprintf("audit_log_%04d_%02d", start.Year(), int(start.Month()))
		q := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_log FOR VALUES FROM ('%s') TO ('%s')",
			name, start.Format("2006-01-02"), end.Format("2006-01-02"),
		)
		if err := db.WithContext(ctx).Exec(q).Error; err != nil {
			return err
		}
	}
	return nil
}

// DropAuditPartitionsBefore drops monthly audit_log partitions whose month
// starts strictly before (now - retentionMonths). The DEFAULT partition and any
// non-monthly child are skipped (their names don't parse). Returns the count
// dropped. This is the retention lever — a metadata-only DROP, no row scan.
func (s *PostgresStore) DropAuditPartitionsBefore(ctx context.Context, now time.Time, retentionMonths int) (int, error) {
	cutoff := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -retentionMonths, 0)
	var names []string
	const q = `SELECT c.relname
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname = 'audit_log'`
	if err := db.WithContext(ctx).Raw(q).Scan(&names).Error; err != nil {
		return 0, err
	}
	dropped := 0
	for _, n := range names {
		var y, m int
		if _, err := fmt.Sscanf(n, "audit_log_%04d_%02d", &y, &m); err != nil {
			continue // audit_log_default or any non-monthly child
		}
		monthStart := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		if monthStart.Before(cutoff) {
			if err := db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", n)).Error; err != nil {
				return dropped, err
			}
			dropped++
		}
	}
	return dropped, nil
}
