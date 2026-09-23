package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxArticleTitleLength   = 500  // characters
	MaxArticleSummaryLength = 2000 // characters

	// publishedAtSkew tolerates small client/server clock differences when
	// rejecting publication times in the future.
	publishedAtSkew = 5 * time.Minute
)

var (
	ErrArticleNotFound  = errors.New("article not found")
	ErrArticleURLExists = errors.New("an article with this URL already exists")
)

// Article is a piece of public information published by a Source.
type Article struct {
	ID          string
	SourceID    string
	Title       string
	URL         string
	Summary     string
	PublishedAt *time.Time // as stated by the Source; nil when unknown
	RetrievedAt time.Time  // when Nocturne recorded the Article
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewArticle validates input and returns an unsaved Article. Title and summary
// are trimmed; the URL is provenance and is kept exactly as given. Whether the
// Source exists is checked when the Article is stored.
func NewArticle(sourceID, title, rawURL, summary string, publishedAt *time.Time) (Article, error) {
	title = strings.TrimSpace(title)
	summary = strings.TrimSpace(summary)

	if !IsValidID(sourceID) {
		return Article{}, &ValidationError{"source_id", "must be a UUID"}
	}
	if title == "" {
		return Article{}, &ValidationError{"title", "is required"}
	}
	if utf8.RuneCountInString(title) > MaxArticleTitleLength {
		return Article{}, &ValidationError{"title", fmt.Sprintf("must be at most %d characters", MaxArticleTitleLength)}
	}
	if err := validateURL(rawURL); err != nil {
		return Article{}, err
	}
	if utf8.RuneCountInString(summary) > MaxArticleSummaryLength {
		return Article{}, &ValidationError{"summary", fmt.Sprintf("must be at most %d characters", MaxArticleSummaryLength)}
	}
	if publishedAt != nil && publishedAt.After(time.Now().Add(publishedAtSkew)) {
		return Article{}, &ValidationError{"published_at", "must not be in the future"}
	}

	return Article{SourceID: sourceID, Title: title, URL: rawURL, Summary: summary, PublishedAt: publishedAt}, nil
}

// FeedTime is the Article's position in feeds: its publication time when
// known, otherwise when it was recorded. Mirrors the articles.feed_at column.
func (a Article) FeedTime() time.Time {
	if a.PublishedAt != nil {
		return *a.PublishedAt
	}
	return a.RetrievedAt
}
