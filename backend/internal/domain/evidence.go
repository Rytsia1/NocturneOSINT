package domain

import (
	"errors"
	"time"
)

var (
	ErrEvidenceNotFound = errors.New("evidence not found")
	ErrEvidenceExists   = errors.New("evidence already links this article and event")
)

// Evidence records that an Article provides information about an Event. It is
// provenance only: it does not establish that the Event is true or that the
// Article is reliable.
type Evidence struct {
	ID        string
	ArticleID string
	EventID   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewEvidence validates input and returns unsaved Evidence. Whether the Article
// and Event exist is checked when it is stored.
func NewEvidence(articleID, eventID string) (Evidence, error) {
	if !IsValidID(articleID) {
		return Evidence{}, &ValidationError{"article_id", "must be a UUID"}
	}
	if !IsValidID(eventID) {
		return Evidence{}, &ValidationError{"event_id", "must be a UUID"}
	}
	return Evidence{ArticleID: articleID, EventID: eventID}, nil
}
