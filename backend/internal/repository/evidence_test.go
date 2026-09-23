package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"nocturne-backend/internal/domain"
)

// Integration tests against real PostgreSQL; skipped without DATABASE_URL.

type evidenceFixture struct {
	articles *ArticleRepository
	events   *EventRepository
	evidence *EvidenceRepository
	sourceID string
}

func newEvidenceFixture(t *testing.T) evidenceFixture {
	t.Helper()
	sources, articles := newTestRepos(t)
	return evidenceFixture{
		articles: articles,
		events:   NewEventRepository(sources.db),
		evidence: NewEvidenceRepository(sources.db),
		sourceID: createTestSource(t, sources, "Evidence source").ID,
	}
}

func createTestEvidence(t *testing.T, repo *EvidenceRepository, articleID, eventID string) domain.Evidence {
	t.Helper()
	e, err := domain.NewEvidence(articleID, eventID)
	if err != nil {
		t.Fatalf("NewEvidence: %v", err)
	}
	created, err := repo.Create(context.Background(), e)
	if err != nil {
		t.Fatalf("Create evidence: %v", err)
	}
	t.Cleanup(func() { repo.Delete(context.Background(), created.ID) })
	return created
}

func TestIntegration_EvidenceCreateGetDelete(t *testing.T) {
	f := newEvidenceFixture(t)
	ctx := context.Background()
	article := createTestArticle(t, f.articles, f.sourceID, nil)
	event := createTestEvent(t, f.events, "Port fire", nil, "")

	created := createTestEvidence(t, f.evidence, article.ID, event.ID)
	got, err := f.evidence.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ArticleID != article.ID || got.EventID != event.ID || got.CreatedAt.IsZero() {
		t.Errorf("evidence = %+v", got)
	}

	if err := f.evidence.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := f.evidence.GetByID(ctx, created.ID); !errors.Is(err, domain.ErrEvidenceNotFound) {
		t.Errorf("GetByID after delete err = %v, want ErrEvidenceNotFound", err)
	}
	if err := f.evidence.Delete(ctx, created.ID); !errors.Is(err, domain.ErrEvidenceNotFound) {
		t.Errorf("Delete twice err = %v, want ErrEvidenceNotFound", err)
	}
	// Removing the link leaves both ends in place.
	if _, err := f.articles.GetByID(ctx, article.ID); err != nil {
		t.Errorf("Article gone after its Evidence was deleted: %v", err)
	}
	if _, err := f.events.GetByID(ctx, event.ID); err != nil {
		t.Errorf("Event gone after its Evidence was deleted: %v", err)
	}
}

func TestIntegration_EvidenceRejected(t *testing.T) {
	f := newEvidenceFixture(t)
	ctx := context.Background()
	article := createTestArticle(t, f.articles, f.sourceID, nil)
	event := createTestEvent(t, f.events, "Port fire", nil, "")
	missing := "00000000-0000-4000-8000-000000000000"

	create := func(articleID, eventID string) error {
		_, err := f.evidence.Create(ctx, domain.Evidence{ArticleID: articleID, EventID: eventID})
		return err
	}
	if err := create(missing, event.ID); !errors.Is(err, domain.ErrArticleNotFound) {
		t.Errorf("unknown article err = %v, want ErrArticleNotFound", err)
	}
	if err := create(article.ID, missing); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("unknown event err = %v, want ErrEventNotFound", err)
	}
	createTestEvidence(t, f.evidence, article.ID, event.ID)
	if err := create(article.ID, event.ID); !errors.Is(err, domain.ErrEvidenceExists) {
		t.Errorf("duplicate err = %v, want ErrEvidenceExists", err)
	}
	if _, err := f.evidence.GetByID(ctx, missing); !errors.Is(err, domain.ErrEvidenceNotFound) {
		t.Errorf("GetByID unknown err = %v, want ErrEvidenceNotFound", err)
	}
}

func TestIntegration_EvidenceQueriesBothDirections(t *testing.T) {
	f := newEvidenceFixture(t)
	ctx := context.Background()
	older := createTestArticle(t, f.articles, f.sourceID, timePtr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	newer := createTestArticle(t, f.articles, f.sourceID, timePtr(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)))
	unlinked := createTestArticle(t, f.articles, f.sourceID, nil)
	fire := createTestEvent(t, f.events, "Port fire", timePtr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), "day")
	strike := createTestEvent(t, f.events, "Dock strike", timePtr(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)), "month")
	quiet := createTestEvent(t, f.events, "No coverage", nil, "")

	e1 := createTestEvidence(t, f.evidence, older.ID, fire.ID)
	createTestEvidence(t, f.evidence, newer.ID, fire.ID)
	createTestEvidence(t, f.evidence, older.ID, strike.ID)

	// Event → Articles: newest publication first, paged one at a time.
	var got []string
	var cursor *Cursor
	for {
		page, err := f.evidence.ArticlesForEvent(ctx, fire.ID, 1, cursor)
		if err != nil {
			t.Fatalf("ArticlesForEvent: %v", err)
		}
		if len(page) == 0 {
			break
		}
		got = append(got, page[0].ID)
		cursor = &Cursor{At: page[0].FeedTime(), ID: page[0].ID}
	}
	if len(got) != 2 || got[0] != newer.ID || got[1] != older.ID {
		t.Errorf("ArticlesForEvent = %v, want [newer older]; unlinked %s must not appear", got, unlinked.ID)
	}

	// Article → Events: newest occurrence first, with the linking Evidence ID.
	events, err := f.evidence.EventsForArticle(ctx, older.ID, 10, nil)
	if err != nil {
		t.Fatalf("EventsForArticle: %v", err)
	}
	if len(events) != 2 || events[0].ID != strike.ID || events[1].ID != fire.ID || events[1].EvidenceID != e1.ID {
		t.Errorf("EventsForArticle = %+v, want [strike fire(evidence %s)]", events, e1.ID)
	}

	// Existing but unlinked: empty. Missing: not found.
	if items, err := f.evidence.ArticlesForEvent(ctx, quiet.ID, 10, nil); err != nil || len(items) != 0 {
		t.Errorf("ArticlesForEvent(unlinked) = %v, %v; want empty", items, err)
	}
	if items, err := f.evidence.EventsForArticle(ctx, unlinked.ID, 10, nil); err != nil || len(items) != 0 {
		t.Errorf("EventsForArticle(unlinked) = %v, %v; want empty", items, err)
	}
	missing := "00000000-0000-4000-8000-000000000000"
	if _, err := f.evidence.ArticlesForEvent(ctx, missing, 10, nil); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("ArticlesForEvent(missing) err = %v, want ErrEventNotFound", err)
	}
	if _, err := f.evidence.EventsForArticle(ctx, missing, 10, nil); !errors.Is(err, domain.ErrArticleNotFound) {
		t.Errorf("EventsForArticle(missing) err = %v, want ErrArticleNotFound", err)
	}
}

func TestIntegration_EvidenceReferentialIntegrity(t *testing.T) {
	f := newEvidenceFixture(t)
	ctx := context.Background()
	count := func(col, id string) int {
		var n int
		if err := f.evidence.db.QueryRow(ctx, `SELECT count(*) FROM evidence WHERE `+col+` = $1::uuid`, id).Scan(&n); err != nil {
			t.Fatalf("count evidence: %v", err)
		}
		return n
	}

	// Deleting an Article removes its Evidence; the Event stays.
	article := createTestArticle(t, f.articles, f.sourceID, nil)
	event := createTestEvent(t, f.events, "Port fire", nil, "")
	createTestEvidence(t, f.evidence, article.ID, event.ID)
	if err := f.articles.Delete(ctx, article.ID); err != nil {
		t.Fatalf("Delete article: %v", err)
	}
	if n := count("article_id", article.ID); n != 0 {
		t.Errorf("orphaned evidence after article delete = %d, want 0", n)
	}
	if _, err := f.events.GetByID(ctx, event.ID); err != nil {
		t.Errorf("Event removed with its Article: %v", err)
	}

	// Deleting an Event removes its Evidence; the Article stays.
	article2 := createTestArticle(t, f.articles, f.sourceID, nil)
	createTestEvidence(t, f.evidence, article2.ID, event.ID)
	if err := f.events.Delete(ctx, event.ID); err != nil {
		t.Fatalf("Delete event: %v", err)
	}
	if n := count("event_id", event.ID); n != 0 {
		t.Errorf("orphaned evidence after event delete = %d, want 0", n)
	}
	if _, err := f.articles.GetByID(ctx, article2.ID); err != nil {
		t.Errorf("Article removed with its Event: %v", err)
	}
}

// Articles and Events stay uncoupled: the link lives only in evidence.
func TestIntegration_NoDirectArticleEventColumns(t *testing.T) {
	f := newEvidenceFixture(t)
	var n int
	err := f.evidence.db.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.columns
		 WHERE (table_name = 'articles' AND column_name = 'event_id')
		    OR (table_name = 'events' AND column_name = 'article_id')`).Scan(&n)
	if err != nil {
		t.Fatalf("query columns: %v", err)
	}
	if n != 0 {
		t.Errorf("found %d direct Article/Event columns, want 0", n)
	}
}
