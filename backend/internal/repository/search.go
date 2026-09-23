package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"nocturne-backend/internal/domain"
)

// ArticleMatch is an Article found by search with its textual relevance
// score (PostgreSQL ts_rank; title words weigh more than summary words).
type ArticleMatch struct {
	domain.Article
	Score float32
}

// SearchCursor is the position after which a search page continues: results
// are ordered by score, then feed time, then id, all descending.
type SearchCursor struct {
	Score float32
	At    time.Time
	ID    string
}

// SearchArticles returns up to limit Articles matching s, most relevant first,
// starting after the cursor. The query goes through websearch_to_tsquery,
// which accepts any input (quotes, OR, -word) without syntax errors.
func (r *ArticleRepository) SearchArticles(ctx context.Context, s domain.ArticleSearch, limit int, after *SearchCursor) ([]ArticleMatch, error) {
	var sourceID, score, at, id any // NULL unless set
	if s.SourceID != "" {
		sourceID = s.SourceID
	}
	if after != nil {
		score, at, id = after.Score, after.At, after.ID
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+articleColumns+`, score FROM (
		     SELECT a.id, a.source_id, a.title, a.url, a.summary, a.published_at, a.retrieved_at,
		            a.created_at, a.updated_at, a.feed_at, ts_rank(a.search_vector, q) AS score
		     FROM articles a, websearch_to_tsquery('simple', $1) q
		     WHERE a.search_vector @@ q
		       AND ($2::uuid IS NULL OR a.source_id = $2::uuid)
		       AND ($3::timestamptz IS NULL OR a.published_at >= $3::timestamptz)
		       AND ($4::timestamptz IS NULL OR a.published_at <= $4::timestamptz)
		 ) m
		 WHERE $5::real IS NULL OR (score, feed_at, id) < ($5::real, $6::timestamptz, $7::uuid)
		 ORDER BY score DESC, feed_at DESC, id DESC
		 LIMIT $8`,
		s.Query, sourceID, s.PublishedFrom, s.PublishedTo, score, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("search articles: %w", err)
	}
	matches, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ArticleMatch, error) {
		var m ArticleMatch
		a, err := scanArticle(row, &m.Score)
		m.Article = a
		return m, err
	})
	if err != nil {
		return nil, fmt.Errorf("search articles: %w", err)
	}
	return matches, nil
}
