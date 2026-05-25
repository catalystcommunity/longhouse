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
	return res, nil
}
