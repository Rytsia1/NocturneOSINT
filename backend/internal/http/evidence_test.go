package http

import (
	"fmt"
	"net/http"
	"testing"
)

// Requests rejected before any database access run without a database.
func TestEvidenceRequestValidation(t *testing.T) {
	router := NewRouter(discardLogger, nil)
	id := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"malformed JSON", "POST", "/api/evidence", `{"article_id":`, 400, "invalid_request"},
		{"unknown field", "POST", "/api/evidence", `{"article_id":"` + id + `","event_id":"` + id + `","confidence":0.9}`, 400, "invalid_request"},
		{"missing article_id", "POST", "/api/evidence", `{"event_id":"` + id + `"}`, 400, "validation_failed"},
		{"invalid article_id", "POST", "/api/evidence", `{"article_id":"x","event_id":"` + id + `"}`, 400, "validation_failed"},
		{"invalid event_id", "POST", "/api/evidence", `{"article_id":"` + id + `","event_id":"x"}`, 400, "validation_failed"},
		{"get invalid id", "GET", "/api/evidence/nope", "", 400, "invalid_id"},
		{"delete invalid id", "DELETE", "/api/evidence/123", "", 400, "invalid_id"},
		{"article events: invalid id", "GET", "/api/articles/nope/events", "", 400, "invalid_id"},
		{"article events: invalid limit", "GET", "/api/articles/" + id + "/events?limit=0", "", 400, "invalid_limit"},
		{"event articles: invalid id", "GET", "/api/events/nope/articles", "", 400, "invalid_id"},
		{"event articles: invalid cursor", "GET", "/api/events/" + id + "/articles?cursor=***", "", 400, "invalid_cursor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, do(router, tt.method, tt.path, tt.body), tt.status, tt.code)
		})
	}
}

// Integration tests: full HTTP → repository → PostgreSQL round trips.

type evidenceBody struct {
	ID        string `json:"id"`
	ArticleID string `json:"article_id"`
	EventID   string `json:"event_id"`
}

type itemsBody struct {
	Items      []map[string]any `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

func TestIntegration_EvidenceLifecycle(t *testing.T) {
	router := integrationRouter(t)
	src := createSource(t, router, "Evidence source")
	older := createArticle(t, router, src.ID, "2026-01-01T00:00:00Z")
	newer := createArticle(t, router, src.ID, "2026-02-01T00:00:00Z")
	event := createEvent(t, router, `{"title":"Port fire","occurred_at":"2026-01-01T00:00:00Z","occurred_at_precision":"day"}`)
	missing := "00000000-0000-4000-8000-000000000000"
	body := func(articleID, eventID string) string {
		return fmt.Sprintf(`{"article_id":%q,"event_id":%q}`, articleID, eventID)
	}

	// Create.
	rec := do(router, "POST", "/api/evidence", body(older.ID, event.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d (body %s)", rec.Code, rec.Body.String())
	}
	ev := decode[evidenceBody](t, rec)
	if ev.ArticleID != older.ID || ev.EventID != event.ID || rec.Header().Get("Location") != "/api/evidence/"+ev.ID {
		t.Errorf("created = %+v, Location %q", ev, rec.Header().Get("Location"))
	}
	if rec = do(router, "POST", "/api/evidence", body(newer.ID, event.ID)); rec.Code != http.StatusCreated {
		t.Fatalf("POST second status = %d (body %s)", rec.Code, rec.Body.String())
	}

	// Create failures.
	assertError(t, do(router, "POST", "/api/evidence", body(older.ID, event.ID)), http.StatusConflict, "evidence_exists")
	assertError(t, do(router, "POST", "/api/evidence", body(missing, event.ID)), http.StatusUnprocessableEntity, "article_not_found")
	assertError(t, do(router, "POST", "/api/evidence", body(older.ID, missing)), http.StatusUnprocessableEntity, "event_not_found")

	// Get.
	if rec = do(router, "GET", "/api/evidence/"+ev.ID, ""); rec.Code != http.StatusOK || decode[evidenceBody](t, rec).ArticleID != older.ID {
		t.Errorf("GET status = %d (body %s)", rec.Code, rec.Body.String())
	}

	// Event → Articles: compact items, newest first, cursor-paged.
	rec = do(router, "GET", "/api/events/"+event.ID+"/articles?limit=1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("event articles status = %d", rec.Code)
	}
	page := decode[itemsBody](t, rec)
	if len(page.Items) != 1 || page.Items[0]["article_id"] != newer.ID || page.NextCursor == nil {
		t.Fatalf("event articles page 1 = %+v", page)
	}
	if _, hasSummary := page.Items[0]["summary"]; hasSummary || page.Items[0]["url"] != newer.URL || page.Items[0]["source_id"] != src.ID {
		t.Errorf("event article item = %v, want compact provenance fields without summary", page.Items[0])
	}
	rec = do(router, "GET", "/api/events/"+event.ID+"/articles?limit=1&cursor="+*page.NextCursor, "")
	if items := decode[itemsBody](t, rec).Items; len(items) != 1 || items[0]["article_id"] != older.ID || items[0]["evidence_id"] != ev.ID {
		t.Errorf("event articles page 2 = %v", items)
	}

	// Article → Events.
	rec = do(router, "GET", "/api/articles/"+older.ID+"/events", "")
	items := decode[itemsBody](t, rec).Items
	if rec.Code != http.StatusOK || len(items) != 1 || items[0]["event_id"] != event.ID || items[0]["occurred_at_precision"] != "day" {
		t.Fatalf("article events = %d %v", rec.Code, items)
	}
	if _, hasDescription := items[0]["description"]; hasDescription {
		t.Errorf("article event item includes description: %v", items[0])
	}

	// Unknown parents.
	assertError(t, do(router, "GET", "/api/events/"+missing+"/articles", ""), http.StatusNotFound, "event_not_found")
	assertError(t, do(router, "GET", "/api/articles/"+missing+"/events", ""), http.StatusNotFound, "article_not_found")

	// Delete the link only.
	if rec = do(router, "DELETE", "/api/evidence/"+ev.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d", rec.Code)
	}
	assertError(t, do(router, "GET", "/api/evidence/"+ev.ID, ""), http.StatusNotFound, "evidence_not_found")
	assertError(t, do(router, "DELETE", "/api/evidence/"+ev.ID, ""), http.StatusNotFound, "evidence_not_found")
	if rec = do(router, "GET", "/api/articles/"+older.ID, ""); rec.Code != http.StatusOK {
		t.Errorf("Article gone after its Evidence was deleted: status %d", rec.Code)
	}

	// Deleting the Event cascades to its remaining Evidence.
	if rec = do(router, "DELETE", "/api/events/"+event.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE event status = %d", rec.Code)
	}
	rec = do(router, "GET", "/api/articles/"+newer.ID+"/events", "")
	if n := len(decode[itemsBody](t, rec).Items); rec.Code != http.StatusOK || n != 0 {
		t.Errorf("article events after event delete = %d items (status %d), want 0", n, rec.Code)
	}
}
