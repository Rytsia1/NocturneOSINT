package database

import (
	"context"
	"os"
	"testing"
)

func TestConnect_EmptyURL(t *testing.T) {
	if _, err := Connect(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty DATABASE_URL")
	}
}

// Integration tests below run against a real PostgreSQL/PostGIS instance
// (docker compose up -d) and are skipped when DATABASE_URL is unset.
func connectOrSkip(t *testing.T) context.Context {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	return context.Background()
}

func TestIntegration_ConnectAndPostGIS(t *testing.T) {
	ctx := connectOrSkip(t)

	pool, err := Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	var version string
	if err := pool.QueryRow(ctx, "SELECT PostGIS_Version()").Scan(&version); err != nil {
		t.Fatalf("PostGIS_Version(): %v", err)
	}
	if version == "" {
		t.Fatal("PostGIS_Version() returned empty string")
	}
	t.Logf("PostGIS version: %s", version)
}

func TestIntegration_MigrationsApplied(t *testing.T) {
	ctx := connectOrSkip(t)

	pool, err := Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	var version int64
	var dirty bool
	if err := pool.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("read schema_migrations (have migrations run?): %v", err)
	}
	if dirty {
		t.Fatalf("migration version %d is dirty", version)
	}
	if version < 1 {
		t.Fatalf("migration version = %d, want >= 1", version)
	}

	var exists bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'postgis')").Scan(&exists); err != nil {
		t.Fatalf("query pg_extension: %v", err)
	}
	if !exists {
		t.Fatal("postgis extension is not installed")
	}
}
