package models

import "time"

type Project struct {
	ProjectID   string    `gorm:"column:project_id;primaryKey;default:generate_ulid()" json:"project_id"`
	HouseID     string    `gorm:"column:house_id;not null" json:"house_id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description string    `gorm:"column:description;not null;default:''" json:"description"`
	Category    *string   `gorm:"column:category" json:"category,omitempty"`
	Status      string    `gorm:"column:status;not null;default:'active'" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Project) TableName() string { return "projects" }

type ProjectTask struct {
	ProjectID string    `gorm:"column:project_id;primaryKey" json:"project_id"`
	TaskID    string    `gorm:"column:task_id;primaryKey" json:"task_id"`
	Position  int       `gorm:"column:position;not null" json:"position"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (ProjectTask) TableName() string { return "project_tasks" }

// ProjectMember + ProjectOwner are separate join tables — owners are a
// smaller list the UI surfaces independently. Semantically a strict subset
// of members, not enforced at the storage layer.
type ProjectMember struct {
	ProjectID string    `gorm:"column:project_id;primaryKey" json:"project_id"`
	MemberID  string    `gorm:"column:member_id;primaryKey"  json:"member_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null"   json:"created_at"`
}

func (ProjectMember) TableName() string { return "project_members" }

type ProjectOwner struct {
	ProjectID string    `gorm:"column:project_id;primaryKey" json:"project_id"`
	MemberID  string    `gorm:"column:member_id;primaryKey"  json:"member_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null"   json:"created_at"`
}

func (ProjectOwner) TableName() string { return "project_owners" }

// Milestone is a project timeline marker; `Position` orders the ribbon,
// `State` drives the visual (done/current/future).
type Milestone struct {
	MilestoneID string    `gorm:"column:milestone_id;primaryKey;default:gen_random_uuid()" json:"milestone_id"`
	ProjectID   string    `gorm:"column:project_id;not null" json:"project_id"`
	Label       string    `gorm:"column:label;not null" json:"label"`
	WhenLabel   string    `gorm:"column:when_label;not null" json:"when_label"`
	State       string    `gorm:"column:state;type:milestone_state;not null" json:"state"`
	Position    int       `gorm:"column:position;not null" json:"position"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Milestone) TableName() string { return "milestones" }

const (
	MilestoneStateDone    = "done"
	MilestoneStateCurrent = "current"
	MilestoneStateFuture  = "future"
)
