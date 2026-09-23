package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"nocturne-backend/internal/domain"
)

// Integration tests against real PostgreSQL; skipped without DATABASE_URL.

func newTestMediaRepos(t *testing.T) (*ArticleRepository, *ArticleMediaRepository, string) {
	t.Helper()
	sources, articles := newTestRepos(t)
	src := createTestSource(t, sources, "Media source")
	return articles, NewArticleMediaRepository(sources.db), src.ID
}

func createTestMedia(t *testing.T, repo *ArticleMediaRepository, articleID, url string, width, height *int) domain.ArticleMedia {
	t.Helper()
	m, err := domain.NewArticleMedia(articleID, url, "image", width, height)
	if err != nil {
		t.Fatalf("NewArticleMedia: %v", err)
	}
	created, err := repo.Create(context.Background(), m)
	if err != nil {
		t.Fatalf("Create media: %v", err)
	}
	return created
}

func intp(n int) *int { return &n }

func TestIntegration_ArticleMediaCreateGetDelete(t *testing.T) {
	articles, media, sourceID := newTestMediaRepos(t)
	ctx := context.Background()
	article := createTestArticle(t, articles, sourceID, nil)
	url := fmt.Sprintf("https://cdn.example.test/%s/1.jpg?utm_source=rss", uniq())

	created := createTestMedia(t, media, article.ID, url, intp(1200), intp(800))
	got, err := media.GetByID(ctx, article.ID, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.URL != url || got.MediaType != "image" || got.Width == nil || *got.Width != 1200 || *got.Height != 800 || got.CreatedAt.IsZero() {
		t.Errorf("media = %+v", got)
	}

	noDims := createTestMedia(t, media, article.ID, url+"&v=2", nil, nil)
	if got, _ := media.GetByID(ctx, article.ID, noDims.ID); got.Width != nil || got.Height != nil {
		t.Errorf("unstated dimensions stored as %v×%v, want NULL", got.Width, got.Height)
	}

	if err := media.Delete(ctx, article.ID, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := media.GetByID(ctx, article.ID, created.ID); !errors.Is(err, domain.ErrArticleMediaNotFound) {
		t.Errorf("GetByID after delete err = %v, want ErrArticleMediaNotFound", err)
	}
	if err := media.Delete(ctx, article.ID, created.ID); !errors.Is(err, domain.ErrArticleMediaNotFound) {
		t.Errorf("Delete twice err = %v, want ErrArticleMediaNotFound", err)
	}
	if _, err := articles.GetByID(ctx, article.ID); err != nil {
		t.Errorf("Article gone after its media was deleted: %v", err)
	}
}

func TestIntegration_ArticleMediaRejected(t *testing.T) {
	articles, media, sourceID := newTestMediaRepos(t)
	ctx := context.Background()
	article := createTestArticle(t, articles, sourceID, nil)
	other := createTestArticle(t, articles, sourceID, nil)
	url := fmt.Sprintf("https://cdn.example.test/%s/shared.jpg", uniq())
	missing := "00000000-0000-4000-8000-000000000000"

	if _, err := media.Create(ctx, domain.ArticleMedia{ArticleID: missing, URL: url, MediaType: "image"}); !errors.Is(err, domain.ErrArticleNotFound) {
		t.Errorf("unknown article err = %v, want ErrArticleNotFound", err)
	}
	m := createTestMedia(t, media, article.ID, url, nil, nil)
	if _, err := media.Create(ctx, domain.ArticleMedia{ArticleID: article.ID, URL: url, MediaType: "image"}); !errors.Is(err, domain.ErrArticleMediaExists) {
		t.Errorf("duplicate err = %v, want ErrArticleMediaExists", err)
	}
	// The same image may belong to another Article.
	createTestMedia(t, media, other.ID, url, nil, nil)

	// Media is only reachable through its own Article.
	if _, err := media.GetByID(ctx, other.ID, m.ID); !errors.Is(err, domain.ErrArticleMediaNotFound) {
		t.Errorf("cross-article GetByID err = %v, want ErrArticleMediaNotFound", err)
	}
	if err := media.Delete(ctx, other.ID, m.ID); !errors.Is(err, domain.ErrArticleMediaNotFound) {
		t.Errorf("cross-article Delete err = %v, want ErrArticleMediaNotFound", err)
	}
	if _, err := media.GetByID(ctx, article.ID, m.ID); err != nil {
		t.Errorf("media removed by a cross-article delete: %v", err)
	}

	// Database checks back the domain rules.
	for name, bad := range map[string]domain.ArticleMedia{
		"video type":  {ArticleID: article.ID, URL: url + "?v", MediaType: "video"},
		"zero width":  {ArticleID: article.ID, URL: url + "?w", MediaType: "image", Width: intp(0)},
		"huge height": {ArticleID: article.ID, URL: url + "?h", MediaType: "image", Height: intp(20001)},
	} {
		if _, err := media.Create(ctx, bad); err == nil {
			t.Errorf("%s accepted by the database", name)
		}
	}
}

func TestIntegration_ArticleMediaListOrderAndCascade(t *testing.T) {
	articles, media, sourceID := newTestMediaRepos(t)
	ctx := context.Background()
	article := createTestArticle(t, articles, sourceID, nil)
	empty := createTestArticle(t, articles, sourceID, nil)
	run := uniq()

	var want []string
	for i := range 3 {
		want = append(want, createTestMedia(t, media, article.ID, fmt.Sprintf("https://cdn.example.test/%s/%d.jpg", run, i), nil, nil).ID)
	}

	got, err := media.ListByArticle(ctx, article.ID, 10)
	if err != nil {
		t.Fatalf("ListByArticle: %v", err)
	}
	if len(got) != 3 || got[0].ID != want[0] || got[1].ID != want[1] || got[2].ID != want[2] {
		t.Errorf("ListByArticle order = %v, want creation order %v", got, want)
	}
	if capped, _ := media.ListByArticle(ctx, article.ID, 2); len(capped) != 2 || capped[0].ID != want[0] {
		t.Errorf("ListByArticle(limit 2) = %v", capped)
	}
	if items, err := media.ListByArticle(ctx, empty.ID, 10); err != nil || len(items) != 0 {
		t.Errorf("ListByArticle(no media) = %v, %v; want empty", items, err)
	}
	if _, err := media.ListByArticle(ctx, "00000000-0000-4000-8000-000000000000", 10); !errors.Is(err, domain.ErrArticleNotFound) {
		t.Errorf("ListByArticle(missing) err = %v, want ErrArticleNotFound", err)
	}

	// Deleting the Article removes its media: no orphans.
	if err := articles.Delete(ctx, article.ID); err != nil {
		t.Fatalf("Delete article: %v", err)
	}
	var n int
	if err := media.db.QueryRow(ctx, `SELECT count(*) FROM article_media WHERE article_id = $1::uuid`, article.ID).Scan(&n); err != nil {
		t.Fatalf("count media: %v", err)
	}
	if n != 0 {
		t.Errorf("orphaned media after article delete = %d, want 0", n)
	}
}
