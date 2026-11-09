package csilservices

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

var errDeadlineReview = errors.New("task deadline calendar needs administrator review")

type deadlineSync struct {
	HouseID                      string `gorm:"primaryKey"`
	Generation, SyncedGeneration int64
	NextAttemptAt                time.Time
	LastError                    string
}

func (deadlineSync) TableName() string { return "calendar_deadline_sync" }

type categoryCalendar struct {
	HouseID    string `gorm:"primaryKey"`
	Category   string `gorm:"primaryKey"`
	CalendarID string
}

func (categoryCalendar) TableName() string { return "calendar_categories" }

type taskDeadline struct {
	HouseID       string `gorm:"primaryKey"`
	Category      string `gorm:"primaryKey"`
	TaskID        string `gorm:"primaryKey"`
	CreationKey   string `gorm:"default:gen_random_uuid()::text"`
	CreateRequest []byte
	RemoteEventID string
	AppliedBody   []byte
}

func (taskDeadline) TableName() string { return "calendar_deadlines" }

// Descendants and spawned task occurrences inherit their project's category.
// UNION removes duplicates and terminates if old data contains a task cycle.
const deadlineTasksCTE = `WITH RECURSIVE project_deadline_tasks AS (
 SELECT p.house_id, p.project_id, btrim(coalesce(p.category,'')) AS category, t.task_id
 FROM projects p JOIN project_tasks pt ON pt.project_id=p.project_id JOIN tasks t ON t.task_id=pt.task_id
 WHERE p.house_id=? AND p.deleted_at IS NULL AND t.house_id=p.house_id AND t.deleted_at IS NULL
 UNION
 SELECT tree.house_id, tree.project_id, tree.category, child.task_id
 FROM project_deadline_tasks tree JOIN tasks child
 ON (child.parent_task_id=tree.task_id OR child.recurrence_root_task_id=tree.task_id)
 WHERE child.house_id=tree.house_id AND child.deleted_at IS NULL
) `

type deadlineSource struct {
	models.Task `gorm:"embedded"`
	Category    string
}

// SyncTaskDeadlines processes durable jobs. Task writes do not wait for the
// calendar service. A periodic scan also repairs jobs missed by older versions.
func (s *CalendarService) SyncTaskDeadlines(ctx context.Context) error {
	if s.Remote == nil {
		return nil
	}
	var jobs []deadlineSync
	if err := s.DB.WithContext(ctx).Table("calendar_deadline_sync d").Select("d.*").Joins("JOIN house_calendars h ON h.house_id=d.house_id").Where("h.enabled AND h.ready AND NOT h.deleted AND d.next_attempt_at <= now()").Order("d.house_id").Find(&jobs).Error; err != nil {
		return err
	}
	var failures []error
	for _, job := range jobs {
		err := s.calendarLocked(ctx, job.HouseID, func(c *CalendarService) error { return c.syncDeadlineHouse(ctx, job.HouseID) })
		if err != nil {
			failures = append(failures, fmt.Errorf("house %s: %w", job.HouseID, err))
			message := "Task deadline calendar update failed. The server will retry."
			if errors.Is(err, errDeadlineReview) {
				message = "Task deadline calendar needs administrator review. An entry was moved, removed, or made inaccessible."
			}
			s.DB.WithContext(ctx).Model(&deadlineSync{}).Where("house_id=?", job.HouseID).Updates(map[string]any{"last_error": message, "next_attempt_at": time.Now().Add(time.Minute)})
		}
	}
	return errors.Join(failures...)
}

func (s *CalendarService) syncDeadlineHouse(ctx context.Context, house string) error {
	// Recheck after acquiring the same lock used by house setup and cleanup.
	h, err := s.house(ctx, house)
	if err != nil || !h.Enabled || !h.Ready || h.Deleted {
		return err
	}
	var job deadlineSync
	if err = s.DB.WithContext(ctx).First(&job, "house_id=?", house).Error; err != nil {
		return err
	}
	if job.NextAttemptAt.After(time.Now()) {
		return nil
	}
	var sources []deadlineSource
	if err = s.DB.WithContext(ctx).Raw(deadlineTasksCTE+`SELECT DISTINCT t.*, tree.category
 FROM tasks t JOIN project_deadline_tasks tree ON tree.task_id=t.task_id
 WHERE t.due_at IS NOT NULL ORDER BY category, task_id`, house).Scan(&sources).Error; err != nil {
		return err
	}
	calendars := map[string]categoryCalendar{}
	// Include categories without deadlines so each live project category has a calendar.
	var categories []string
	if err = s.DB.WithContext(ctx).Raw("SELECT DISTINCT btrim(coalesce(category,'')) FROM projects WHERE house_id=? AND deleted_at IS NULL ORDER BY 1", house).Scan(&categories).Error; err != nil {
		return err
	}
	for _, category := range categories {
		c, err := s.ensureCategoryCalendar(ctx, house, category)
		if err != nil {
			return err
		}
		calendars[category] = c
	}
	desired := map[string]cal.Event{}
	for _, source := range sources {
		c, ok := calendars[source.Category]
		if !ok { // A concurrent category edit will queue another pass.
			continue
		}
		desired[source.Category+"\x00"+source.TaskID] = deadlineEvent(source, c.CalendarID)
	}
	var rows []taskDeadline
	if err = s.DB.WithContext(ctx).Where("house_id=?", house).Order("category, task_id").Find(&rows).Error; err != nil {
		return err
	}
	events := cal.NewEventClient(s.Remote.ApplicationTransport())
	for i := range rows {
		row := &rows[i]
		// Finish a saved create before applying a later change or removal.
		if err = s.resolveDeadlineCreate(ctx, row); err != nil {
			return err
		}
		key := row.Category + "\x00" + row.TaskID
		want, ok := desired[key]
		if !ok {
			current, readErr := events.GetEvent(ctx, cal.EventID(row.RemoteEventID))
			if readErr != nil && !remoteNotFound(readErr) {
				return readErr
			}
			var category categoryCalendar
			if err = s.DB.WithContext(ctx).First(&category, "house_id=? AND category=?", house, row.Category).Error; err != nil {
				return err
			}
			if readErr == nil && string(current.CalendarId) != category.CalendarID {
				return fmt.Errorf("%w: task deadline moved out of its category calendar", errDeadlineReview)
			}
			if _, err = events.DeleteEvent(ctx, cal.DeleteEventRequest{EventId: cal.EventID(row.RemoteEventID)}); !remoteNotFound(err) {
				return err
			}
			if err = s.DB.WithContext(ctx).Where("house_id=? AND category=? AND task_id=?", row.HouseID, row.Category, row.TaskID).Delete(&taskDeadline{}).Error; err != nil {
				return err
			}
			continue
		}
		body := cal.EncodeEvent(want)
		changed := !bytes.Equal(body, row.AppliedBody)
		// The hourly pass verifies remote links, even when no source changed.
		var current cal.Event
		if changed || job.Generation == job.SyncedGeneration {
			current, err = events.GetEvent(ctx, cal.EventID(row.RemoteEventID))
			if err != nil {
				var remote *cal.ClientError
				if errors.As(err, &remote) && remote.Code == 404 {
					return fmt.Errorf("%w: task deadline is unavailable", errDeadlineReview)
				}
				return err
			}
			if current.CalendarId != want.CalendarId {
				return fmt.Errorf("%w: task deadline moved out of its category calendar", errDeadlineReview)
			}
		}
		if changed {
			want.EventId = current.EventId
			if _, err = events.UpdateEvent(ctx, cal.UpdateEventRequest{Event: want}); err != nil {
				return err
			}
			if err = s.DB.WithContext(ctx).Model(&taskDeadline{}).Where("house_id=? AND category=? AND task_id=?", row.HouseID, row.Category, row.TaskID).Update("applied_body", body).Error; err != nil {
				return err
			}
		}
		delete(desired, key)
	}
	// Sort to make retry order stable.
	keys := make([]string, 0, len(desired))
	for key := range desired {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.SplitN(key, "\x00", 2)
		row := taskDeadline{HouseID: house, Category: parts[0], TaskID: parts[1], CreateRequest: cal.EncodeEvent(desired[key])}
		// This commit must precede the remote create; the request survives a restart.
		if err = s.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
		if err = s.resolveDeadlineCreate(ctx, &row); err != nil {
			return err
		}
	}
	// A concurrent source write increments generation and requests another pass.
	return s.DB.WithContext(ctx).Exec(`UPDATE calendar_deadline_sync SET synced_generation=?, last_error='',
 next_attempt_at=CASE WHEN generation=? THEN now()+interval '1 hour' ELSE now() END WHERE house_id=?`, job.Generation, job.Generation, house).Error
}

func categoryLabel(category string) string {
	if category == "" {
		return "Uncategorized"
	}
	return category
}
func deadlineEvent(source deadlineSource, calendar string) cal.Event {
	start := cal.Timestamp(source.DueAt.UTC().Truncate(time.Second).Format(time.RFC3339))
	end := cal.Timestamp(source.DueAt.UTC().Truncate(time.Second).Add(time.Minute).Format(time.RFC3339))
	transparent := cal.EventTransparency("transparent")
	title := source.Title
	switch source.Status {
	case "done":
		title = "[Done] " + title
	case "cancelled":
		title = "[Cancelled] " + title
	}
	title = boundedCalendarText(title, 1024)
	return cal.Event{CalendarId: cal.CalendarID(calendar), Title: title, Start: &start, End: &end,
		Timezone: strPtrCopy("UTC"), Transparency: &transparent, Categories: []string{categoryLabel(source.Category)},
		Description: strPtrCopy("Task deadline. Edit this task in Longhouse."), Attendees: []cal.Attendee{}, Reminders: []cal.Reminder{}}
}

func boundedCalendarText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	end := 0
	for offset := range value {
		if offset > limit {
			break
		}
		end = offset
	}
	return value[:end]
}

func categoryCalendarName(house, category string) string {
	// Catalystcal limits names in bytes. Keep the readable prefix valid UTF-8.
	label := boundedCalendarText(categoryLabel(category), 80)
	return fmt.Sprintf("%s · longhouse:%s:deadlines:%x", label, house, sha256.Sum256([]byte(category)))
}

func (s *CalendarService) findCategoryCalendar(ctx context.Context, house, category string) (string, error) {
	list, err := cal.NewCalendarClient(s.Remote.ApplicationTransport()).ListCalendars(ctx, cal.EmptyRequest{})
	if err != nil {
		return "", err
	}
	app := s.Remote.ApplicationIdentity()
	id := ""
	for _, remote := range list.Calendars {
		if remote.Name == categoryCalendarName(house, category) && remote.Owner.ApplicationId != nil && *remote.Owner.ApplicationId == app.ApplicationId && remote.Owner.Domain == app.SubjectDomain && remote.Owner.Subject == app.SubjectUserId {
			if id != "" {
				return "", errors.New("duplicate category calendars require operator review")
			}
			id = string(remote.CalendarId)
		}
	}
	return id, nil
}

func (s *CalendarService) ensureCategoryCalendar(ctx context.Context, house, category string) (categoryCalendar, error) {
	c := categoryCalendar{HouseID: house, Category: category}
	err := s.DB.WithContext(ctx).First(&c, "house_id=? AND category=?", house, category).Error
	if err == nil && c.CalendarID != "" {
		return c, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return c, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Save the intent before the remote create, including for house cleanup.
		if err = s.DB.WithContext(ctx).Create(&c).Error; err != nil {
			return c, err
		}
	}
	c.CalendarID, err = s.findCategoryCalendar(ctx, house, category)
	if err != nil {
		return c, err
	}
	if c.CalendarID == "" {
		remote, err := cal.NewCalendarClient(s.Remote.ApplicationTransport()).CreateCalendar(ctx, cal.CreateCalendarRequest{
			Name: categoryCalendarName(house, category), Description: strPtrCopy("Longhouse task deadlines: " + categoryLabel(category)), Timezone: strPtrCopy("UTC"),
		})
		if err != nil {
			return c, err
		}
		c.CalendarID = string(remote.CalendarId)
	}
	// No shares: project and task grants are enforced by Longhouse on each read.
	return c, s.DB.WithContext(ctx).Model(&categoryCalendar{}).Where("house_id=? AND category=?", house, category).Update("calendar_id", c.CalendarID).Error
}

func (s *CalendarService) resolveDeadlineCreate(ctx context.Context, row *taskDeadline) error {
	if row.RemoteEventID != "" {
		return nil
	}
	event, err := cal.DecodeEvent(row.CreateRequest)
	if err != nil {
		return err
	}
	client := cal.NewEventClient(s.Remote.ApplicationTransport())
	receipt, err := client.GetCreateReceipt(ctx, cal.CreateReceiptRequest{CalendarId: event.CalendarId, IdempotencyKey: row.CreationKey})
	var remote *cal.ClientError
	if errors.As(err, &remote) && remote.Code == 404 {
		receipt, err = client.CreateEventOnce(ctx, cal.CreateEventOnceRequest{IdempotencyKey: row.CreationKey, Event: event})
	}
	if err != nil {
		return err
	}
	if receipt.State != "live" || receipt.Event == nil || receipt.Event.CalendarId != event.CalendarId || receipt.EventId == "" {
		return fmt.Errorf("%w: task deadline receipt is not live in its category calendar", errDeadlineReview)
	}
	row.RemoteEventID = string(receipt.EventId)
	row.AppliedBody = row.CreateRequest
	// The original payload is needed only until receipt recovery completes.
	row.CreateRequest = []byte{}
	return s.DB.WithContext(ctx).Model(&taskDeadline{}).Where("house_id=? AND category=? AND task_id=?", row.HouseID, row.Category, row.TaskID).Updates(map[string]any{
		"remote_event_id": row.RemoteEventID, "applied_body": row.AppliedBody, "create_request": row.CreateRequest,
	}).Error
}

func (s *CalendarService) deleteCategoryCalendars(ctx context.Context, db *gorm.DB, house string) error {
	var rows []categoryCalendar
	if err := db.Where("house_id=?", house).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if row.CalendarID == "" {
			id, err := s.findCategoryCalendar(ctx, house, row.Category)
			if err != nil {
				return err
			}
			row.CalendarID = id
		}
		if row.CalendarID == "" {
			continue
		}
		_, err := cal.NewCalendarClient(s.Remote.ApplicationTransport()).DeleteCalendar(ctx, cal.CalendarID(row.CalendarID))
		if !remoteNotFound(err) {
			return err
		}
	}
	return nil
}

func (s *CalendarService) ListProjectDeadlines(ctx context.Context, in csil.ProjectDeadlineRequest) (out csil.ProjectDeadlineList, err error) {
	out.Deadlines = []csil.ProjectDeadline{}
	project, err := s.Store.GetProjectByID(ctx, string(in.ProjectId))
	if err != nil || project.DeletedAt != nil {
		return out, csilrpc.NotFound("project not found")
	}
	member, err := s.member(ctx, project.HouseID, false)
	if err != nil {
		return out, err
	}
	identity, err := requireIdentity(ctx)
	if err != nil {
		return out, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, identity, project.HouseID, member.MemberID)
	// Calendar reads use current roles as well as current membership.
	g.isAdmin = false
	roles, err := s.Store.ListRolesForMember(ctx, member.MemberID)
	if err != nil {
		return out, err
	}
	for _, role := range roles {
		if role.HouseID == project.HouseID && role.Name == "admin" && role.DeletedAt == nil {
			g.isAdmin = true
		}
	}
	if !canRead(pol.projectAccess(ctx, project, g)) {
		return out, csilrpc.NotFound("project not found")
	}
	start, e1 := time.Parse(time.RFC3339Nano, string(in.RangeStart))
	end, e2 := time.Parse(time.RFC3339Nano, string(in.RangeEnd))
	if e1 != nil || e2 != nil || !end.After(start) || end.Sub(start) > 93*24*time.Hour {
		return out, csilrpc.BadRequest("deadline range must be greater than zero and at most 93 days")
	}
	category := strings.Trim(derefStr(project.Category), " ")
	out.Category = categoryLabel(category)
	h, err := s.house(ctx, project.HouseID)
	if err != nil {
		return out, err
	}
	out.Enabled = s.Remote != nil && h.Enabled
	if !out.Enabled {
		return out, nil
	}
	var job deadlineSync
	if err = s.DB.WithContext(ctx).First(&job, "house_id=?", project.HouseID).Error; err != nil {
		return out, err
	}
	out.Pending = !h.Ready || job.Generation != job.SyncedGeneration
	out.LastError = strPtrCopy(job.LastError)
	var c categoryCalendar
	err = s.DB.WithContext(ctx).First(&c, "house_id=? AND category=?", project.HouseID, category).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && c.CalendarID == "") {
		out.Pending = true
		return out, nil
	}
	if err != nil {
		return out, err
	}
	// Resolve current project membership and task access, not remote group shares.
	var tasks []models.Task
	if err = s.DB.WithContext(ctx).Raw(deadlineTasksCTE+`SELECT DISTINCT t.* FROM tasks t JOIN project_deadline_tasks tree ON tree.task_id=t.task_id WHERE tree.project_id=? AND t.due_at IS NOT NULL`, project.HouseID, project.ProjectID).Scan(&tasks).Error; err != nil {
		return out, err
	}
	permitted := map[string]models.Task{}
	for _, task := range tasks {
		if canRead(pol.taskAccess(ctx, &task, g)) {
			permitted[task.TaskID] = task
		}
	}
	var refs []taskDeadline
	if err = s.DB.WithContext(ctx).Where("house_id=? AND category=? AND remote_event_id<>''", project.HouseID, category).Find(&refs).Error; err != nil {
		return out, err
	}
	ids := map[string]models.Task{}
	for _, ref := range refs {
		if task, ok := permitted[ref.TaskID]; ok {
			ids[ref.RemoteEventID] = task
		}
	}
	no := false
	result, err := cal.NewEventClient(s.Remote.ApplicationTransport()).ListEvents(ctx, cal.ListEventsRequest{RangeStart: cal.Timestamp(in.RangeStart), RangeEnd: cal.Timestamp(in.RangeEnd), CalendarIds: []cal.CalendarID{cal.CalendarID(c.CalendarID)}, IncludeInvitations: &no})
	if err != nil {
		return out, remoteError(err)
	}
	if result.Truncated {
		return out, csilrpc.Conflict("too many deadlines; use a shorter date range")
	}
	for _, occ := range result.Occurrences {
		task, ok := ids[string(occ.Event.EventId)]
		at, parseErr := time.Parse(time.RFC3339Nano, string(occ.Start))
		if !ok || occ.Event.CalendarId != cal.CalendarID(c.CalendarID) || parseErr != nil || at.Before(start) || !at.Before(end) {
			continue
		}
		// Do not show an old date while a deadline change is waiting for sync.
		if !at.Equal(task.DueAt.Truncate(time.Second)) {
			out.Pending = true
			continue
		}
		out.Deadlines = append(out.Deadlines, csil.ProjectDeadline{TaskId: csil.TaskID(task.TaskID), Title: task.Title, DueAt: csil.Timestamp(occ.Start)})
	}
	if !out.Pending && job.LastError == "" {
		expected := 0
		for _, task := range permitted {
			due := task.DueAt.Truncate(time.Second)
			if !due.Before(start) && due.Before(end) {
				expected++
			}
		}
		if len(out.Deadlines) < expected {
			out.LastError = strPtrCopy("Some deadlines are unavailable in the calendar. Ask a house administrator to check the calendar status.")
		}
	}
	sort.Slice(out.Deadlines, func(i, j int) bool { return out.Deadlines[i].DueAt < out.Deadlines[j].DueAt })
	return out, nil
}
