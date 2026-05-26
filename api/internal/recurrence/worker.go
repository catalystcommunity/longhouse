package recurrence

import (
	"context"
	"errors"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// WorkerStore is the subset of the application Store the recurrence worker
// needs. Defined here so the package stays import-free of the global
// store.AppStore — and so the Tick function is straightforward to test.
type WorkerStore interface {
	ListDueRecurringTasks(ctx context.Context, before time.Time, limit int) ([]models.Task, error)
	LatestRecurrenceChildOf(ctx context.Context, rootTaskID string) (*models.Task, error)
	CreateTask(ctx context.Context, task *models.Task) error
	UpdateTask(ctx context.Context, task *models.Task) error
	CreateComment(ctx context.Context, comment *models.Comment) error

	// Used to copy the root task's assignees onto the freshly-spawned child.
	// The recurrence flow is read-then-write, so this happens after the
	// child has its task_id from CreateTask.
	ListTaskAssignees(ctx context.Context, taskID string) ([]models.Member, error)
	AddTaskAssignee(ctx context.Context, taskID, memberID string) error

	// Event recurrence (parallels the task surface above).
	ListDueRecurringEvents(ctx context.Context, before time.Time, limit int) ([]models.Event, error)
	LatestRecurrenceChildOfEvent(ctx context.Context, rootEventID string) (*models.Event, error)
	CreateEvent(ctx context.Context, event *models.Event) error
	UpdateEvent(ctx context.Context, event *models.Event) error
}

// TickResult summarizes one Tick — useful for tests + observability.
type TickResult struct {
	Considered     int
	Spawned        int
	MissedComments int
	Errors         []error
}

// Tick scans for recurring tasks due at-or-before now, applies the spawn
// decision for each, and updates the root task's NextRecurrenceAt. It is
// safe to call repeatedly; callers (a goroutine ticker, a test, an admin
// "force tick" button) get a TickResult back.
func Tick(ctx context.Context, store WorkerStore, now time.Time) (*TickResult, error) {
	if store == nil {
		return nil, errors.New("recurrence: nil store")
	}
	due, err := store.ListDueRecurringTasks(ctx, now, 0)
	if err != nil {
		return nil, err
	}
	res := &TickResult{Considered: len(due)}

	for i := range due {
		root := due[i]
		prior, _ := store.LatestRecurrenceChildOf(ctx, root.TaskID) // not-found is fine

		dec, err := Plan(now, &root, prior)
		if err != nil {
			res.Errors = append(res.Errors, err)
			continue
		}

		if dec.MarkMissedOnID != "" && prior != nil {
			cm := &models.Comment{
				HouseID:    prior.HouseID,
				MemberID:   prior.OwnerMemberID,
				TargetType: "task",
				TargetID:   prior.TaskID,
				Body:       dec.MarkMissedReply,
			}
			if err := store.CreateComment(ctx, cm); err != nil {
				res.Errors = append(res.Errors, err)
				// We deliberately continue: the missed-marker is a nice-to-
				// have, the next-occurrence spawn is the load-bearing thing.
			} else {
				res.MissedComments++
			}
		}

		if dec.SpawnChild != nil {
			if err := store.CreateTask(ctx, dec.SpawnChild); err != nil {
				res.Errors = append(res.Errors, err)
				continue
			}
			res.Spawned++

			// Carry the root's assignees onto the child. A failure here is
			// noted but doesn't abort the tick — an unassigned occurrence is
			// still strictly better than no occurrence, and the next tick is
			// not load-bearing on this one having succeeded.
			if assignees, err := store.ListTaskAssignees(ctx, root.TaskID); err == nil {
				for _, m := range assignees {
					if err := store.AddTaskAssignee(ctx, dec.SpawnChild.TaskID, m.MemberID); err != nil {
						res.Errors = append(res.Errors, err)
					}
				}
			} else {
				res.Errors = append(res.Errors, err)
			}
		}

		// Bump the root forward.
		nextAt := dec.NextDueAt
		root.NextRecurrenceAt = &nextAt
		if err := store.UpdateTask(ctx, &root); err != nil {
			res.Errors = append(res.Errors, err)
		}
	}

	// Event recurrence pass — same shape as tasks but spawns Event rows.
	// Capped at ~2 years out by EventSpawnHorizon so a daily series doesn't
	// fill the table indefinitely.
	dueEvents, err := store.ListDueRecurringEvents(ctx, now.Add(EventSpawnHorizon), 0)
	if err == nil {
		for i := range dueEvents {
			res.Considered++
			root := dueEvents[i]
			if root.StartsAt == nil {
				continue
			}
			latest, _ := store.LatestRecurrenceChildOfEvent(ctx, root.EventID)
			anchorStart := *root.StartsAt
			if latest != nil && latest.StartsAt != nil {
				anchorStart = *latest.StartsAt
			}
			next, advErr := advanceEvent(anchorStart, *root.RecurrenceFreq, root.RecurrenceInterval)
			if advErr != nil {
				res.Errors = append(res.Errors, advErr)
				continue
			}
			// Walk forward spawning every occurrence up to either now or
			// the horizon, whichever is closer. The worker is bounded by
			// the loop count so a misconfigured root can't melt the DB.
			horizon := now.Add(EventSpawnHorizon)
			spawnedAny := false
			for k := 0; k < 50; k++ {
				if next.After(horizon) {
					break
				}
				child := spawnEventChild(&root, next)
				if err := store.CreateEvent(ctx, child); err != nil {
					res.Errors = append(res.Errors, err)
					break
				}
				res.Spawned++
				spawnedAny = true
				anchorStart = next
				next, advErr = advanceEvent(anchorStart, *root.RecurrenceFreq, root.RecurrenceInterval)
				if advErr != nil {
					res.Errors = append(res.Errors, advErr)
					break
				}
			}
			// Update the root's next_recurrence_at. If we already pushed
			// past the horizon, clear it so we don't keep scanning every
			// tick; the next tick after horizon time naturally re-queues
			// because we never set next_recurrence_at >= now somewhere
			// the WHERE clause cares about.
			if next.After(horizon) {
				root.NextRecurrenceAt = nil
			} else {
				root.NextRecurrenceAt = &next
			}
			if err := store.UpdateEvent(ctx, &root); err != nil {
				res.Errors = append(res.Errors, err)
			}
			_ = spawnedAny
		}
	} else {
		res.Errors = append(res.Errors, err)
	}

	return res, nil
}

// EventSpawnHorizon caps how far in the future the worker pre-creates
// event occurrences. Two years is generous enough for "every month" or
// "every quarter" series without filling the table for daily ones.
const EventSpawnHorizon = 2 * 365 * 24 * time.Hour

// spawnEventChild builds the next-occurrence Event row, copying the
// metadata of the root and shifting starts_at / ends_at by the same
// delta. The child carries `recurrence_root_event_id` and never sets
// its own recurrence_freq (children aren't roots).
func spawnEventChild(root *models.Event, occStart time.Time) *models.Event {
	var occEnd *time.Time
	if root.StartsAt != nil && root.EndsAt != nil {
		dur := root.EndsAt.Sub(*root.StartsAt)
		t := occStart.Add(dur)
		occEnd = &t
	}
	rootID := root.EventID
	return &models.Event{
		HouseID:               root.HouseID,
		OwnerMemberID:         root.OwnerMemberID,
		Title:                 root.Title,
		Description:           root.Description,
		Location:              root.Location,
		StartsAt:              &occStart,
		EndsAt:                occEnd,
		AllDay:                root.AllDay,
		RecurrenceRootEventID: &rootID,
	}
}

// advanceEvent walks a starts_at forward by interval × freq. Mirrors
// Next() for tasks but in a simpler form — events don't have a weekday
// list to honor.
func advanceEvent(start time.Time, freq string, interval int) (time.Time, error) {
	n := interval
	if n < 1 {
		n = 1
	}
	switch freq {
	case "hourly":
		return start.Add(time.Duration(n) * time.Hour), nil
	case "daily":
		return start.AddDate(0, 0, n), nil
	case "weekly":
		return start.AddDate(0, 0, 7*n), nil
	case "monthly":
		return start.AddDate(0, n, 0), nil
	case "quarterly":
		return start.AddDate(0, 3*n, 0), nil
	case "yearly":
		return start.AddDate(n, 0, 0), nil
	default:
		return time.Time{}, errors.New("recurrence: unknown event freq " + freq)
	}
}
