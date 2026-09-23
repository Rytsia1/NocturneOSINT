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

// EvidenceRepository stores Evidence: the provenance links between Articles
// and Events.
type EvidenceRepository struct {
	db *pgxpool.Pool
}

func NewEvidenceRepository(db *pgxpool.Pool) *EvidenceRepository {
	return &EvidenceRepository{db: db}
}

// EvidenceArticle is an Article linked to an Event by the Evidence EvidenceID.
type EvidenceArticle struct {
	EvidenceID string
	domain.Article
}

// EvidenceEvent is an Event linked to an Article by the Evidence EvidenceID.
type EvidenceEvent struct {
	EvidenceID string
	domain.Event
}

const evidenceColumns = "id::text, article_id::text, event_id::text, created_at, updated_at"

func scanEvidence(row pgx.Row) (domain.Evidence, error) {
	var e domain.Evidence
	err := row.Scan(&e.ID, &e.ArticleID, &e.EventID, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

// Create stores e. The foreign keys reject unknown Articles and Events
// atomically; the unique constraint rejects a second link for the same pair.
func (r *EvidenceRepository) Create(ctx context.Context, e domain.Evidence) (domain.Evidence, error) {
	created, err := scanEvidence(r.db.QueryRow(ctx,
		`INSERT INTO evidence (article_id, event_id) VALUES ($1::uuid, $2::uuid) RETURNING `+evidenceColumns,
		e.ArticleID, e.EventID))

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "evidence_article_id_fkey":
			return domain.Evidence{}, domain.ErrArticleNotFound
		case "evidence_event_id_fkey":
			return domain.Evidence{}, domain.ErrEventNotFound
		case "evidence_article_event_key":
			return domain.Evidence{}, domain.ErrEvidenceExists
		}
	}
	if err != nil {
		return domain.Evidence{}, fmt.Errorf("insert evidence: %w", err)
	}
	return created, nil
}

// GetByID returns domain.ErrEvidenceNotFound when no Evidence has the given ID.
func (r *EvidenceRepository) GetByID(ctx context.Context, id string) (domain.Evidence, error) {
	e, err := scanEvidence(r.db.QueryRow(ctx, `SELECT `+evidenceColumns+` FROM evidence WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Evidence{}, domain.ErrEvidenceNotFound
	}
	if err != nil {
		return domain.Evidence{}, fmt.Errorf("get evidence: %w", err)
	}
	return e, nil
}

// Delete removes the link only; the Article and Event stay.
func (r *EvidenceRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM evidence WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete evidence: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEvidenceNotFound
	}
	return nil
}

// ArticlesForEvent returns up to limit Articles linked to the Event, in Article
// feed order (newest first), starting after the cursor. It returns
// domain.ErrEventNotFound when the Event does not exist.
func (r *EvidenceRepository) ArticlesForEvent(ctx context.Context, eventID string, limit int, after *Cursor) ([]EvidenceArticle, error) {
	at, id := cursorArgs(after)
	rows, err := r.db.Query(ctx,
		`SELECT `+articleColumns+`, ev.evidence_id::text
		 FROM articles JOIN (SELECT id AS evidence_id, article_id FROM evidence WHERE event_id = $1::uuid) ev
		      ON ev.article_id = articles.id
		 WHERE $3::timestamptz IS NULL OR (feed_at, id) < ($3::timestamptz, $4::uuid)
		 ORDER BY feed_at DESC, id DESC LIMIT $2`,
		eventID, limit, at, id)
	if err != nil {
		return nil, fmt.Errorf("list event articles: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (EvidenceArticle, error) {
		var x EvidenceArticle
		a, err := scanArticle(row, &x.EvidenceID)
		x.Article = a
		return x, err
	})
	if err != nil {
		return nil, fmt.Errorf("list event articles: %w", err)
	}
	if len(items) == 0 {
		if err := r.mustExist(ctx, "events", eventID, domain.ErrEventNotFound); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// EventsForArticle returns up to limit Events linked to the Article, in Event
// feed order (newest first), starting after the cursor. It returns
// domain.ErrArticleNotFound when the Article does not exist.
func (r *EvidenceRepository) EventsForArticle(ctx context.Context, articleID string, limit int, after *Cursor) ([]EvidenceEvent, error) {
	at, id := cursorArgs(after)
	rows, err := r.db.Query(ctx,
		`SELECT `+eventColumns+`, ev.evidence_id::text
		 FROM events JOIN (SELECT id AS evidence_id, event_id FROM evidence WHERE article_id = $1::uuid) ev
		      ON ev.event_id = events.id
		 WHERE $3::timestamptz IS NULL OR (feed_at, id) < ($3::timestamptz, $4::uuid)
		 ORDER BY feed_at DESC, id DESC LIMIT $2`,
		articleID, limit, at, id)
	if err != nil {
		return nil, fmt.Errorf("list article events: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (EvidenceEvent, error) {
		var x EvidenceEvent
		e, err := scanEvent(row, &x.EvidenceID)
		x.Event = e
		return x, err
	})
	if err != nil {
		return nil, fmt.Errorf("list article events: %w", err)
	}
	if len(items) == 0 {
		if err := r.mustExist(ctx, "articles", articleID, domain.ErrArticleNotFound); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// cursorArgs turns an optional cursor into nullable query parameters.
func cursorArgs(after *Cursor) (any, any) {
	if after == nil {
		return nil, nil
	}
	return after.At, after.ID
}

// mustExist returns notFound when table has no row with the given id.
// table is always a constant from this file, never user input.
func (r *EvidenceRepository) mustExist(ctx context.Context, table, id string, notFound error) error {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = $1::uuid)`, id).Scan(&exists); err != nil {
		return fmt.Errorf("check %s: %w", table, err)
	}
	if !exists {
		return notFound
	}
	return nil
}
