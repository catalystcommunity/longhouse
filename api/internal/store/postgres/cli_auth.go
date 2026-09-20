package postgres

import (
	"context"
	"errors"
	"time"

	appstore "github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *PostgresStore) CreateCliLogin(ctx context.Context, login *models.CliLoginRequest) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Keep short-lived requests and expired sessions bounded without a
		// separate worker. Session deletion also deletes its refresh-token rows.
		if err := tx.Where("expires_at < ?", time.Now().UTC().Add(-24*time.Hour)).
			Delete(&models.CliLoginRequest{}).Error; err != nil {
			return err
		}
		if err := tx.Where("expires_at < ?", time.Now().UTC().Add(-24*time.Hour)).
			Delete(&models.CliSession{}).Error; err != nil {
			return err
		}
		return tx.Create(login).Error
	})
}

func (s *PostgresStore) GetCliLoginByUserCode(ctx context.Context, userCodeHash []byte, now time.Time) (*models.CliLoginRequest, error) {
	var login models.CliLoginRequest
	err := db.WithContext(ctx).Where("user_code_hash = ?", userCodeHash).First(&login).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appstore.ErrCliLoginNotFound
	}
	if err != nil {
		return nil, err
	}
	if !login.ExpiresAt.After(now) {
		return nil, appstore.ErrCliLoginExpired
	}
	if login.Status != "pending" {
		return nil, appstore.ErrCliLoginUsed
	}
	return &login, nil
}

func (s *PostgresStore) ApproveCliLogin(ctx context.Context, userCodeHash []byte, domain, userID, displayName string, now time.Time) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		login, err := lockedCliLoginByUserCode(tx, userCodeHash)
		if err != nil {
			return err
		}
		if !login.ExpiresAt.After(now) {
			return appstore.ErrCliLoginExpired
		}
		if login.Status != "pending" {
			return appstore.ErrCliLoginUsed
		}
		login.Status = "approved"
		login.SubjectDomain = &domain
		login.SubjectUserID = &userID
		login.DisplayName = &displayName
		login.ApprovedAt = &now
		return tx.Save(login).Error
	})
}

func (s *PostgresStore) DenyCliLogin(ctx context.Context, userCodeHash []byte, now time.Time) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		login, err := lockedCliLoginByUserCode(tx, userCodeHash)
		if err != nil {
			return err
		}
		if !login.ExpiresAt.After(now) {
			return appstore.ErrCliLoginExpired
		}
		if login.Status != "pending" {
			return appstore.ErrCliLoginUsed
		}
		login.Status = "denied"
		return tx.Save(login).Error
	})
}

func lockedCliLoginByUserCode(tx *gorm.DB, hash []byte) (*models.CliLoginRequest, error) {
	var login models.CliLoginRequest
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_code_hash = ?", hash).First(&login).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appstore.ErrCliLoginNotFound
	}
	if err != nil {
		return nil, err
	}
	return &login, nil
}

func (s *PostgresStore) ExchangeCliLogin(ctx context.Context, deviceCodeHash, refreshTokenHash []byte, now, sessionExpiresAt time.Time) (*models.CliSession, error) {
	var out *models.CliSession
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var login models.CliLoginRequest
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("device_code_hash = ?", deviceCodeHash).First(&login).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appstore.ErrCliLoginNotFound
		}
		if err != nil {
			return err
		}
		if !login.ExpiresAt.After(now) {
			return appstore.ErrCliLoginExpired
		}
		switch login.Status {
		case "pending":
			return appstore.ErrCliLoginPending
		case "denied":
			return appstore.ErrCliLoginDenied
		case "consumed":
			return appstore.ErrCliLoginUsed
		case "approved":
			// Continue below.
		default:
			return appstore.ErrCliLoginUsed
		}
		if login.SubjectDomain == nil || login.SubjectUserID == nil {
			return errors.New("approved CLI login has no subject")
		}
		displayName := ""
		if login.DisplayName != nil {
			displayName = *login.DisplayName
		}
		session := &models.CliSession{
			SubjectDomain: *login.SubjectDomain,
			SubjectUserID: *login.SubjectUserID,
			DisplayName:   displayName,
			ClientName:    login.ClientName,
			CreatedAt:     now,
			LastUsedAt:    now,
			ExpiresAt:     sessionExpiresAt,
		}
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.CliRefreshToken{
			TokenHash: refreshTokenHash,
			SessionID: session.SessionID,
			CreatedAt: now,
		}).Error; err != nil {
			return err
		}
		login.Status = "consumed"
		login.ConsumedAt = &now
		if err := tx.Save(&login).Error; err != nil {
			return err
		}
		out = session
		return nil
	})
	return out, err
}

func (s *PostgresStore) RotateCliRefreshToken(ctx context.Context, tokenHash, nextTokenHash []byte, now time.Time) (*models.CliSession, error) {
	var out *models.CliSession
	var resultErr error
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token models.CliRefreshToken
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", tokenHash).First(&token).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resultErr = appstore.ErrCliSessionInvalid
			return nil
		}
		if err != nil {
			return err
		}

		var session models.CliSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_id = ?", token.SessionID).First(&session).Error; err != nil {
			return err
		}
		if token.UsedAt != nil {
			if session.RevokedAt == nil {
				session.RevokedAt = &now
				if err := tx.Save(&session).Error; err != nil {
					return err
				}
			}
			resultErr = appstore.ErrRefreshTokenReuse
			return nil
		}
		if session.RevokedAt != nil || !session.ExpiresAt.After(now) {
			resultErr = appstore.ErrCliSessionInvalid
			return nil
		}

		token.UsedAt = &now
		if err := tx.Save(&token).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.CliRefreshToken{
			TokenHash: nextTokenHash,
			SessionID: session.SessionID,
			CreatedAt: now,
		}).Error; err != nil {
			return err
		}
		session.LastUsedAt = now
		if err := tx.Save(&session).Error; err != nil {
			return err
		}
		out = &session
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, resultErr
}

func (s *PostgresStore) ListCliSessions(ctx context.Context, domain, userID string) ([]models.CliSession, error) {
	var sessions []models.CliSession
	err := db.WithContext(ctx).
		Where("subject_domain = ? AND subject_user_id = ?", domain, userID).
		Order("created_at DESC").Limit(100).Find(&sessions).Error
	return sessions, err
}

func (s *PostgresStore) RevokeCliSession(ctx context.Context, sessionID, domain, userID string, now time.Time) error {
	result := db.WithContext(ctx).Model(&models.CliSession{}).
		Where("session_id = ? AND subject_domain = ? AND subject_user_id = ? AND revoked_at IS NULL", sessionID, domain, userID).
		Update("revoked_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return appstore.ErrCliSessionInvalid
	}
	return nil
}
