package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"nocturne-backend/internal/domain"
)

// Integration tests against real PostgreSQL; skipped when DATABASE_URL is unset.

func newTestRepos(t *testing.T) (*SourceRepository, *ArticleRepository) {
	t.Helper()
	sources := newTestRepo(t)
	return sources, NewArticleRepository(sources.db)
}

func timePtr(t time.Time) *time.Time { return &t }

// createTestArticle creates an Article with a URL unique to this test run and
// deletes it when the test ends (before its Source, since cleanups run LIFO).
func createTestArticle(t *testing.T, repo *ArticleRepository, sourceID string, publishedAt *time.Time) domain.Article {
	t.Helper()
	url := fmt.Sprintf("https://example.test/articles/%s/%s", t.Name(), uniq())
	a, err := domain.NewArticle(sourceID, "Test article", url, "test summary", publishedAt)
	if err != nil {
		t.Fatalf("NewArticle: %v", err)
	}
	created, err := repo.Create(context.Background(), a)
	if err != nil {
		t.Fatalf("Create article: %v", err)
	}
	t.Cleanup(func() { repo.Delete(context.Background(), created.ID) })
	return created
}

func TestIntegration_ArticleProvenance(t *testing.T) {
	sources, articles := newTestRepos(t)
	ctx := context.Background()
	sourceA := createTestSource(t, sources, "Source A")
	published := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)

	created := createTestArticle(t, articles, sourceA.ID, &published)
	if created.SourceID != sourceA.ID {
		t.Fatalf("created SourceID = %s, want Source A %s", created.SourceID, sourceA.ID)
	}

	got, err := articles.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.SourceID != sourceA.ID {
		t.Errorf("stored SourceID = %s, want Source A %s", got.SourceID, sourceA.ID)
	}
	if got.PublishedAt == nil || !got.PublishedAt.Equal(published) {
		t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, published)
	}
	if got.RetrievedAt.IsZero() || time.Since(got.RetrievedAt) > time.Minute {
		t.Errorf("RetrievedAt = %v, want the recording time", got.RetrievedAt)
	}
}

func TestIntegration_ArticleWithoutPublicationTime(t *testing.T) {
	sources, articles := newTestRepos(t)
	source := createTestSource(t, sources, "Undated")

	created := createTestArticle(t, articles, source.ID, nil)
	got, err := articles.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want NULL preserved", got.PublishedAt)
	}
}

func TestIntegration_ArticleUnknownSourceRejected(t *testing.T) {
	sources, articles := newTestRepos(t)
	ctx := context.Background()
	url := fmt.Sprintf("https://example.test/orphan/%s", uniq())

	a, _ := domain.NewArticle("00000000-0000-4000-8000-000000000000", "Orphan", url, "", nil)
	if _, err := articles.Create(ctx, a); !errors.Is(err, domain.ErrSourceNotFound) {
		t.Fatalf("Create with unknown source err = %v, want ErrSourceNotFound", err)
	}

	var n int
	if err := sources.db.QueryRow(ctx, `SELECT count(*) FROM articles WHERE url = $1`, url).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("orphan article rows = %d, want 0", n)
	}
}

func TestIntegration_ArticleDuplicateURL(t *testing.T) {
	sources, articles := newTestRepos(t)
	source := createTestSource(t, sources, "Dup")
	existing := createTestArticle(t, articles, source.ID, nil)

	dup, _ := domain.NewArticle(source.ID, "Copy", existing.URL, "", nil)
	if _, err := articles.Create(context.Background(), dup); !errors.Is(err, domain.ErrArticleURLExists) {
		t.Fatalf("Create duplicate URL err = %v, want ErrArticleURLExists", err)
	}
}

func TestIntegration_ArticleNotFound(t *testing.T) {
	_, articles := newTestRepos(t)
	missing := "00000000-0000-4000-8000-000000000000"

	if _, err := articles.GetByID(context.Background(), missing); !errors.Is(err, domain.ErrArticleNotFound) {
		t.Errorf("GetByID err = %v, want ErrArticleNotFound", err)
	}
	if err := articles.Delete(context.Background(), missing); !errors.Is(err, domain.ErrArticleNotFound) {
		t.Errorf("Delete err = %v, want ErrArticleNotFound", err)
	}
}

func TestIntegration_ArticleDelete(t *testing.T) {
	sources, articles := newTestRepos(t)
	ctx := context.Background()
	source := createTestSource(t, sources, "Delete")
	created := createTestArticle(t, articles, source.ID, nil)

	if err := articles.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := articles.GetByID(ctx, created.ID); !errors.Is(err, domain.ErrArticleNotFound) {
		t.Errorf("GetByID after delete err = %v, want ErrArticleNotFound", err)
	}
}

// listAll walks every page of List with a small page size.
func listAll(t *testing.T, articles *ArticleRepository, sourceID string) []string {
	t.Helper()
	var ids []string
	var cursor *Cursor
	for {
		page, err := articles.List(context.Background(), 2, sourceID, cursor)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, a := range page {
			ids = append(ids, a.ID)
		}
		if len(page) < 2 {
			return ids
		}
		last := page[len(page)-1]
		cursor = &Cursor{At: last.FeedTime(), ID: last.ID}
	}
}

func TestIntegration_ArticleListFeedOrderAndSourceFilter(t *testing.T) {
	sources, articles := newTestRepos(t)
	sourceA := createTestSource(t, sources, "Feed A")
	sourceB := createTestSource(t, sources, "Feed B")

	older := createTestArticle(t, articles, sourceA.ID, timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)))
	newer := createTestArticle(t, articles, sourceA.ID, timePtr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	undated := createTestArticle(t, articles, sourceA.ID, nil) // sorts by retrieval time (now)
	other := createTestArticle(t, articles, sourceB.ID, nil)

	// Source filter: exactly Source A's articles, most recent feed time first.
	got := listAll(t, articles, sourceA.ID)
	if want := []string{undated.ID, newer.ID, older.ID}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Source A feed = %v, want %v", got, want)
	}

	// Unfiltered feed includes Source B and keeps Source A's relative order.
	all := listAll(t, articles, "")
	pos := map[string]int{}
	for i, id := range all {
		if _, dup := pos[id]; dup {
			t.Fatalf("article %s returned twice across pages", id)
		}
		pos[id] = i
	}
	if _, ok := pos[other.ID]; !ok {
		t.Error("Source B article missing from unfiltered feed")
	}
	if !(pos[undated.ID] < pos[newer.ID] && pos[newer.ID] < pos[older.ID]) {
		t.Errorf("unfiltered feed order wrong: undated=%d newer=%d older=%d", pos[undated.ID], pos[newer.ID], pos[older.ID])
	}
}

func TestIntegration_SourceDeleteRestrictedWhileArticlesExist(t *testing.T) {
	sources, articles := newTestRepos(t)
	ctx := context.Background()
	source := createTestSource(t, sources, "Restricted")
	article := createTestArticle(t, articles, source.ID, nil)

	if err := sources.Delete(ctx, source.ID); !errors.Is(err, domain.ErrSourceHasArticles) {
		t.Fatalf("Delete source with articles err = %v, want ErrSourceHasArticles", err)
	}
	if _, err := articles.GetByID(ctx, article.ID); err != nil {
		t.Fatalf("article lost after refused source delete: %v", err)
	}

	if err := articles.Delete(ctx, article.ID); err != nil {
		t.Fatalf("Delete article: %v", err)
	}
	if err := sources.Delete(ctx, source.ID); err != nil {
		t.Fatalf("Delete source after its articles are gone: %v", err)
	}
}
