// Package domain holds Nocturne's core concepts, independent of HTTP and SQL.
package domain

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxSourceNameLength        = 200  // characters
	MaxURLLength               = 2048 // bytes
	MaxSourceDescriptionLength = 2000 // characters
)

var (
	ErrSourceNotFound    = errors.New("source not found")
	ErrSourceURLExists   = errors.New("a source with this URL already exists")
	ErrSourceHasArticles = errors.New("source still has articles")
)

// ValidationError describes input that violates a domain rule.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + " " + e.Message }

// Source is a publisher or public information origin: where information came from.
type Source struct {
	ID          string
	Name        string
	URL         string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewSource validates input and returns an unsaved Source. Name and
// description are trimmed; the URL is provenance and is kept exactly as given.
func NewSource(name, rawURL, description string) (Source, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)

	if name == "" {
		return Source{}, &ValidationError{"name", "is required"}
	}
	if utf8.RuneCountInString(name) > MaxSourceNameLength {
		return Source{}, &ValidationError{"name", fmt.Sprintf("must be at most %d characters", MaxSourceNameLength)}
	}
	if err := validateURL(rawURL); err != nil {
		return Source{}, err
	}
	if utf8.RuneCountInString(description) > MaxSourceDescriptionLength {
		return Source{}, &ValidationError{"description", fmt.Sprintf("must be at most %d characters", MaxSourceDescriptionLength)}
	}

	return Source{Name: name, URL: rawURL, Description: description}, nil
}

// validateURL checks that raw is an absolute http(s) URL without rewriting it.
func validateURL(raw string) error {
	if raw == "" {
		return &ValidationError{"url", "is required"}
	}
	if len(raw) > MaxURLLength {
		return &ValidationError{"url", fmt.Sprintf("must be at most %d bytes", MaxURLLength)}
	}
	u, err := url.Parse(raw)
	if err != nil || strings.ContainsFunc(raw, unicode.IsSpace) ||
		(u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return &ValidationError{"url", "must be an absolute http or https URL"}
	}
	return nil
}

var idPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsValidID reports whether id is a well-formed UUID.
func IsValidID(id string) bool {
	return idPattern.MatchString(id)
}
