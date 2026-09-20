package models

import "time"

type CliLoginRequest struct {
	LoginID        string     `gorm:"column:login_id;primaryKey;default:generate_ulid()"`
	DeviceCodeHash []byte     `gorm:"column:device_code_hash;not null"`
	UserCodeHash   []byte     `gorm:"column:user_code_hash;not null"`
	ClientName     string     `gorm:"column:client_name;not null"`
	Status         string     `gorm:"column:status;not null;default:'pending'"`
	SubjectDomain  *string    `gorm:"column:subject_domain"`
	SubjectUserID  *string    `gorm:"column:subject_user_id"`
	DisplayName    *string    `gorm:"column:display_name"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;not null"`
	ApprovedAt     *time.Time `gorm:"column:approved_at"`
	ConsumedAt     *time.Time `gorm:"column:consumed_at"`
}

func (CliLoginRequest) TableName() string { return "cli_login_requests" }

type CliSession struct {
	SessionID     string     `gorm:"column:session_id;primaryKey;default:generate_ulid()"`
	SubjectDomain string     `gorm:"column:subject_domain;not null"`
	SubjectUserID string     `gorm:"column:subject_user_id;not null"`
	DisplayName   string     `gorm:"column:display_name;not null;default:''"`
	ClientName    string     `gorm:"column:client_name;not null"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null"`
	LastUsedAt    time.Time  `gorm:"column:last_used_at;not null"`
	ExpiresAt     time.Time  `gorm:"column:expires_at;not null"`
	RevokedAt     *time.Time `gorm:"column:revoked_at"`
}

func (CliSession) TableName() string { return "cli_sessions" }

type CliRefreshToken struct {
	TokenHash []byte     `gorm:"column:token_hash;primaryKey"`
	SessionID string     `gorm:"column:session_id;not null"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	UsedAt    *time.Time `gorm:"column:used_at"`
}

func (CliRefreshToken) TableName() string { return "cli_refresh_tokens" }
