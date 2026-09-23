package http

import (
	"fmt"
	"net/http"
	"testing"
)

// Requests rejected before any database access run without a database.
func TestArticleMediaRequestValidation(t *testing.T) {
	router := NewRouter(discardLogger, nil)
	id := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	media := "/api/articles/" + id + "/media"

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"malformed JSON", "POST", media, `{"url":`, 400, "invalid_request"},
		{"unknown field", "POST", media, `{"url":"https://e.test/1.jpg","media_type":"image","caption":"x"}`, 400, "invalid_request"},
		{"fractional width", "POST", media, `{"url":"https://e.test/1.jpg","media_type":"image","width":12.5}`, 400, "invalid_request"},
		{"missing url", "POST", media, `{"media_type":"image"}`, 400, "validation_failed"},
		{"non-http url", "POST", media, `{"url":"data:image/png;base64,AA","media_type":"image"}`, 400, "validation_failed"},
		{"invalid media_type", "POST", media, `{"url":"https://e.test/1.jpg","media_type":"video"}`, 400, "validation_failed"},
		{"zero width", "POST", media, `{"url":"https://e.test/1.jpg","media_type":"image","width":0}`, 400, "validation_failed"},
		{"height above maximum", "POST", media, `{"url":"https://e.test/1.jpg","media_type":"image","height":20001}`, 400, "validation_failed"},
		{"create: invalid article id", "POST", "/api/articles/nope/media", `{"url":"https://e.test/1.jpg","media_type":"image"}`, 400, "invalid_id"},
		{"list: invalid article id", "GET", "/api/articles/nope/media", "", 400, "invalid_id"},
		{"get: invalid media id", "GET", media + "/nope", "", 400, "invalid_id"},
		{"get: invalid article id", "GET", "/api/articles/nope/media/" + id, "", 400, "invalid_id"},
		{"delete: invalid media id", "DELETE", media + "/123", "", 400, "invalid_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, do(router, tt.method, tt.path, tt.body), tt.status, tt.code)
		})
	}
}

// Integration tests: full HTTP → repository → PostgreSQL round trips.

type mediaBody struct {
	ID        string `json:"id"`
	ArticleID string `json:"article_id"`
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
	Width     *int   `json:"width"`
	Height    *int   `json:"height"`
}

func TestIntegration_ArticleMediaLifecycle(t *testing.T) {
	router := integrationRouter(t)
	src := createSource(t, router, "Media source")
	article := createArticle(t, router, src.ID, "")
	other := createArticle(t, router, src.ID, "")
	missing := "00000000-0000-4000-8000-000000000000"
	base := "/api/articles/" + article.ID + "/media"
	img := fmt.Sprintf("https://cdn.example.test/%s/1.jpg?utm_source=rss", uniq())

	// Create with and without dimensions.
	rec := do(router, "POST", base, fmt.Sprintf(`{"url":%q,"media_type":"image","width":1200,"height":800}`, img))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d (%s)", rec.Code, rec.Body.String())
	}
	first := decode[mediaBody](t, rec)
	if first.URL != img || first.ArticleID != article.ID || first.Width == nil || *first.Width != 1200 ||
		rec.Header().Get("Location") != base+"/"+first.ID {
		t.Errorf("created = %+v, Location %q", first, rec.Header().Get("Location"))
	}
	rec = do(router, "POST", base, fmt.Sprintf(`{"url":%q,"media_type":"image"}`, img+"&v=2"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST second status = %d (%s)", rec.Code, rec.Body.String())
	}
	second := decode[mediaBody](t, rec)
	if second.Width != nil || second.Height != nil {
		t.Errorf("unstated dimensions = %v×%v, want null", second.Width, second.Height)
	}

	// Create failures.
	assertError(t, do(router, "POST", base, fmt.Sprintf(`{"url":%q,"media_type":"image"}`, img)), http.StatusConflict, "article_media_exists")
	assertError(t, do(router, "POST", "/api/articles/"+missing+"/media", fmt.Sprintf(`{"url":%q,"media_type":"image"}`, img)),
		http.StatusNotFound, "article_not_found")

	// List: creation order, compact items.
	rec = do(router, "GET", base, "")
	list := decode[struct {
		Items     []mediaBody `json:"items"`
		Truncated bool        `json:"truncated"`
	}](t, rec)
	if rec.Code != http.StatusOK || len(list.Items) != 2 || list.Items[0].ID != first.ID || list.Items[1].ID != second.ID || list.Truncated {
		t.Errorf("list = %d %+v", rec.Code, list)
	}
	rec = do(router, "GET", "/api/articles/"+other.ID+"/media", "")
	if n := len(decode[struct {
		Items []mediaBody `json:"items"`
	}](t, rec).Items); rec.Code != http.StatusOK || n != 0 {
		t.Errorf("list for article without media = %d, %d items; want 200, 0", rec.Code, n)
	}
	assertError(t, do(router, "GET", "/api/articles/"+missing+"/media", ""), http.StatusNotFound, "article_not_found")

	// Get, including no cross-Article access.
	if rec = do(router, "GET", base+"/"+first.ID, ""); rec.Code != http.StatusOK || decode[mediaBody](t, rec).URL != img {
		t.Errorf("GET status = %d (%s)", rec.Code, rec.Body.String())
	}
	assertError(t, do(router, "GET", "/api/articles/"+other.ID+"/media/"+first.ID, ""), http.StatusNotFound, "article_media_not_found")
	assertError(t, do(router, "DELETE", "/api/articles/"+other.ID+"/media/"+first.ID, ""), http.StatusNotFound, "article_media_not_found")
	assertError(t, do(router, "GET", base+"/"+missing, ""), http.StatusNotFound, "article_media_not_found")

	// Delete the media only, twice.
	if rec = do(router, "DELETE", base+"/"+first.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d", rec.Code)
	}
	assertError(t, do(router, "DELETE", base+"/"+first.ID, ""), http.StatusNotFound, "article_media_not_found")
	if rec = do(router, "GET", "/api/articles/"+article.ID, ""); rec.Code != http.StatusOK {
		t.Errorf("Article gone after its media was deleted: status %d", rec.Code)
	}

	// Deleting the Article removes its remaining media.
	if rec = do(router, "DELETE", "/api/articles/"+article.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE article status = %d", rec.Code)
	}
	assertError(t, do(router, "GET", base, ""), http.StatusNotFound, "article_not_found")
}
