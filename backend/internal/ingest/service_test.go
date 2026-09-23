package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nocturne-backend/internal/database"
	"nocturne-backend/internal/domain"
	"nocturne-backend/internal/repository"
)

// Integration tests against real PostgreSQL, fetching feeds from an
// in-process server; skipped when DATABASE_URL is unset.

var seq atomic.Int64

// run makes Article URLs unique per test run: articles.url is globally unique.
func run() string { return fmt.Sprintf("%d-%d", time.Now().UnixNano(), seq.Add(1)) }

func xmlItem(title, link, pubDate string) string {
	s := "<item><title>" + title + "</title><link>" + link + "</link>"
	if pubDate != "" {
		s += "<pubDate>" + pubDate + "</pubDate>"
	}
	return s + "<description>summary of " + title + "</description></item>"
}

func rssFeed(items ...string) string {
	return `<?xml version="1.0"?><rss version="2.0"><channel><title>Test</title>` + strings.Join(items, "") + `</channel></rss>`
}

type fixture struct {
	db       *pgxpool.Pool
	sources  *repository.SourceRepository
	articles *repository.ArticleRepository
	media    *repository.ArticleMediaRepository
	service  *Service
	server   *httptest.Server
	feeds    sync.Map // path → body served with 200
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	db, err := database.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(db.Close)

	f := &fixture{db: db}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/down" {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		body, ok := f.feeds.Load(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(body.(string)))
	}))
	t.Cleanup(f.server.Close)

	f.sources = repository.NewSourceRepository(db)
	f.articles = repository.NewArticleRepository(db)
	f.media = repository.NewArticleMediaRepository(db)
	f.service = f.newService(NewFetcher(5*time.Second, MaxFeedBytes, allowAll))
	return f
}

func (f *fixture) newService(fetcher *Fetcher) *Service {
	return NewService(f.sources, f.articles, f.media, fetcher, slog.New(slog.DiscardHandler))
}

// source serves body at path and creates a Source with that URL; the Source
// and its Articles are removed when the test ends.
func (f *fixture) source(t *testing.T, path, body string) domain.Source {
	t.Helper()
	if body != "" {
		f.feeds.Store(path, body)
	}
	s, err := domain.NewSource("Test feed", f.server.URL+path+"?run="+run(), "")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	created, err := f.sources.Create(context.Background(), s)
	if err != nil {
		t.Fatalf("Create source: %v", err)
	}
	t.Cleanup(func() {
		f.db.Exec(context.Background(), `DELETE FROM articles WHERE source_id = $1::uuid`, created.ID)
		f.sources.Delete(context.Background(), created.ID)
	})
	return created
}

// stored returns the Source's Articles by URL, failing on duplicate URLs.
func (f *fixture) stored(t *testing.T, sourceID string) map[string]domain.Article {
	t.Helper()
	out := map[string]domain.Article{}
	var cursor *repository.Cursor
	for {
		page, err := f.articles.List(context.Background(), 100, sourceID, cursor)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, a := range page {
			if _, dup := out[a.URL]; dup {
				t.Fatalf("URL %s stored twice", a.URL)
			}
			out[a.URL] = a
		}
		if len(page) < 100 {
			return out
		}
		last := page[len(page)-1]
		cursor = &repository.Cursor{At: last.FeedTime(), ID: last.ID}
	}
}

func assertCounts(t *testing.T, res Result, fetched, inserted, duplicates, invalid int) {
	t.Helper()
	if res.Fetched != fetched || res.Inserted != inserted || res.Duplicates != duplicates || res.Invalid != invalid {
		t.Errorf("result = fetched %d, inserted %d, duplicates %d, invalid %d; want %d, %d, %d, %d",
			res.Fetched, res.Inserted, res.Duplicates, res.Invalid, fetched, inserted, duplicates, invalid)
	}
}

func TestIntegration_IngestIsIdempotent(t *testing.T) {
	f := newFixture(t)
	r := run()
	links := []string{"https://example.test/" + r + "/1", "https://example.test/" + r + "/2", "https://example.test/" + r + "/3"}
	src := f.source(t, "/rss", rssFeed(
		xmlItem("First", links[0], "Sun, 20 Sep 2026 08:00:00 +0700"),
		xmlItem("Second", links[1], ""),
		xmlItem("Third", links[2], "Mon, 21 Sep 2026 10:00:00 GMT"),
	))

	res, err := f.service.Ingest(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	assertCounts(t, res, 3, 3, 0, 0)

	stored := f.stored(t, src.ID)
	if len(stored) != 3 {
		t.Fatalf("stored %d articles for the source, want 3", len(stored))
	}
	first := stored[links[0]]
	if first.SourceID != src.ID || first.Title != "First" || first.PublishedAt == nil ||
		!first.PublishedAt.Equal(time.Date(2026, 9, 20, 1, 0, 0, 0, time.UTC)) || first.RetrievedAt.IsZero() {
		t.Errorf("stored article = %+v", first)
	}
	if stored[links[1]].PublishedAt != nil {
		t.Errorf("undated item stored with published_at %v", stored[links[1]].PublishedAt)
	}

	// Unchanged feed: nothing new, nothing rewritten.
	res, err = f.service.Ingest(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	assertCounts(t, res, 3, 0, 3, 0)
	if again := f.stored(t, src.ID); len(again) != 3 || !again[links[0]].UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("after re-ingest: %d articles, first updated_at %v → %v", len(again), first.UpdatedAt, again[links[0]].UpdatedAt)
	}
}

func TestIntegration_IngestMixedItems(t *testing.T) {
	f := newFixture(t)
	r := run()
	a, b := "https://example.test/"+r+"/a", "https://example.test/"+r+"/b"
	src := f.source(t, "/mixed", rssFeed(
		xmlItem("Valid A", a, ""),
		xmlItem("", "https://example.test/"+r+"/untitled", ""), // 1: no title
		xmlItem("No link", "", ""),                             // 2: no URL
		xmlItem("Valid A again", a, ""),                        // 3: repeated in the feed
		xmlItem("Valid B", b, ""),
		xmlItem("Relative", "/news/relative", ""), // 5: not absolute
	))

	res, err := f.service.Ingest(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	assertCounts(t, res, 6, 2, 1, 3)
	if len(res.Errors) != 3 || res.Errors[0].Item != 1 || res.Errors[1].Item != 2 || res.Errors[2].Item != 5 {
		t.Errorf("errors = %+v, want items 1, 2, 5", res.Errors)
	}
	if stored := f.stored(t, src.ID); len(stored) != 2 || stored[a].Title != "Valid A" || stored[b].ID == "" {
		t.Errorf("stored = %v, want Valid A and Valid B only", stored)
	}
}

func TestIntegration_IngestNeverOverwritesExistingArticles(t *testing.T) {
	f := newFixture(t)
	link := "https://example.test/" + run() + "/existing"
	src := f.source(t, "/existing", rssFeed(xmlItem("Rewritten title", link, "Sun, 20 Sep 2026 08:00:00 +0700")))

	original, err := domain.NewArticle(src.ID, "Original title", link, "original summary", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.articles.Create(context.Background(), original); err != nil {
		t.Fatalf("Create: %v", err)
	}

	res, err := f.service.Ingest(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	assertCounts(t, res, 1, 0, 1, 0)
	got := f.stored(t, src.ID)[link]
	if got.Title != "Original title" || got.Summary != "original summary" || got.PublishedAt != nil {
		t.Errorf("existing article changed: %+v", got)
	}
}

func TestIntegration_IngestTruncatesLargeFeeds(t *testing.T) {
	f := newFixture(t)
	r := run()
	items := make([]string, MaxFeedItems+5)
	for i := range items {
		items[i] = xmlItem(fmt.Sprintf("Item %d", i), fmt.Sprintf("https://example.test/%s/%d", r, i), "")
	}
	src := f.source(t, "/large", rssFeed(items...))

	res, err := f.service.Ingest(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	assertCounts(t, res, MaxFeedItems, MaxFeedItems, 0, 0)
	if !res.Truncated {
		t.Error("Truncated = false, want true")
	}
}

func TestIntegration_IngestFailures(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.service.Ingest(ctx, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, domain.ErrSourceNotFound) {
		t.Errorf("missing source err = %v, want ErrSourceNotFound", err)
	}

	empty := f.source(t, "/empty", rssFeed())
	res, err := f.service.Ingest(ctx, empty.ID)
	if err != nil {
		t.Fatalf("empty feed: %v", err)
	}
	assertCounts(t, res, 0, 0, 0, 0)

	malformed := f.source(t, "/malformed", `<rss version="2.0"><channel><item><title>x</item>`)
	if _, err := f.service.Ingest(ctx, malformed.ID); !errors.Is(err, ErrInvalidFeed) {
		t.Errorf("malformed feed err = %v, want ErrInvalidFeed", err)
	}
	if n := len(f.stored(t, malformed.ID)); n != 0 {
		t.Errorf("malformed feed stored %d articles", n)
	}

	down := f.source(t, "/down", "")
	if _, err := f.service.Ingest(ctx, down.ID); !errors.Is(err, ErrUpstream) {
		t.Errorf("upstream 503 err = %v, want ErrUpstream", err)
	}

	// The production fetcher refuses the loopback test server outright.
	blocked := f.newService(NewDefaultFetcher())
	if _, err := blocked.Ingest(ctx, empty.ID); !errors.Is(err, ErrBlockedURL) {
		t.Errorf("loopback source with default fetcher err = %v, want ErrBlockedURL", err)
	}
}
