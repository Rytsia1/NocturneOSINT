package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nocturne-backend/internal/ingest"
)

func TestIngestRequestValidation(t *testing.T) {
	assertError(t, do(NewRouter(discardLogger, nil), "POST", "/api/sources/nope/ingest", ""), 400, "invalid_id")
}

// Integration tests: HTTP → ingest service → in-process feed server → PostgreSQL.

// ingestEnv serves fixed feeds on 127.0.0.1 and routes with a Fetcher that
// may reach it; the production router refuses loopback addresses.
type ingestEnv struct {
	pool    *pgxpool.Pool
	router  http.Handler // loopback allowed, 300 ms timeout
	prod    http.Handler // NewRouter: public addresses only
	feedURL string
}

func newIngestEnv(t *testing.T) *ingestEnv {
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

	run := uniq()
	mux := http.NewServeMux()
	mux.HandleFunc("/rss", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>
			<item><title>RSS one</title><link>https://example.test/%[1]s/rss/1</link><pubDate>Sun, 20 Sep 2026 08:00:00 GMT</pubDate><description>&lt;p&gt;One&lt;/p&gt;</description></item>
			<item><title>RSS two</title><link>https://example.test/%[1]s/rss/2</link></item>
			<item><title></title><link>https://example.test/%[1]s/rss/untitled</link></item>
			</channel></rss>`, run)
	})
	mux.HandleFunc("/atom", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<feed xmlns="http://www.w3.org/2005/Atom"><title>T</title>
			<entry><title>Atom one</title><link href="https://example.test/%[1]s/atom/1"/><published>2026-09-19T10:00:00Z</published></entry>
			<entry><title>Atom two</title><link href="https://example.test/%[1]s/atom/2"/><updated>2026-09-21T10:00:00Z</updated></entry>
			</feed>`, run)
	})
	mux.HandleFunc("/html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml") // the body decides, not the header
		w.Write([]byte(`<html><body><a href="https://example.test/">not a feed</a></body></html>`))
	})
	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusBadGateway) })
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	allowAll := func(netip.Addr) bool { return true }
	fetcher := ingest.NewFetcher(300*time.Millisecond, ingest.MaxFeedBytes, allowAll)
	return &ingestEnv{pool: pool, router: newRouter(discardLogger, pool, fetcher), prod: NewRouter(discardLogger, pool), feedURL: srv.URL}
}

// source creates a Source whose URL is path on the feed server; when the test
// ends its ingested Articles are removed, then the Source (RESTRICT order).
func (e *ingestEnv) source(t *testing.T, path string) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":"Feed","url":%q}`, e.feedURL+path+"?run="+uniq())
	rec := do(e.router, "POST", "/api/sources", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/sources = %d (%s)", rec.Code, rec.Body.String())
	}
	id := decode[sourceBody](t, rec).ID
	t.Cleanup(func() {
		e.pool.Exec(context.Background(), `DELETE FROM articles WHERE source_id = $1::uuid`, id)
		do(e.router, "DELETE", "/api/sources/"+id, "")
	})
	return id
}

type ingestBody struct {
	SourceID   string `json:"source_id"`
	Fetched    int    `json:"fetched"`
	Inserted   int    `json:"inserted"`
	Duplicates int    `json:"duplicates"`
	Invalid    int    `json:"invalid"`
	Truncated  bool   `json:"truncated"`
	Errors     []struct {
		Item   int    `json:"item"`
		Reason string `json:"reason"`
	} `json:"errors"`
}

func ingestOK(t *testing.T, router http.Handler, sourceID string) ingestBody {
	t.Helper()
	rec := do(router, "POST", "/api/sources/"+sourceID+"/ingest", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("ingest status = %d (%s)", rec.Code, rec.Body.String())
	}
	return decode[ingestBody](t, rec)
}

func TestIntegration_IngestRSS(t *testing.T) {
	env := newIngestEnv(t)
	src := env.source(t, "/rss")

	// Compact response: counts and a bounded error list, no feed content.
	rec := do(env.router, "POST", "/api/sources/"+src+"/ingest", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var keys []string
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if got := fmt.Sprint(keys); got != "[duplicates errors fetched inserted invalid invalid_media media source_id truncated]" {
		t.Errorf("response keys = %s", got)
	}
	first := decode[ingestBody](t, rec)
	if first.SourceID != src || first.Fetched != 3 || first.Inserted != 2 || first.Duplicates != 0 || first.Invalid != 1 ||
		len(first.Errors) != 1 || first.Errors[0].Item != 2 {
		t.Errorf("first ingest = %+v", first)
	}

	// The Articles belong to the Source, with plain-text summaries.
	list := decode[articleListBody](t, do(env.router, "GET", "/api/articles?source_id="+src, ""))
	if len(list.Items) != 2 {
		t.Fatalf("articles for source = %d, want 2", len(list.Items))
	}
	for _, a := range list.Items {
		if a.SourceID != src {
			t.Errorf("article %s source_id = %s, want %s", a.ID, a.SourceID, src)
		}
		if a.Title == "RSS one" && a.Summary != "One" {
			t.Errorf("summary = %q, want plain text", a.Summary)
		}
	}

	// Same feed again: idempotent.
	second := ingestOK(t, env.router, src)
	if second.Fetched != 3 || second.Inserted != 0 || second.Duplicates != 2 || second.Invalid != 1 {
		t.Errorf("second ingest = %+v, want 0 inserted, 2 duplicates", second)
	}
}

func TestIntegration_IngestAtom(t *testing.T) {
	env := newIngestEnv(t)
	src := env.source(t, "/atom")

	if got := ingestOK(t, env.router, src); got.Fetched != 2 || got.Inserted != 2 || got.Invalid != 0 {
		t.Errorf("atom ingest = %+v", got)
	}
	if got := ingestOK(t, env.router, src); got.Inserted != 0 || got.Duplicates != 2 {
		t.Errorf("atom re-ingest = %+v", got)
	}
	byTitle := map[string]articleBody{}
	for _, a := range decode[articleListBody](t, do(env.router, "GET", "/api/articles?source_id="+src, "")).Items {
		byTitle[a.Title] = a
	}
	if p := byTitle["Atom one"].PublishedAt; p == nil || !p.Equal(time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("Atom one published_at = %v", p)
	}
	if p := byTitle["Atom two"].PublishedAt; p != nil {
		t.Errorf("Atom two (updated only) published_at = %v, want null", p)
	}
}

func TestIntegration_IngestErrors(t *testing.T) {
	env := newIngestEnv(t)
	missing := "00000000-0000-4000-8000-000000000000"
	ingestPath := func(feedPath string) string { return "/api/sources/" + env.source(t, feedPath) + "/ingest" }

	assertError(t, do(env.router, "POST", "/api/sources/"+missing+"/ingest", ""), http.StatusNotFound, "source_not_found")
	assertError(t, do(env.router, "POST", ingestPath("/html"), ""), http.StatusUnprocessableEntity, "invalid_feed")
	assertError(t, do(env.router, "POST", ingestPath("/down"), ""), http.StatusBadGateway, "upstream_error")
	assertError(t, do(env.router, "POST", ingestPath("/nowhere"), ""), http.StatusBadGateway, "upstream_error")
	assertError(t, do(env.router, "POST", ingestPath("/slow"), ""), http.StatusGatewayTimeout, "upstream_timeout")

	// Production routing refuses a Source that points at a loopback address.
	assertError(t, do(env.prod, "POST", ingestPath("/rss"), ""), http.StatusUnprocessableEntity, "source_not_ingestible")
}
