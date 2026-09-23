package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const testSourceID = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

func TestNewArticle_Valid(t *testing.T) {
	published := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	raw := "https://www.reuters.com/Tech/TSMC?utm=x#top"

	a, err := NewArticle(testSourceID, "  TSMC expands capacity ", raw, " Summary. ", &published)
	if err != nil {
		t.Fatalf("NewArticle: %v", err)
	}
	if a.SourceID != testSourceID || a.Title != "TSMC expands capacity" || a.Summary != "Summary." {
		t.Errorf("article = %+v", a)
	}
	if a.URL != raw {
		t.Errorf("URL = %q, want verbatim %q", a.URL, raw)
	}
	if a.PublishedAt == nil || !a.PublishedAt.Equal(published) {
		t.Errorf("PublishedAt = %v, want %v", a.PublishedAt, published)
	}
}

func TestNewArticle_UnknownPublicationTimeStaysUnknown(t *testing.T) {
	a, err := NewArticle(testSourceID, "Title", "https://example.com/a", "", nil)
	if err != nil {
		t.Fatalf("NewArticle: %v", err)
	}
	if a.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want nil (never fabricated)", a.PublishedAt)
	}
}

func TestNewArticle_Invalid(t *testing.T) {
	future := time.Now().Add(time.Hour)
	tests := []struct {
		name, sourceID, title, url, summary string
		publishedAt                         *time.Time
		field                               string
	}{
		{"missing source id", "", "T", "https://e.com/a", "", nil, "source_id"},
		{"malformed source id", "not-a-uuid", "T", "https://e.com/a", "", nil, "source_id"},
		{"missing title", testSourceID, "", "https://e.com/a", "", nil, "title"},
		{"whitespace title", testSourceID, "   ", "https://e.com/a", "", nil, "title"},
		{"title too long", testSourceID, strings.Repeat("a", MaxArticleTitleLength+1), "https://e.com/a", "", nil, "title"},
		{"missing url", testSourceID, "T", "", "", nil, "url"},
		{"relative url", testSourceID, "T", "/news/1", "", nil, "url"},
		{"non-http url", testSourceID, "T", "ftp://e.com/a", "", nil, "url"},
		{"summary too long", testSourceID, "T", "https://e.com/a", strings.Repeat("a", MaxArticleSummaryLength+1), nil, "summary"},
		{"published in the future", testSourceID, "T", "https://e.com/a", "", &future, "published_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewArticle(tt.sourceID, tt.title, tt.url, tt.summary, tt.publishedAt)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if verr.Field != tt.field {
				t.Errorf("Field = %q, want %q", verr.Field, tt.field)
			}
		})
	}
}

func TestNewArticle_AllowsSmallClockSkew(t *testing.T) {
	skewed := time.Now().Add(time.Minute)
	if _, err := NewArticle(testSourceID, "T", "https://e.com/a", "", &skewed); err != nil {
		t.Errorf("published_at one minute ahead rejected: %v", err)
	}
}

func TestArticleFeedTime(t *testing.T) {
	retrieved := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	published := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if got := (Article{RetrievedAt: retrieved, PublishedAt: &published}).FeedTime(); !got.Equal(published) {
		t.Errorf("FeedTime with publication time = %v, want %v", got, published)
	}
	if got := (Article{RetrievedAt: retrieved}).FeedTime(); !got.Equal(retrieved) {
		t.Errorf("FeedTime without publication time = %v, want retrieval time %v", got, retrieved)
	}
}
