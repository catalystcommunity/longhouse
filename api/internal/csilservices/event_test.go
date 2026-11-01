package csilservices

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

type eventTestStore struct {
	store.Store
	member      *models.Member
	memberErr   error
	created     *models.Event
	filter      store.EventFilter
	limit       int
	offset      int
	listed      bool
	memberReads int
}

func (s *eventTestStore) GetMemberByID(context.Context, string) (*models.Member, error) {
	s.memberReads++
	return s.member, s.memberErr
}

func (s *eventTestStore) CreateEvent(_ context.Context, event *models.Event) error {
	s.created = event
	return nil
}

func (s *eventTestStore) ListEvents(_ context.Context, filter store.EventFilter, limit, offset int) ([]models.Event, error) {
	s.filter, s.limit, s.offset, s.listed = filter, limit, offset, true
	return []models.Event{}, nil
}

func eventMemberContext(roles ...string) context.Context {
	return auth.WithIdentity(context.Background(), &auth.Identity{
		Houses: []auth.HouseRoles{{House: "house-1", Member: "member-1", Roles: roles}},
	})
}

func TestCreateEventOwnerAuthorization(t *testing.T) {
	inactive := time.Now()
	for _, tt := range []struct {
		name      string
		admin     bool
		owner     csil.MemberID
		member    *models.Member
		memberErr error
		wantOwner string
		wantCode  int
	}{
		{name: "default owner", wantOwner: "member-1"},
		{name: "explicit self", owner: "member-1", wantOwner: "member-1"},
		{name: "member cannot assign another owner", owner: "member-2", wantCode: 403},
		{name: "member cannot assign foreign owner", owner: "foreign", wantCode: 403},
		{name: "admin default owner", admin: true, wantOwner: "member-1"},
		{name: "admin can assign active house member", admin: true, owner: "member-2",
			member: &models.Member{MemberID: "member-2", HouseID: "house-1"}, wantOwner: "member-2"},
		{name: "admin cannot assign foreign member", admin: true, owner: "foreign",
			member: &models.Member{MemberID: "foreign", HouseID: "house-2"}, wantCode: 400},
		{name: "admin cannot assign inactive member", admin: true, owner: "inactive",
			member: &models.Member{MemberID: "inactive", HouseID: "house-1", DeactivatedAt: &inactive}, wantCode: 400},
		{name: "admin cannot assign missing member", admin: true, owner: "missing", memberErr: gorm.ErrRecordNotFound, wantCode: 400},
		{name: "lookup failure", admin: true, owner: "member-2", memberErr: errors.New("database unavailable"), wantCode: 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := &eventTestStore{member: tt.member, memberErr: tt.memberErr}
			svc := EventService{Store: st}
			role := "member"
			if tt.admin {
				role = "admin"
			}
			got, err := svc.CreateEvent(eventMemberContext(role), csil.Event{HouseId: "house-1", Title: "Meeting", OwnerMemberId: tt.owner})
			if tt.wantCode != 0 {
				var se *csilrpc.Error
				if !errors.As(err, &se) || se.Code != tt.wantCode {
					t.Fatalf("got %v, want status %d", err, tt.wantCode)
				}
				if st.created != nil {
					t.Fatal("rejected request wrote an event")
				}
			} else if err != nil || st.created == nil || st.created.OwnerMemberID != tt.wantOwner || string(got.OwnerMemberId) != tt.wantOwner {
				t.Fatalf("created %+v, response %+v, error %v", st.created, got, err)
			}
			if !tt.admin && st.memberReads != 0 {
				t.Fatal("member request looked up another member before checking permission")
			}
		})
	}
}

func TestEventsRejectOtherHouseBeforeStoreAccess(t *testing.T) {
	svc := EventService{}
	_, err := svc.CreateEvent(eventMemberContext("admin"), csil.Event{HouseId: "house-2", Title: "Meeting"})
	var se *csilrpc.Error
	if !errors.As(err, &se) || se.Code != 403 {
		t.Fatalf("create in another house: %v", err)
	}
	_, err = svc.ListEvents(eventMemberContext("admin"), csil.EventListRequest{HouseId: "house-2"})
	if !errors.As(err, &se) || se.Code != 403 {
		t.Fatalf("list another house: %v", err)
	}
}

func TestListEventsRejectsInvalidRange(t *testing.T) {
	ts := func(s string) *csil.Timestamp { v := csil.Timestamp(s); return &v }
	for _, tt := range []struct {
		name       string
		start, end *csil.Timestamp
	}{
		{"invalid start", ts("not a date"), nil},
		{"empty start", ts(""), nil},
		{"invalid end", nil, ts("2026-11-01")},
		{"empty end", nil, ts("")},
		{"equal", ts("2026-11-01T00:00:00Z"), ts("2026-11-01T00:00:00Z")},
		{"reversed", ts("2026-11-02T00:00:00Z"), ts("2026-11-01T00:00:00Z")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := &eventTestStore{}
			svc := EventService{Store: st}
			_, err := svc.ListEvents(eventMemberContext("member"), csil.EventListRequest{HouseId: "house-1", RangeStart: tt.start, RangeEnd: tt.end})
			var se *csilrpc.Error
			if !errors.As(err, &se) || se.Code != 400 || st.listed {
				t.Fatalf("invalid range: error %v, queried store %t", err, st.listed)
			}
		})
	}
}

func TestListEventsRangeAndLegacyPaging(t *testing.T) {
	// Old clients send the same house_id, limit, and offset map fields.
	limit, offset := uint64(1000), uint64(500)
	raw := csil.EncodeHouseScopedListRequest(csil.HouseScopedListRequest{HouseId: "house-1", Limit: &limit, Offset: &offset})
	req, err := csil.DecodeEventListRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	st := &eventTestStore{}
	svc := EventService{Store: st}
	if _, err := svc.ListEvents(eventMemberContext("member"), req); err != nil {
		t.Fatal(err)
	}
	if st.limit != 500 || st.offset != 500 || st.filter.RangeStart != nil || st.filter.RangeEnd != nil {
		t.Fatalf("legacy page: %+v", st)
	}
	start, end := csil.Timestamp("2026-11-01T00:00:00-06:00"), csil.Timestamp("2026-11-02T00:00:00-07:00")
	for _, bounds := range [][2]*csil.Timestamp{{&start, &end}, {&start, nil}, {nil, &end}} {
		req.RangeStart, req.RangeEnd = bounds[0], bounds[1]
		if _, err := svc.ListEvents(eventMemberContext("member"), req); err != nil {
			t.Fatal(err)
		}
		if st.filter.HouseID != "house-1" || (st.filter.RangeStart == nil) != (bounds[0] == nil) || (st.filter.RangeEnd == nil) != (bounds[1] == nil) {
			t.Fatalf("range not passed to store: %+v", st.filter)
		}
		if st.filter.RangeStart != nil && st.filter.RangeEnd != nil && st.filter.RangeEnd.Sub(*st.filter.RangeStart) != 25*time.Hour {
			t.Fatal("range lost the daylight saving offset")
		}
	}
}
