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

type articleMediaHandler struct {
	logger *slog.Logger
	media  *repository.ArticleMediaRepository
}

// articleMediaResponse is metadata only: the client loads the external URL
// itself; Nocturne never serves or proxies the image.
type articleMediaResponse struct {
	ID        string    `json:"id"`
	ArticleID string    `json:"article_id"`
	URL       string    `json:"url"`
	MediaType string    `json:"media_type"`
	Width     *int      `json:"width"`
	Height    *int      `json:"height"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type createArticleMediaRequest struct {
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
	Width     *int   `json:"width"`
	Height    *int   `json:"height"`
}

func toArticleMediaResponse(m domain.ArticleMedia) articleMediaResponse {
	return articleMediaResponse{
		ID: m.ID, ArticleID: m.ArticleID, URL: m.URL, MediaType: m.MediaType, Width: m.Width, Height: m.Height,
		CreatedAt: m.CreatedAt.UTC(), UpdatedAt: m.UpdatedAt.UTC(),
	}
}

func (h *articleMediaHandler) create(w http.ResponseWriter, r *http.Request) {
	articleID, ok := pathID(w, r, "id", "article")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createArticleMediaRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"request body must be a JSON object with fields url, media_type, width (integer), height (integer)")
		return
	}
	media, err := domain.NewArticleMedia(articleID, req.URL, req.MediaType, req.Width, req.Height)
	if writeValidationError(w, err) {
		return
	}

	created, err := h.media.Create(r.Context(), media)
	switch {
	case errors.Is(err, domain.ErrArticleNotFound):
		writeError(w, http.StatusNotFound, "article_not_found", "article not found")
	case errors.Is(err, domain.ErrArticleMediaExists):
		writeError(w, http.StatusConflict, "article_media_exists", "this media URL is already attached to the article")
	case err != nil:
		h.internalError(w, err)
	default:
		w.Header().Set("Location", "/api/articles/"+articleID+"/media/"+created.ID)
		writeJSON(w, http.StatusCreated, toArticleMediaResponse(created))
	}
}

func (h *articleMediaHandler) list(w http.ResponseWriter, r *http.Request) {
	articleID, ok := pathID(w, r, "id", "article")
	if !ok {
		return
	}
	media, err := h.media.ListByArticle(r.Context(), articleID, maxPageSize+1)
	if errors.Is(err, domain.ErrArticleNotFound) {
		writeError(w, http.StatusNotFound, "article_not_found", "article not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}

	resp := struct {
		Items     []articleMediaResponse `json:"items"`
		Truncated bool                   `json:"truncated"`
	}{Truncated: len(media) > maxPageSize}
	media = media[:min(len(media), maxPageSize)]
	resp.Items = make([]articleMediaResponse, 0, len(media))
	for _, m := range media {
		resp.Items = append(resp.Items, toArticleMediaResponse(m))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *articleMediaHandler) get(w http.ResponseWriter, r *http.Request) {
	articleID, mediaID, ok := h.ids(w, r)
	if !ok {
		return
	}
	m, err := h.media.GetByID(r.Context(), articleID, mediaID)
	if errors.Is(err, domain.ErrArticleMediaNotFound) {
		writeError(w, http.StatusNotFound, "article_media_not_found", "media not found for this article")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toArticleMediaResponse(m))
}

func (h *articleMediaHandler) delete(w http.ResponseWriter, r *http.Request) {
	articleID, mediaID, ok := h.ids(w, r)
	if !ok {
		return
	}
	err := h.media.Delete(r.Context(), articleID, mediaID)
	if errors.Is(err, domain.ErrArticleMediaNotFound) {
		writeError(w, http.StatusNotFound, "article_media_not_found", "media not found for this article")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *articleMediaHandler) ids(w http.ResponseWriter, r *http.Request) (articleID, mediaID string, ok bool) {
	if articleID, ok = pathID(w, r, "id", "article"); !ok {
		return "", "", false
	}
	if mediaID, ok = pathID(w, r, "media_id", "media"); !ok {
		return "", "", false
	}
	return articleID, mediaID, true
}

func (h *articleMediaHandler) internalError(w http.ResponseWriter, err error) {
	h.logger.Error("article media request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}
