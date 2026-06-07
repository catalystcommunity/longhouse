package models

import "time"

// Dependency node types — either end of an edge is a task or a project.
const (
	DependencyTask    = "task"
	DependencyProject = "project"
)

// Dependency is one directed edge: the (DependentType, DependentID) work
// item depends on the (DependencyType, DependencyID) work item. Only this
// direction is stored; the reverse view is computed by querying with the
// columns swapped. See migration 000013.
type Dependency struct {
	HouseID        string    `gorm:"column:house_id;not null" json:"house_id"`
	DependentType  string    `gorm:"column:dependent_type;type:dependency_node_type;primaryKey" json:"dependent_type"`
	DependentID    string    `gorm:"column:dependent_id;primaryKey" json:"dependent_id"`
	DependencyType string    `gorm:"column:dependency_type;type:dependency_node_type;primaryKey" json:"dependency_type"`
	DependencyID   string    `gorm:"column:dependency_id;primaryKey" json:"dependency_id"`
	CreatedAt      time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (Dependency) TableName() string { return "dependencies" }

// ValidDependencyNodeType reports whether s is a known edge-endpoint type.
func ValidDependencyNodeType(s string) bool {
	return s == DependencyTask || s == DependencyProject
}
