package csilservices

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

func TestDeadlineEventMatchesCatalystcalLimits(t *testing.T) {
	due := time.Date(2026, 11, 8, 15, 0, 0, 123456000, time.UTC)
	event := deadlineEvent(deadlineSource{Task: models.Task{Title: strings.Repeat("🌳", 300), DueAt: &due}, Category: "Garden"}, "calendar")
	if len(event.Title) > 1024 || !utf8.ValidString(event.Title) {
		t.Fatal("invalid bounded title")
	}
	if string(*event.Start) != "2026-11-08T15:00:00Z" || string(*event.End) != "2026-11-08T15:01:00Z" {
		t.Fatalf("unexpected marker times: %s %s", *event.Start, *event.End)
	}
	name := categoryCalendarName("00000000-0000-0000-0000-000000000000", strings.Repeat("🌳", 100))
	if len(name) > 256 || !utf8.ValidString(name) {
		t.Fatalf("invalid calendar name: %d bytes", len(name))
	}
}
