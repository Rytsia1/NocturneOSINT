package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"nocturne-backend/internal/domain"
)

// Integration tests against real PostgreSQL/PostGIS; skipped without DATABASE_URL.
// Locations are created before Events so that, with LIFO cleanups, each Event
// (and its CASCADEd associations) is deleted before the Locations it uses.

func newTestEventRepos(t *testing.T) (*EventRepository, *LocationRepository) {
	t.Helper()
	db := newTestRepo(t).db
	return NewEventRepository(db), NewLocationRepository(db)
}

func createTestEvent(t *testing.T, repo *EventRepository, title string, at *time.Time, precision string) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(title, "test event", at, precision)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	created, err := repo.Create(context.Background(), e)
	if err != nil {
		t.Fatalf("Create event: %v", err)
	}
	t.Cleanup(func() { repo.Delete(context.Background(), created.ID) })
	return created
}

func TestIntegration_EventCreateAndGet(t *testing.T) {
	events, _ := newTestEventRepos(t)
	ctx := context.Background()

	undated := createTestEvent(t, events, "Undated", nil, "")
	day := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	dated := createTestEvent(t, events, "Dated", &day, "day")

	got, err := events.GetByID(ctx, undated.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.OccurredAt != nil || got.OccurredAtPrecision != "" {
		t.Errorf("undated event time = %v/%q, want unknown (not invented)", got.OccurredAt, got.OccurredAtPrecision)
	}

	got, err = events.GetByID(ctx, dated.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.OccurredAt == nil || !got.OccurredAt.Equal(day) || got.OccurredAtPrecision != "day" {
		t.Errorf("dated event time = %v/%q, want %v/day", got.OccurredAt, got.OccurredAtPrecision, day)
	}
}

func TestIntegration_EventNotFound(t *testing.T) {
	events, _ := newTestEventRepos(t)
	missing := "00000000-0000-4000-8000-000000000000"

	if _, err := events.GetByID(context.Background(), missing); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("GetByID err = %v, want ErrEventNotFound", err)
	}
	if err := events.Delete(context.Background(), missing); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("Delete err = %v, want ErrEventNotFound", err)
	}
	if _, err := events.Locations(context.Background(), missing, 10); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("Locations err = %v, want ErrEventNotFound", err)
	}
}

func TestIntegration_EventListFeedOrder(t *testing.T) {
	events, _ := newTestEventRepos(t)
	older := createTestEvent(t, events, "Older", timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)), "year")
	newer := createTestEvent(t, events, "Newer", timePtr(time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)), "exact")
	undated := createTestEvent(t, events, "Undated", nil, "") // sorts by creation time (now)

	var order []string
	var cursor *Cursor
	for {
		page, err := events.List(context.Background(), 2, cursor)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, e := range page {
			order = append(order, e.ID)
		}
		if len(page) < 2 {
			break
		}
		last := page[len(page)-1]
		cursor = &Cursor{At: last.FeedTime(), ID: last.ID}
	}

	pos := map[string]int{}
	for i, id := range order {
		if _, dup := pos[id]; dup {
			t.Fatalf("event %s returned twice across pages", id)
		}
		pos[id] = i
	}
	if !(pos[undated.ID] < pos[newer.ID] && pos[newer.ID] < pos[older.ID]) {
		t.Errorf("feed order: undated=%d newer=%d older=%d, want undated < newer < older", pos[undated.ID], pos[newer.ID], pos[older.ID])
	}
}

func TestIntegration_EventLocations(t *testing.T) {
	events, locations := newTestEventRepos(t)
	ctx := context.Background()
	stationLoc := createTestLocation(t, locations, "Tokyo Station", tokyo)
	shinjukuLoc := createTestLocation(t, locations, "Shinjuku Station", shinjuku)
	event := createTestEvent(t, events, "Rail disruption", nil, "")
	missing := "00000000-0000-4000-8000-000000000000"

	// No references yet: empty, not an error.
	if refs, err := events.Locations(ctx, event.ID, 10); err != nil || len(refs) != 0 {
		t.Fatalf("Locations of new event = %v, %v; want empty", refs, err)
	}

	// Attach two Locations with different roles.
	related, err := events.AddLocation(ctx, event.ID, shinjukuLoc.ID, "related")
	if err != nil {
		t.Fatalf("AddLocation related: %v", err)
	}
	if related.Role != "related" || related.Location.ID != shinjukuLoc.ID || related.Location.Latitude != shinjuku[0] {
		t.Errorf("AddLocation returned %+v", related)
	}
	if _, err := events.AddLocation(ctx, event.ID, stationLoc.ID, "site"); err != nil {
		t.Fatalf("AddLocation site: %v", err)
	}

	refs, err := events.Locations(ctx, event.ID, 10)
	if err != nil {
		t.Fatalf("Locations: %v", err)
	}
	if len(refs) != 2 || refs[0].Role != "site" || refs[0].Location.ID != stationLoc.ID || refs[1].Location.ID != shinjukuLoc.ID {
		t.Errorf("Locations = %+v, want site (Tokyo Station) then related (Shinjuku)", refs)
	}

	// Rejections: unknown Event, unknown Location, duplicates (any role).
	if _, err := events.AddLocation(ctx, missing, stationLoc.ID, "site"); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("AddLocation unknown event err = %v, want ErrEventNotFound", err)
	}
	if _, err := events.AddLocation(ctx, event.ID, missing, "site"); !errors.Is(err, domain.ErrLocationNotFound) {
		t.Errorf("AddLocation unknown location err = %v, want ErrLocationNotFound", err)
	}
	for _, role := range []string{"site", "related"} {
		if _, err := events.AddLocation(ctx, event.ID, stationLoc.ID, role); !errors.Is(err, domain.ErrEventLocationExists) {
			t.Errorf("AddLocation duplicate (%s) err = %v, want ErrEventLocationExists", role, err)
		}
	}

	// Detach.
	if err := events.RemoveLocation(ctx, event.ID, shinjukuLoc.ID); err != nil {
		t.Fatalf("RemoveLocation: %v", err)
	}
	if err := events.RemoveLocation(ctx, event.ID, shinjukuLoc.ID); !errors.Is(err, domain.ErrEventLocationNotFound) {
		t.Errorf("RemoveLocation twice err = %v, want ErrEventLocationNotFound", err)
	}
	if _, err := locations.GetByID(ctx, shinjukuLoc.ID); err != nil {
		t.Errorf("detached Location was deleted: %v", err)
	}
}

func TestIntegration_EventLocationReferentialIntegrity(t *testing.T) {
	events, locations := newTestEventRepos(t)
	ctx := context.Background()
	loc := createTestLocation(t, locations, "Port", jakarta)
	event := createTestEvent(t, events, "Port fire", nil, "")
	if _, err := events.AddLocation(ctx, event.ID, loc.ID, "site"); err != nil {
		t.Fatalf("AddLocation: %v", err)
	}

	// A referenced Location cannot be deleted.
	if err := locations.Delete(ctx, loc.ID); !errors.Is(err, domain.ErrLocationInUse) {
		t.Fatalf("Delete referenced location err = %v, want ErrLocationInUse", err)
	}

	// Deleting the Event removes its associations but keeps the Location.
	if err := events.Delete(ctx, event.ID); err != nil {
		t.Fatalf("Delete event: %v", err)
	}
	var n int
	if err := events.db.QueryRow(ctx, `SELECT count(*) FROM event_locations WHERE event_id = $1::uuid`, event.ID).Scan(&n); err != nil {
		t.Fatalf("count associations: %v", err)
	}
	if n != 0 {
		t.Errorf("orphaned associations after event delete = %d, want 0", n)
	}
	if _, err := locations.GetByID(ctx, loc.ID); err != nil {
		t.Fatalf("Location removed with its Event: %v", err)
	}

	// Now unreferenced, the Location can be deleted.
	if err := locations.Delete(ctx, loc.ID); err != nil {
		t.Errorf("Delete unreferenced location: %v", err)
	}
}

func TestIntegration_EventLocationsStoreNoCoordinates(t *testing.T) {
	events, _ := newTestEventRepos(t)
	var cols []string
	rows, err := events.db.Query(context.Background(),
		`SELECT column_name FROM information_schema.columns WHERE table_name = 'event_locations' ORDER BY ordinal_position`)
	if err != nil {
		t.Fatalf("query columns: %v", err)
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols = append(cols, c)
	}
	if got := fmt.Sprint(cols); got != "[event_id location_id role]" {
		t.Errorf("event_locations columns = %s; coordinates must live only in locations", got)
	}
}
