package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nocturne-backend/internal/domain"
)

// ArticleRepository stores Articles in the articles table.
type ArticleRepository struct {
	db *pgxpool.Pool
}

func NewArticleRepository(db *pgxpool.Pool) *ArticleRepository {
	return &ArticleRepository{db: db}
}

const articleColumns = "id::text, source_id::text, title, url, summary, published_at, retrieved_at, created_at, updated_at"

func scanArticle(row pgx.Row, extra ...any) (domain.Article, error) {
	var a domain.Article
	dest := append([]any{&a.ID, &a.SourceID, &a.Title, &a.URL, &a.Summary, &a.PublishedAt, &a.RetrievedAt, &a.CreatedAt, &a.UpdatedAt}, extra...)
	return a, row.Scan(dest...)
}

// Create inserts a and returns it with its generated ID and timestamps.
// retrieved_at is set by the database: the moment Nocturne recorded the
// Article. The foreign key rejects a nonexistent Source atomically, so
// no orphan Article can be created.
func (r *ArticleRepository) Create(ctx context.Context, a domain.Article) (domain.Article, error) {
	created, err := scanArticle(r.db.QueryRow(ctx,
		`INSERT INTO articles (source_id, title, url, summary, published_at)
		 VALUES ($1::uuid, $2, $3, $4, $5) RETURNING `+articleColumns,
		a.SourceID, a.Title, a.URL, a.Summary, a.PublishedAt))

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23503" && pgErr.ConstraintName == "articles_source_id_fkey":
			return domain.Article{}, domain.ErrSourceNotFound
		case pgErr.Code == "23505" && pgErr.ConstraintName == "articles_url_key":
			return domain.Article{}, domain.ErrArticleURLExists
		}
	}
	if err != nil {
		return domain.Article{}, fmt.Errorf("insert article: %w", err)
	}
	return created, nil
}

// GetByID returns domain.ErrArticleNotFound when no Article has the given ID.
func (r *ArticleRepository) GetByID(ctx context.Context, id string) (domain.Article, error) {
	a, err := scanArticle(r.db.QueryRow(ctx,
		`SELECT `+articleColumns+` FROM articles WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Article{}, domain.ErrArticleNotFound
	}
	if err != nil {
		return domain.Article{}, fmt.Errorf("get article: %w", err)
	}
	return a, nil
}

// List returns up to limit Articles in feed order (most recent feed time
// first, see domain.Article.FeedTime), optionally only those of one Source
// (sourceID != ""), starting after the cursor.
func (r *ArticleRepository) List(ctx context.Context, limit int, sourceID string, after *Cursor) ([]domain.Article, error) {
	// Only fixed fragments are concatenated; every value is a parameter.
	args := []any{limit}
	var conds []string
	if sourceID != "" {
		args = append(args, sourceID)
		conds = append(conds, fmt.Sprintf("source_id = $%d::uuid", len(args)))
	}
	if after != nil {
		args = append(args, after.At, after.ID)
		conds = append(conds, fmt.Sprintf("(feed_at, id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}
	query := `SELECT ` + articleColumns + ` FROM articles`
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY feed_at DESC, id DESC LIMIT $1"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	articles, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Article, error) {
		return scanArticle(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	return articles, nil
}

// Delete returns domain.ErrArticleNotFound when no Article has the given ID.
func (r *ArticleRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM articles WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete article: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrArticleNotFound
	}
	return nil
}
