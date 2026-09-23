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

// LocationRepository stores Locations as PostGIS points.
type LocationRepository struct {
	db *pgxpool.Pool
}

func NewLocationRepository(db *pgxpool.Pool) *LocationRepository {
	return &LocationRepository{db: db}
}

// NearbyLocation is a Location with its spheroidal distance, in metres, from
// the query centre.
type NearbyLocation struct {
	domain.Location
	DistanceM float64
}

// The point is POINT(longitude latitude): ST_Y is latitude, ST_X longitude.
const locationColumns = "id::text, name, ST_Y(point), ST_X(point), precision, created_at, updated_at"

func scanLocation(row pgx.Row, extra ...any) (domain.Location, error) {
	var l domain.Location
	dest := append([]any{&l.ID, &l.Name, &l.Latitude, &l.Longitude, &l.Precision, &l.CreatedAt, &l.UpdatedAt}, extra...)
	return l, row.Scan(dest...)
}

func collectLocations(rows pgx.Rows, err error) ([]domain.Location, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Location, error) {
		return scanLocation(row)
	})
}

// Create inserts l and returns it with its generated ID and timestamps.
func (r *LocationRepository) Create(ctx context.Context, l domain.Location) (domain.Location, error) {
	created, err := scanLocation(r.db.QueryRow(ctx,
		`INSERT INTO locations (name, point, precision)
		 VALUES ($1, ST_SetSRID(ST_MakePoint($2::float8, $3::float8), 4326), $4)
		 RETURNING `+locationColumns,
		l.Name, l.Longitude, l.Latitude, l.Precision)) // ST_MakePoint(x = longitude, y = latitude)
	if err != nil {
		return domain.Location{}, fmt.Errorf("insert location: %w", err)
	}
	return created, nil
}

// GetByID returns domain.ErrLocationNotFound when no Location has the given ID.
func (r *LocationRepository) GetByID(ctx context.Context, id string) (domain.Location, error) {
	l, err := scanLocation(r.db.QueryRow(ctx,
		`SELECT `+locationColumns+` FROM locations WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Location{}, domain.ErrLocationNotFound
	}
	if err != nil {
		return domain.Location{}, fmt.Errorf("get location: %w", err)
	}
	return l, nil
}

// List returns up to limit Locations, newest first, starting after the cursor.
func (r *LocationRepository) List(ctx context.Context, limit int, after *Cursor) ([]domain.Location, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if after == nil {
		rows, err = r.db.Query(ctx,
			`SELECT `+locationColumns+` FROM locations ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT `+locationColumns+` FROM locations
			 WHERE (created_at, id) < ($2::timestamptz, $3::uuid)
			 ORDER BY created_at DESC, id DESC LIMIT $1`,
			limit, after.At, after.ID)
	}
	locations, err := collectLocations(rows, err)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	return locations, nil
}

// FindWithinBoundingBox returns up to limit Locations inside b (edges
// inclusive), newest first.
func (r *LocationRepository) FindWithinBoundingBox(ctx context.Context, b domain.BoundingBox, limit int) ([]domain.Location, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+locationColumns+` FROM locations
		 WHERE point && ST_MakeEnvelope($1::float8, $2::float8, $3::float8, $4::float8, 4326)
		 ORDER BY created_at DESC, id DESC LIMIT $5`,
		b.MinLon, b.MinLat, b.MaxLon, b.MaxLat, limit) // envelope is (xmin, ymin, xmax, ymax)
	locations, err := collectLocations(rows, err)
	if err != nil {
		return nil, fmt.Errorf("find locations in bounding box: %w", err)
	}
	return locations, nil
}

// FindNearby returns up to limit Locations within radiusM metres (inclusive)
// of the centre, nearest first, measured on the WGS84 spheroid by PostGIS.
func (r *LocationRepository) FindNearby(ctx context.Context, lat, lon, radiusM float64, limit int) ([]NearbyLocation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+locationColumns+`, ST_Distance(point::geography, c.g) AS distance_m
		 FROM locations,
		      (SELECT ST_SetSRID(ST_MakePoint($1::float8, $2::float8), 4326)::geography AS g) c
		 WHERE ST_DWithin(point::geography, c.g, $3::float8)
		 ORDER BY distance_m, id LIMIT $4`,
		lon, lat, radiusM, limit)
	if err != nil {
		return nil, fmt.Errorf("find nearby locations: %w", err)
	}
	nearby, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (NearbyLocation, error) {
		var n NearbyLocation
		l, err := scanLocation(row, &n.DistanceM)
		n.Location = l
		return n, err
	})
	if err != nil {
		return nil, fmt.Errorf("find nearby locations: %w", err)
	}
	return nearby, nil
}

// Delete returns domain.ErrLocationNotFound when no Location has the given ID
// and domain.ErrLocationInUse while an Event still references it.
func (r *LocationRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM locations WHERE id = $1::uuid`, id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "event_locations_location_id_fkey" {
		return domain.ErrLocationInUse
	}
	if err != nil {
		return fmt.Errorf("delete location: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLocationNotFound
	}
	return nil
}
