package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nocturne-backend/internal/domain"
)

// ArticleMediaRepository stores external image metadata of Articles. Every
// lookup is scoped to its Article, so media is never reachable through
// another Article.
type ArticleMediaRepository struct {
	db *pgxpool.Pool
}

func NewArticleMediaRepository(db *pgxpool.Pool) *ArticleMediaRepository {
	return &ArticleMediaRepository{db: db}
}

const articleMediaColumns = "id::text, article_id::text, url, media_type, width, height, created_at, updated_at"

func scanArticleMedia(row pgx.Row) (domain.ArticleMedia, error) {
	var m domain.ArticleMedia
	err := row.Scan(&m.ID, &m.ArticleID, &m.URL, &m.MediaType, &m.Width, &m.Height, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

// Create stores m. The foreign key rejects an unknown Article atomically; the
// unique constraint rejects the same URL twice for one Article.
func (r *ArticleMediaRepository) Create(ctx context.Context, m domain.ArticleMedia) (domain.ArticleMedia, error) {
	created, err := scanArticleMedia(r.db.QueryRow(ctx,
		`INSERT INTO article_media (article_id, url, media_type, width, height)
		 VALUES ($1::uuid, $2, $3, $4, $5) RETURNING `+articleMediaColumns,
		m.ArticleID, m.URL, m.MediaType, m.Width, m.Height))

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "article_media_article_id_fkey":
			return domain.ArticleMedia{}, domain.ErrArticleNotFound
		case "article_media_article_url_key":
			return domain.ArticleMedia{}, domain.ErrArticleMediaExists
		}
	}
	if err != nil {
		return domain.ArticleMedia{}, fmt.Errorf("insert article media: %w", err)
	}
	return created, nil
}

// GetByID returns domain.ErrArticleMediaNotFound unless media id belongs to
// the Article.
func (r *ArticleMediaRepository) GetByID(ctx context.Context, articleID, id string) (domain.ArticleMedia, error) {
	m, err := scanArticleMedia(r.db.QueryRow(ctx,
		`SELECT `+articleMediaColumns+` FROM article_media WHERE id = $1::uuid AND article_id = $2::uuid`, id, articleID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArticleMedia{}, domain.ErrArticleMediaNotFound
	}
	if err != nil {
		return domain.ArticleMedia{}, fmt.Errorf("get article media: %w", err)
	}
	return m, nil
}

// Delete removes media id of the Article; the Article itself stays.
func (r *ArticleMediaRepository) Delete(ctx context.Context, articleID, id string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM article_media WHERE id = $1::uuid AND article_id = $2::uuid`, id, articleID)
	if err != nil {
		return fmt.Errorf("delete article media: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrArticleMediaNotFound
	}
	return nil
}

// ListByArticle returns up to limit media of the Article, oldest first (the
// first is the feed-card thumbnail candidate). It returns
// domain.ErrArticleNotFound when the Article does not exist.
func (r *ArticleMediaRepository) ListByArticle(ctx context.Context, articleID string, limit int) ([]domain.ArticleMedia, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+articleMediaColumns+` FROM article_media
		 WHERE article_id = $1::uuid ORDER BY created_at, id LIMIT $2`, articleID, limit)
	if err != nil {
		return nil, fmt.Errorf("list article media: %w", err)
	}
	media, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ArticleMedia, error) {
		return scanArticleMedia(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list article media: %w", err)
	}

	if len(media) == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM articles WHERE id = $1::uuid)`, articleID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check article: %w", err)
		}
		if !exists {
			return nil, domain.ErrArticleNotFound
		}
	}
	return media, nil
}
