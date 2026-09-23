package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"nocturne-backend/internal/database"
	"nocturne-backend/internal/domain"
)

// These are integration tests against the real PostgreSQL instance
// (docker compose up -d); they are skipped when DATABASE_URL is unset.
func newTestRepo(t *testing.T) *SourceRepository {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	pool, err := database.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewSourceRepository(pool)
}

// createTestSource creates a Source with a URL unique to this test run and
// deletes it when the test ends.
func createTestSource(t *testing.T, repo *SourceRepository, name string) domain.Source {
	t.Helper()
	s, err := domain.NewSource(name, fmt.Sprintf("https://example.test/%s/%d", t.Name(), time.Now().UnixNano()), "test source")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	created, err := repo.Create(context.Background(), s)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { repo.Delete(context.Background(), created.ID) })
	return created
}

func TestIntegration_SourceCreateAndGet(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	created := createTestSource(t, repo, "Reuters")
	if !domain.IsValidID(created.ID) {
		t.Fatalf("ID = %q, want UUID", created.ID)
	}
	if created.CreatedAt.IsZero() || !created.UpdatedAt.Equal(created.CreatedAt) {
		t.Errorf("timestamps = %v / %v, want equal and set", created.CreatedAt, created.UpdatedAt)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != created.Name || got.URL != created.URL || got.Description != created.Description ||
		!got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("GetByID = %+v, want %+v", got, created)
	}
}

func TestIntegration_SourceURLStoredVerbatim(t *testing.T) {
	repo := newTestRepo(t)
	raw := fmt.Sprintf("HTTPS://Example.TEST/Path/../x?b=2&a=1#frag-%d", time.Now().UnixNano())

	s, err := domain.NewSource("Verbatim", raw, "")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	created, err := repo.Create(context.Background(), s)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { repo.Delete(context.Background(), created.ID) })

	got, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.URL != raw {
		t.Errorf("URL = %q, want verbatim %q", got.URL, raw)
	}
}

func TestIntegration_SourceDuplicateURL(t *testing.T) {
	repo := newTestRepo(t)
	existing := createTestSource(t, repo, "Original")

	dup, _ := domain.NewSource("Duplicate", existing.URL, "")
	if _, err := repo.Create(context.Background(), dup); !errors.Is(err, domain.ErrSourceURLExists) {
		t.Fatalf("Create duplicate URL err = %v, want ErrSourceURLExists", err)
	}
}

func TestIntegration_SourceNotFound(t *testing.T) {
	repo := newTestRepo(t)
	missing := "00000000-0000-4000-8000-000000000000"

	if _, err := repo.GetByID(context.Background(), missing); !errors.Is(err, domain.ErrSourceNotFound) {
		t.Errorf("GetByID err = %v, want ErrSourceNotFound", err)
	}
	if err := repo.Delete(context.Background(), missing); !errors.Is(err, domain.ErrSourceNotFound) {
		t.Errorf("Delete err = %v, want ErrSourceNotFound", err)
	}
}

func TestIntegration_SourceDelete(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	created := createTestSource(t, repo, "To delete")

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, domain.ErrSourceNotFound) {
		t.Errorf("GetByID after delete err = %v, want ErrSourceNotFound", err)
	}
}

func TestIntegration_SourceListOrderAndCursor(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	var mine []string // creation order, oldest first
	for i := range 3 {
		mine = append(mine, createTestSource(t, repo, fmt.Sprintf("List %d", i)).ID)
	}

	// Walk every page with a small page size; the table may hold other rows.
	seen := map[string]bool{}
	var order []string
	var cursor *Cursor
	for {
		page, err := repo.List(ctx, 2, cursor)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(page) > 2 {
			t.Fatalf("page size = %d, want <= 2", len(page))
		}
		for _, s := range page {
			if seen[s.ID] {
				t.Fatalf("source %s returned twice across pages", s.ID)
			}
			seen[s.ID] = true
			order = append(order, s.ID)
		}
		if len(page) < 2 {
			break
		}
		last := page[len(page)-1]
		cursor = &Cursor{At: last.CreatedAt, ID: last.ID}
	}

	// Newest first: our sources must appear in reverse creation order.
	var got []string
	for _, id := range order {
		for _, m := range mine {
			if id == m {
				got = append(got, id)
			}
		}
	}
	want := []string{mine[2], mine[1], mine[0]}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("order of created sources = %v, want %v", got, want)
	}
}
