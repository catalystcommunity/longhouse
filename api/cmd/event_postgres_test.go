//go:build integration

package cmd

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilservices"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func TestListEvents_Postgres_RangeAndPaging(t *testing.T) {
	freshStore(t, requireTestDB(t))
	ctx := context.Background()
	house := mkHouse(t, ctx, "Event range tests")
	owner := mkMember(t, ctx, house, "owner")
	otherHouse := mkHouse(t, ctx, "Other house")
	otherOwner := mkMember(t, ctx, otherHouse, "other")
	caller := auth.WithIdentity(ctx, &auth.Identity{
		Houses: []auth.HouseRoles{{House: house, Member: owner, Roles: []string{"member"}}},
	})
	svc := csilservices.EventService{Store: store.AppStore}
	start := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	end := start.Add(25 * time.Hour)
	before, inside, after := start.Add(-time.Hour), start.Add(time.Hour), end.Add(time.Hour)
	add := func(title, houseID, ownerID string, from, to *time.Time) string {
		t.Helper()
		e := &models.Event{HouseID: houseID, OwnerMemberID: ownerID, Title: title, StartsAt: from, EndsAt: to}
		if err := store.AppStore.CreateEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
		return e.EventID
	}
	// Old rows must not consume the first page of the requested date range.
	for i := 0; i < 60; i++ {
		add(fmt.Sprintf("past-%d", i), house, owner, &before, &before)
	}
	var want []string
	for i := 0; i < 505; i++ {
		want = append(want, add(fmt.Sprintf("in-range-%d", i), house, owner, &inside, &inside))
	}
	startTS, endTS := csil.Timestamp(start.Format(time.RFC3339)), csil.Timestamp(end.Format(time.RFC3339))
	limit, offset := uint64(500), uint64(0)
	req := csil.EventListRequest{HouseId: csil.HouseID(house), RangeStart: &startTS, RangeEnd: &endTS, Limit: &limit, Offset: &offset}
	first, err := svc.ListEvents(caller, req)
	if err != nil || len(first) != 500 {
		t.Fatalf("first page: %d rows, %v", len(first), err)
	}
	offset = 500
	second, err := svc.ListEvents(caller, req)
	if err != nil || len(second) != 5 {
		t.Fatalf("second page: %d rows, %v", len(second), err)
	}
	var got []string
	for _, e := range append(first, second...) {
		got = append(got, string(e.EventId))
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatal("pages omitted, repeated, or misordered events with equal start times")
	}

	// Test range boundaries, spans, missing times, and house/deletion filters.
	for _, tt := range []struct {
		title    string
		from, to *time.Time
		want     bool
	}{
		{"overlaps start", &before, &inside, true},
		{"spans range", &before, &after, true},
		{"ends at start", &before, &start, false},
		{"starts at start", &start, &inside, true},
		{"starts at end", &end, &after, false},
		{"point at start", &start, &start, true},
		{"point at end", &end, &end, false},
		{"start only", &start, nil, true},
		{"end only", nil, &start, true},
		{"end only at end", nil, &end, false},
		{"no timestamps", nil, nil, false},
	} {
		id := add(tt.title, house, owner, tt.from, tt.to)
		if tt.want {
			want = append(want, id)
		}
	}
	add("foreign event", otherHouse, otherOwner, &inside, &inside)
	deleted := add("deleted event", house, owner, &inside, &inside)
	op, err := store.AppStore.NewID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppStore.SoftDeleteEvent(ctx, deleted, owner, op); err != nil {
		t.Fatal(err)
	}
	got = nil
	limit = 50
	for offset = 0; ; offset += limit {
		page, err := svc.ListEvents(caller, req)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page {
			got = append(got, string(e.EventId))
		}
		if len(page) < int(limit) {
			break
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("range returned wrong rows: got %d, want %d", len(got), len(want))
	}
	// The dashboard uses a lower bound only. Undated and past rows are absent.
	req.RangeEnd = nil
	offset = 0
	page, err := svc.ListEvents(caller, req)
	if err != nil || len(page) != 50 {
		t.Fatalf("open-ended range: %d rows, %v", len(page), err)
	}
	for _, e := range page {
		if e.Title == "ends at start" || e.Title == "no timestamps" || len(e.Title) >= 5 && e.Title[:5] == "past-" {
			t.Fatalf("dashboard range included %q", e.Title)
		}
	}
}
