package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"nocturne-backend/internal/domain"
	"nocturne-backend/internal/repository"
)

// Service ingests a Source's feed into Articles and their image metadata. It
// is triggered manually; there is no scheduler.
type Service struct {
	sources  *repository.SourceRepository
	articles *repository.ArticleRepository
	media    *repository.ArticleMediaRepository
	fetcher  *Fetcher
	logger   *slog.Logger
}

func NewService(sources *repository.SourceRepository, articles *repository.ArticleRepository,
	media *repository.ArticleMediaRepository, fetcher *Fetcher, logger *slog.Logger) *Service {
	return &Service{sources: sources, articles: articles, media: media, fetcher: fetcher, logger: logger}
}

// Result summarises one ingestion run.
type Result struct {
	SourceID     string
	Fetched      int  // items considered (at most MaxFeedItems)
	Truncated    bool // the feed had more than MaxFeedItems items
	Inserted     int
	Duplicates   int // URL already stored, or repeated within the feed
	Invalid      int // skipped: failed Article validation
	Media        int // image metadata rows stored for inserted Articles
	InvalidMedia int // image metadata skipped as malformed; the Article is kept
	Errors       []ItemError
}

// ItemError explains why one item was skipped; Item is its 0-based position.
type ItemError struct {
	Item   int
	Reason string
}

type candidate struct {
	index   int
	article domain.Article
	media   []rawMedia
}

// Ingest fetches the Source's URL as a feed and stores its new items as
// Articles of that Source, with any image metadata the feed states. Items are
// inserted one by one, so an invalid item never discards the valid ones;
// existing Articles and their media are never modified. Image URLs are never
// requested.
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

	var valid []candidate
	for i, it := range items {
		a, err := normalize(it, source.ID)
		if err != nil {
			res.Invalid++
			if len(res.Errors) < maxErrReports {
				res.Errors = append(res.Errors, ItemError{Item: i, Reason: err.Error()})
			}
			continue
		}
		valid = append(valid, candidate{index: i, article: a, media: it.Media})
	}
	if len(valid) == 0 {
		return res, nil
	}

	urls := make([]string, len(valid))
	for i, c := range valid {
		urls[i] = c.article.URL
	}
	seen, err := s.articles.ExistingURLs(ctx, urls)
	if err != nil {
		return res, err
	}
	for _, c := range valid {
		if seen[c.article.URL] {
			res.Duplicates++
			continue
		}
		seen[c.article.URL] = true
		// The unique constraint stays authoritative: a concurrent insert of
		// the same URL surfaces here as a duplicate, not an error.
		created, err := s.articles.Create(ctx, c.article)
		switch {
		case errors.Is(err, domain.ErrArticleURLExists):
			res.Duplicates++
			continue
		case err != nil:
			return res, fmt.Errorf("store article: %w", err)
		}
		res.Inserted++
		if err := s.storeMedia(ctx, &res, source.ID, c.index, created.ID, c.media); err != nil {
			return res, err
		}
	}
	return res, nil
}

// storeMedia records the first MaxMediaPerItem distinct image URLs of a newly
// created Article. Malformed entries are logged and skipped; no image is ever
// fetched.
func (s *Service) storeMedia(ctx context.Context, res *Result, sourceID string, index int, articleID string, media []rawMedia) error {
	stored := map[string]bool{}
	for _, raw := range media {
		if len(stored) == MaxMediaPerItem {
			break
		}
		if stored[raw.URL] {
			continue // the same image stated twice for one item
		}
		m, err := normalizeMedia(raw, articleID)
		if err != nil {
			res.InvalidMedia++
			s.logger.Warn("feed image metadata skipped", "source_id", sourceID, "item", index,
				"article_id", articleID, "url", raw.URL, "error", err)
			continue
		}
		stored[raw.URL] = true
		_, err = s.media.Create(ctx, m)
		switch {
		case errors.Is(err, domain.ErrArticleMediaExists):
			// already attached; nothing to add
		case err != nil:
			return fmt.Errorf("store article media: %w", err)
		default:
			res.Media++
		}
	}
	return nil
}
