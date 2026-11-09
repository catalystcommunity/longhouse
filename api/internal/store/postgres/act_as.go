package postgres

import (
	"context"
	"time"
)

// ConsumeActAsNonce atomically refuses reuse across processes and restarts.
func (s *PostgresStore) ConsumeActAsNonce(ctx context.Context, domain, grantID, nonce string, expiresAt time.Time) (bool, error) {
	if err := db.WithContext(ctx).Exec("DELETE FROM act_as_nonces WHERE expires_at < NOW()").Error; err != nil {
		return false, err
	}
	result := db.WithContext(ctx).Exec(`INSERT INTO act_as_nonces (subject_domain, grant_id, nonce, expires_at)
 VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`, domain, grantID, nonce, expiresAt)
	return result.RowsAffected == 1, result.Error
}
