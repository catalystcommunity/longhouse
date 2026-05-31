package models

import "time"

// Access levels, ordered none < read < edit < full. Mirrors the CSIL
// AccessLevel enum and the access_level Postgres enum (migration 000011).
const (
	AccessNone = "none"
	AccessRead = "read"
	AccessEdit = "edit"
	AccessFull = "full"
)

// Grantee types. External (linkkeys) sharing lives in the shares table, not
// here. Mirrors the grantee_type Postgres enum.
const (
	GranteeMember = "member"
	GranteeGroup  = "group"
)

// TaskGrant is one (grantee, level) row against a task. grantee_id is a
// member_id or group_id per grantee_type. See docs/rbac.md.
type TaskGrant struct {
	TaskID      string    `gorm:"column:task_id;primaryKey" json:"task_id"`
	HouseID     string    `gorm:"column:house_id;not null" json:"house_id"`
	GranteeType string    `gorm:"column:grantee_type;type:grantee_type;primaryKey" json:"grantee_type"`
	GranteeID   string    `gorm:"column:grantee_id;primaryKey" json:"grantee_id"`
	AccessLevel string    `gorm:"column:access_level;type:access_level;not null" json:"access_level"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (TaskGrant) TableName() string { return "task_grants" }

// ProjectGrant is one (grantee, level) row against a project.
type ProjectGrant struct {
	ProjectID   string    `gorm:"column:project_id;primaryKey" json:"project_id"`
	HouseID     string    `gorm:"column:house_id;not null" json:"house_id"`
	GranteeType string    `gorm:"column:grantee_type;type:grantee_type;primaryKey" json:"grantee_type"`
	GranteeID   string    `gorm:"column:grantee_id;primaryKey" json:"grantee_id"`
	AccessLevel string    `gorm:"column:access_level;type:access_level;not null" json:"access_level"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (ProjectGrant) TableName() string { return "project_grants" }

// AccessRank maps an access level to an ordinal so callers can take MAX/MIN
// over surfaces. Unknown/empty ranks as none (0) — fail closed.
func AccessRank(level string) int {
	switch level {
	case AccessRead:
		return 1
	case AccessEdit:
		return 2
	case AccessFull:
		return 3
	default:
		return 0
	}
}

// MaxAccess returns the more-permissive of two levels.
func MaxAccess(a, b string) string {
	if AccessRank(a) >= AccessRank(b) {
		if AccessRank(a) == 0 {
			return AccessNone
		}
		return a
	}
	return b
}

// MinAccess returns the less-permissive of two levels (used by the umbrella
// guardrail: a resource can't be more visible than its least-visible
// container).
func MinAccess(a, b string) string {
	if AccessRank(a) <= AccessRank(b) {
		if AccessRank(a) == 0 {
			return AccessNone
		}
		return a
	}
	return b
}
