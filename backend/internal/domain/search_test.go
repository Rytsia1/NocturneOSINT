package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNewArticleSearch_Valid(t *testing.T) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	src := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

	s, err := NewArticleSearch("  earthquake Indonesia  ", src, &from, &to)
	if err != nil {
		t.Fatalf("NewArticleSearch: %v", err)
	}
	if s.Query != "earthquake Indonesia" || s.SourceID != src || !s.PublishedFrom.Equal(from) || !s.PublishedTo.Equal(to) {
		t.Errorf("search = %+v", s)
	}
	for name, q := range map[string]string{
		"at the maximum length": strings.Repeat("é", MaxSearchQueryLength),
		"search operators":      `"Taiwan Strait" OR earthquake -drill`,
		"non-Latin script":      "台湾 地震",
	} {
		if _, err := NewArticleSearch(q, "", nil, nil); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := NewArticleSearch("q", "", &from, &from); err != nil {
		t.Errorf("single-instant range rejected: %v", err)
	}
}

func TestNewArticleSearch_Invalid(t *testing.T) {
	from := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name, query, sourceID string
		from, to              *time.Time
		field                 string
	}{
		{"empty query", "", "", nil, nil, "q"},
		{"whitespace-only query", " \t\n ", "", nil, nil, "q"},
		{"query too long (not truncated)", strings.Repeat("a", MaxSearchQueryLength+1), "", nil, nil, "q"},
		{"NUL character", "quake\x00", "", nil, nil, "q"},
		{"invalid UTF-8", "quake\xff", "", nil, nil, "q"},
		{"invalid source id", "q", "abc", nil, nil, "source_id"},
		{"reversed date range", "q", "", &from, &to, "published_to"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewArticleSearch(tt.query, tt.sourceID, tt.from, tt.to)
			assertField(t, err, tt.field)
		})
	}
}
