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

type articleHandler struct {
	logger   *slog.Logger
	articles *repository.ArticleRepository
}

type articleResponse struct {
	ID          string     `json:"id"`
	SourceID    string     `json:"source_id"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	Summary     string     `json:"summary"`
	PublishedAt *time.Time `json:"published_at"`
	RetrievedAt time.Time  `json:"retrieved_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// retrieved_at is deliberately absent: the server records it.
type createArticleRequest struct {
	SourceID    string     `json:"source_id"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	Summary     string     `json:"summary"`
	PublishedAt *time.Time `json:"published_at"`
}

func toArticleResponse(a domain.Article) articleResponse {
	resp := articleResponse{
		ID:          a.ID,
		SourceID:    a.SourceID,
		Title:       a.Title,
		URL:         a.URL,
		Summary:     a.Summary,
		RetrievedAt: a.RetrievedAt.UTC(),
		CreatedAt:   a.CreatedAt.UTC(),
		UpdatedAt:   a.UpdatedAt.UTC(),
	}
	if a.PublishedAt != nil {
		p := a.PublishedAt.UTC()
		resp.PublishedAt = &p
	}
	return resp
}

func (h *articleHandler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createArticleRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"request body must be a JSON object with fields source_id, title, url, summary, published_at (RFC 3339)")
		return
	}

	article, err := domain.NewArticle(req.SourceID, req.Title, req.URL, req.Summary, req.PublishedAt)
	var verr *domain.ValidationError
	if errors.As(err, &verr) {
		writeError(w, http.StatusBadRequest, "validation_failed", verr.Error())
		return
	}

	created, err := h.articles.Create(r.Context(), article)
	switch {
	case errors.Is(err, domain.ErrSourceNotFound):
		writeError(w, http.StatusUnprocessableEntity, "source_not_found", "source_id does not reference an existing source")
		return
	case errors.Is(err, domain.ErrArticleURLExists):
		writeError(w, http.StatusConflict, "article_already_exists", "an article with this URL already exists")
		return
	case err != nil:
		h.internalError(w, err)
		return
	}

	w.Header().Set("Location", "/api/articles/"+created.ID)
	writeJSON(w, http.StatusCreated, toArticleResponse(created))
}

func (h *articleHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !domain.IsValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "article id must be a UUID")
		return
	}

	article, err := h.articles.GetByID(r.Context(), id)
	if errors.Is(err, domain.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article_not_found", "article not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toArticleResponse(article))
}

func (h *articleHandler) list(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("source_id")
	if sourceID != "" && !domain.IsValidID(sourceID) {
		writeError(w, http.StatusBadRequest, "invalid_source_id", "source_id must be a UUID")
		return
	}
	limit, cursor, ok := parsePage(w, r)
	if !ok {
		return
	}

	articles, err := h.articles.List(r.Context(), limit+1, sourceID, cursor)
	if err != nil {
		h.internalError(w, err)
		return
	}

	articles, next := trimPage(articles, limit, func(a domain.Article) (time.Time, string) { return a.FeedTime(), a.ID })
	resp := listResponse[articleResponse]{Items: make([]articleResponse, 0, len(articles)), NextCursor: next}
	for _, a := range articles {
		resp.Items = append(resp.Items, toArticleResponse(a))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *articleHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !domain.IsValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "article id must be a UUID")
		return
	}

	err := h.articles.Delete(r.Context(), id)
	if errors.Is(err, domain.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article_not_found", "article not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *articleHandler) internalError(w http.ResponseWriter, err error) {
	h.logger.Error("article request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}
