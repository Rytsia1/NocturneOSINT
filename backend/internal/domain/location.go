package domain

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxLocationNameLength = 200 // characters

	// MaxNearbyRadiusM caps nearby queries at 50 km: city-scale research on a
	// phone, matching the PRD's "events within 50 km of a location".
	MaxNearbyRadiusM = 50_000
)

var ErrLocationNotFound = errors.New("location not found")

// LocationPrecisions lists how precise a Location's coordinate is. A point
// must never imply more precision than its source supports.
var LocationPrecisions = []string{"country", "region", "city", "specific", "approximate"}

// Location is a named geographic point (WGS84 latitude/longitude).
type Location struct {
	ID        string
	Name      string
	Latitude  float64
	Longitude float64
	Precision string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewLocation validates input and returns an unsaved Location. Coordinates are
// taken as given: out-of-range values are rejected, never clamped or geocoded.
func NewLocation(name string, latitude, longitude float64, precision string) (Location, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return Location{}, &ValidationError{"name", "is required"}
	}
	if utf8.RuneCountInString(name) > MaxLocationNameLength {
		return Location{}, &ValidationError{"name", fmt.Sprintf("must be at most %d characters", MaxLocationNameLength)}
	}
	if err := CheckLatitude("latitude", latitude); err != nil {
		return Location{}, err
	}
	if err := CheckLongitude("longitude", longitude); err != nil {
		return Location{}, err
	}
	if !isLocationPrecision(precision) {
		return Location{}, &ValidationError{"precision", "must be one of " + strings.Join(LocationPrecisions, ", ")}
	}

	return Location{Name: name, Latitude: latitude, Longitude: longitude, Precision: precision}, nil
}

func isLocationPrecision(p string) bool {
	for _, v := range LocationPrecisions {
		if p == v {
			return true
		}
	}
	return false
}

// CheckLatitude rejects NaN, ±Inf and values outside [-90, 90].
func CheckLatitude(field string, v float64) error {
	if math.IsNaN(v) || v < -90 || v > 90 {
		return &ValidationError{field, "must be a number between -90 and 90"}
	}
	return nil
}

// CheckLongitude rejects NaN, ±Inf and values outside [-180, 180].
func CheckLongitude(field string, v float64) error {
	if math.IsNaN(v) || v < -180 || v > 180 {
		return &ValidationError{field, "must be a number between -180 and 180"}
	}
	return nil
}

// BoundingBox is a latitude/longitude rectangle, edges inclusive.
type BoundingBox struct {
	MinLat, MinLon, MaxLat, MaxLon float64
}

// NewBoundingBox validates a box. Boxes crossing the antimeridian
// (min_lon > max_lon) are rejected rather than wrapped.
func NewBoundingBox(minLat, minLon, maxLat, maxLon float64) (BoundingBox, error) {
	for _, err := range []error{
		CheckLatitude("min_lat", minLat),
		CheckLongitude("min_lon", minLon),
		CheckLatitude("max_lat", maxLat),
		CheckLongitude("max_lon", maxLon),
	} {
		if err != nil {
			return BoundingBox{}, err
		}
	}
	if minLat > maxLat {
		return BoundingBox{}, &ValidationError{"min_lat", "must be less than or equal to max_lat"}
	}
	if minLon > maxLon {
		return BoundingBox{}, &ValidationError{"min_lon", "must be less than or equal to max_lon (boxes crossing the antimeridian are not supported)"}
	}
	return BoundingBox{MinLat: minLat, MinLon: minLon, MaxLat: maxLat, MaxLon: maxLon}, nil
}

// CheckNearby validates a nearby query centre and radius in metres.
func CheckNearby(lat, lon, radiusM float64) error {
	if err := CheckLatitude("lat", lat); err != nil {
		return err
	}
	if err := CheckLongitude("lon", lon); err != nil {
		return err
	}
	if math.IsNaN(radiusM) || radiusM <= 0 || radiusM > MaxNearbyRadiusM {
		return &ValidationError{"radius_m", fmt.Sprintf("must be greater than 0 and at most %d", MaxNearbyRadiusM)}
	}
	return nil
}
