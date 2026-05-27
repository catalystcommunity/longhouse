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
// frequency, interval, byWeekday, and bySetpos.
//
// Semantics:
//   - hourly/daily: simple from + interval units.
//   - weekly: from + 7*interval days, then advanced to the next matching
//     weekday in byWeekday (if non-empty).
//   - monthly/quarterly/yearly: when byWeekday is non-empty, jumps to the
//     bySetpos-th matching weekday inside the *next period* (next month /
//     next quarter / next year). bySetpos can be 1..5 (first..fifth) or
//     -1 ("last"). nil bySetpos with non-empty byWeekday defaults to 1.
//     With empty byWeekday, falls back to `from + interval` of the unit.
//
// Calendar arithmetic notes:
//   - Monthly/Quarterly/Yearly without byWeekday use Go's time.AddDate,
//     which carries overflow forward (e.g. Jan 31 + 1 month = Mar 3 in
//     non-leap years). With byWeekday + bySetpos that's moot — we land on
//     a specific weekday inside the target period.
//   - The from value should be the previous occurrence's start (not
//     "now"). The worker passes prev.NextRecurrenceAt.
func Next(from time.Time, freq Frequency, interval int, byWeekday []int, bySetpos *int) (time.Time, error) {
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
		if len(byWeekday) == 0 {
			return from.AddDate(0, 0, 7*interval), nil
		}
		// With a weekday filter the spawning cadence becomes "every
		// matching weekday inside an active week". For interval == 1
		// every week is active, so we scan forward day-by-day from
		// `from+1` and return the first allowed weekday — that fires
		// once per matching weekday (e.g. Mon, Wed, Fri rather than
		// "the same weekday every week").
		// For interval > 1 we keep the older "jump to the active week
		// first, then find the first matching weekday in it" logic so
		// patterns like "every other Tuesday" still work.
		allowed := dedupeSortedWeekdays(byWeekday)
		if interval <= 1 {
			for offset := 1; offset <= 7; offset++ {
				candidate := from.AddDate(0, 0, offset)
				if containsWeekday(allowed, int(candidate.Weekday())) {
					return candidate, nil
				}
			}
			return from.AddDate(0, 0, 7), nil // unreachable
		}
		base := from.AddDate(0, 0, 7*interval)
		for offset := 0; offset < 7; offset++ {
			candidate := base.AddDate(0, 0, offset)
			if containsWeekday(allowed, int(candidate.Weekday())) {
				return candidate, nil
			}
		}
		return base, nil // unreachable in practice; keeps Go happy
	case Monthly, Quarterly, Yearly:
		// Nth-weekday-in-period: when both filters are set, land on the
		// bySetpos-th matching weekday in the next period. Time-of-day
		// is preserved from `from`.
		if len(byWeekday) > 0 {
			setpos := 1
			if bySetpos != nil && *bySetpos != 0 {
				setpos = *bySetpos
			}
			ps, pe := nextPeriodBounds(from, freq, interval)
			found := findSetposWeekday(ps, pe, byWeekday, setpos)
			return time.Date(
				found.Year(), found.Month(), found.Day(),
				from.Hour(), from.Minute(), from.Second(), from.Nanosecond(),
				from.Location(),
			), nil
		}
		// Plain interval bump when no weekday filter is in play.
		switch freq {
		case Monthly:
			return from.AddDate(0, interval, 0), nil
		case Quarterly:
			return from.AddDate(0, 3*interval, 0), nil
		case Yearly:
			return from.AddDate(interval, 0, 0), nil
		}
		return from, nil // unreachable
	default:
		return time.Time{}, ErrUnknownFrequency
	}
}

// nextPeriodBounds returns the [start, end] (inclusive, by day) of the
// monthly/quarterly/yearly period `interval` units after the period
// containing `from`. Boundaries are midnight at the start day in from's
// location.
func nextPeriodBounds(from time.Time, freq Frequency, interval int) (start, end time.Time) {
	loc := from.Location()
	switch freq {
	case Monthly:
		// First day of (current month + interval months); last day = day-0
		// of the month after that.
		first := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, loc).
			AddDate(0, interval, 0)
		last := first.AddDate(0, 1, -1)
		return first, last
	case Quarterly:
		// Quarter index of from (0..3), advance by interval, normalize.
		curQ := int(from.Month()-1) / 3
		nextQ := curQ + interval
		years := from.Year() + nextQ/4
		nextQ %= 4
		if nextQ < 0 {
			nextQ += 4
			years--
		}
		startMonth := time.Month(nextQ*3 + 1)
		first := time.Date(years, startMonth, 1, 0, 0, 0, 0, loc)
		last := first.AddDate(0, 3, -1)
		return first, last
	case Yearly:
		first := time.Date(from.Year()+interval, 1, 1, 0, 0, 0, 0, loc)
		last := time.Date(from.Year()+interval, 12, 31, 0, 0, 0, 0, loc)
		return first, last
	}
	return from, from
}

// findSetposWeekday returns the date in [start, end] matching the
// setpos-th occurrence of any weekday in byWeekday. setpos == -1 means
// "last matching weekday in the period". Falls back to the latest
// matching weekday if setpos overshoots the period (e.g. asking for the
// 5th Monday of a 4-Monday month).
func findSetposWeekday(start, end time.Time, byWeekday []int, setpos int) time.Time {
	allowed := dedupeSortedWeekdays(byWeekday)
	if len(allowed) == 0 {
		return start
	}
	if setpos == -1 {
		for day := end; !day.Before(start); day = day.AddDate(0, 0, -1) {
			if containsWeekday(allowed, int(day.Weekday())) {
				return day
			}
		}
		return end
	}
	count := 0
	var last time.Time
	matched := false
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		if containsWeekday(allowed, int(day.Weekday())) {
			count++
			last = day
			matched = true
			if count == setpos {
				return day
			}
		}
	}
	if matched {
		return last
	}
	return start
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

	next, err := Next(*root.NextRecurrenceAt, freq, interval, []int(root.RecurrenceByWeekday), root.RecurrenceBySetpos)
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
