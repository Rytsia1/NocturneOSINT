package http

import (
	"errors"
	"log/slog"
	"net/http"

	"nocturne-backend/internal/domain"
	"nocturne-backend/internal/ingest"
)

type ingestHandler struct {
	logger  *slog.Logger
	service *ingest.Service
}

// ingestResponse is counts only: no feed content, no Articles.
type ingestResponse struct {
	SourceID   string            `json:"source_id"`
	Fetched    int               `json:"fetched"`
	Inserted   int               `json:"inserted"`
	Duplicates int               `json:"duplicates"`
	Invalid    int               `json:"invalid"`
	Truncated  bool              `json:"truncated"`
	Errors     []itemErrorOutput `json:"errors"`
}

type itemErrorOutput struct {
	Item   int    `json:"item"`
	Reason string `json:"reason"`
}

// ingest fetches the Source's own URL as an RSS/Atom feed. It takes no body:
// the URL always comes from the stored Source, never from the request.
func (h *ingestHandler) ingest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "source")
	if !ok {
		return
	}

	res, err := h.service.Ingest(r.Context(), id)
	switch {
	case errors.Is(err, domain.ErrSourceNotFound):
		writeError(w, http.StatusNotFound, "source_not_found", "source not found")
	case errors.Is(err, ingest.ErrBlockedURL):
		writeError(w, http.StatusUnprocessableEntity, "source_not_ingestible", "source URL is not an allowed public http(s) address")
	case errors.Is(err, ingest.ErrInvalidFeed):
		writeError(w, http.StatusUnprocessableEntity, "invalid_feed", "source URL did not return a valid RSS or Atom feed")
	case errors.Is(err, ingest.ErrFeedTooLarge):
		writeError(w, http.StatusUnprocessableEntity, "feed_too_large", "feed exceeds the size limit")
	case errors.Is(err, ingest.ErrUpstreamTimeout):
		writeError(w, http.StatusGatewayTimeout, "upstream_timeout", "feed server did not respond in time")
	case errors.Is(err, ingest.ErrUpstream):
		h.logger.Warn("feed fetch failed", "source_id", id, "error", err)
		writeError(w, http.StatusBadGateway, "upstream_error", "feed server request failed")
	case err != nil:
		h.logger.Error("ingest request failed", "source_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	default:
		resp := ingestResponse{
			SourceID: res.SourceID, Fetched: res.Fetched, Inserted: res.Inserted,
			Duplicates: res.Duplicates, Invalid: res.Invalid, Truncated: res.Truncated,
			Errors: make([]itemErrorOutput, 0, len(res.Errors)),
		}
		for _, e := range res.Errors {
			resp.Errors = append(resp.Errors, itemErrorOutput{Item: e.Item, Reason: e.Reason})
		}
		h.logger.Info("feed ingested", "source_id", id, "fetched", res.Fetched, "inserted", res.Inserted,
			"duplicates", res.Duplicates, "invalid", res.Invalid)
		writeJSON(w, http.StatusOK, resp)
	}
}
