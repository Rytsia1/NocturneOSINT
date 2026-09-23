package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nocturne-backend/internal/domain"
	"nocturne-backend/internal/repository"
)

const (
	defaultSourcePageSize = 20
	maxSourcePageSize     = 100
	maxRequestBodyBytes   = 16 << 10
)

type sourceHandler struct {
	logger  *slog.Logger
	sources *repository.SourceRepository
}

type sourceResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type sourceListResponse struct {
	Items      []sourceResponse `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

type createSourceRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

func toSourceResponse(s domain.Source) sourceResponse {
	return sourceResponse{
		ID:          s.ID,
		Name:        s.Name,
		URL:         s.URL,
		Description: s.Description,
		CreatedAt:   s.CreatedAt.UTC(),
		UpdatedAt:   s.UpdatedAt.UTC(),
	}
}

func (h *sourceHandler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createSourceRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"request body must be a JSON object with fields name, url, description")
		return
	}

	source, err := domain.NewSource(req.Name, req.URL, req.Description)
	var verr *domain.ValidationError
	if errors.As(err, &verr) {
		writeError(w, http.StatusBadRequest, "validation_failed", verr.Error())
		return
	}

	created, err := h.sources.Create(r.Context(), source)
	if errors.Is(err, domain.ErrSourceURLExists) {
		writeError(w, http.StatusConflict, "source_already_exists", "a source with this URL already exists")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}

	w.Header().Set("Location", "/api/sources/"+created.ID)
	writeJSON(w, http.StatusCreated, toSourceResponse(created))
}

func (h *sourceHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !domain.IsValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "source id must be a UUID")
		return
	}

	source, err := h.sources.GetByID(r.Context(), id)
	if errors.Is(err, domain.ErrSourceNotFound) {
		writeError(w, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toSourceResponse(source))
}

func (h *sourceHandler) list(w http.ResponseWriter, r *http.Request) {
	limit := defaultSourcePageSize
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxSourcePageSize {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be an integer between 1 and 100")
			return
		}
		limit = n
	}

	var cursor *repository.SourceCursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		c, ok := decodeSourceCursor(raw)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor is malformed")
			return
		}
		cursor = c
	}

	// Fetch one extra row to learn whether another page exists.
	sources, err := h.sources.List(r.Context(), limit+1, cursor)
	if err != nil {
		h.internalError(w, err)
		return
	}

	resp := sourceListResponse{Items: make([]sourceResponse, 0, min(len(sources), limit))}
	if len(sources) > limit {
		sources = sources[:limit]
		next := encodeSourceCursor(sources[limit-1])
		resp.NextCursor = &next
	}
	for _, s := range sources {
		resp.Items = append(resp.Items, toSourceResponse(s))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *sourceHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !domain.IsValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "source id must be a UUID")
		return
	}

	err := h.sources.Delete(r.Context(), id)
	if errors.Is(err, domain.ErrSourceNotFound) {
		writeError(w, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *sourceHandler) internalError(w http.ResponseWriter, err error) {
	h.logger.Error("source request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

// Cursors are opaque to clients: base64url("<created_at RFC3339Nano>|<id>").
func encodeSourceCursor(s domain.Source) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + s.ID))
}

func decodeSourceCursor(raw string) (*repository.SourceCursor, bool) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, false
	}
	ts, id, ok := strings.Cut(string(b), "|")
	if !ok || !domain.IsValidID(id) {
		return nil, false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, false
	}
	return &repository.SourceCursor{CreatedAt: createdAt, ID: id}, true
}
