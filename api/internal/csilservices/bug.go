package csilservices

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

// BugService implements the in-app "Report a bug" affordance. It's a thin
// policy layer on top of ProjectService + TaskService: enforce the
// per-house bug_reports_enabled flag, find (or recreate) the configured
// target project, attribute the reporter in the description, then create
// the Task and attach it to the project.
type BugService struct{ Store store.Store }

// bugProjectName is the deterministic name used when the server has to
// create the target project itself (first ever submission, or admin
// re-pointed the setting at a deleted row). Admins can rename it after
// the fact; the pointer in settings — not the name — is what we rely on.
const bugProjectName = "Longhouse Bug Fixes from Users"

func (s *BugService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("bug", "ReportBug", csilrpc.Route(s.ReportBug, csil.DecodeBugReportBugRequest, csil.EncodeBugReportBugResponse))
}

func (s *BugService) ReportBug(ctx context.Context, req csil.BugReportRequest) (csil.Task, error) {
	if req.HouseId == "" {
		return csil.Task{}, csilrpc.BadRequest("house_id is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return csil.Task{}, csilrpc.BadRequest("title is required")
	}
	identity, callerMemberID, err := requireMemberForHouse(ctx, string(req.HouseId))
	if err != nil {
		return csil.Task{}, err
	}

	// Gate on the per-house feature flag. Server-side re-check so a client
	// on a different house can't bypass the UI hide.
	if !s.bugReportsEnabled(ctx, string(req.HouseId)) {
		return csil.Task{}, csilrpc.Forbidden("bug reports are not enabled for this house")
	}

	project, err := s.resolveTargetProject(ctx, string(req.HouseId), callerMemberID)
	if err != nil {
		return csil.Task{}, err
	}

	// Pick a task owner: first ProjectOwner by display_name (the store
	// already orders that way). Fall back to the caller if the project
	// has no owners yet — shouldn't happen because resolveTargetProject
	// always seeds one, but harmless if it does.
	owner := callerMemberID
	owners, _ := s.Store.ListProjectOwners(ctx, project.ProjectID)
	if len(owners) > 0 {
		owner = owners[0].MemberID
	}

	desc := derefStr(req.Description)
	desc = appendReporterAttribution(desc, identity, callerMemberID, s.Store, ctx)

	bugTag := "bug"
	task := &models.Task{
		HouseID:       string(req.HouseId),
		OwnerMemberID: owner,
		Title:         strings.TrimSpace(req.Title),
		Description:   desc,
		Status:        "open",
		Tag:           &bugTag,
	}
	if err := s.Store.CreateTask(ctx, task); err != nil {
		return csil.Task{}, csilrpc.Internal("internal error")
	}
	if err := s.Store.AddProjectTask(ctx, project.ProjectID, task.TaskID, nextProjectTaskPosition(ctx, s.Store, project.ProjectID)); err != nil {
		return csil.Task{}, csilrpc.Internal("internal error")
	}
	// Assign the picked owner so it shows on the task card too — matches
	// how regular TaskService.CreateTask defaults assignees when none are
	// supplied.
	_ = s.Store.AddTaskAssignee(ctx, task.TaskID, owner)

	assignees, _ := s.Store.ListTaskAssignees(ctx, task.TaskID)
	return taskToCSIL(task, assignees), nil
}

// bugReportsEnabled reads the per-house feature flag. Missing key = false.
// Decode errors are also treated as false — fail closed on bad data.
func (s *BugService) bugReportsEnabled(ctx context.Context, houseID string) bool {
	raw, present, err := readHouseSetting(ctx, s.Store, houseID, settingBugReportsEnabled)
	if err != nil || !present {
		return false
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false
	}
	return v
}

// resolveTargetProject returns the project the bug task should land in,
// recreating it if the configured pointer is stale (or unset). When
// recreated, the project gets one ProjectOwner seeded from the house's
// admin roster (deterministically the earliest-created admin). The
// pointer in house_settings is updated to the (possibly new) project id.
//
// If the configured pointer resolves to a real project, the project is
// used as-is — admins can re-point the setting at any project they want.
func (s *BugService) resolveTargetProject(ctx context.Context, houseID, callerMemberID string) (*models.Project, error) {
	if pidRaw, ok, err := readHouseSetting(ctx, s.Store, houseID, settingBugReportsProjectID); err == nil && ok {
		var pid string
		if err := json.Unmarshal(pidRaw, &pid); err == nil && pid != "" {
			p, err := s.Store.GetProjectByID(ctx, pid)
			if err == nil && p.HouseID == houseID {
				return p, nil
			}
			// Pointer is stale or points across houses — fall through to
			// recreate. We don't surface the original error to the caller;
			// the recovery path is silent by design.
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, csilrpc.Internal("internal error")
			}
		}
	}
	return s.createTargetProject(ctx, houseID, callerMemberID)
}

// createTargetProject creates the bug-reports project, seeds an admin
// ProjectOwner, and writes the new id back into house_settings.
func (s *BugService) createTargetProject(ctx context.Context, houseID, callerMemberID string) (*models.Project, error) {
	cat := "system"
	p := &models.Project{
		HouseID:     houseID,
		Name:        bugProjectName,
		Description: "User-submitted bug reports from the in-app reporter.",
		Category:    &cat,
		Status:      "active",
	}
	if err := s.Store.CreateProject(ctx, p); err != nil {
		return nil, csilrpc.Internal("internal error")
	}

	// Seed exactly one admin as ProjectOwner. "Any admin" per the design;
	// we pick the earliest-created admin so it's deterministic. If somehow
	// no admin exists (initial-admin seeding always provides one) fall
	// back to the caller so we don't end up with an ownerless project.
	admins, _ := s.Store.ListMembersWithRoleName(ctx, houseID, "admin")
	seedOwner := callerMemberID
	if len(admins) > 0 {
		seedOwner = admins[0].MemberID
	}
	_ = s.Store.AddProjectOwner(ctx, p.ProjectID, seedOwner)
	_ = s.Store.AddProjectMember(ctx, p.ProjectID, seedOwner)

	// Persist the pointer so subsequent reports find the same project.
	raw, _ := json.Marshal(p.ProjectID)
	_ = s.Store.UpsertHouseSetting(ctx, &models.HouseSetting{
		HouseID: houseID,
		Key:     settingBugReportsProjectID,
		Value:   raw,
		// updated_by intentionally nil — this is a system write, not an
		// admin action. The audit trail shows the absent updater as
		// "server-created".
	})
	return p, nil
}

// appendReporterAttribution adds a "Reported by <name>" footer to the
// description. Hidden from the reporter's own UI (they typed the body;
// they don't see what the server appends), but visible to the bug-fixers
// reading the task. Falls back to the caller's identity user_id if the
// member row has no display_name.
func appendReporterAttribution(desc string, identity *auth.Identity, memberID string, st store.Store, ctx context.Context) string {
	name := ""
	if identity != nil && identity.DisplayName != "" {
		name = identity.DisplayName
	}
	if name == "" {
		if m, err := st.GetMemberByID(ctx, memberID); err == nil && m.DisplayName != "" {
			name = m.DisplayName
		}
	}
	if name == "" && identity != nil {
		name = identity.UserID
	}
	if name == "" {
		name = "unknown"
	}
	if desc == "" {
		return "\n\n_Reported by " + name + "._"
	}
	return desc + "\n\n_Reported by " + name + "._"
}

// nextProjectTaskPosition appends the new task at the end of the project's
// task list. Reads the current task count and uses that as the position.
// Not race-free with concurrent writers but good enough for the bug-report
// flow where churn is low.
func nextProjectTaskPosition(ctx context.Context, st store.Store, projectID string) int {
	tasks, err := st.ListProjectTasks(ctx, projectID, 500, 0)
	if err != nil {
		return 0
	}
	return len(tasks)
}
