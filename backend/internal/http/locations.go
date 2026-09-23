package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"nocturne-backend/internal/domain"
	"nocturne-backend/internal/repository"
)

type locationHandler struct {
	logger    *slog.Logger
	locations *repository.LocationRepository
}

type locationResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Precision string    `json:"precision"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DistanceM *float64  `json:"distance_m,omitempty"` // nearby results only
}

// spatialResponse is returned by bounding-box and nearby queries, which are
// capped rather than paginated; truncated tells a map client to zoom in.
type spatialResponse struct {
	Items     []locationResponse `json:"items"`
	Truncated bool               `json:"truncated"`
}

// Pointers distinguish a missing coordinate from a real 0 (Null Island).
type createLocationRequest struct {
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Precision string   `json:"precision"`
}

var bboxParams = []string{"min_lat", "min_lon", "max_lat", "max_lon"}

func toLocationResponse(l domain.Location) locationResponse {
	return locationResponse{
		ID:        l.ID,
		Name:      l.Name,
		Latitude:  l.Latitude,
		Longitude: l.Longitude,
		Precision: l.Precision,
		CreatedAt: l.CreatedAt.UTC(),
		UpdatedAt: l.UpdatedAt.UTC(),
	}
}

func (h *locationHandler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createLocationRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"request body must be a JSON object with fields name, latitude, longitude, precision")
		return
	}
	if req.Latitude == nil || req.Longitude == nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "latitude and longitude are required")
		return
	}

	location, err := domain.NewLocation(req.Name, *req.Latitude, *req.Longitude, req.Precision)
	if writeValidationError(w, err) {
		return
	}

	created, err := h.locations.Create(r.Context(), location)
	if err != nil {
		h.internalError(w, err)
		return
	}
	w.Header().Set("Location", "/api/locations/"+created.ID)
	writeJSON(w, http.StatusCreated, toLocationResponse(created))
}

func (h *locationHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !domain.IsValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "location id must be a UUID")
		return
	}

	location, err := h.locations.GetByID(r.Context(), id)
	if errors.Is(err, domain.ErrLocationNotFound) {
		writeError(w, http.StatusNotFound, "location_not_found", "location not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toLocationResponse(location))
}

// list serves the paginated list, or a bounding-box query when any of
// min_lat/min_lon/max_lat/max_lon is present.
func (h *locationHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	for _, p := range bboxParams {
		if q.Has(p) {
			h.listInBoundingBox(w, r, q)
			return
		}
	}

	limit, cursor, ok := parsePage(w, r)
	if !ok {
		return
	}
	locations, err := h.locations.List(r.Context(), limit+1, cursor)
	if err != nil {
		h.internalError(w, err)
		return
	}

	locations, next := trimPage(locations, limit, func(l domain.Location) (time.Time, string) { return l.CreatedAt, l.ID })
	resp := listResponse[locationResponse]{Items: make([]locationResponse, 0, len(locations)), NextCursor: next}
	for _, l := range locations {
		resp.Items = append(resp.Items, toLocationResponse(l))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *locationHandler) listInBoundingBox(w http.ResponseWriter, r *http.Request, q url.Values) {
	if q.Has("cursor") {
		writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor is not supported with a bounding box; zoom in instead")
		return
	}
	var v [4]float64
	for i, p := range bboxParams {
		f, err := queryFloat(q, p)
		if writeValidationError(w, err) {
			return
		}
		v[i] = f
	}
	box, err := domain.NewBoundingBox(v[0], v[1], v[2], v[3])
	if writeValidationError(w, err) {
		return
	}
	limit, ok := parseLimit(w, r, maxPageSize)
	if !ok {
		return
	}

	locations, err := h.locations.FindWithinBoundingBox(r.Context(), box, limit+1)
	if err != nil {
		h.internalError(w, err)
		return
	}
	resp := spatialResponse{Truncated: len(locations) > limit}
	locations = locations[:min(len(locations), limit)]
	resp.Items = make([]locationResponse, 0, len(locations))
	for _, l := range locations {
		resp.Items = append(resp.Items, toLocationResponse(l))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *locationHandler) nearby(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var v [3]float64
	for i, p := range []string{"lat", "lon", "radius_m"} {
		f, err := queryFloat(q, p)
		if writeValidationError(w, err) {
			return
		}
		v[i] = f
	}
	lat, lon, radiusM := v[0], v[1], v[2]
	if writeValidationError(w, domain.CheckNearby(lat, lon, radiusM)) {
		return
	}
	limit, ok := parseLimit(w, r, maxPageSize)
	if !ok {
		return
	}

	nearby, err := h.locations.FindNearby(r.Context(), lat, lon, radiusM, limit+1)
	if err != nil {
		h.internalError(w, err)
		return
	}
	resp := spatialResponse{Truncated: len(nearby) > limit}
	nearby = nearby[:min(len(nearby), limit)]
	resp.Items = make([]locationResponse, 0, len(nearby))
	for _, n := range nearby {
		item := toLocationResponse(n.Location)
		item.DistanceM = &n.DistanceM
		resp.Items = append(resp.Items, item)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *locationHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !domain.IsValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "location id must be a UUID")
		return
	}

	err := h.locations.Delete(r.Context(), id)
	if errors.Is(err, domain.ErrLocationNotFound) {
		writeError(w, http.StatusNotFound, "location_not_found", "location not found")
		return
	}
	if errors.Is(err, domain.ErrLocationInUse) {
		writeError(w, http.StatusConflict, "location_in_use", "detach the location from its events before deleting it")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *locationHandler) internalError(w http.ResponseWriter, err error) {
	h.logger.Error("location request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

// queryFloat parses a required numeric query parameter. NaN and ±Inf parse
// successfully here and are rejected by domain validation.
func queryFloat(q url.Values, field string) (float64, error) {
	raw := q.Get(field)
	if raw == "" {
		return 0, &domain.ValidationError{Field: field, Message: "is required"}
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, &domain.ValidationError{Field: field, Message: "must be a number"}
	}
	return v, nil
}

// writeValidationError writes a 400 for a *domain.ValidationError and reports
// whether it did.
func writeValidationError(w http.ResponseWriter, err error) bool {
	var verr *domain.ValidationError
	if errors.As(err, &verr) {
		writeError(w, http.StatusBadRequest, "validation_failed", verr.Error())
		return true
	}
	return false
}
