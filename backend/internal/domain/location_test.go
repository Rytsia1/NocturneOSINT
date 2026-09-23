package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func assertField(t *testing.T, err error, field string) {
	t.Helper()
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if verr.Field != field {
		t.Errorf("Field = %q, want %q", verr.Field, field)
	}
}

func TestNewLocation_Valid(t *testing.T) {
	l, err := NewLocation("  Tokyo Station ", 35.6812, 139.7671, "specific")
	if err != nil {
		t.Fatalf("NewLocation: %v", err)
	}
	if l.Name != "Tokyo Station" || l.Latitude != 35.6812 || l.Longitude != 139.7671 || l.Precision != "specific" {
		t.Errorf("location = %+v", l)
	}
}

func TestNewLocation_AcceptsExactLimitsAndNullIsland(t *testing.T) {
	for _, c := range [][2]float64{{90, 180}, {-90, -180}, {0, 0}} {
		if _, err := NewLocation("Edge", c[0], c[1], "approximate"); err != nil {
			t.Errorf("NewLocation(lat=%v, lon=%v): %v", c[0], c[1], err)
		}
	}
}

func TestNewLocation_Invalid(t *testing.T) {
	tests := []struct {
		name, locName string
		lat, lon      float64
		precision     string
		field         string
	}{
		{"latitude below -90", "L", -90.0001, 0, "city", "latitude"},
		{"latitude above 90 is rejected, not clamped", "L", 91, 0, "city", "latitude"},
		{"longitude below -180", "L", 0, -180.0001, "city", "longitude"},
		{"longitude above 180", "L", 0, 181, "city", "longitude"},
		{"latitude NaN", "L", math.NaN(), 0, "city", "latitude"},
		{"longitude NaN", "L", 0, math.NaN(), "city", "longitude"},
		{"latitude +Inf", "L", math.Inf(1), 0, "city", "latitude"},
		{"longitude -Inf", "L", 0, math.Inf(-1), "city", "longitude"},
		{"empty name", "", 0, 0, "city", "name"},
		{"whitespace name", "   ", 0, 0, "city", "name"},
		{"name too long", strings.Repeat("a", MaxLocationNameLength+1), 0, 0, "city", "name"},
		{"missing precision", "L", 0, 0, "", "precision"},
		{"unknown precision", "L", 0, 0, "exact", "precision"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLocation(tt.locName, tt.lat, tt.lon, tt.precision)
			assertField(t, err, tt.field)
		})
	}
}

func TestNewBoundingBox(t *testing.T) {
	if _, err := NewBoundingBox(35, 139, 36, 140); err != nil {
		t.Errorf("valid box rejected: %v", err)
	}
	if _, err := NewBoundingBox(35, 139.5, 35, 139.5); err != nil {
		t.Errorf("degenerate (point) box rejected: %v", err)
	}

	tests := []struct {
		name                   string
		minLat, minLon, maxLat float64
		maxLon                 float64
		field                  string
	}{
		{"min_lat > max_lat", 36, 139, 35, 140, "min_lat"},
		{"crosses antimeridian", -10, 170, 10, -170, "min_lon"},
		{"min_lat out of range", -91, 0, 0, 1, "min_lat"},
		{"max_lat out of range", 0, 0, 91, 1, "max_lat"},
		{"min_lon out of range", 0, -181, 1, 0, "min_lon"},
		{"max_lon NaN", 0, 0, 1, math.NaN(), "max_lon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBoundingBox(tt.minLat, tt.minLon, tt.maxLat, tt.maxLon)
			assertField(t, err, tt.field)
		})
	}
}

func TestCheckNearby(t *testing.T) {
	if err := CheckNearby(35.6812, 139.7671, MaxNearbyRadiusM); err != nil {
		t.Errorf("maximum radius rejected: %v", err)
	}

	tests := []struct {
		name        string
		lat, lon, r float64
		field       string
	}{
		{"zero radius", 0, 0, 0, "radius_m"},
		{"negative radius", 0, 0, -1, "radius_m"},
		{"radius above maximum", 0, 0, MaxNearbyRadiusM + 1, "radius_m"},
		{"absurd radius", 0, 0, 999999999, "radius_m"},
		{"NaN radius", 0, 0, math.NaN(), "radius_m"},
		{"infinite radius", 0, 0, math.Inf(1), "radius_m"},
		{"latitude out of range", 95, 0, 100, "lat"},
		{"longitude out of range", 0, 200, 100, "lon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertField(t, CheckNearby(tt.lat, tt.lon, tt.r), tt.field)
		})
	}
}
