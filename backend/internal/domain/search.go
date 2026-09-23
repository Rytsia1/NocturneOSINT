package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxSearchQueryLength bounds a search query, in characters.
const MaxSearchQueryLength = 200

// ArticleSearch is a validated full-text search over Article titles and
// summaries. Its score is textual relevance only.
type ArticleSearch struct {
	Query         string
	SourceID      string     // "" = any Source
	PublishedFrom *time.Time // inclusive; Articles without published_at never match a range
	PublishedTo   *time.Time // inclusive
}

// NewArticleSearch validates search input. The query is trimmed, never
// truncated.
func NewArticleSearch(query, sourceID string, from, to *time.Time) (ArticleSearch, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return ArticleSearch{}, &ValidationError{"q", "is required"}
	}
	if !utf8.ValidString(query) || strings.ContainsRune(query, 0) {
		return ArticleSearch{}, &ValidationError{"q", "must be valid UTF-8 text without NUL characters"}
	}
	if utf8.RuneCountInString(query) > MaxSearchQueryLength {
		return ArticleSearch{}, &ValidationError{"q", fmt.Sprintf("must be at most %d characters", MaxSearchQueryLength)}
	}
	if sourceID != "" && !IsValidID(sourceID) {
		return ArticleSearch{}, &ValidationError{"source_id", "must be a UUID"}
	}
	if from != nil && to != nil && from.After(*to) {
		return ArticleSearch{}, &ValidationError{"published_to", "must not be before published_from"}
	}
	return ArticleSearch{Query: query, SourceID: sourceID, PublishedFrom: from, PublishedTo: to}, nil
}
