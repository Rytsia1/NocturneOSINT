package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func get(t *testing.T, router http.Handler, path string) (int, map[string]string) {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, body
}

func TestHealthEndpoint(t *testing.T) {
	code, body := get(t, NewRouter(discardLogger, nil), "/health")

	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want %q", body["status"], "ok")
	}
}

func TestHealthEndpoint_WrongMethod(t *testing.T) {
	router := NewRouter(discardLogger, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/health", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestReadyEndpoint_DatabaseUnreachable(t *testing.T) {
	// pgxpool connects lazily, so this pool only fails when /ready pings it.
	pool, err := pgxpool.New(context.Background(), "postgres://nocturne:nocturne@127.0.0.1:1/nocturne?connect_timeout=1")
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	code, body := get(t, NewRouter(discardLogger, pool), "/ready")

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", code, http.StatusServiceUnavailable)
	}
	if body["status"] != "unavailable" {
		t.Errorf("status field = %q, want %q", body["status"], "unavailable")
	}
}

func TestIntegration_ReadyEndpoint(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	code, body := get(t, NewRouter(discardLogger, pool), "/ready")

	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if body["status"] != "ready" {
		t.Errorf("status field = %q, want %q", body["status"], "ready")
	}
}
