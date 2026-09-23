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

// EventRepository stores Events and their Location references.
type EventRepository struct {
	db *pgxpool.Pool
}

func NewEventRepository(db *pgxpool.Pool) *EventRepository {
	return &EventRepository{db: db}
}

const eventColumns = "id::text, title, description, occurred_at, COALESCE(occurred_at_precision, ''), created_at, updated_at"

func scanEvent(row pgx.Row) (domain.Event, error) {
	var e domain.Event
	err := row.Scan(&e.ID, &e.Title, &e.Description, &e.OccurredAt, &e.OccurredAtPrecision, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

// Create inserts e and returns it with its generated ID and timestamps.
func (r *EventRepository) Create(ctx context.Context, e domain.Event) (domain.Event, error) {
	created, err := scanEvent(r.db.QueryRow(ctx,
		`INSERT INTO events (title, description, occurred_at, occurred_at_precision)
		 VALUES ($1, $2, $3, NULLIF($4, '')) RETURNING `+eventColumns,
		e.Title, e.Description, e.OccurredAt, e.OccurredAtPrecision))
	if err != nil {
		return domain.Event{}, fmt.Errorf("insert event: %w", err)
	}
	return created, nil
}

// GetByID returns domain.ErrEventNotFound when no Event has the given ID.
func (r *EventRepository) GetByID(ctx context.Context, id string) (domain.Event, error) {
	e, err := scanEvent(r.db.QueryRow(ctx, `SELECT `+eventColumns+` FROM events WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, domain.ErrEventNotFound
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("get event: %w", err)
	}
	return e, nil
}

// List returns up to limit Events in feed order (see domain.Event.FeedTime),
// most recent first, starting after the cursor.
func (r *EventRepository) List(ctx context.Context, limit int, after *Cursor) ([]domain.Event, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if after == nil {
		rows, err = r.db.Query(ctx,
			`SELECT `+eventColumns+` FROM events ORDER BY feed_at DESC, id DESC LIMIT $1`, limit)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT `+eventColumns+` FROM events
			 WHERE (feed_at, id) < ($2::timestamptz, $3::uuid)
			 ORDER BY feed_at DESC, id DESC LIMIT $1`,
			limit, after.At, after.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Event, error) {
		return scanEvent(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return events, nil
}

// Delete removes the Event; its Location references go with it (CASCADE).
func (r *EventRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM events WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEventNotFound
	}
	return nil
}

// AddLocation attaches an existing Location to an existing Event. The foreign
// keys reject unknown IDs atomically; the primary key rejects duplicates.
func (r *EventRepository) AddLocation(ctx context.Context, eventID, locationID, role string) (domain.EventLocation, error) {
	var el domain.EventLocation
	l, err := scanLocation(r.db.QueryRow(ctx,
		`WITH ins AS (
		     INSERT INTO event_locations (event_id, location_id, role)
		     VALUES ($1::uuid, $2::uuid, $3) RETURNING location_id, role
		 )
		 SELECT `+locationColumns+`, ins.role FROM locations JOIN ins ON ins.location_id = locations.id`,
		eventID, locationID, role), &el.Role)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "event_locations_event_id_fkey":
			return domain.EventLocation{}, domain.ErrEventNotFound
		case "event_locations_location_id_fkey":
			return domain.EventLocation{}, domain.ErrLocationNotFound
		case "event_locations_pkey":
			return domain.EventLocation{}, domain.ErrEventLocationExists
		}
	}
	if err != nil {
		return domain.EventLocation{}, fmt.Errorf("attach location: %w", err)
	}
	el.Location = l
	return el, nil
}

// Locations returns up to limit of the Event's Locations, sites first.
// It returns domain.ErrEventNotFound when the Event does not exist.
func (r *EventRepository) Locations(ctx context.Context, eventID string, limit int) ([]domain.EventLocation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+locationColumns+`, el.role
		 FROM event_locations el JOIN locations ON locations.id = el.location_id
		 WHERE el.event_id = $1::uuid
		 ORDER BY el.role DESC, locations.id LIMIT $2`, // 'site' sorts before 'related' descending
		eventID, limit)
	if err != nil {
		return nil, fmt.Errorf("list event locations: %w", err)
	}
	refs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.EventLocation, error) {
		var el domain.EventLocation
		l, err := scanLocation(row, &el.Role)
		el.Location = l
		return el, err
	})
	if err != nil {
		return nil, fmt.Errorf("list event locations: %w", err)
	}

	if len(refs) == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM events WHERE id = $1::uuid)`, eventID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check event: %w", err)
		}
		if !exists {
			return nil, domain.ErrEventNotFound
		}
	}
	return refs, nil
}

// RemoveLocation detaches a Location from an Event; the Location itself stays.
func (r *EventRepository) RemoveLocation(ctx context.Context, eventID, locationID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM event_locations WHERE event_id = $1::uuid AND location_id = $2::uuid`, eventID, locationID)
	if err != nil {
		return fmt.Errorf("detach location: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEventLocationNotFound
	}
	return nil
}
