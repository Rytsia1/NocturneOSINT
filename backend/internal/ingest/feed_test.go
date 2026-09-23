package ingest

import (
	"errors"
	"strings"
	"testing"
	"time"

	"nocturne-backend/internal/domain"
)

const sourceID = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

const rssFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
<channel>
  <title>Example</title>
  <link>https://example.test/</link>
  <atom:link href="https://example.test/feed.xml" rel="self"/>
  <item>
    <title>  Port fire in Tanjung Priok  </title>
    <link>
      https://example.test/news/1?utm_source=rss
    </link>
    <atom:link href="https://example.test/other" rel="related"/>
    <description><![CDATA[<p>Fire &amp; smoke <b>reported</b>.</p>]]></description>
    <pubDate>Sun, 20 Sep 2026 08:00:00 +0700</pubDate>
    <guid>urn:item:1</guid>
  </item>
  <item>
    <title>Undated report</title>
    <link>https://example.test/news/2</link>
    <guid>urn:item:2</guid>
  </item>
  <item>
    <title>Same GUID, same link</title>
    <link>https://example.test/news/2</link>
    <guid>urn:item:2</guid>
  </item>
</channel>
</rss>`

const atomFixture = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example Atom</title>
  <entry>
    <title>Published entry</title>
    <link rel="self" href="https://example.test/atom/1.xml"/>
    <link rel="alternate" href="https://example.test/atom/1"/>
    <id>tag:example.test,2026:1</id>
    <published>2026-09-19T10:00:00+02:00</published>
    <updated>2026-09-21T10:00:00Z</updated>
    <summary>Short summary</summary>
  </entry>
  <entry>
    <title>Updated only</title>
    <link href="https://example.test/atom/2"/>
    <id>tag:example.test,2026:2</id>
    <updated>2026-09-21T10:00:00Z</updated>
  </entry>
</feed>`

func normalizeAll(t *testing.T, body string) []domain.Article {
	t.Helper()
	items, err := parseFeed([]byte(body))
	if err != nil {
		t.Fatalf("parseFeed: %v", err)
	}
	var out []domain.Article
	for _, it := range items {
		a, err := normalize(it, sourceID)
		if err != nil {
			t.Fatalf("normalize %+v: %v", it, err)
		}
		out = append(out, a)
	}
	return out
}

func TestParseRSS(t *testing.T) {
	got := normalizeAll(t, rssFixture)
	if len(got) != 3 {
		t.Fatalf("items = %d, want 3 (duplicates are the service's job)", len(got))
	}

	a := got[0]
	want := time.Date(2026, 9, 20, 1, 0, 0, 0, time.UTC)
	if a.Title != "Port fire in Tanjung Priok" || a.SourceID != sourceID {
		t.Errorf("title/source = %q/%q", a.Title, a.SourceID)
	}
	if a.URL != "https://example.test/news/1?utm_source=rss" {
		t.Errorf("url = %q, want it kept as given (tracking parameters included)", a.URL)
	}
	if a.Summary != "Fire & smoke reported ." {
		t.Errorf("summary = %q, want plain text", a.Summary)
	}
	if a.PublishedAt == nil || !a.PublishedAt.Equal(want) || a.PublishedAt.Location() != time.UTC {
		t.Errorf("published_at = %v, want %v in UTC", a.PublishedAt, want)
	}

	// Missing pubDate stays unknown; duplicate GUIDs are both parsed.
	if got[1].PublishedAt != nil {
		t.Errorf("undated item published_at = %v, want nil (never invented)", got[1].PublishedAt)
	}
	if got[1].URL != got[2].URL {
		t.Errorf("duplicate-GUID items have URLs %q and %q", got[1].URL, got[2].URL)
	}
}

func TestParseAtom(t *testing.T) {
	got := normalizeAll(t, atomFixture)
	if len(got) != 2 {
		t.Fatalf("entries = %d, want 2", len(got))
	}
	if got[0].URL != "https://example.test/atom/1" {
		t.Errorf("url = %q, want the alternate link, not rel=self", got[0].URL)
	}
	if want := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC); got[0].PublishedAt == nil || !got[0].PublishedAt.Equal(want) {
		t.Errorf("published_at = %v, want <published> %v, not <updated>", got[0].PublishedAt, want)
	}
	if got[0].Summary != "Short summary" {
		t.Errorf("summary = %q", got[0].Summary)
	}
	if got[1].PublishedAt != nil {
		t.Errorf("updated-only entry published_at = %v, want nil: <updated> is not a publication time", got[1].PublishedAt)
	}
}

func TestParseRejectsNonFeeds(t *testing.T) {
	tests := map[string]string{
		"malformed XML":        `<rss version="2.0"><channel><item><title>x</item></channel></rss>`,
		"truncated RSS":        `<rss version="2.0"><channel><item><title>x</title>`,
		"HTML page":            `<!DOCTYPE html><html><body><a href="https://example.test/">link</a></body></html>`,
		"XHTML document":       `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`,
		"RSS 1.0 (RDF)":        `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><item/></rdf:RDF>`,
		"feed without Atom ns": `<feed><entry><title>x</title></entry></feed>`,
		"lone Atom entry":      `<entry xmlns="http://www.w3.org/2005/Atom"><title>x</title></entry>`,
		"other XML root":       `<urlset><url/></urlset>`,
		"unsupported charset":  `<?xml version="1.0" encoding="ISO-8859-1"?><rss version="2.0"><channel/></rss>`,
		"JSON":                 `{"items":[]}`,
		"empty body":           ``,
		"plain text":           `not xml at all`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseFeed([]byte(body)); !errors.Is(err, ErrInvalidFeed) {
				t.Errorf("err = %v, want ErrInvalidFeed", err)
			}
		})
	}
}

func TestParseEmptyFeed(t *testing.T) {
	items, err := parseFeed([]byte(`<rss version="2.0"><channel><title>Empty</title></channel></rss>`))
	if err != nil || len(items) != 0 {
		t.Errorf("empty feed = %v, %v; want no items and no error", items, err)
	}
}

func TestNormalizeInvalidItems(t *testing.T) {
	tests := []struct {
		name  string
		it    item
		field string
	}{
		{"missing title", item{Link: "https://example.test/a"}, "title"},
		{"blank title", item{Title: "   ", Link: "https://example.test/a"}, "title"},
		{"missing URL", item{Title: "T"}, "url"},
		{"relative URL", item{Title: "T", Link: "/news/1"}, "url"},
		{"non-http URL", item{Title: "T", Link: "javascript:alert(1)"}, "url"},
		{"future publication", item{Title: "T", Link: "https://example.test/a", Published: "2999-01-01T00:00:00Z"}, "published_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalize(tt.it, sourceID)
			var verr *domain.ValidationError
			if !errors.As(err, &verr) || verr.Field != tt.field {
				t.Errorf("err = %v, want validation error on %s", err, tt.field)
			}
		})
	}
}

func TestNormalizeRules(t *testing.T) {
	if got := parseDate("yesterday"); got != nil {
		t.Errorf("unreadable date = %v, want nil", got)
	}
	long := strings.Repeat("é", domain.MaxArticleSummaryLength+50)
	a, err := normalize(item{Title: "T", Link: "https://example.test/a", Summary: long}, sourceID)
	if err != nil {
		t.Fatalf("normalize long summary: %v", err)
	}
	if n := len([]rune(a.Summary)); n != domain.MaxArticleSummaryLength {
		t.Errorf("summary length = %d runes, want truncated to %d", n, domain.MaxArticleSummaryLength)
	}
}
