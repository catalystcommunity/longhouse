package models

import "time"

// AuditEntry is one row in the partitioned audit_log: an append-only record of
// a mutation or security event. It is never updated or deleted in normal
// operation (retention is by dropping whole monthly partitions). It carries NO
// foreign keys — it must outlive the rows it references.
//
// created_at is part of the primary key because audit_log is RANGE-partitioned
// on it; GORM's NowFunc stamps it on create, routing the row to the right
// monthly partition.
type AuditEntry struct {
	AuditID       string  `gorm:"column:audit_id;primaryKey;default:generate_ulid()" json:"audit_id"`
	HouseID       *string `gorm:"column:house_id" json:"house_id,omitempty"`
	ActorMemberID *string `gorm:"column:actor_member_id" json:"actor_member_id,omitempty"`
	ActorDomain   string  `gorm:"column:actor_domain;not null;default:''" json:"actor_domain"`
	ActorUserID   string  `gorm:"column:actor_user_id;not null;default:''" json:"actor_user_id"`
	Service       string  `gorm:"column:service;not null;default:''" json:"service"`
	Method        string  `gorm:"column:method;not null;default:''" json:"method"`
	Action        string  `gorm:"column:action;not null" json:"action"`
	ResourceType  *string `gorm:"column:resource_type" json:"resource_type,omitempty"`
	ResourceID    *string `gorm:"column:resource_id" json:"resource_id,omitempty"`
	Outcome       string  `gorm:"column:outcome;not null;default:'ok'" json:"outcome"`
	Before        JSONMap `gorm:"column:before;type:jsonb" json:"before,omitempty"`
	After         JSONMap `gorm:"column:after;type:jsonb" json:"after,omitempty"`
	Detail        JSONMap `gorm:"column:detail;type:jsonb" json:"detail,omitempty"`
	CreatedAt     time.Time `gorm:"column:created_at;primaryKey;not null" json:"created_at"`
}

func (AuditEntry) TableName() string { return "audit_log" }

// AuditFilter parameterizes a house-scoped audit query. All filters are
// optional (nil = unfiltered). Results are newest-first; paginate with the
// keyset cursor (CursorCreatedAt + CursorAuditID = the last row of the prior
// page) rather than OFFSET, which degrades on a huge log.
type AuditFilter struct {
	ActorMemberID   *string
	ResourceType    *string
	ResourceID      *string
	Action          *string
	Since           *time.Time
	Until           *time.Time
	Limit           int
	CursorCreatedAt *time.Time
	CursorAuditID   *string
}

// Audit action vocabulary. Free-form text in the column; these constants keep
// call sites consistent.
const (
	AuditActionCreate      = "create"
	AuditActionUpdate      = "update"
	AuditActionDelete      = "delete"
	AuditActionRestore     = "restore"
	AuditActionPurge       = "purge"
	AuditActionLogin       = "login"
	AuditActionLoginFailed = "login_failed"
	AuditActionLogout      = "logout"
	AuditActionRefresh     = "refresh"
	AuditActionDevLogin    = "dev_login"
)

// Audit outcomes.
const (
	AuditOutcomeOK     = "ok"
	AuditOutcomeDenied = "denied"
	AuditOutcomeError  = "error"
)
