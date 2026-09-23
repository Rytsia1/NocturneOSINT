package repository

import (
	"context"
	"errors"
	"math"
	"testing"

	"nocturne-backend/internal/domain"
)

// Integration tests against real PostgreSQL/PostGIS; skipped without DATABASE_URL.

// Known points: Tokyo Station, Shinjuku Station (~6.9 km west), Jakarta.
var (
	tokyo    = [2]float64{35.6812, 139.7671}
	shinjuku = [2]float64{35.6895, 139.6917}
	jakarta  = [2]float64{-6.2088, 106.8456}
)

func newTestLocationRepo(t *testing.T) *LocationRepository {
	t.Helper()
	return NewLocationRepository(newTestRepo(t).db)
}

func createTestLocation(t *testing.T, repo *LocationRepository, name string, latLon [2]float64) domain.Location {
	t.Helper()
	l, err := domain.NewLocation(name, latLon[0], latLon[1], "specific")
	if err != nil {
		t.Fatalf("NewLocation: %v", err)
	}
	created, err := repo.Create(context.Background(), l)
	if err != nil {
		t.Fatalf("Create location: %v", err)
	}
	t.Cleanup(func() { repo.Delete(context.Background(), created.ID) })
	return created
}

func ids(locations []domain.Location) map[string]bool {
	m := map[string]bool{}
	for _, l := range locations {
		m[l.ID] = true
	}
	return m
}

// TestIntegration_LocationPointIsLongitudeLatitude reads the stored geometry
// directly, so a swap in both the insert and the read cannot cancel out.
func TestIntegration_LocationPointIsLongitudeLatitude(t *testing.T) {
	repo := newTestLocationRepo(t)
	created := createTestLocation(t, repo, "Tokyo Station", tokyo)

	var x, y float64
	var srid int
	var wkt, geomType string
	err := repo.db.QueryRow(context.Background(),
		`SELECT ST_X(point), ST_Y(point), ST_SRID(point), ST_AsText(point), GeometryType(point)
		 FROM locations WHERE id = $1::uuid`, created.ID).Scan(&x, &y, &srid, &wkt, &geomType)
	if err != nil {
		t.Fatalf("read geometry: %v", err)
	}
	if x != tokyo[1] || y != tokyo[0] {
		t.Errorf("stored X/Y = %v/%v, want longitude %v / latitude %v", x, y, tokyo[1], tokyo[0])
	}
	if wkt != "POINT(139.7671 35.6812)" {
		t.Errorf("WKT = %q, want POINT(139.7671 35.6812)", wkt)
	}
	if srid != 4326 || geomType != "POINT" {
		t.Errorf("SRID/type = %d/%s, want 4326/POINT", srid, geomType)
	}

	got, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Latitude != tokyo[0] || got.Longitude != tokyo[1] || got.Precision != "specific" {
		t.Errorf("round trip = %+v, want lat %v lon %v", got, tokyo[0], tokyo[1])
	}
}

func TestIntegration_LocationColumnRejectsOtherSRID(t *testing.T) {
	repo := newTestLocationRepo(t)
	_, err := repo.db.Exec(context.Background(),
		`INSERT INTO locations (name, point, precision) VALUES ('bad', ST_SetSRID(ST_MakePoint(0, 0), 3857), 'city')`)
	if err == nil {
		t.Fatal("insert with SRID 3857 succeeded; the column must only accept SRID 4326")
	}
}

func TestIntegration_LocationNotFoundAndDelete(t *testing.T) {
	repo := newTestLocationRepo(t)
	ctx := context.Background()
	missing := "00000000-0000-4000-8000-000000000000"

	if _, err := repo.GetByID(ctx, missing); !errors.Is(err, domain.ErrLocationNotFound) {
		t.Errorf("GetByID err = %v, want ErrLocationNotFound", err)
	}
	if err := repo.Delete(ctx, missing); !errors.Is(err, domain.ErrLocationNotFound) {
		t.Errorf("Delete err = %v, want ErrLocationNotFound", err)
	}

	created := createTestLocation(t, repo, "To delete", jakarta)
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, domain.ErrLocationNotFound) {
		t.Errorf("GetByID after delete err = %v, want ErrLocationNotFound", err)
	}
}

func TestIntegration_LocationList(t *testing.T) {
	repo := newTestLocationRepo(t)
	older := createTestLocation(t, repo, "Older", jakarta)
	newer := createTestLocation(t, repo, "Newer", tokyo)

	page, err := repo.List(context.Background(), 100, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	pos := map[string]int{}
	for i, l := range page {
		pos[l.ID] = i
	}
	if _, ok := pos[older.ID]; !ok {
		t.Fatal("created location missing from list")
	}
	if pos[newer.ID] > pos[older.ID] {
		t.Error("list is not newest first")
	}
}

func TestIntegration_LocationBoundingBox(t *testing.T) {
	repo := newTestLocationRepo(t)
	ctx := context.Background()
	a := createTestLocation(t, repo, "A Tokyo", tokyo)
	b := createTestLocation(t, repo, "B Shinjuku", shinjuku)
	c := createTestLocation(t, repo, "C Jakarta", jakarta)

	find := func(minLat, minLon, maxLat, maxLon float64) map[string]bool {
		t.Helper()
		box, err := domain.NewBoundingBox(minLat, minLon, maxLat, maxLon)
		if err != nil {
			t.Fatalf("NewBoundingBox: %v", err)
		}
		found, err := repo.FindWithinBoundingBox(ctx, box, 100)
		if err != nil {
			t.Fatalf("FindWithinBoundingBox: %v", err)
		}
		return ids(found)
	}

	// Box around Tokyo Station only (east of Shinjuku).
	got := find(35.6, 139.75, 35.7, 139.8)
	if !got[a.ID] || got[b.ID] || got[c.ID] {
		t.Errorf("Tokyo box: A=%v B=%v C=%v, want only A", got[a.ID], got[b.ID], got[c.ID])
	}

	// Greater Tokyo: both Tokyo points, not Jakarta.
	got = find(35, 139, 36, 140)
	if !got[a.ID] || !got[b.ID] || got[c.ID] {
		t.Errorf("greater Tokyo box: A=%v B=%v C=%v, want A and B", got[a.ID], got[b.ID], got[c.ID])
	}

	// Boundary is inclusive: a box whose west edge is exactly A's longitude
	// and whose south edge is exactly A's latitude contains A.
	got = find(tokyo[0], tokyo[1], 36, 140)
	if !got[a.ID] {
		t.Error("point on the box boundary was not returned (edges are inclusive)")
	}
	// A box ending just west of A does not contain it.
	got = find(35, 139, 36, tokyo[1]-0.0001)
	if got[a.ID] {
		t.Error("point just outside the box was returned")
	}
}

func TestIntegration_LocationNearby(t *testing.T) {
	repo := newTestLocationRepo(t)
	ctx := context.Background()
	a := createTestLocation(t, repo, "A Tokyo", tokyo)
	b := createTestLocation(t, repo, "B Shinjuku", shinjuku)
	c := createTestLocation(t, repo, "C Jakarta", jakarta)

	near := func(radiusM float64) map[string]NearbyLocation {
		t.Helper()
		found, err := repo.FindNearby(ctx, tokyo[0], tokyo[1], radiusM, 100)
		if err != nil {
			t.Fatalf("FindNearby: %v", err)
		}
		m := map[string]NearbyLocation{}
		for i, n := range found {
			if i > 0 && found[i-1].DistanceM > n.DistanceM {
				t.Fatalf("results not ordered by distance: %v then %v", found[i-1].DistanceM, n.DistanceM)
			}
			m[n.ID] = n
		}
		return m
	}

	// 1 km: only the point at the centre.
	got := near(1000)
	if _, ok := got[a.ID]; !ok {
		t.Fatal("point at the centre not returned")
	}
	if got[a.ID].DistanceM > 0.01 {
		t.Errorf("distance to itself = %v m, want ~0", got[a.ID].DistanceM)
	}
	if _, ok := got[b.ID]; ok {
		t.Error("Shinjuku (~6.9 km) returned within 1 km")
	}

	// 10 km: Shinjuku too, at a sensible distance; Jakarta never.
	got = near(10_000)
	nb, ok := got[b.ID]
	if !ok {
		t.Fatal("Shinjuku not returned within 10 km")
	}
	if nb.DistanceM < 6500 || nb.DistanceM > 7300 {
		t.Errorf("Tokyo→Shinjuku distance = %.1f m, want ~6.9 km", nb.DistanceM)
	}
	if nb.Latitude != shinjuku[0] || nb.Longitude != shinjuku[1] {
		t.Errorf("nearby result coordinates = %v/%v, want %v/%v", nb.Latitude, nb.Longitude, shinjuku[0], shinjuku[1])
	}
	if _, ok := got[c.ID]; ok {
		t.Error("Jakarta returned within 10 km of Tokyo")
	}

	// Radius boundary: 1–2 m either side of the measured distance.
	d := math.Floor(nb.DistanceM)
	if _, ok := near(d + 2)[b.ID]; !ok {
		t.Errorf("Shinjuku excluded at radius %.0f m (distance %.1f m)", d+2, nb.DistanceM)
	}
	if _, ok := near(d - 1)[b.ID]; ok {
		t.Errorf("Shinjuku included at radius %.0f m (distance %.1f m)", d-1, nb.DistanceM)
	}
}
