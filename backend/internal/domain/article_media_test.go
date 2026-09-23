package domain

import (
	"strings"
	"testing"
)

func ip(n int) *int { return &n }

func TestNewArticleMedia_Valid(t *testing.T) {
	article := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	tests := []struct {
		name          string
		width, height *int
	}{
		{"no dimensions", nil, nil},
		{"both dimensions", ip(1200), ip(800)},
		{"width only", ip(640), nil},
		{"at the maximum", ip(MaxMediaDimension), ip(MaxMediaDimension)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "https://cdn.example.test/img/1.jpg?w=1200&utm_source=rss"
			m, err := NewArticleMedia(article, url, "image", tt.width, tt.height)
			if err != nil {
				t.Fatalf("NewArticleMedia: %v", err)
			}
			if m.URL != url || m.MediaType != "image" || m.Width != tt.width || m.Height != tt.height {
				t.Errorf("media = %+v, want URL kept verbatim and dimensions as stated", m)
			}
		})
	}
}

func TestNewArticleMedia_Invalid(t *testing.T) {
	article := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	img := "https://cdn.example.test/1.jpg"
	tests := []struct {
		name, article, url, mediaType string
		width, height                 *int
		field                         string
	}{
		{"missing article_id", "", img, "image", nil, nil, "article_id"},
		{"invalid article_id", "abc", img, "image", nil, nil, "article_id"},
		{"missing url", article, "", "image", nil, nil, "url"},
		{"relative url", article, "/img/1.jpg", "image", nil, nil, "url"},
		{"javascript url", article, "javascript:alert(1)", "image", nil, nil, "url"},
		{"data url", article, "data:image/png;base64,AAAA", "image", nil, nil, "url"},
		{"file url", article, "file:///etc/passwd", "image", nil, nil, "url"},
		{"ftp url", article, "ftp://example.test/1.jpg", "image", nil, nil, "url"},
		{"whitespace in url", article, " https://cdn.example.test/1.jpg", "image", nil, nil, "url"},
		{"oversized url", article, "https://cdn.example.test/" + strings.Repeat("a", MaxURLLength), "image", nil, nil, "url"},
		{"missing media_type", article, img, "", nil, nil, "media_type"},
		{"video media_type", article, img, "video", nil, nil, "media_type"},
		{"MIME type instead of media_type", article, img, "image/jpeg", nil, nil, "media_type"},
		{"zero width", article, img, "image", ip(0), nil, "width"},
		{"negative width", article, img, "image", ip(-5), nil, "width"},
		{"width above maximum", article, img, "image", ip(MaxMediaDimension + 1), nil, "width"},
		{"zero height", article, img, "image", nil, ip(0), "height"},
		{"height above maximum", article, img, "image", ip(1200), ip(MaxMediaDimension + 1), "height"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewArticleMedia(tt.article, tt.url, tt.mediaType, tt.width, tt.height)
			assertField(t, err, tt.field)
		})
	}
}
