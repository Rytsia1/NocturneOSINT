package ingest

import (
	"context"
	"errors"
	"fmt"

	"nocturne-backend/internal/domain"
	"nocturne-backend/internal/repository"
)

// Service ingests a Source's feed into Articles. It is triggered manually;
// there is no scheduler.
type Service struct {
	sources  *repository.SourceRepository
	articles *repository.ArticleRepository
	fetcher  *Fetcher
}

func NewService(sources *repository.SourceRepository, articles *repository.ArticleRepository, fetcher *Fetcher) *Service {
	return &Service{sources: sources, articles: articles, fetcher: fetcher}
}

// Result summarises one ingestion run.
type Result struct {
	SourceID   string
	Fetched    int  // items considered (at most MaxFeedItems)
	Truncated  bool // the feed had more than MaxFeedItems items
	Inserted   int
	Duplicates int // URL already stored, or repeated within the feed
	Invalid    int // skipped: failed Article validation
	Errors     []ItemError
}

// ItemError explains why one item was skipped; Item is its 0-based position.
type ItemError struct {
	Item   int
	Reason string
}

// Ingest fetches the Source's URL as a feed and stores its new items as
// Articles of that Source. Items are inserted one by one, so an invalid item
// never discards the valid ones; existing Articles are never modified.
func (s *Service) Ingest(ctx context.Context, sourceID string) (Result, error) {
	source, err := s.sources.GetByID(ctx, sourceID)
	if err != nil {
		return Result{}, err
	}
	body, err := s.fetcher.Fetch(ctx, source.URL)
	if err != nil {
		return Result{}, err
	}
	items, err := parseFeed(body)
	if err != nil {
		return Result{}, err
	}

	res := Result{SourceID: source.ID}
	if len(items) > MaxFeedItems {
		items, res.Truncated = items[:MaxFeedItems], true
	}
	res.Fetched = len(items)

	var valid []domain.Article
	for i, it := range items {
		a, err := normalize(it, source.ID)
		if err != nil {
			res.Invalid++
			if len(res.Errors) < maxErrReports {
				res.Errors = append(res.Errors, ItemError{Item: i, Reason: err.Error()})
			}
			continue
		}
		valid = append(valid, a)
	}
	if len(valid) == 0 {
		return res, nil
	}

	urls := make([]string, len(valid))
	for i, a := range valid {
		urls[i] = a.URL
	}
	seen, err := s.articles.ExistingURLs(ctx, urls)
	if err != nil {
		return res, err
	}
	for _, a := range valid {
		if seen[a.URL] {
			res.Duplicates++
			continue
		}
		seen[a.URL] = true
		// The unique constraint stays authoritative: a concurrent insert of
		// the same URL surfaces here as a duplicate, not an error.
		_, err := s.articles.Create(ctx, a)
		switch {
		case errors.Is(err, domain.ErrArticleURLExists):
			res.Duplicates++
		case err != nil:
			return res, fmt.Errorf("store article: %w", err)
		default:
			res.Inserted++
		}
	}
	return res, nil
}
