package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Requests rejected before any database access run without a database.
func TestSearchRequestValidation(t *testing.T) {
	router := NewRouter(discardLogger, nil)
	id := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	cur := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

	tests := []struct {
		name, query string
		code        string
	}{
		{"missing q", "", "invalid_query"},
		{"empty q", "q=", "invalid_query"},
		{"whitespace-only q", "q=%20%20%09", "invalid_query"},
		{"q too long", "q=" + strings.Repeat("a", 201), "invalid_query"},
		{"NUL in q", "q=quake%00", "invalid_query"},
		{"malformed source_id", "q=quake&source_id=abc", "invalid_source_id"},
		{"malformed published_from", "q=quake&published_from=2026-09-01", "invalid_date"},
		{"malformed published_to", "q=quake&published_to=yesterday", "invalid_date"},
		{"reversed date range", "q=quake&published_from=2026-09-30T00:00:00Z&published_to=2026-09-01T00:00:00Z", "invalid_date_range"},
		{"limit zero", "q=quake&limit=0", "invalid_limit"},
		{"limit too large", "q=quake&limit=101", "invalid_limit"},
		{"limit not a number", "q=quake&limit=ten", "invalid_limit"},
		{"cursor not base64", "q=quake&cursor=***", "invalid_cursor"},
		{"cursor from a list endpoint", "q=quake&cursor=" + cur("2026-09-01T00:00:00Z|"+id), "invalid_cursor"},
		{"cursor with NaN score", "q=quake&cursor=" + cur("NaN|2026-09-01T00:00:00Z|"+id), "invalid_cursor"},
		{"cursor with bad id", "q=quake&cursor=" + cur("0.5|2026-09-01T00:00:00Z|x"), "invalid_cursor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, do(router, "GET", "/api/search/articles?"+tt.query, ""), http.StatusBadRequest, tt.code)
		})
	}
}

// Integration tests: full HTTP → repository → PostgreSQL round trips.

type searchPage struct {
	Items []struct {
		ID       string  `json:"id"`
		SourceID string  `json:"source_id"`
		Title    string  `json:"title"`
		Score    float64 `json:"score"`
	} `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

func postArticle(t *testing.T, router http.Handler, sourceID, title, summary, publishedAt string) string {
	t.Helper()
	body := fmt.Sprintf(`{"source_id":%q,"title":%q,"summary":%q,"url":%q`, sourceID, title, summary,
		"https://example.test/http-search/"+uniq())
	if publishedAt != "" {
		body += fmt.Sprintf(`,"published_at":%q`, publishedAt)
	}
	rec := do(router, "POST", "/api/articles", body+"}")
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/articles = %d (%s)", rec.Code, rec.Body.String())
	}
	id := decode[articleBody](t, rec).ID
	t.Cleanup(func() { do(router, "DELETE", "/api/articles/"+id, "") })
	return id
}

func searchOK(t *testing.T, router http.Handler, query string) searchPage {
	t.Helper()
	rec := do(router, "GET", "/api/search/articles?"+query, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET search?%s = %d (%s)", query, rec.Code, rec.Body.String())
	}
	return decode[searchPage](t, rec)
}

func TestIntegration_SearchArticlesHTTP(t *testing.T) {
	router := integrationRouter(t)
	srcA := createSource(t, router, "Search A").ID
	srcB := createSource(t, router, "Search B").ID
	tok := "httpsrch" + strings.ReplaceAll(uniq(), "-", "x")

	inTitle := postArticle(t, router, srcA, "Earthquake "+tok, "shaking reported", "2026-09-10T00:00:00Z")
	inSummary := postArticle(t, router, srcA, "Regional update", "an "+tok+" was felt", "2026-09-20T00:00:00Z")
	otherSource := postArticle(t, router, srcB, tok+" aftershock", "", "")

	// Relevance order and compact items.
	page := searchOK(t, router, "q="+tok)
	if len(page.Items) != 3 || page.Items[2].ID != inSummary || page.Items[0].Score <= page.Items[2].Score || page.NextCursor != nil {
		t.Fatalf("search = %+v, want 3 items, summary-only match last", page)
	}
	rec := do(router, "GET", "/api/search/articles?q="+tok, "")
	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, k := range []string{"id", "source_id", "title", "url", "summary", "published_at", "retrieved_at", "score"} {
		if _, ok := raw.Items[0][k]; !ok {
			t.Errorf("result item lacks %q", k)
		}
	}
	for _, k := range []string{"search_vector", "feed_at", "source", "media", "events"} {
		if _, ok := raw.Items[0][k]; ok {
			t.Errorf("result item exposes %q", k)
		}
	}

	// Limit + cursor: same order, no repeats.
	first := searchOK(t, router, "limit=2&q="+tok)
	if len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("page 1 = %+v", first)
	}
	second := searchOK(t, router, "limit=2&q="+tok+"&cursor="+*first.NextCursor)
	if len(second.Items) != 1 || second.Items[0].ID != page.Items[2].ID || second.NextCursor != nil {
		t.Errorf("page 2 = %+v, want the third result only", second)
	}

	// Filters.
	if got := searchOK(t, router, "q="+tok+"&source_id="+srcB); len(got.Items) != 1 || got.Items[0].ID != otherSource {
		t.Errorf("source filter = %+v", got)
	}
	if got := searchOK(t, router, "q="+tok+"&published_from=2026-09-15T00:00:00Z"); len(got.Items) != 1 || got.Items[0].ID != inSummary {
		t.Errorf("published_from filter = %+v", got)
	}
	if got := searchOK(t, router, "q="+tok+"&published_to=2026-09-15T00:00:00%2B07:00"); len(got.Items) != 1 || got.Items[0].ID != inTitle {
		t.Errorf("published_to filter = %+v", got)
	}
	if got := searchOK(t, router, "q="+tok+"&source_id=00000000-0000-4000-8000-000000000000"); len(got.Items) != 0 {
		t.Errorf("unknown source filter = %+v, want empty (as GET /api/articles)", got)
	}

	// Operators work; malformed syntax and SQL fragments are plain words.
	if got := searchOK(t, router, "q="+url.QueryEscape(tok+" -aftershock")); len(got.Items) != 2 {
		t.Errorf("exclusion search = %d items, want 2", len(got.Items))
	}
	for _, q := range []string{`"unterminated`, `a & | ! ((`, `'; DROP TABLE articles; --`} {
		searchOK(t, router, "q="+url.QueryEscape(q))
	}
	if got := searchOK(t, router, "q="+tok); len(got.Items) != 3 {
		t.Errorf("articles changed after hostile queries: %d results", len(got.Items))
	}
}
