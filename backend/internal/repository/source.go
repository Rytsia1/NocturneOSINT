// Package repository persists domain objects in PostgreSQL.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nocturne-backend/internal/domain"
)

// SourceRepository stores Sources in the sources table.
type SourceRepository struct {
	db *pgxpool.Pool
}

func NewSourceRepository(db *pgxpool.Pool) *SourceRepository {
	return &SourceRepository{db: db}
}

// SourceCursor identifies the last Source of a previous List page.
type SourceCursor struct {
	CreatedAt time.Time
	ID        string
}

const sourceColumns = "id::text, name, url, description, created_at, updated_at"

func scanSource(row pgx.Row) (domain.Source, error) {
	var s domain.Source
	err := row.Scan(&s.ID, &s.Name, &s.URL, &s.Description, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// Create inserts s and returns it with its generated ID and timestamps.
func (r *SourceRepository) Create(ctx context.Context, s domain.Source) (domain.Source, error) {
	created, err := scanSource(r.db.QueryRow(ctx,
		`INSERT INTO sources (name, url, description) VALUES ($1, $2, $3) RETURNING `+sourceColumns,
		s.Name, s.URL, s.Description))

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "sources_url_key" {
		return domain.Source{}, domain.ErrSourceURLExists
	}
	if err != nil {
		return domain.Source{}, fmt.Errorf("insert source: %w", err)
	}
	return created, nil
}

// GetByID returns domain.ErrSourceNotFound when no Source has the given ID.
func (r *SourceRepository) GetByID(ctx context.Context, id string) (domain.Source, error) {
	s, err := scanSource(r.db.QueryRow(ctx,
		`SELECT `+sourceColumns+` FROM sources WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Source{}, domain.ErrSourceNotFound
	}
	if err != nil {
		return domain.Source{}, fmt.Errorf("get source: %w", err)
	}
	return s, nil
}

// List returns up to limit Sources, newest first, starting after the cursor
// (or from the newest Source when after is nil).
func (r *SourceRepository) List(ctx context.Context, limit int, after *SourceCursor) ([]domain.Source, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if after == nil {
		rows, err = r.db.Query(ctx,
			`SELECT `+sourceColumns+` FROM sources ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT `+sourceColumns+` FROM sources
			 WHERE (created_at, id) < ($2::timestamptz, $3::uuid)
			 ORDER BY created_at DESC, id DESC LIMIT $1`,
			limit, after.CreatedAt, after.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}

	sources, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Source, error) {
		return scanSource(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	return sources, nil
}

// Delete returns domain.ErrSourceNotFound when no Source has the given ID.
func (r *SourceRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM sources WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSourceNotFound
	}
	return nil
}
