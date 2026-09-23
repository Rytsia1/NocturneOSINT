package http

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Requests rejected before any database access run without a database.
func TestArticleRequestValidation(t *testing.T) {
	router := NewRouter(discardLogger, nil)
	src := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"malformed JSON", "POST", "/api/articles", `{"title":`, 400, "invalid_request"},
		{"client-supplied retrieved_at", "POST", "/api/articles",
			`{"source_id":"` + src + `","title":"T","url":"https://e.com/a","retrieved_at":"2026-01-01T00:00:00Z"}`, 400, "invalid_request"},
		{"published_at not RFC 3339", "POST", "/api/articles",
			`{"source_id":"` + src + `","title":"T","url":"https://e.com/a","published_at":"yesterday"}`, 400, "invalid_request"},
		{"published_at without timezone", "POST", "/api/articles",
			`{"source_id":"` + src + `","title":"T","url":"https://e.com/a","published_at":"2026-01-01T10:00:00"}`, 400, "invalid_request"},
		{"missing title", "POST", "/api/articles", `{"source_id":"` + src + `","url":"https://e.com/a"}`, 400, "validation_failed"},
		{"invalid source_id", "POST", "/api/articles", `{"source_id":"abc","title":"T","url":"https://e.com/a"}`, 400, "validation_failed"},
		{"invalid url", "POST", "/api/articles", `{"source_id":"` + src + `","title":"T","url":"e.com/a"}`, 400, "validation_failed"},
		{"published in the future", "POST", "/api/articles",
			`{"source_id":"` + src + `","title":"T","url":"https://e.com/a","published_at":"` + future + `"}`, 400, "validation_failed"},
		{"get invalid id", "GET", "/api/articles/nope", "", 400, "invalid_id"},
		{"delete invalid id", "DELETE", "/api/articles/123", "", 400, "invalid_id"},
		{"list invalid source_id", "GET", "/api/articles?source_id=abc", "", 400, "invalid_source_id"},
		{"list invalid limit", "GET", "/api/articles?limit=500", "", 400, "invalid_limit"},
		{"list invalid cursor", "GET", "/api/articles?cursor=***", "", 400, "invalid_cursor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, do(router, tt.method, tt.path, tt.body), tt.status, tt.code)
		})
	}
}

// Integration tests: full HTTP → repository → PostgreSQL round trips.

type articleBody struct {
	ID          string     `json:"id"`
	SourceID    string     `json:"source_id"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	Summary     string     `json:"summary"`
	PublishedAt *time.Time `json:"published_at"`
	RetrievedAt time.Time  `json:"retrieved_at"`
}

type articleListBody struct {
	Items      []articleBody `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

// createArticle posts an Article and deletes it when the test ends; cleanups
// run LIFO, so it is removed before the Source created earlier in the test.
func createArticle(t *testing.T, router http.Handler, sourceID, publishedAt string) articleBody {
	t.Helper()
	url := fmt.Sprintf("https://example.test/http-articles/%s/%d", t.Name(), time.Now().UnixNano())
	body := fmt.Sprintf(`{"source_id":%q,"title":"Test article","url":%q,"summary":"s"`, sourceID, url)
	if publishedAt != "" {
		body += fmt.Sprintf(`,"published_at":%q`, publishedAt)
	}
	rec := do(router, "POST", "/api/articles", body+"}")
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/articles status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	a := decode[articleBody](t, rec)
	t.Cleanup(func() { do(router, "DELETE", "/api/articles/"+a.ID, "") })
	return a
}

func TestIntegration_ArticleLifecycle(t *testing.T) {
	router := integrationRouter(t)
	source := createSource(t, router, "Reuters")
	url := fmt.Sprintf("https://www.reuters.com/tech/tsmc-%d", time.Now().UnixNano())

	// Create
	rec := do(router, "POST", "/api/articles", fmt.Sprintf(
		`{"source_id":%q,"title":"TSMC expands capacity","url":%q,"summary":"Hsinchu fab.","published_at":"2026-09-18T12:00:00+02:00"}`,
		source.ID, url))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	created := decode[articleBody](t, rec)
	t.Cleanup(func() { do(router, "DELETE", "/api/articles/"+created.ID, "") })

	if created.SourceID != source.ID {
		t.Errorf("source_id = %s, want %s", created.SourceID, source.ID)
	}
	if created.URL != url {
		t.Errorf("url = %q, want verbatim %q", created.URL, url)
	}
	wantPublished := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	if created.PublishedAt == nil || !created.PublishedAt.Equal(wantPublished) {
		t.Errorf("published_at = %v, want %v", created.PublishedAt, wantPublished)
	}
	if created.RetrievedAt.IsZero() {
		t.Error("retrieved_at not set")
	}
	if loc := rec.Header().Get("Location"); loc != "/api/articles/"+created.ID {
		t.Errorf("Location = %q", loc)
	}

	// Get
	rec = do(router, "GET", "/api/articles/"+created.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}
	if got := decode[articleBody](t, rec); got.ID != created.ID || got.SourceID != source.ID {
		t.Errorf("GET = %+v", got)
	}

	// List filtered by Source: exactly this Article.
	rec = do(router, "GET", "/api/articles?source_id="+source.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("LIST status = %d, want 200", rec.Code)
	}
	if items := decode[articleListBody](t, rec).Items; len(items) != 1 || items[0].ID != created.ID {
		t.Errorf("filtered list = %+v, want only %s", items, created.ID)
	}

	// Unfiltered list works and an undated Article (recorded now) tops the feed.
	undated := createArticle(t, router, source.ID, "")
	rec = do(router, "GET", "/api/articles?limit=5", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("LIST status = %d, want 200", rec.Code)
	}
	found := false
	for _, a := range decode[articleListBody](t, rec).Items {
		found = found || a.ID == undated.ID
	}
	if !found {
		t.Error("newly recorded undated article missing from first page of GET /api/articles")
	}

	// Delete
	if rec = do(router, "DELETE", "/api/articles/"+created.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", rec.Code)
	}
	assertError(t, do(router, "GET", "/api/articles/"+created.ID, ""), http.StatusNotFound, "article_not_found")
	assertError(t, do(router, "DELETE", "/api/articles/"+created.ID, ""), http.StatusNotFound, "article_not_found")
}

func TestIntegration_CreateArticle_UnknownSource(t *testing.T) {
	router := integrationRouter(t)
	rec := do(router, "POST", "/api/articles",
		`{"source_id":"00000000-0000-4000-8000-000000000000","title":"Orphan","url":"https://example.test/orphan-http"}`)
	assertError(t, rec, http.StatusUnprocessableEntity, "source_not_found")
}

func TestIntegration_CreateArticle_DuplicateURL(t *testing.T) {
	router := integrationRouter(t)
	source := createSource(t, router, "Dup")
	existing := createArticle(t, router, source.ID, "")

	rec := do(router, "POST", "/api/articles", fmt.Sprintf(`{"source_id":%q,"title":"Copy","url":%q}`, source.ID, existing.URL))
	assertError(t, rec, http.StatusConflict, "article_already_exists")
}

func TestIntegration_ListArticlesBySource_Pagination(t *testing.T) {
	router := integrationRouter(t)
	source := createSource(t, router, "Paged")
	oldest := createArticle(t, router, source.ID, "2024-01-01T00:00:00Z")
	middle := createArticle(t, router, source.ID, "2025-01-01T00:00:00Z")
	newest := createArticle(t, router, source.ID, "2026-01-01T00:00:00Z")

	var got []string
	path := "/api/articles?limit=2&source_id=" + source.ID
	for page := 0; ; page++ {
		if page > 10 {
			t.Fatal("pagination did not terminate")
		}
		rec := do(router, "GET", path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("LIST status = %d (body %s)", rec.Code, rec.Body.String())
		}
		body := decode[articleListBody](t, rec)
		for _, a := range body.Items {
			got = append(got, a.ID)
		}
		if body.NextCursor == nil {
			break
		}
		path = "/api/articles?limit=2&source_id=" + source.ID + "&cursor=" + *body.NextCursor
	}
	if want := []string{newest.ID, middle.ID, oldest.ID}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("paged feed = %v, want newest first %v", got, want)
	}
}

func TestIntegration_DeleteSourceWithArticles(t *testing.T) {
	router := integrationRouter(t)
	source := createSource(t, router, "Has articles")
	article := createArticle(t, router, source.ID, "")

	assertError(t, do(router, "DELETE", "/api/sources/"+source.ID, ""), http.StatusConflict, "source_has_articles")

	if rec := do(router, "DELETE", "/api/articles/"+article.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE article status = %d, want 204", rec.Code)
	}
	if rec := do(router, "DELETE", "/api/sources/"+source.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE source after its articles status = %d, want 204 (body %s)", rec.Code, rec.Body.String())
	}
}
