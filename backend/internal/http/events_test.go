package http

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Requests rejected before any database access run without a database.
func TestEventRequestValidation(t *testing.T) {
	router := NewRouter(discardLogger, nil)
	ev := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"malformed JSON", "POST", "/api/events", `{"title":`, 400, "invalid_request"},
		{"unknown field", "POST", "/api/events", `{"title":"T","event_type":"incident"}`, 400, "invalid_request"},
		{"occurred_at without timezone", "POST", "/api/events", `{"title":"T","occurred_at":"2026-09-18T10:00:00","occurred_at_precision":"exact"}`, 400, "invalid_request"},
		{"missing title", "POST", "/api/events", `{"description":"d"}`, 400, "validation_failed"},
		{"precision without time", "POST", "/api/events", `{"title":"T","occurred_at_precision":"day"}`, 400, "validation_failed"},
		{"time without precision", "POST", "/api/events", `{"title":"T","occurred_at":"2026-09-18T10:00:00Z"}`, 400, "validation_failed"},
		{"day precision with a time of day", "POST", "/api/events", `{"title":"T","occurred_at":"2026-09-18T10:00:00Z","occurred_at_precision":"day"}`, 400, "validation_failed"},
		{"get invalid id", "GET", "/api/events/nope", "", 400, "invalid_id"},
		{"delete invalid id", "DELETE", "/api/events/123", "", 400, "invalid_id"},
		{"list invalid limit", "GET", "/api/events?limit=0", "", 400, "invalid_limit"},
		{"locations: invalid event id", "GET", "/api/events/nope/locations", "", 400, "invalid_id"},
		{"attach: invalid event id", "POST", "/api/events/nope/locations", `{"location_id":"` + ev + `","role":"site"}`, 400, "invalid_id"},
		{"attach: malformed JSON", "POST", "/api/events/" + ev + "/locations", `{"location_id":`, 400, "invalid_request"},
		{"attach: invalid location id", "POST", "/api/events/" + ev + "/locations", `{"location_id":"x","role":"site"}`, 400, "validation_failed"},
		{"attach: missing role", "POST", "/api/events/" + ev + "/locations", `{"location_id":"` + ev + `"}`, 400, "validation_failed"},
		{"attach: unknown role", "POST", "/api/events/" + ev + "/locations", `{"location_id":"` + ev + `","role":"origin"}`, 400, "validation_failed"},
		{"detach: invalid location id", "DELETE", "/api/events/" + ev + "/locations/x", "", 400, "invalid_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, do(router, tt.method, tt.path, tt.body), tt.status, tt.code)
		})
	}
}

// Integration tests: full HTTP → repository → PostgreSQL round trips.

type eventBody struct {
	ID                  string     `json:"id"`
	Title               string     `json:"title"`
	OccurredAt          *time.Time `json:"occurred_at"`
	OccurredAtPrecision *string    `json:"occurred_at_precision"`
	Locations           []struct {
		LocationID string `json:"location_id"`
		Role       string `json:"role"`
	} `json:"locations"`
}

type eventLocationBody struct {
	Role     string       `json:"role"`
	Location locationBody `json:"location"`
}

// createEvent posts an Event and deletes it when the test ends. Create
// Locations first: LIFO cleanups then delete the Event (and its associations)
// before the Locations.
func createEvent(t *testing.T, router http.Handler, body string) eventBody {
	t.Helper()
	rec := do(router, "POST", "/api/events", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/events status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	e := decode[eventBody](t, rec)
	t.Cleanup(func() { do(router, "DELETE", "/api/events/"+e.ID, "") })
	return e
}

func TestIntegration_EventLifecycle(t *testing.T) {
	router := integrationRouter(t)
	station := createLocation(t, router, "Tokyo Station", 35.6812, 139.7671)
	shinjuku := createLocation(t, router, "Shinjuku Station", 35.6895, 139.6917)
	missing := "00000000-0000-4000-8000-000000000000"

	// Create: a date known only to the day stays a day.
	event := createEvent(t, router, `{"title":"Rail disruption","description":"Reported delays.","occurred_at":"2026-09-18T00:00:00Z","occurred_at_precision":"day"}`)
	if event.OccurredAtPrecision == nil || *event.OccurredAtPrecision != "day" || event.Locations == nil || len(event.Locations) != 0 {
		t.Errorf("created = %+v, want day precision and empty locations", event)
	}
	undated := createEvent(t, router, `{"title":"Undated report"}`)
	if undated.OccurredAt != nil || undated.OccurredAtPrecision != nil {
		t.Errorf("undated event time = %v/%v, want null/null", undated.OccurredAt, undated.OccurredAtPrecision)
	}

	// Attach: site and related.
	loc := "/api/events/" + event.ID + "/locations"
	rec := do(router, "POST", loc, fmt.Sprintf(`{"location_id":%q,"role":"site"}`, station.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("attach site status = %d (body %s)", rec.Code, rec.Body.String())
	}
	if got := decode[eventLocationBody](t, rec); got.Role != "site" || got.Location.ID != station.ID || got.Location.Latitude != 35.6812 {
		t.Errorf("attach response = %+v", got)
	}
	if rec = do(router, "POST", loc, fmt.Sprintf(`{"location_id":%q,"role":"related"}`, shinjuku.ID)); rec.Code != http.StatusCreated {
		t.Fatalf("attach related status = %d (body %s)", rec.Code, rec.Body.String())
	}

	// Attach failures.
	assertError(t, do(router, "POST", loc, fmt.Sprintf(`{"location_id":%q,"role":"related"}`, station.ID)), http.StatusConflict, "event_location_exists")
	assertError(t, do(router, "POST", loc, fmt.Sprintf(`{"location_id":%q,"role":"site"}`, missing)), http.StatusUnprocessableEntity, "location_not_found")
	assertError(t, do(router, "POST", "/api/events/"+missing+"/locations", fmt.Sprintf(`{"location_id":%q,"role":"site"}`, station.ID)), http.StatusNotFound, "event_not_found")

	// Detail: lightweight references, site first.
	rec = do(router, "GET", "/api/events/"+event.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}
	detail := decode[eventBody](t, rec)
	if len(detail.Locations) != 2 || detail.Locations[0].LocationID != station.ID || detail.Locations[0].Role != "site" {
		t.Errorf("detail locations = %+v, want site (Tokyo Station) first of 2", detail.Locations)
	}

	// Locations: full Location representation.
	rec = do(router, "GET", loc, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET locations status = %d", rec.Code)
	}
	items := decode[struct {
		Items []eventLocationBody `json:"items"`
	}](t, rec).Items
	if len(items) != 2 || items[1].Location.ID != shinjuku.ID || items[1].Location.Longitude != 139.6917 {
		t.Errorf("event locations = %+v", items)
	}

	// List includes the undated event (feed time = now, so on the first page).
	rec = do(router, "GET", "/api/events?limit=100", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("LIST status = %d", rec.Code)
	}
	found := map[string]bool{}
	for _, e := range decode[struct {
		Items []eventBody `json:"items"`
	}](t, rec).Items {
		found[e.ID] = true
	}
	if !found[undated.ID] {
		t.Error("undated event missing from first page of GET /api/events")
	}

	// A referenced Location cannot be deleted.
	assertError(t, do(router, "DELETE", "/api/locations/"+station.ID, ""), http.StatusConflict, "location_in_use")

	// Detach.
	if rec = do(router, "DELETE", loc+"/"+shinjuku.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("detach status = %d", rec.Code)
	}
	assertError(t, do(router, "DELETE", loc+"/"+shinjuku.ID, ""), http.StatusNotFound, "event_location_not_found")

	// Delete the Event: its associations go, its Locations stay.
	if rec = do(router, "DELETE", "/api/events/"+event.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE event status = %d", rec.Code)
	}
	assertError(t, do(router, "GET", "/api/events/"+event.ID, ""), http.StatusNotFound, "event_not_found")
	assertError(t, do(router, "GET", loc, ""), http.StatusNotFound, "event_not_found")
	if rec = do(router, "GET", "/api/locations/"+station.ID, ""); rec.Code != http.StatusOK {
		t.Errorf("Location gone after its Event was deleted: status %d", rec.Code)
	}
	if rec = do(router, "DELETE", "/api/locations/"+station.ID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("DELETE now-unreferenced location status = %d, want 204 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestIntegration_EventNotFound(t *testing.T) {
	router := integrationRouter(t)
	missing := "00000000-0000-4000-8000-000000000000"
	assertError(t, do(router, "GET", "/api/events/"+missing, ""), http.StatusNotFound, "event_not_found")
	assertError(t, do(router, "DELETE", "/api/events/"+missing, ""), http.StatusNotFound, "event_not_found")
}
