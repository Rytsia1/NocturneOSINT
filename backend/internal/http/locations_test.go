package http

import (
	"fmt"
	"math"
	"net/http"
	"testing"
)

// Requests rejected before any database access run without a database.
func TestLocationRequestValidation(t *testing.T) {
	router := NewRouter(discardLogger, nil)
	bbox := "/api/locations?min_lat=35&min_lon=139&max_lat=36&max_lon=140"

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"malformed JSON", "POST", "/api/locations", `{"name":`, 400, "invalid_request"},
		{"latitude not a number", "POST", "/api/locations", `{"name":"L","latitude":"35","longitude":139,"precision":"city"}`, 400, "invalid_request"},
		{"unknown field", "POST", "/api/locations", `{"name":"L","latitude":35,"longitude":139,"precision":"city","geom":"x"}`, 400, "invalid_request"},
		{"missing latitude (no Null Island default)", "POST", "/api/locations", `{"name":"L","longitude":139,"precision":"city"}`, 400, "validation_failed"},
		{"latitude out of range", "POST", "/api/locations", `{"name":"L","latitude":91,"longitude":139,"precision":"city"}`, 400, "validation_failed"},
		{"longitude out of range", "POST", "/api/locations", `{"name":"L","latitude":35,"longitude":-181,"precision":"city"}`, 400, "validation_failed"},
		{"missing name", "POST", "/api/locations", `{"latitude":35,"longitude":139,"precision":"city"}`, 400, "validation_failed"},
		{"bad precision", "POST", "/api/locations", `{"name":"L","latitude":35,"longitude":139,"precision":"exact"}`, 400, "validation_failed"},
		{"get invalid id", "GET", "/api/locations/nope", "", 400, "invalid_id"},
		{"delete invalid id", "DELETE", "/api/locations/123", "", 400, "invalid_id"},
		{"bbox incomplete", "GET", "/api/locations?min_lat=35&max_lat=36", "", 400, "validation_failed"},
		{"bbox not a number", "GET", "/api/locations?min_lat=a&min_lon=139&max_lat=36&max_lon=140", "", 400, "validation_failed"},
		{"bbox NaN", "GET", "/api/locations?min_lat=NaN&min_lon=139&max_lat=36&max_lon=140", "", 400, "validation_failed"},
		{"bbox latitude out of range", "GET", "/api/locations?min_lat=-95&min_lon=139&max_lat=36&max_lon=140", "", 400, "validation_failed"},
		{"bbox min_lat > max_lat", "GET", "/api/locations?min_lat=37&min_lon=139&max_lat=36&max_lon=140", "", 400, "validation_failed"},
		{"bbox crosses antimeridian", "GET", "/api/locations?min_lat=-10&min_lon=170&max_lat=10&max_lon=-170", "", 400, "validation_failed"},
		{"bbox with cursor", "GET", bbox + "&cursor=abc", "", 400, "invalid_cursor"},
		{"bbox limit too big", "GET", bbox + "&limit=1000", "", 400, "invalid_limit"},
		{"nearby missing lat", "GET", "/api/locations/nearby?lon=139&radius_m=100", "", 400, "validation_failed"},
		{"nearby longitude out of range", "GET", "/api/locations/nearby?lat=35&lon=200&radius_m=100", "", 400, "validation_failed"},
		{"nearby zero radius", "GET", "/api/locations/nearby?lat=35&lon=139&radius_m=0", "", 400, "validation_failed"},
		{"nearby absurd radius", "GET", "/api/locations/nearby?lat=35&lon=139&radius_m=999999999", "", 400, "validation_failed"},
		{"nearby infinite radius", "GET", "/api/locations/nearby?lat=35&lon=139&radius_m=Inf", "", 400, "validation_failed"},
		{"nearby radius not a number", "GET", "/api/locations/nearby?lat=35&lon=139&radius_m=far", "", 400, "validation_failed"},
		{"nearby limit invalid", "GET", "/api/locations/nearby?lat=35&lon=139&radius_m=100&limit=0", "", 400, "invalid_limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, do(router, tt.method, tt.path, tt.body), tt.status, tt.code)
		})
	}
}

// Integration tests: full HTTP → repository → PostGIS round trips.

type locationBody struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Precision string   `json:"precision"`
	DistanceM *float64 `json:"distance_m"`
}

type spatialBody struct {
	Items     []locationBody `json:"items"`
	Truncated bool           `json:"truncated"`
}

func createLocation(t *testing.T, router http.Handler, name string, lat, lon float64) locationBody {
	t.Helper()
	rec := do(router, "POST", "/api/locations",
		fmt.Sprintf(`{"name":%q,"latitude":%v,"longitude":%v,"precision":"specific"}`, name, lat, lon))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/locations status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	l := decode[locationBody](t, rec)
	t.Cleanup(func() { do(router, "DELETE", "/api/locations/"+l.ID, "") })
	return l
}

func findItem(items []locationBody, id string) (locationBody, bool) {
	for _, l := range items {
		if l.ID == id {
			return l, true
		}
	}
	return locationBody{}, false
}

func TestIntegration_LocationLifecycle(t *testing.T) {
	router := integrationRouter(t)
	tokyo := createLocation(t, router, "Tokyo Station", 35.6812, 139.7671)
	shinjuku := createLocation(t, router, "Shinjuku Station", 35.6895, 139.6917)

	if tokyo.Latitude != 35.6812 || tokyo.Longitude != 139.7671 || tokyo.Precision != "specific" || tokyo.DistanceM != nil {
		t.Errorf("created = %+v", tokyo)
	}

	// Get
	rec := do(router, "GET", "/api/locations/"+tokyo.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}
	if got := decode[locationBody](t, rec); got.Latitude != 35.6812 || got.Longitude != 139.7671 {
		t.Errorf("GET coordinates = %v/%v, want 35.6812/139.7671", got.Latitude, got.Longitude)
	}

	// Paginated list
	rec = do(router, "GET", "/api/locations?limit=5", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("LIST status = %d, want 200", rec.Code)
	}
	if _, ok := findItem(decode[spatialBody](t, rec).Items, tokyo.ID); !ok {
		t.Error("created location missing from first page of GET /api/locations")
	}

	// Bounding box east of Shinjuku: Tokyo Station only.
	rec = do(router, "GET", "/api/locations?min_lat=35.6&min_lon=139.75&max_lat=35.7&max_lon=139.8", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("BBOX status = %d (body %s)", rec.Code, rec.Body.String())
	}
	box := decode[spatialBody](t, rec)
	if _, ok := findItem(box.Items, tokyo.ID); !ok {
		t.Error("Tokyo Station missing from its bounding box")
	}
	if _, ok := findItem(box.Items, shinjuku.ID); ok {
		t.Error("Shinjuku returned from a box east of it")
	}

	// Nearby within 10 km of Tokyo Station, with distances in metres.
	rec = do(router, "GET", "/api/locations/nearby?lat=35.6812&lon=139.7671&radius_m=10000", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("NEARBY status = %d (body %s)", rec.Code, rec.Body.String())
	}
	near := decode[spatialBody](t, rec)
	self, ok := findItem(near.Items, tokyo.ID)
	if !ok || self.DistanceM == nil || *self.DistanceM > 0.01 {
		t.Errorf("Tokyo Station in nearby = %+v, want distance_m ~0", self)
	}
	other, ok := findItem(near.Items, shinjuku.ID)
	if !ok || other.DistanceM == nil || math.Abs(*other.DistanceM-6900) > 400 {
		t.Errorf("Shinjuku in nearby = %+v, want distance_m ~6900", other)
	}

	// 1 km excludes Shinjuku.
	rec = do(router, "GET", "/api/locations/nearby?lat=35.6812&lon=139.7671&radius_m=1000", "")
	if _, ok := findItem(decode[spatialBody](t, rec).Items, shinjuku.ID); ok {
		t.Error("Shinjuku returned within 1 km of Tokyo Station")
	}

	// Delete
	if rec = do(router, "DELETE", "/api/locations/"+tokyo.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", rec.Code)
	}
	assertError(t, do(router, "GET", "/api/locations/"+tokyo.ID, ""), http.StatusNotFound, "location_not_found")
	assertError(t, do(router, "DELETE", "/api/locations/"+tokyo.ID, ""), http.StatusNotFound, "location_not_found")
}

func TestIntegration_SpatialResultsAreCapped(t *testing.T) {
	router := integrationRouter(t)
	for i := range 3 {
		createLocation(t, router, fmt.Sprintf("Cap %d", i), -33.8568+float64(i)*0.0001, 151.2153)
	}

	rec := do(router, "GET", "/api/locations?min_lat=-33.9&min_lon=151.2&max_lat=-33.8&max_lon=151.3&limit=2", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("BBOX status = %d (body %s)", rec.Code, rec.Body.String())
	}
	body := decode[spatialBody](t, rec)
	if len(body.Items) != 2 || !body.Truncated {
		t.Errorf("bbox with limit=2 over 3 points: %d items, truncated=%v; want 2, true", len(body.Items), body.Truncated)
	}

	rec = do(router, "GET", "/api/locations/nearby?lat=-33.8568&lon=151.2153&radius_m=500&limit=2", "")
	body = decode[spatialBody](t, rec)
	if len(body.Items) != 2 || !body.Truncated {
		t.Errorf("nearby with limit=2 over 3 points: %d items, truncated=%v; want 2, true", len(body.Items), body.Truncated)
	}
}
