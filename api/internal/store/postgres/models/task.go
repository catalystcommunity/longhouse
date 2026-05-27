package models

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// IntList is a custom type for PostgreSQL integer[] columns.
type IntList []int

func (i IntList) Value() (driver.Value, error) {
	if i == nil {
		return nil, nil
	}
	parts := make([]string, len(i))
	for idx, v := range i {
		parts[idx] = strconv.Itoa(v)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func (i *IntList) Scan(src interface{}) error {
	if src == nil {
		*i = nil
		return nil
	}
	var str string
	switch v := src.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into IntList", src)
	}
	str = strings.Trim(str, "{}")
	if str == "" {
		*i = IntList{}
		return nil
	}
	parts := strings.Split(str, ",")
	out := make(IntList, len(parts))
	for idx, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return fmt.Errorf("scan IntList element %q: %w", p, err)
		}
		out[idx] = n
	}
	*i = out
	return nil
}

type Task struct {
	TaskID               string     `gorm:"column:task_id;primaryKey;default:generate_ulid()" json:"task_id"`
	HouseID              string     `gorm:"column:house_id;not null" json:"house_id"`
	OwnerMemberID        string     `gorm:"column:owner_member_id;not null" json:"owner_member_id"`
	AssignedToSkillID    *string    `gorm:"column:assigned_to_skill_id" json:"assigned_to_skill_id,omitempty"`
	ParentTaskID         *string    `gorm:"column:parent_task_id" json:"parent_task_id,omitempty"`
	Title                string     `gorm:"column:title;not null" json:"title"`
	Description          string     `gorm:"column:description;not null;default:''" json:"description"`
	Status               string     `gorm:"column:status;type:task_status;not null;default:'open'" json:"status"`
	DueAt                *time.Time `gorm:"column:due_at" json:"due_at,omitempty"`
	Tag                  *string    `gorm:"column:tag" json:"tag,omitempty"`
	EstimateMinutes      *int       `gorm:"column:estimate_minutes" json:"estimate_minutes,omitempty"`
	RecurrenceFreq       *string    `gorm:"column:recurrence_freq;type:recurrence_freq" json:"recurrence_freq,omitempty"`
	RecurrenceInterval   int        `gorm:"column:recurrence_interval;not null;default:1" json:"recurrence_interval"`
	RecurrenceByWeekday  IntList    `gorm:"column:recurrence_by_weekday;type:integer[]" json:"recurrence_by_weekday,omitempty"`
	RecurrenceBySetpos   *int       `gorm:"column:recurrence_by_setpos" json:"recurrence_by_setpos,omitempty"`
	NextRecurrenceAt     *time.Time `gorm:"column:next_recurrence_at" json:"next_recurrence_at,omitempty"`
	RecurrenceRootTaskID *string    `gorm:"column:recurrence_root_task_id" json:"recurrence_root_task_id,omitempty"`
	DeletedAt            *time.Time `gorm:"column:deleted_at" json:"deleted_at,omitempty"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Task) TableName() string { return "tasks" }

// TaskAssignee is the join row in task_assignees.
type TaskAssignee struct {
	TaskID    string    `gorm:"column:task_id;primaryKey" json:"task_id"`
	MemberID  string    `gorm:"column:member_id;primaryKey" json:"member_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (TaskAssignee) TableName() string { return "task_assignees" }

// Recurrence frequency values matching the recurrence_freq enum in the DB.
const (
	RecurrenceHourly    = "hourly"
	RecurrenceDaily     = "daily"
	RecurrenceWeekly    = "weekly"
	RecurrenceMonthly   = "monthly"
	RecurrenceQuarterly = "quarterly"
	RecurrenceYearly    = "yearly"
)
