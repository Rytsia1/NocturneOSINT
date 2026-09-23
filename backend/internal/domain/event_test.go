package domain

import (
	"strings"
	"testing"
	"time"
)

func tp(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestNewEvent_Valid(t *testing.T) {
	tests := []struct {
		name      string
		at        *time.Time
		precision string
	}{
		{"unknown time", nil, ""},
		{"exact time with offset", tp("2026-09-18T12:34:56+09:00"), "exact"},
		{"known day", tp("2026-09-18T00:00:00Z"), "day"},
		{"known day given as equivalent offset", tp("2026-09-18T09:00:00+09:00"), "day"},
		{"known month", tp("2026-09-01T00:00:00Z"), "month"},
		{"known year", tp("2026-01-01T00:00:00Z"), "year"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := NewEvent("  Port fire  ", " Reported. ", tt.at, tt.precision)
			if err != nil {
				t.Fatalf("NewEvent: %v", err)
			}
			if e.Title != "Port fire" || e.Description != "Reported." {
				t.Errorf("event = %+v", e)
			}
			if (e.OccurredAt == nil) != (tt.at == nil) || e.OccurredAtPrecision != tt.precision {
				t.Errorf("time = %v/%q, want %v/%q (never invented)", e.OccurredAt, e.OccurredAtPrecision, tt.at, tt.precision)
			}
		})
	}
}

func TestNewEvent_Invalid(t *testing.T) {
	tests := []struct {
		name, title, desc string
		at                *time.Time
		precision, field  string
	}{
		{"missing title", "", "", nil, "", "title"},
		{"whitespace title", "   ", "", nil, "", "title"},
		{"title too long", strings.Repeat("a", MaxEventTitleLength+1), "", nil, "", "title"},
		{"description too long", "T", strings.Repeat("a", MaxEventDescriptionLength+1), nil, "", "description"},
		{"precision without time", "T", "", nil, "day", "occurred_at_precision"},
		{"time without precision", "T", "", tp("2026-09-18T10:00:00Z"), "", "occurred_at_precision"},
		{"unknown precision", "T", "", tp("2026-09-18T10:00:00Z"), "hour", "occurred_at_precision"},
		{"day precision with a time of day", "T", "", tp("2026-09-18T10:00:00Z"), "day", "occurred_at"},
		{"month precision not on the 1st", "T", "", tp("2026-09-18T00:00:00Z"), "month", "occurred_at"},
		{"year precision not on 1 January", "T", "", tp("2026-09-01T00:00:00Z"), "year", "occurred_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEvent(tt.title, tt.desc, tt.at, tt.precision)
			assertField(t, err, tt.field)
		})
	}
}

func TestEventFeedTime(t *testing.T) {
	created := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	occurred := tp("2026-01-01T00:00:00Z")

	if got := (Event{CreatedAt: created, OccurredAt: occurred}).FeedTime(); !got.Equal(*occurred) {
		t.Errorf("FeedTime with occurrence time = %v, want %v", got, *occurred)
	}
	if got := (Event{CreatedAt: created}).FeedTime(); !got.Equal(created) {
		t.Errorf("FeedTime without occurrence time = %v, want creation time %v", got, created)
	}
}

func TestCheckEventLocationRole(t *testing.T) {
	for _, role := range EventLocationRoles {
		if err := CheckEventLocationRole(role); err != nil {
			t.Errorf("role %q rejected: %v", role, err)
		}
	}
	for _, role := range []string{"", "origin", "SITE"} {
		assertField(t, CheckEventLocationRole(role), "role")
	}
}
