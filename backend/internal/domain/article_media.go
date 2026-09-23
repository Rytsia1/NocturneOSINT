package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// MaxMediaDimension bounds stated image width and height, in pixels.
const MaxMediaDimension = 20000

var (
	ErrArticleMediaNotFound = errors.New("article media not found")
	ErrArticleMediaExists   = errors.New("this media URL is already attached to the article")
)

// MediaTypes lists supported media types; Step 8 records images only.
var MediaTypes = []string{"image"}

// ArticleMedia is metadata about an externally hosted image of an Article.
// Nocturne stores the URL and stated dimensions only, never the image itself.
type ArticleMedia struct {
	ID        string
	ArticleID string
	URL       string // external; stored exactly as given, never fetched
	MediaType string
	Width     *int // as stated by the source; nil when not stated
	Height    *int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewArticleMedia validates input and returns unsaved ArticleMedia. Whether the
// Article exists is checked when it is stored.
func NewArticleMedia(articleID, rawURL, mediaType string, width, height *int) (ArticleMedia, error) {
	if !IsValidID(articleID) {
		return ArticleMedia{}, &ValidationError{"article_id", "must be a UUID"}
	}
	if err := validateURL(rawURL); err != nil {
		return ArticleMedia{}, err
	}
	if !contains(MediaTypes, mediaType) {
		return ArticleMedia{}, &ValidationError{"media_type", "must be one of " + strings.Join(MediaTypes, ", ")}
	}
	if err := checkDimension("width", width); err != nil {
		return ArticleMedia{}, err
	}
	if err := checkDimension("height", height); err != nil {
		return ArticleMedia{}, err
	}
	return ArticleMedia{ArticleID: articleID, URL: rawURL, MediaType: mediaType, Width: width, Height: height}, nil
}

func checkDimension(field string, v *int) error {
	if v != nil && (*v < 1 || *v > MaxMediaDimension) {
		return &ValidationError{field, fmt.Sprintf("must be between 1 and %d", MaxMediaDimension)}
	}
	return nil
}
