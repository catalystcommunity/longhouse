package csilservices

import (
	"context"
	"errors"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	bridge "github.com/catalystcommunity/longhouse/api/internal/calendar"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type houseCalendar struct {
	HouseID                                   string `gorm:"primaryKey"`
	Enabled                                   bool
	CalendarID, MembersGroupID, AdminsGroupID string
	Ready                                     bool
	SyncGeneration, SyncedGeneration          int64
	SyncPendingSince, NextAttemptAt           time.Time
	LastError                                 string
	Deleted                                   bool
}

func (houseCalendar) TableName() string { return "house_calendars" }

type calendarConnection struct {
	MemberID       string `gorm:"primaryKey"`
	GrantID, Nonce string
	NonceExpiresAt *time.Time
	Broken         bool
}
type calendarRef struct {
	RefID                                string `gorm:"primaryKey"`
	HouseID, RemoteEventID, RecurrenceID string
}
type CalendarRemote interface {
	ApplicationTransport() cal.Transport
	ApplicationIdentity() cal.ApplicationRef
	MemberDomain() string
	Begin(context.Context, string) (string, error)
	Member(context.Context, string, string, string) (cal.Transport, error)
}

type CalendarService struct {
	lockedHouse string
	Store       store.Store
	DB          *gorm.DB
	Remote      CalendarRemote
}

func (s *CalendarService) native() *EventService { return &EventService{Store: s.Store} }
func (s *CalendarService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("calendar", "GetStatus", csilrpc.Route(s.GetStatus, csil.DecodeCalendarGetStatusRequest, csil.EncodeCalendarStatus))
	d.RegisterTyped("calendar", "ConfigureCalendar", csilrpc.Route(s.ConfigureCalendar, csil.DecodeConfigureCalendarRequest, csil.EncodeCalendarStatus))
	d.RegisterTyped("calendar", "BeginConnect", csilrpc.Route(s.BeginConnect, csil.DecodeCalendarBeginConnectRequest, csil.EncodeCalendarConnectResponse))
	d.RegisterTyped("calendar", "CompleteConnect", csilrpc.Route(s.CompleteConnect, csil.DecodeCompleteCalendarConnectRequest, csil.EncodeCalendarStatus))
	d.RegisterTyped("calendar", "Disconnect", csilrpc.Route(s.Disconnect, csil.DecodeCalendarDisconnectRequest, csil.EncodeEmptyResponse))
	d.RegisterTyped("calendar", "PreviewMigration", csilrpc.Route(s.PreviewMigration, csil.DecodeCalendarPreviewMigrationRequest, csil.EncodeCalendarPreviewMigrationResponse))
	d.RegisterTyped("calendar", "ListTrash", csilrpc.Route(s.ListTrash, csil.DecodeCalendarListTrashRequest, csil.EncodeCalendarListTrashResponse))
	d.RegisterTyped("calendar", "RestoreEvent", csilrpc.Route(s.RestoreEvent, csil.DecodeRestoreCalendarEventRequest, csil.EncodeEvent))
}

// Recheck membership and roles from the database. A stale bearer cannot retain
// calendar rights after a removal or demotion.
func (s *CalendarService) member(ctx context.Context, house string, admin bool) (*models.Member, error) {
	identity, id, err := requireMemberForHouse(ctx, house)
	if err != nil {
		return nil, err
	}
	member, err := s.Store.GetMemberByID(ctx, id)
	if err != nil {
		return nil, csilrpc.Forbidden("active house membership required")
	}
	if member.HouseID != house || member.DeactivatedAt != nil || member.LinkkeysDomain != identity.Domain || member.LinkkeysUserID != identity.UserID {
		return nil, csilrpc.Forbidden("active house membership required")
	}
	if admin {
		roles, err := s.Store.ListRolesForMember(ctx, id)
		if err != nil {
			return nil, err
		}
		ok := false
		for _, r := range roles {
			if r.HouseID == house && r.Name == "admin" && r.DeletedAt == nil {
				ok = true
			}
		}
		if !ok {
			return nil, csilrpc.Forbidden("house admin required")
		}
	}
	return member, nil
}
func (s *CalendarService) house(ctx context.Context, id string) (houseCalendar, error) {
	var h houseCalendar
	err := s.DB.WithContext(ctx).First(&h, "house_id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return houseCalendar{HouseID: id}, nil
	}
	return h, err
}
func (s *CalendarService) GetStatus(ctx context.Context, id csil.HouseID) (csil.CalendarStatus, error) {
	m, err := s.member(ctx, string(id), false)
	if err != nil {
		return csil.CalendarStatus{}, err
	}
	h, err := s.house(ctx, string(id))
	if err != nil {
		return csil.CalendarStatus{}, err
	}
	out := csil.CalendarStatus{HouseId: id, Available: s.Remote != nil, Enabled: h.Enabled, Ready: h.Ready, LastError: strPtrCopy(h.LastError)}
	if h.SyncGeneration != h.SyncedGeneration {
		out.SyncPendingSince = tsPtr(&h.SyncPendingSince)
	}
	var c calendarConnection
	err = s.DB.WithContext(ctx).First(&c, "member_id = ?", m.MemberID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	out.Connected = c.GrantID != "" && !c.Broken
	return out, nil
}
func (s *CalendarService) ConfigureCalendar(ctx context.Context, in csil.ConfigureCalendarRequest) (csil.CalendarStatus, error) {
	if _, err := s.member(ctx, string(in.HouseId), true); err != nil {
		return csil.CalendarStatus{}, err
	}
	if in.Enabled && s.Remote == nil {
		return csil.CalendarStatus{}, csilrpc.Conflict("catalystcal is not configured on this server")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "calendar-house:"+string(in.HouseId)).Error; err != nil {
			return err
		}
		var h houseCalendar
		err := tx.First(&h, "house_id = ?", in.HouseId).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if !in.Enabled && h.Enabled {
			return csilrpc.Conflict("export and migrate published events before disabling this house calendar")
		}
		if h.Enabled {
			return nil
		}
		if !in.Enabled {
			return nil
		}
		// The old owner column does not prove consent. Preserve every source row.
		if err = tx.Exec("INSERT INTO calendar_legacy_backup(event_id,house_id,body) SELECT event_id,house_id,to_jsonb(events) FROM events WHERE house_id=? ON CONFLICT DO NOTHING", in.HouseId).Error; err != nil {
			return err
		}
		h = houseCalendar{HouseID: string(in.HouseId), Enabled: true, SyncGeneration: 1, SyncPendingSince: time.Now(), NextAttemptAt: time.Now()}
		return tx.Create(&h).Error
	})
	if err != nil {
		return csil.CalendarStatus{}, err
	}
	return s.GetStatus(ctx, in.HouseId)
}
func (s *CalendarService) BeginConnect(ctx context.Context, id csil.HouseID) (csil.CalendarConnectResponse, error) {
	m, err := s.member(ctx, string(id), false)
	if err != nil {
		return csil.CalendarConnectResponse{}, err
	}
	h, err := s.house(ctx, string(id))
	if err != nil {
		return csil.CalendarConnectResponse{}, err
	}
	if s.Remote == nil || !h.Enabled || !h.Ready {
		return csil.CalendarConnectResponse{}, csilrpc.Conflict("house calendar is not ready")
	}
	if m.LinkkeysDomain != s.Remote.MemberDomain() {
		return csil.CalendarConnectResponse{}, csilrpc.BadRequest("calendar consent is not configured for this identity domain")
	}
	nonce := bridge.Nonce()
	url, err := s.Remote.Begin(ctx, nonce)
	if err != nil {
		return csil.CalendarConnectResponse{}, remoteError(err)
	}
	expiry := time.Now().Add(10 * time.Minute)
	c := calendarConnection{MemberID: m.MemberID, Nonce: nonce, NonceExpiresAt: &expiry}
	err = s.DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "member_id"}}, DoUpdates: clause.AssignmentColumns([]string{"nonce", "nonce_expires_at"})}).Create(&c).Error
	return csil.CalendarConnectResponse{Url: url}, err
}
func (s *CalendarService) CompleteConnect(ctx context.Context, in csil.CompleteCalendarConnectRequest) (csil.CalendarStatus, error) {
	m, err := s.member(ctx, string(in.HouseId), false)
	if err != nil {
		return csil.CalendarStatus{}, err
	}
	if s.Remote == nil {
		return csil.CalendarStatus{}, csilrpc.Conflict("catalystcal is not configured")
	}
	var c calendarConnection
	if err = s.DB.WithContext(ctx).First(&c, "member_id = ? AND nonce = ? AND nonce <> '' AND nonce_expires_at > now()", m.MemberID, in.Nonce).Error; err != nil {
		return csil.CalendarStatus{}, csilrpc.BadRequest("calendar connection expired or belongs to another member")
	}
	if _, err = s.Remote.Member(ctx, m.LinkkeysDomain, m.LinkkeysUserID, in.GrantId); err != nil {
		return csil.CalendarStatus{}, remoteError(err)
	}
	result := s.DB.WithContext(ctx).Model(&c).Where("nonce = ? AND nonce_expires_at > now()", in.Nonce).Updates(map[string]any{"grant_id": in.GrantId, "nonce": "", "nonce_expires_at": nil, "broken": false})
	if result.Error != nil {
		return csil.CalendarStatus{}, result.Error
	}
	if result.RowsAffected != 1 {
		return csil.CalendarStatus{}, csilrpc.Conflict("calendar connection was already completed")
	}
	return s.GetStatus(ctx, in.HouseId)
}
func (s *CalendarService) Disconnect(ctx context.Context, id csil.HouseID) (csil.EmptyResponse, error) {
	m, err := s.member(ctx, string(id), false)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	return csil.EmptyResponse{}, s.DB.WithContext(ctx).Where("member_id = ?", m.MemberID).Delete(&calendarConnection{}).Error
}
func (s *CalendarService) writer(ctx context.Context, m *models.Member) (cal.Transport, error) {
	if s.Remote == nil {
		return nil, csilrpc.Conflict("catalystcal is not configured")
	}
	var c calendarConnection
	if err := s.DB.WithContext(ctx).First(&c, "member_id = ?", m.MemberID).Error; err != nil || c.GrantID == "" || c.Broken {
		return nil, csilrpc.Conflict("connect your calendar before changing published events")
	}
	transport, err := s.Remote.Member(ctx, m.LinkkeysDomain, m.LinkkeysUserID, c.GrantID)
	if err != nil {
		var remote *cal.ClientError
		if errors.Is(err, bridge.ErrGrantUnavailable) {
			if updateErr := s.DB.WithContext(ctx).Model(&c).Update("broken", true).Error; updateErr != nil {
				return nil, updateErr
			}
			return nil, csilrpc.Conflict("connect your calendar again; its authorization could not be verified")
		}
		if errors.As(err, &remote) && (remote.Code == 401 || remote.Code == 403) {
			return nil, csilrpc.Conflict("calendar authorization could not be checked; retry shortly, or reconnect if the problem continues")
		}
		return nil, remoteError(err)
	}
	return transport, nil
}
func remoteError(err error) error {
	if err == nil {
		return nil
	}
	var e *cal.ClientError
	if errors.As(err, &e) && e.Code >= 400 && e.Code < 500 {
		return &csilrpc.Error{Code: int(e.Code), Message: e.Message}
	}
	log.WithError(err).Warn("catalystcal request failed")
	return csilrpc.Internal("calendar service is unavailable; retry later")
}
func (s *CalendarService) PreviewMigration(ctx context.Context, id csil.HouseID) ([]csil.CalendarMigrationRow, error) {
	if _, err := s.member(ctx, string(id), true); err != nil {
		return nil, err
	}
	var rows []models.Event
	if err := s.DB.WithContext(ctx).Where("house_id = ? AND calendar_state <> 'remote'", id).Order("event_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := []csil.CalendarMigrationRow{}
	for _, e := range rows {
		disposition, reason := "owner_review", "The owner field does not prove consent. The owner must accept this event."
		switch {
		case e.DeletedAt != nil:
			disposition, reason = "retained_trash", "Keep this deleted source row in the migration backup. Do not publish it."
		case e.CalendarState == "pending":
			disposition, reason = "pending_acceptance", "Keep this assignment in Longhouse until its owner accepts the saved revision."
		case e.CalendarState == "declined":
			disposition, reason = "declined", "Keep this declined assignment in Longhouse. It is not approved for publication."
		case e.CalendarState == "publishing":
			disposition, reason = "publication_recovery", "Recover the saved publication request. Do not create a replacement."
		case e.StartsAt == nil:
			disposition, reason = "unscheduled", "Keep this item in Longhouse until the owner gives it a start and end time."
		}
		if e.DeletedAt == nil && (e.CalendarState == "legacy" || e.CalendarState == "local") && (e.RecurrenceFreq != nil || e.RecurrenceRootEventID != nil) {
			reason += " Keep this stored occurrence as a single event. Review the future schedule separately."
		}
		out = append(out, csil.CalendarMigrationRow{EventId: csil.EventID(e.EventID), Title: e.Title, Disposition: disposition, Reason: reason})
	}

	return out, nil
}

func (s *CalendarService) Run(ctx context.Context) {
	if s.Remote == nil {
		return
	}
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	repair := time.Now().Add(-time.Hour)
	for {
		if time.Since(repair) >= time.Hour {
			s.DB.WithContext(ctx).Exec("UPDATE house_calendars SET sync_generation=sync_generation+1, sync_pending_since=CASE WHEN sync_generation=synced_generation THEN now() ELSE sync_pending_since END, next_attempt_at=now() WHERE enabled AND NOT deleted")
			repair = time.Now()
		}
		var houses []houseCalendar
		if err := s.DB.WithContext(ctx).Where("(enabled OR deleted) AND next_attempt_at <= now() AND (NOT ready OR sync_generation <> synced_generation OR deleted)").Find(&houses).Error; err == nil {
			for _, h := range houses {
				if ctx.Err() != nil {
					return
				}
				if err = s.reconcile(ctx, h.HouseID); err != nil {
					log.WithError(err).WithField("house", h.HouseID).Warn("calendar synchronization failed")
					s.DB.WithContext(ctx).Model(&houseCalendar{}).Where("house_id = ?", h.HouseID).Updates(map[string]any{"last_error": "Calendar setup or membership update failed. The server will retry.", "next_attempt_at": time.Now().Add(time.Minute)})
				}
				if time.Since(h.SyncPendingSince) > 5*time.Minute {
					log.WithField("house", h.HouseID).Error("calendar membership synchronization delayed more than five minutes")
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (s *CalendarService) reconcile(ctx context.Context, id string) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked bool
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(hashtextextended(?,0))", "calendar-house:"+id).Scan(&locked).Error; err != nil {
			return err
		}
		if !locked {
			return nil
		}
		var h houseCalendar
		if err := tx.First(&h, "house_id = ?", id).Error; err != nil {
			return err
		}
		calendars := cal.NewCalendarClient(s.Remote.ApplicationTransport())
		groups := cal.NewGroupClient(s.Remote.ApplicationTransport())
		if h.Deleted {
			if h.CalendarID != "" {
				_, err := calendars.DeleteCalendar(ctx, cal.CalendarID(h.CalendarID))
				if !remoteNotFound(err) {
					return err
				}
			}
			for _, gid := range []string{h.MembersGroupID, h.AdminsGroupID} {
				if gid != "" {
					_, err := groups.DeleteGroup(ctx, cal.GroupID(gid))
					if !remoteNotFound(err) {
						return err
					}
				}
			}
			return tx.Delete(&h).Error
		}
		if !h.Enabled {
			return nil
		}
		// Names are deterministic so an uncertain create can be recovered.
		name := "longhouse:" + id
		if h.CalendarID == "" {
			list, err := calendars.ListCalendars(ctx, cal.EmptyRequest{})
			if err != nil {
				return err
			}
			for _, c := range list.Calendars {
				if c.Name == name && c.Owner.ApplicationId != nil && *c.Owner.ApplicationId == s.Remote.ApplicationIdentity().ApplicationId && c.Owner.Subject == s.Remote.ApplicationIdentity().SubjectUserId && c.Owner.Domain == s.Remote.ApplicationIdentity().SubjectDomain {
					if h.CalendarID != "" {
						return errors.New("duplicate house calendars require operator review")
					}
					h.CalendarID = string(c.CalendarId)
				}
			}
			if h.CalendarID == "" {
				c, err := calendars.CreateCalendar(ctx, cal.CreateCalendarRequest{Name: name, Timezone: strPtrCopy("UTC")})
				if err != nil {
					return err
				}
				h.CalendarID = string(c.CalendarId)
			}
		}
		for _, part := range []struct {
			name string
			id   *string
		}{{name + ":members", &h.MembersGroupID}, {name + ":admins", &h.AdminsGroupID}} {
			if *part.id == "" {
				list, err := groups.ListGroups(ctx, cal.EmptyRequest{})
				if err != nil {
					return err
				}
				for _, g := range list.Groups {
					if g.Name == part.name {
						*part.id = string(g.GroupId)
					}
				}
				if *part.id == "" {
					g, err := groups.CreateGroup(ctx, cal.CreateGroupRequest{Name: part.name})
					if err != nil {
						return err
					}
					*part.id = string(g.GroupId)
				}
			}
		}
		var members []models.Member
		if err := tx.Where("house_id = ? AND deactivated_at IS NULL", id).Find(&members).Error; err != nil {
			return err
		}
		all, admins := []cal.GroupMember{}, []cal.GroupMember{}
		for _, m := range members {
			uid := m.LinkkeysUserID
			entry := cal.GroupMember{Identity: &cal.IdentityRef{Domain: m.LinkkeysDomain, UserId: &uid}}
			all = append(all, entry)
			var n int64
			if err := tx.Table("member_roles mr").Joins("JOIN roles r ON r.role_id=mr.role_id").Where("mr.member_id=? AND r.house_id=? AND r.name='admin' AND r.deleted_at IS NULL", m.MemberID, id).Count(&n).Error; err != nil {
				return err
			}
			if n > 0 {
				admins = append(admins, entry)
			}
		}
		for _, part := range []struct {
			id      string
			members []cal.GroupMember
			role    cal.ShareRole
		}{{h.MembersGroupID, all, "contribute"}, {h.AdminsGroupID, admins, "write"}} {
			g, err := groups.GetGroup(ctx, cal.GroupID(part.id))
			if err != nil {
				return err
			}
			if _, err = groups.SetMembers(ctx, cal.SetGroupMembersRequest{GroupId: g.GroupId, ExpectedRevision: &g.Revision, Members: part.members}); err != nil {
				return err
			}
			gid := g.GroupId
			if _, err = cal.NewShareClient(s.Remote.ApplicationTransport()).PutShare(ctx, cal.Share{CalendarId: cal.CalendarID(h.CalendarID), GranteeGroupId: &gid, Role: part.role}); err != nil {
				return err
			}
		}
		// Do not overwrite the generation: a membership transaction may have queued
		// a newer one while the remote calls were in progress.
		return tx.Model(&houseCalendar{}).Where("house_id = ?", id).Updates(map[string]any{"calendar_id": h.CalendarID, "members_group_id": h.MembersGroupID, "admins_group_id": h.AdminsGroupID, "ready": true, "synced_generation": h.SyncGeneration, "last_error": ""}).Error
	})
}
func remoteNotFound(err error) bool {
	var e *cal.ClientError
	return err == nil || (errors.As(err, &e) && e.Code == 404)
}
