package http

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nocturne-backend/internal/domain"
	"nocturne-backend/internal/repository"
)

type searchHandler struct {
	logger   *slog.Logger
	articles *repository.ArticleRepository
}

// articleMatchResponse is the standard Article representation plus its textual
// relevance score. The score is not credibility, certainty or confidence.
type articleMatchResponse struct {
	articleResponse
	Score float32 `json:"score"`
}

// searchArticles handles GET /api/search/articles?q=…&source_id=…
// &published_from=…&published_to=…&limit=…&cursor=…
func (h *searchHandler) searchArticles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, ok := parseLimit(w, r, defaultPageSize)
	if !ok {
		return
	}
	var cursor *repository.SearchCursor
	if raw := query.Get("cursor"); raw != "" {
		if cursor, ok = decodeSearchCursor(raw); !ok {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor is malformed")
			return
		}
	}
	from, ok := parseTimeParam(w, r, "published_from")
	if !ok {
		return
	}
	to, ok := parseTimeParam(w, r, "published_to")
	if !ok {
		return
	}

	search, err := domain.NewArticleSearch(query.Get("q"), query.Get("source_id"), from, to)
	var verr *domain.ValidationError
	if errors.As(err, &verr) {
		code := map[string]string{"q": "invalid_query", "source_id": "invalid_source_id", "published_to": "invalid_date_range"}[verr.Field]
		writeError(w, http.StatusBadRequest, code, verr.Error())
		return
	}

	matches, err := h.articles.SearchArticles(r.Context(), search, limit+1, cursor)
	if err != nil {
		h.logger.Error("search request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	resp := listResponse[articleMatchResponse]{Items: make([]articleMatchResponse, 0, min(len(matches), limit))}
	if len(matches) > limit {
		matches = matches[:limit]
		last := matches[limit-1]
		next := encodeSearchCursor(repository.SearchCursor{Score: last.Score, At: last.FeedTime(), ID: last.ID})
		resp.NextCursor = &next
	}
	for _, m := range matches {
		resp.Items = append(resp.Items, articleMatchResponse{articleResponse: toArticleResponse(m.Article), Score: m.Score})
	}
	writeJSON(w, http.StatusOK, resp)
}

// parseTimeParam reads an optional RFC 3339 query parameter (the format used
// for published_at everywhere); on invalid input it writes a 400.
func parseTimeParam(w http.ResponseWriter, r *http.Request, name string) (*time.Time, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", name+" must be an RFC 3339 timestamp, e.g. 2026-09-01T00:00:00Z")
		return nil, false
	}
	return &t, true
}

// Search cursors are opaque: base64url("<score>|<feed time RFC3339Nano>|<id>").
// The score is the exact float32 PostgreSQL returned, so pages continue
// without gaps or repeats.
func encodeSearchCursor(c repository.SearchCursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(
		strconv.FormatFloat(float64(c.Score), 'g', -1, 32) + "|" + c.At.UTC().Format(time.RFC3339Nano) + "|" + c.ID))
}

func decodeSearchCursor(raw string) (*repository.SearchCursor, bool) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, false
	}
	parts := strings.Split(string(b), "|")
	if len(parts) != 3 || !domain.IsValidID(parts[2]) {
		return nil, false
	}
	score, err := strconv.ParseFloat(parts[0], 32)
	if err != nil || math.IsNaN(score) || math.IsInf(score, 0) || score < 0 {
		return nil, false
	}
	at, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return nil, false
	}
	return &repository.SearchCursor{Score: float32(score), At: at, ID: parts[2]}, true
}
