package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSONMap is a thin wrapper that stores arbitrary JSON in a jsonb column.
type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(src interface{}) error {
	if src == nil {
		*m = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into JSONMap", src)
	}
	return json.Unmarshal(b, m)
}

// MemberAudit records role/skill/group attachments and other admin actions
// taken against a member. It is append-only — entries are never updated or
// deleted in normal operation.
type MemberAudit struct {
	AuditID         string    `gorm:"column:audit_id;primaryKey;default:generate_ulid()" json:"audit_id"`
	HouseID         string    `gorm:"column:house_id;not null" json:"house_id"`
	SubjectMemberID string    `gorm:"column:subject_member_id;not null" json:"subject_member_id"`
	ActorMemberID   *string   `gorm:"column:actor_member_id" json:"actor_member_id,omitempty"`
	Action          string    `gorm:"column:action;not null" json:"action"`
	TargetType      *string   `gorm:"column:target_type" json:"target_type,omitempty"`
	TargetID        *string   `gorm:"column:target_id" json:"target_id,omitempty"`
	Detail          JSONMap   `gorm:"column:detail;type:jsonb" json:"detail,omitempty"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (MemberAudit) TableName() string { return "member_audits" }

// Action values used by SeedInitialAdmin and other role-management flows.
// Free-form strings; these constants exist to keep call sites consistent.
const (
	AuditActionRoleGranted          = "role_granted"
	AuditActionRoleRevoked          = "role_revoked"
	AuditActionMemberAutoCreated    = "member_auto_created"
	AuditActionTrustedDomainAdded   = "trusted_domain_added"
	AuditActionTrustedDomainRemoved = "trusted_domain_removed"
	AuditActionShareCreated         = "share_created"
	AuditActionShareRevoked         = "share_revoked"
)
