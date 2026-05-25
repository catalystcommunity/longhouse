// Package recurrence computes the next-occurrence timestamp for a task
// given its (RecurrenceFreq, RecurrenceInterval, RecurrenceByWeekday)
// triple plus a reference time. It also defines the "spawn" decision a
// scheduler/worker uses to roll the next occurrence and to mark the prior
// one as missed when it wasn't completed.
//
// The functions here are intentionally pure: they take inputs, return
// outputs, and never call into the store or the clock. The worker that
// drives them is one level up; this package is straightforward to test
// against any combination of freq/interval/byweekday/now.
package recurrence

import (
	"errors"
	"sort"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// Frequency mirrors the recurrence_freq enum in the DB.
type Frequency = string

const (
	Hourly    Frequency = "hourly"
	Daily     Frequency = "daily"
	Weekly    Frequency = "weekly"
	Monthly   Frequency = "monthly"
	Quarterly Frequency = "quarterly"
	Yearly    Frequency = "yearly"
)

// ErrUnknownFrequency is returned by Next when the recurrence_freq value
// isn't one of the canonical strings.
var ErrUnknownFrequency = errors.New("recurrence: unknown frequency")

// Next returns the next occurrence at-or-after `from` for the given
// frequency, interval, and weekday filter. byWeekday only applies to
// `weekly` recurrence: when set, the next occurrence is bumped forward
// to the next configured weekday after applying the interval.
//
// Calendar arithmetic notes:
//   - Monthly/Quarterly/Yearly use Go's time.AddDate, which carries
//     overflow forward (e.g. Jan 31 + 1 month = Mar 3 in non-leap years).
//     For the operations we support that's acceptable; it matches what
//     most calendaring apps do.
//   - The from value should be the previous occurrence's start (not
//     "now"). The worker passes prev.NextRecurrenceAt.
func Next(from time.Time, freq Frequency, interval int, byWeekday []int) (time.Time, error) {
	if interval < 1 {
		interval = 1
	}
	from = from.UTC()
	switch freq {
	case Hourly:
		return from.Add(time.Duration(interval) * time.Hour), nil
	case Daily:
		return from.AddDate(0, 0, interval), nil
	case Weekly:
		base := from.AddDate(0, 0, 7*interval)
		if len(byWeekday) == 0 {
			return base, nil
		}
		// Scan up to 7 days from base to find the first allowed weekday.
		// We sort + dedupe so callers can pass [5,5,1] without surprises.
		allowed := dedupeSortedWeekdays(byWeekday)
		for offset := 0; offset < 7; offset++ {
			candidate := base.AddDate(0, 0, offset)
			if containsWeekday(allowed, int(candidate.Weekday())) {
				return candidate, nil
			}
		}
		return base, nil // unreachable in practice; keeps Go happy
	case Monthly:
		return from.AddDate(0, interval, 0), nil
	case Quarterly:
		return from.AddDate(0, 3*interval, 0), nil
	case Yearly:
		return from.AddDate(interval, 0, 0), nil
	default:
		return time.Time{}, ErrUnknownFrequency
	}
}

func dedupeSortedWeekdays(in []int) []int {
	clean := make([]int, 0, len(in))
	seen := map[int]bool{}
	for _, d := range in {
		if d < 0 || d > 6 || seen[d] {
			continue
		}
		seen[d] = true
		clean = append(clean, d)
	}
	sort.Ints(clean)
	return clean
}

func containsWeekday(allowed []int, w int) bool {
	for _, d := range allowed {
		if d == w {
			return true
		}
	}
	return false
}

// SpawnDecision describes what the worker should do for a single recurring
// task at tick-time. Exactly one of:
//
//   - SpawnChild non-nil: write a new child Task and bump the parent's
//     NextRecurrenceAt forward.
//   - MarkMissed non-nil: prior child wasn't completed; the worker
//     records a comment on it and STILL spawns SpawnChild for the next
//     occurrence.
//
// The worker can use both fields together for the common "missed previous,
// spawn next" case.
type SpawnDecision struct {
	SpawnChild      *models.Task
	NextDueAt       time.Time
	MarkMissedOnID  string // comment on this task (the prior child) when set
	MarkMissedReply string // body of the missed-occurrence comment
}

// Plan computes a SpawnDecision for a recurrence root task that's due
// (root.NextRecurrenceAt <= now). priorChild is the most-recently-spawned
// occurrence (nil if this is the first spawn). The decision describes what
// the caller should write to the store; this function makes no I/O.
func Plan(now time.Time, root *models.Task, priorChild *models.Task) (*SpawnDecision, error) {
	if root.RecurrenceFreq == nil {
		return nil, errors.New("recurrence: root has no RecurrenceFreq")
	}
	if root.NextRecurrenceAt == nil {
		return nil, errors.New("recurrence: root has no NextRecurrenceAt")
	}
	if root.DeletedAt != nil {
		return nil, errors.New("recurrence: root is soft-deleted")
	}

	freq := *root.RecurrenceFreq
	interval := root.RecurrenceInterval
	if interval < 1 {
		interval = 1
	}

	next, err := Next(*root.NextRecurrenceAt, freq, interval, []int(root.RecurrenceByWeekday))
	if err != nil {
		return nil, err
	}

	dec := &SpawnDecision{
		NextDueAt: next,
		SpawnChild: &models.Task{
			HouseID:              root.HouseID,
			OwnerMemberID:        root.OwnerMemberID,
			AssignedToSkillID:    root.AssignedToSkillID,
			Title:                root.Title,
			Description:          root.Description,
			Status:               "open",
			RecurrenceRootTaskID: ptrString(root.TaskID),
			RecurrenceInterval:   1, // children carry no recurrence themselves
		},
		// TODO(csilrpc-restructure): copy the root's task_assignees rows onto
		// the spawned child too. With the single assigned_to_member_id column
		// gone, that's now a store-level operation the worker has to invoke
		// after the child task is inserted. Tracked in the handler restructure
		// work — until then, recurrence children spawn without assignees.
	}
	if priorChild != nil && priorChild.Status != "done" && priorChild.Status != "cancelled" {
		dec.MarkMissedOnID = priorChild.TaskID
		dec.MarkMissedReply = "missed an occurrence"
	}
	return dec, nil
}

func ptrString(s string) *string { return &s }
