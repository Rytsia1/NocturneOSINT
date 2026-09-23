package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func do(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v (body %q)", err, rec.Body.String())
	}
	return v
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
	if got := decode[errorBody](t, rec).Error.Code; got != code {
		t.Errorf("error code = %q, want %q", got, code)
	}
}

func encodeRaw(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// Requests rejected before any database access run without a database:
// the router gets a nil pool, which these code paths never touch.
func TestSourceRequestValidation(t *testing.T) {
	router := NewRouter(discardLogger, nil)
	valid := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"malformed JSON", "POST", "/api/sources", `{"name":`, 400, "invalid_request"},
		{"not an object", "POST", "/api/sources", `["Reuters"]`, 400, "invalid_request"},
		{"unknown field", "POST", "/api/sources", `{"name":"R","url":"https://r.com","trust":9}`, 400, "invalid_request"},
		{"missing name", "POST", "/api/sources", `{"url":"https://r.com"}`, 400, "validation_failed"},
		{"missing url", "POST", "/api/sources", `{"name":"Reuters"}`, 400, "validation_failed"},
		{"invalid url", "POST", "/api/sources", `{"name":"Reuters","url":"not a url"}`, 400, "validation_failed"},
		{"non-http url", "POST", "/api/sources", `{"name":"Reuters","url":"ftp://r.com"}`, 400, "validation_failed"},
		{"name too long", "POST", "/api/sources", `{"name":"` + strings.Repeat("a", 201) + `","url":"https://r.com"}`, 400, "validation_failed"},
		{"body too large", "POST", "/api/sources", `{"name":"R","url":"https://r.com","description":"` + strings.Repeat("a", 20000) + `"}`, 400, "invalid_request"},
		{"get invalid id", "GET", "/api/sources/not-a-uuid", "", 400, "invalid_id"},
		{"delete invalid id", "DELETE", "/api/sources/123", "", 400, "invalid_id"},
		{"limit zero", "GET", "/api/sources?limit=0", "", 400, "invalid_limit"},
		{"limit too big", "GET", "/api/sources?limit=101", "", 400, "invalid_limit"},
		{"limit not a number", "GET", "/api/sources?limit=ten", "", 400, "invalid_limit"},
		{"cursor not base64", "GET", "/api/sources?cursor=***", "", 400, "invalid_cursor"},
		{"cursor bad id", "GET", "/api/sources?cursor=" + encodeRaw("2026-01-01T00:00:00Z|nope"), "", 400, "invalid_cursor"},
		{"cursor bad time", "GET", "/api/sources?cursor=" + encodeRaw("yesterday|"+valid), "", 400, "invalid_cursor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, do(router, tt.method, tt.path, tt.body), tt.status, tt.code)
		})
	}
}

// Integration tests: full HTTP → repository → PostgreSQL round trips.

type sourceBody struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type sourceListBody struct {
	Items      []sourceBody `json:"items"`
	NextCursor *string      `json:"next_cursor"`
}

func integrationRouter(t *testing.T) http.Handler {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewRouter(discardLogger, pool)
}

func createSource(t *testing.T, router http.Handler, name string) sourceBody {
	t.Helper()
	url := fmt.Sprintf("https://example.test/http/%s/%d", t.Name(), time.Now().UnixNano())
	rec := do(router, "POST", "/api/sources", fmt.Sprintf(`{"name":%q,"url":%q,"description":"test"}`, name, url))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	s := decode[sourceBody](t, rec)
	t.Cleanup(func() { do(router, "DELETE", "/api/sources/"+s.ID, "") })
	return s
}

func TestIntegration_SourceLifecycle(t *testing.T) {
	router := integrationRouter(t)
	url := fmt.Sprintf("https://www.reuters.com/?run=%d", time.Now().UnixNano())

	// Create
	rec := do(router, "POST", "/api/sources", fmt.Sprintf(`{"name":"  Reuters ","url":%q,"description":"Global news organization."}`, url))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	created := decode[sourceBody](t, rec)
	if created.Name != "Reuters" || created.URL != url || created.Description != "Global news organization." {
		t.Errorf("created = %+v", created)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/sources/"+created.ID {
		t.Errorf("Location = %q", loc)
	}

	// Get
	rec = do(router, "GET", "/api/sources/"+created.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}
	if got := decode[sourceBody](t, rec); got.ID != created.ID || got.URL != url || !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("GET = %+v, want %+v", got, created)
	}

	// List (newest first, so the new source is on the first page)
	rec = do(router, "GET", "/api/sources", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("LIST status = %d, want 200", rec.Code)
	}
	found := false
	for _, s := range decode[sourceListBody](t, rec).Items {
		found = found || s.ID == created.ID
	}
	if !found {
		t.Error("created source missing from first page of GET /api/sources")
	}

	// Delete
	if rec = do(router, "DELETE", "/api/sources/"+created.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("DELETE body = %q, want empty", rec.Body.String())
	}

	// Gone
	assertError(t, do(router, "GET", "/api/sources/"+created.ID, ""), http.StatusNotFound, "source_not_found")
	assertError(t, do(router, "DELETE", "/api/sources/"+created.ID, ""), http.StatusNotFound, "source_not_found")
}

func TestIntegration_CreateSource_DuplicateURL(t *testing.T) {
	router := integrationRouter(t)
	existing := createSource(t, router, "Original")

	rec := do(router, "POST", "/api/sources", fmt.Sprintf(`{"name":"Copy","url":%q}`, existing.URL))
	assertError(t, rec, http.StatusConflict, "source_already_exists")
}

func TestIntegration_GetSource_NotFound(t *testing.T) {
	router := integrationRouter(t)
	assertError(t, do(router, "GET", "/api/sources/00000000-0000-4000-8000-000000000000", ""), http.StatusNotFound, "source_not_found")
}

func TestIntegration_ListSources_Pagination(t *testing.T) {
	router := integrationRouter(t)
	var mine []string
	for i := range 3 {
		mine = append(mine, createSource(t, router, fmt.Sprintf("Page %d", i)).ID)
	}

	var order []string
	path := "/api/sources?limit=2"
	for page := 0; ; page++ {
		if page > 1000 {
			t.Fatal("pagination did not terminate")
		}
		rec := do(router, "GET", path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("LIST status = %d (body %s)", rec.Code, rec.Body.String())
		}
		body := decode[sourceListBody](t, rec)
		if len(body.Items) > 2 {
			t.Fatalf("page has %d items, want <= 2", len(body.Items))
		}
		for _, s := range body.Items {
			order = append(order, s.ID)
		}
		if body.NextCursor == nil {
			break
		}
		path = "/api/sources?limit=2&cursor=" + *body.NextCursor
	}

	var got []string
	for _, id := range order {
		for _, m := range mine {
			if id == m {
				got = append(got, id)
			}
		}
	}
	if want := []string{mine[2], mine[1], mine[0]}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("created sources across pages = %v, want newest first %v", got, want)
	}
}
