// Package http wires the backend's HTTP routes and request logging.
package http

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nocturne-backend/internal/domain"
	"nocturne-backend/internal/ingest"
	"nocturne-backend/internal/repository"
)

const (
	defaultPageSize     = 20
	maxPageSize         = 100
	maxRequestBodyBytes = 16 << 10
)

// NewRouter builds the backend's HTTP handler, wrapped with request logging.
// Feed ingestion fetches public addresses only.
func NewRouter(logger *slog.Logger, db *pgxpool.Pool) http.Handler {
	return newRouter(logger, db, ingest.NewDefaultFetcher())
}

// newRouter lets tests supply a Fetcher that may reach a local test server.
func newRouter(logger *slog.Logger, db *pgxpool.Pool, fetcher *ingest.Fetcher) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /ready", handleReady(logger, db))

	sources := &sourceHandler{logger: logger, sources: repository.NewSourceRepository(db)}
	mux.HandleFunc("GET /api/sources", sources.list)
	mux.HandleFunc("POST /api/sources", sources.create)
	mux.HandleFunc("GET /api/sources/{id}", sources.get)
	mux.HandleFunc("DELETE /api/sources/{id}", sources.delete)

	ingester := &ingestHandler{logger: logger, service: ingest.NewService(
		repository.NewSourceRepository(db), repository.NewArticleRepository(db), fetcher)}
	mux.HandleFunc("POST /api/sources/{id}/ingest", ingester.ingest)

	articles := &articleHandler{logger: logger, articles: repository.NewArticleRepository(db)}
	mux.HandleFunc("GET /api/articles", articles.list)
	mux.HandleFunc("POST /api/articles", articles.create)
	mux.HandleFunc("GET /api/articles/{id}", articles.get)
	mux.HandleFunc("DELETE /api/articles/{id}", articles.delete)

	locations := &locationHandler{logger: logger, locations: repository.NewLocationRepository(db)}
	mux.HandleFunc("GET /api/locations", locations.list)
	mux.HandleFunc("POST /api/locations", locations.create)
	mux.HandleFunc("GET /api/locations/nearby", locations.nearby)
	mux.HandleFunc("GET /api/locations/{id}", locations.get)
	mux.HandleFunc("DELETE /api/locations/{id}", locations.delete)

	events := &eventHandler{logger: logger, events: repository.NewEventRepository(db)}
	mux.HandleFunc("GET /api/events", events.list)
	mux.HandleFunc("POST /api/events", events.create)
	mux.HandleFunc("GET /api/events/{id}", events.get)
	mux.HandleFunc("DELETE /api/events/{id}", events.delete)
	mux.HandleFunc("GET /api/events/{id}/locations", events.listLocations)
	mux.HandleFunc("POST /api/events/{id}/locations", events.addLocation)
	mux.HandleFunc("DELETE /api/events/{id}/locations/{location_id}", events.removeLocation)

	evidence := &evidenceHandler{logger: logger, evidence: repository.NewEvidenceRepository(db)}
	mux.HandleFunc("POST /api/evidence", evidence.create)
	mux.HandleFunc("GET /api/evidence/{id}", evidence.get)
	mux.HandleFunc("DELETE /api/evidence/{id}", evidence.delete)
	mux.HandleFunc("GET /api/articles/{id}/events", evidence.articleEvents)
	mux.HandleFunc("GET /api/events/{id}/articles", evidence.eventArticles)

	return withLogging(logger, mux)
}

// handleHealth reports that the process is up; it never touches the database.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady reports whether the application can currently reach the database.
func handleReady(logger *slog.Logger, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			logger.Warn("readiness check failed", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// writeError writes the API's standard error body: {"error":{"code","message"}}.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

// listResponse is the envelope shared by all paginated list endpoints.
type listResponse[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// parsePage reads the limit and cursor query parameters shared by list
// endpoints; on invalid input it writes a 400 response and returns ok=false.
func parsePage(w http.ResponseWriter, r *http.Request) (limit int, cursor *repository.Cursor, ok bool) {
	if limit, ok = parseLimit(w, r, defaultPageSize); !ok {
		return 0, nil, false
	}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		c, valid := decodeCursor(raw)
		if !valid {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor is malformed")
			return 0, nil, false
		}
		cursor = c
	}
	return limit, cursor, true
}

// parseLimit reads the limit query parameter (1–maxPageSize, default def); on
// invalid input it writes a 400 response and returns ok=false.
func parseLimit(w http.ResponseWriter, r *http.Request, def int) (int, bool) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxPageSize {
		writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be an integer between 1 and 100")
		return 0, false
	}
	return n, true
}

// trimPage drops the extra row fetched to detect a next page and returns the
// cursor for that page, if there is one.
func trimPage[T any](items []T, limit int, key func(T) (time.Time, string)) ([]T, *string) {
	if len(items) <= limit {
		return items, nil
	}
	items = items[:limit]
	at, id := key(items[limit-1])
	next := encodeCursor(at, id)
	return items, &next
}

// Cursors are opaque to clients: base64url("<sort time RFC3339Nano>|<id>").
func encodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeCursor(raw string) (*repository.Cursor, bool) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, false
	}
	ts, id, ok := strings.Cut(string(b), "|")
	if !ok || !domain.IsValidID(id) {
		return nil, false
	}
	at, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, false
	}
	return &repository.Cursor{At: at, ID: id}, true
}

// statusRecorder captures the status code written by the wrapped handler so
// it can be logged after the response completes.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		requestID := newRequestID()

		next.ServeHTTP(rec, r)

		logger.Info("request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).String(),
		)
	})
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
