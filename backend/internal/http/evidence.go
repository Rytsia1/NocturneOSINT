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

type evidenceHandler struct {
	logger   *slog.Logger
	evidence *repository.EvidenceRepository
}

type evidenceResponse struct {
	ID        string    `json:"id"`
	ArticleID string    `json:"article_id"`
	EventID   string    `json:"event_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// eventArticleResponse is a compact Article (no summary) with its provenance
// fields, as listed for an Event.
type eventArticleResponse struct {
	EvidenceID  string     `json:"evidence_id"`
	ArticleID   string     `json:"article_id"`
	SourceID    string     `json:"source_id"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	PublishedAt *time.Time `json:"published_at"`
	RetrievedAt time.Time  `json:"retrieved_at"`
}

// articleEventResponse is a compact Event (no description), as listed for an
// Article.
type articleEventResponse struct {
	EvidenceID          string     `json:"evidence_id"`
	EventID             string     `json:"event_id"`
	Title               string     `json:"title"`
	OccurredAt          *time.Time `json:"occurred_at"`
	OccurredAtPrecision *string    `json:"occurred_at_precision"`
}

type createEvidenceRequest struct {
	ArticleID string `json:"article_id"`
	EventID   string `json:"event_id"`
}

func toEvidenceResponse(e domain.Evidence) evidenceResponse {
	return evidenceResponse{ID: e.ID, ArticleID: e.ArticleID, EventID: e.EventID, CreatedAt: e.CreatedAt.UTC(), UpdatedAt: e.UpdatedAt.UTC()}
}

func (h *evidenceHandler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createEvidenceRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be a JSON object with fields article_id, event_id")
		return
	}
	evidence, err := domain.NewEvidence(req.ArticleID, req.EventID)
	if writeValidationError(w, err) {
		return
	}

	created, err := h.evidence.Create(r.Context(), evidence)
	switch {
	case errors.Is(err, domain.ErrArticleNotFound):
		writeError(w, http.StatusUnprocessableEntity, "article_not_found", "article_id does not reference an existing article")
	case errors.Is(err, domain.ErrEventNotFound):
		writeError(w, http.StatusUnprocessableEntity, "event_not_found", "event_id does not reference an existing event")
	case errors.Is(err, domain.ErrEvidenceExists):
		writeError(w, http.StatusConflict, "evidence_exists", "evidence already links this article and event")
	case err != nil:
		h.internalError(w, err)
	default:
		w.Header().Set("Location", "/api/evidence/"+created.ID)
		writeJSON(w, http.StatusCreated, toEvidenceResponse(created))
	}
}

func (h *evidenceHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "evidence")
	if !ok {
		return
	}
	e, err := h.evidence.GetByID(r.Context(), id)
	if errors.Is(err, domain.ErrEvidenceNotFound) {
		writeError(w, http.StatusNotFound, "evidence_not_found", "evidence not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toEvidenceResponse(e))
}

func (h *evidenceHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "evidence")
	if !ok {
		return
	}
	err := h.evidence.Delete(r.Context(), id)
	if errors.Is(err, domain.ErrEvidenceNotFound) {
		writeError(w, http.StatusNotFound, "evidence_not_found", "evidence not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// eventArticles lists the Articles linked to an Event, in Article feed order.
func (h *evidenceHandler) eventArticles(w http.ResponseWriter, r *http.Request) {
	eventID, ok := pathID(w, r, "id", "event")
	if !ok {
		return
	}
	limit, cursor, ok := parsePage(w, r)
	if !ok {
		return
	}
	items, err := h.evidence.ArticlesForEvent(r.Context(), eventID, limit+1, cursor)
	if errors.Is(err, domain.ErrEventNotFound) {
		writeError(w, http.StatusNotFound, "event_not_found", "event not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}

	items, next := trimPage(items, limit, func(x repository.EvidenceArticle) (time.Time, string) { return x.FeedTime(), x.ID })
	resp := listResponse[eventArticleResponse]{Items: make([]eventArticleResponse, 0, len(items)), NextCursor: next}
	for _, x := range items {
		item := eventArticleResponse{
			EvidenceID: x.EvidenceID, ArticleID: x.ID, SourceID: x.SourceID,
			Title: x.Title, URL: x.URL, RetrievedAt: x.RetrievedAt.UTC(),
		}
		if x.PublishedAt != nil {
			at := x.PublishedAt.UTC()
			item.PublishedAt = &at
		}
		resp.Items = append(resp.Items, item)
	}
	writeJSON(w, http.StatusOK, resp)
}

// articleEvents lists the Events linked to an Article, in Event feed order.
func (h *evidenceHandler) articleEvents(w http.ResponseWriter, r *http.Request) {
	articleID, ok := pathID(w, r, "id", "article")
	if !ok {
		return
	}
	limit, cursor, ok := parsePage(w, r)
	if !ok {
		return
	}
	items, err := h.evidence.EventsForArticle(r.Context(), articleID, limit+1, cursor)
	if errors.Is(err, domain.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article_not_found", "article not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}

	items, next := trimPage(items, limit, func(x repository.EvidenceEvent) (time.Time, string) { return x.FeedTime(), x.ID })
	resp := listResponse[articleEventResponse]{Items: make([]articleEventResponse, 0, len(items)), NextCursor: next}
	for _, x := range items {
		e := toEventResponse(x.Event)
		resp.Items = append(resp.Items, articleEventResponse{
			EvidenceID: x.EvidenceID, EventID: e.ID, Title: e.Title,
			OccurredAt: e.OccurredAt, OccurredAtPrecision: e.OccurredAtPrecision,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *evidenceHandler) internalError(w http.ResponseWriter, err error) {
	h.logger.Error("evidence request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}
