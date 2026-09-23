package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"nocturne-backend/internal/domain"
)

// Integration tests against real PostgreSQL; skipped without DATABASE_URL.
// Each test searches for a word unique to its run, so other tests' Articles
// never match.

func searchToken() string { return "srch" + strings.ReplaceAll(uniq(), "-", "x") }

func createSearchArticle(t *testing.T, repo *ArticleRepository, sourceID, title, summary string, publishedAt *time.Time) domain.Article {
	t.Helper()
	a, err := domain.NewArticle(sourceID, title, fmt.Sprintf("https://example.test/search/%s", uniq()), summary, publishedAt)
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

func runSearch(t *testing.T, repo *ArticleRepository, query, sourceID string, from, to *time.Time) []ArticleMatch {
	t.Helper()
	s, err := domain.NewArticleSearch(query, sourceID, from, to)
	if err != nil {
		t.Fatalf("NewArticleSearch: %v", err)
	}
	got, err := repo.SearchArticles(context.Background(), s, 100, nil)
	if err != nil {
		t.Fatalf("SearchArticles(%q): %v", query, err)
	}
	return got
}

func matchIDs(matches []ArticleMatch) []string {
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = m.ID
	}
	return out
}

func TestIntegration_SearchArticles(t *testing.T) {
	sources, articles := newTestRepos(t)
	srcA := createTestSource(t, sources, "Search A").ID
	srcB := createTestSource(t, sources, "Search B").ID
	tok := searchToken()
	day := func(d int) *time.Time { return timePtr(time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC)) }

	both := createSearchArticle(t, articles, srcA, tok+" in both", "and "+tok+" again", day(5))
	inTitle := createSearchArticle(t, articles, srcA, "Report on "+tok, "nothing relevant", day(10))
	inSummary := createSearchArticle(t, articles, srcA, "Other headline", "mentions "+tok+" once", day(15))
	emptySummary := createSearchArticle(t, articles, srcA, strings.ToUpper(tok)+" undated", "", nil)
	otherSource := createSearchArticle(t, articles, srcB, tok+" elsewhere", "", day(20))
	unrelated := createSearchArticle(t, articles, srcA, "Unrelated", "no match here", day(12))

	got := runSearch(t, articles, tok, "", nil, nil)
	found := map[string]ArticleMatch{}
	for _, m := range got {
		found[m.ID] = m
	}
	for name, a := range map[string]domain.Article{"title": inTitle, "summary": inSummary, "both": both, "empty summary (case-insensitive)": emptySummary, "other source": otherSource} {
		if _, ok := found[a.ID]; !ok {
			t.Errorf("%s match missing", name)
		}
	}
	if _, ok := found[unrelated.ID]; ok || len(got) != 5 {
		t.Errorf("got %d results (unrelated included: %v), want the 5 matches only", len(got), ok)
	}

	// Title (weight A) outranks summary (weight B); both outranks either.
	if !(found[both.ID].Score > found[inTitle.ID].Score && found[inTitle.ID].Score > found[inSummary.ID].Score) {
		t.Errorf("scores both=%v title=%v summary=%v, want both > title > summary",
			found[both.ID].Score, found[inTitle.ID].Score, found[inSummary.ID].Score)
	}
	if got[0].ID != both.ID || got[len(got)-1].ID != inSummary.ID {
		t.Errorf("order = %v, want most relevant (both) first and summary-only last", matchIDs(got))
	}
	if again := runSearch(t, articles, tok, "", nil, nil); fmt.Sprint(matchIDs(again)) != fmt.Sprint(matchIDs(got)) {
		t.Errorf("results not deterministic:\n%v\n%v", matchIDs(got), matchIDs(again))
	}

	// Source filter.
	if bySource := runSearch(t, articles, tok, srcB, nil, nil); len(bySource) != 1 || bySource[0].ID != otherSource.ID {
		t.Errorf("source filter = %v, want only %s", matchIDs(bySource), otherSource.ID)
	}

	// Publication range: inclusive; undated Articles never match a range.
	ranged := runSearch(t, articles, tok, "", day(10), day(15))
	if fmt.Sprint(matchIDs(ranged)) != fmt.Sprint([]string{inTitle.ID, inSummary.ID}) {
		t.Errorf("range [10,15] = %v, want title then summary match", matchIDs(ranged))
	}
	if from := runSearch(t, articles, tok, "", day(16), nil); len(from) != 1 || from[0].ID != otherSource.ID {
		t.Errorf("published_from filter = %v", matchIDs(from))
	}
	for _, m := range runSearch(t, articles, tok, "", nil, day(30)) {
		if m.ID == emptySummary.ID {
			t.Error("undated Article matched a publication-date range")
		}
	}

	// Empty result set; deleted Articles disappear.
	if none := runSearch(t, articles, tok+"nomatch", "", nil, nil); len(none) != 0 {
		t.Errorf("unmatched query returned %v", matchIDs(none))
	}
	if err := articles.Delete(context.Background(), inTitle.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	for _, m := range runSearch(t, articles, tok, "", nil, nil) {
		if m.ID == inTitle.ID {
			t.Error("deleted Article still returned")
		}
	}
}

func TestIntegration_SearchArticlesPagination(t *testing.T) {
	sources, articles := newTestRepos(t)
	src := createTestSource(t, sources, "Search pages").ID
	tok := searchToken()
	// Equal scores: ordering falls back to feed time, then id.
	for i := range 7 {
		createSearchArticle(t, articles, src, fmt.Sprintf("%s item %d", tok, i), "", timePtr(time.Date(2026, 9, 1+i%3, 0, 0, 0, 0, time.UTC)))
	}
	all := runSearch(t, articles, tok, "", nil, nil)

	s, _ := domain.NewArticleSearch(tok, "", nil, nil)
	var paged []string
	var cursor *SearchCursor
	for {
		page, err := articles.SearchArticles(context.Background(), s, 3, cursor)
		if err != nil {
			t.Fatalf("SearchArticles: %v", err)
		}
		paged = append(paged, matchIDs(page)...)
		if len(page) < 3 {
			break
		}
		last := page[len(page)-1]
		cursor = &SearchCursor{Score: last.Score, At: last.FeedTime(), ID: last.ID}
	}
	if len(all) != 7 || fmt.Sprint(paged) != fmt.Sprint(matchIDs(all)) {
		t.Errorf("paged = %v\nwant    %v (no gaps, no repeats)", paged, matchIDs(all))
	}
}

// websearch_to_tsquery accepts any text: operators, stray punctuation and
// SQL fragments are search words or ignored, never errors or SQL.
func TestIntegration_SearchArticlesHostileInput(t *testing.T) {
	_, articles := newTestRepos(t)
	for _, q := range []string{
		`"unterminated quote`, `a & | ! ( ) :*`, `(((`, `-`, `OR OR OR`, `!!!`, `<-> <2>`,
		`'; DROP TABLE articles; --`, `\x00 \\ '' ""`, `日本 地震`,
	} {
		s, err := domain.NewArticleSearch(q, "", nil, nil)
		if err != nil {
			t.Fatalf("NewArticleSearch(%q): %v", q, err)
		}
		if _, err := articles.SearchArticles(context.Background(), s, 5, nil); err != nil {
			t.Errorf("SearchArticles(%q) err = %v, want no error", q, err)
		}
	}
	var exists bool
	if err := articles.db.QueryRow(context.Background(), `SELECT to_regclass('public.articles') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Fatalf("articles table missing after hostile queries (%v)", err)
	}
}
