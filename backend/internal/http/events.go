package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"nocturne-backend/internal/domain"
	"nocturne-backend/internal/repository"
)

type eventHandler struct {
	logger *slog.Logger
	events *repository.EventRepository
}

type eventResponse struct {
	ID                  string     `json:"id"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	OccurredAt          *time.Time `json:"occurred_at"`
	OccurredAtPrecision *string    `json:"occurred_at_precision"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// eventDetailResponse adds lightweight location references; clients fetch
// full Locations from /api/events/{id}/locations or /api/locations/{id}.
type eventDetailResponse struct {
	eventResponse
	Locations []eventLocationRef `json:"locations"`
}

type eventLocationRef struct {
	LocationID string `json:"location_id"`
	Role       string `json:"role"`
}

type eventLocationResponse struct {
	Role     string           `json:"role"`
	Location locationResponse `json:"location"`
}

type createEventRequest struct {
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	OccurredAt          *time.Time `json:"occurred_at"`
	OccurredAtPrecision string     `json:"occurred_at_precision"`
}

type attachLocationRequest struct {
	LocationID string `json:"location_id"`
	Role       string `json:"role"`
}

func toEventResponse(e domain.Event) eventResponse {
	resp := eventResponse{
		ID:          e.ID,
		Title:       e.Title,
		Description: e.Description,
		CreatedAt:   e.CreatedAt.UTC(),
		UpdatedAt:   e.UpdatedAt.UTC(),
	}
	if e.OccurredAt != nil {
		at, precision := e.OccurredAt.UTC(), e.OccurredAtPrecision
		resp.OccurredAt, resp.OccurredAtPrecision = &at, &precision
	}
	return resp
}

func toEventLocationResponse(el domain.EventLocation) eventLocationResponse {
	return eventLocationResponse{Role: el.Role, Location: toLocationResponse(el.Location)}
}

func (h *eventHandler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createEventRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"request body must be a JSON object with fields title, description, occurred_at (RFC 3339), occurred_at_precision")
		return
	}

	event, err := domain.NewEvent(req.Title, req.Description, req.OccurredAt, req.OccurredAtPrecision)
	if writeValidationError(w, err) {
		return
	}
	created, err := h.events.Create(r.Context(), event)
	if err != nil {
		h.internalError(w, err)
		return
	}

	w.Header().Set("Location", "/api/events/"+created.ID)
	writeJSON(w, http.StatusCreated, eventDetailResponse{eventResponse: toEventResponse(created), Locations: []eventLocationRef{}})
}

func (h *eventHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "event")
	if !ok {
		return
	}

	event, err := h.events.GetByID(r.Context(), id)
	if err == nil {
		var refs []domain.EventLocation
		refs, err = h.events.Locations(r.Context(), id, maxPageSize)
		if err == nil {
			resp := eventDetailResponse{eventResponse: toEventResponse(event), Locations: make([]eventLocationRef, 0, len(refs))}
			for _, el := range refs {
				resp.Locations = append(resp.Locations, eventLocationRef{LocationID: el.Location.ID, Role: el.Role})
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}
	if errors.Is(err, domain.ErrEventNotFound) {
		writeError(w, http.StatusNotFound, "event_not_found", "event not found")
		return
	}
	h.internalError(w, err)
}

func (h *eventHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, cursor, ok := parsePage(w, r)
	if !ok {
		return
	}
	events, err := h.events.List(r.Context(), limit+1, cursor)
	if err != nil {
		h.internalError(w, err)
		return
	}

	events, next := trimPage(events, limit, func(e domain.Event) (time.Time, string) { return e.FeedTime(), e.ID })
	resp := listResponse[eventResponse]{Items: make([]eventResponse, 0, len(events)), NextCursor: next}
	for _, e := range events {
		resp.Items = append(resp.Items, toEventResponse(e))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *eventHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "event")
	if !ok {
		return
	}

	err := h.events.Delete(r.Context(), id)
	if errors.Is(err, domain.ErrEventNotFound) {
		writeError(w, http.StatusNotFound, "event_not_found", "event not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *eventHandler) listLocations(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "event")
	if !ok {
		return
	}

	refs, err := h.events.Locations(r.Context(), id, maxPageSize+1)
	if errors.Is(err, domain.ErrEventNotFound) {
		writeError(w, http.StatusNotFound, "event_not_found", "event not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}

	resp := struct {
		Items     []eventLocationResponse `json:"items"`
		Truncated bool                    `json:"truncated"`
	}{Truncated: len(refs) > maxPageSize}
	refs = refs[:min(len(refs), maxPageSize)]
	resp.Items = make([]eventLocationResponse, 0, len(refs))
	for _, el := range refs {
		resp.Items = append(resp.Items, toEventLocationResponse(el))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *eventHandler) addLocation(w http.ResponseWriter, r *http.Request) {
	eventID, ok := pathID(w, r, "id", "event")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req attachLocationRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be a JSON object with fields location_id, role")
		return
	}
	if !domain.IsValidID(req.LocationID) {
		writeError(w, http.StatusBadRequest, "validation_failed", "location_id must be a UUID")
		return
	}
	if writeValidationError(w, domain.CheckEventLocationRole(req.Role)) {
		return
	}

	el, err := h.events.AddLocation(r.Context(), eventID, req.LocationID, req.Role)
	switch {
	case errors.Is(err, domain.ErrEventNotFound):
		writeError(w, http.StatusNotFound, "event_not_found", "event not found")
	case errors.Is(err, domain.ErrLocationNotFound):
		writeError(w, http.StatusUnprocessableEntity, "location_not_found", "location_id does not reference an existing location")
	case errors.Is(err, domain.ErrEventLocationExists):
		writeError(w, http.StatusConflict, "event_location_exists", "location is already attached to this event")
	case err != nil:
		h.internalError(w, err)
	default:
		writeJSON(w, http.StatusCreated, toEventLocationResponse(el))
	}
}

func (h *eventHandler) removeLocation(w http.ResponseWriter, r *http.Request) {
	eventID, ok := pathID(w, r, "id", "event")
	if !ok {
		return
	}
	locationID, ok := pathID(w, r, "location_id", "location")
	if !ok {
		return
	}

	err := h.events.RemoveLocation(r.Context(), eventID, locationID)
	if errors.Is(err, domain.ErrEventLocationNotFound) {
		writeError(w, http.StatusNotFound, "event_location_not_found", "location is not attached to this event")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *eventHandler) internalError(w http.ResponseWriter, err error) {
	h.logger.Error("event request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

// pathID reads a UUID path parameter; on a malformed value it writes a 400.
func pathID(w http.ResponseWriter, r *http.Request, param, resource string) (string, bool) {
	id := r.PathValue(param)
	if !domain.IsValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", resource+" id must be a UUID")
		return "", false
	}
	return id, true
}
