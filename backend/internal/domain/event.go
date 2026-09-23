package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxEventTitleLength       = 500  // characters
	MaxEventDescriptionLength = 2000 // characters
)

var (
	ErrEventNotFound         = errors.New("event not found")
	ErrEventLocationExists   = errors.New("location is already attached to this event")
	ErrEventLocationNotFound = errors.New("location is not attached to this event")
	ErrLocationInUse         = errors.New("location is referenced by an event")
)

// EventTimePrecisions says how much of occurred_at is actually known.
// "day", "month" and "year" are UTC dates stored at the start of that period.
var EventTimePrecisions = []string{"exact", "day", "month", "year"}

// EventLocationRoles distinguish where an Event happened ("site") from other
// places connected to it ("related").
var EventLocationRoles = []string{"site", "related"}

// Event is an occurrence being described or investigated. It is not an
// Article: Articles are published information that may later cite Events.
type Event struct {
	ID                  string
	Title               string
	Description         string
	OccurredAt          *time.Time // nil when unknown
	OccurredAtPrecision string     // "" exactly when OccurredAt is nil
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// EventLocation is one of an Event's geographic references.
type EventLocation struct {
	Role     string
	Location Location
}

// NewEvent validates input and returns an unsaved Event. An unknown time stays
// unknown; a partially known time must be stored at the start of its period
// (e.g. precision "day" → 00:00:00 UTC) so no finer precision is implied.
func NewEvent(title, description string, occurredAt *time.Time, precision string) (Event, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)

	if title == "" {
		return Event{}, &ValidationError{"title", "is required"}
	}
	if utf8.RuneCountInString(title) > MaxEventTitleLength {
		return Event{}, &ValidationError{"title", fmt.Sprintf("must be at most %d characters", MaxEventTitleLength)}
	}
	if utf8.RuneCountInString(description) > MaxEventDescriptionLength {
		return Event{}, &ValidationError{"description", fmt.Sprintf("must be at most %d characters", MaxEventDescriptionLength)}
	}
	if err := checkOccurredAt(occurredAt, precision); err != nil {
		return Event{}, err
	}

	return Event{Title: title, Description: description, OccurredAt: occurredAt, OccurredAtPrecision: precision}, nil
}

func checkOccurredAt(at *time.Time, precision string) error {
	if at == nil {
		if precision != "" {
			return &ValidationError{"occurred_at_precision", "requires occurred_at"}
		}
		return nil
	}
	if !contains(EventTimePrecisions, precision) {
		return &ValidationError{"occurred_at_precision", "is required with occurred_at and must be one of " + strings.Join(EventTimePrecisions, ", ")}
	}

	u := at.UTC()
	start := map[string]time.Time{
		"day":   time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC),
		"month": time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC),
		"year":  time.Date(u.Year(), time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	if s, ok := start[precision]; ok && !u.Equal(s) {
		return &ValidationError{"occurred_at", fmt.Sprintf("must be %s for precision %q", s.Format(time.RFC3339), precision)}
	}
	return nil
}

// FeedTime is the Event's position in feeds: its occurrence time when known,
// otherwise when it was recorded. Mirrors the events.feed_at column.
func (e Event) FeedTime() time.Time {
	if e.OccurredAt != nil {
		return *e.OccurredAt
	}
	return e.CreatedAt
}

// CheckEventLocationRole validates an Event → Location role.
func CheckEventLocationRole(role string) error {
	if !contains(EventLocationRoles, role) {
		return &ValidationError{"role", "must be one of " + strings.Join(EventLocationRoles, ", ")}
	}
	return nil
}

func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
