package ingest

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"nocturne-backend/internal/domain"
)

// ErrInvalidFeed means the document is not well-formed RSS 2.0 or Atom XML.
var ErrInvalidFeed = errors.New("not a valid RSS or Atom feed")

const atomNS = "http://www.w3.org/2005/Atom"

// item is one feed entry as found in the document, before normalization.
// Both formats map onto it; nothing here is exposed through the API.
type item struct {
	Title     string
	Link      string
	Summary   string
	Published string     // raw date text; "" when the feed gives none
	Media     []rawMedia // image metadata stated by the feed, in document order
}

// rawMedia is an image reference exactly as the feed states it. The image
// itself is never fetched.
type rawMedia struct {
	URL, Width, Height string // Width/Height "" when not stated
}

type feedDoc struct {
	XMLName xml.Name
	Items   []rssItem   `xml:"channel>item"` // RSS 2.0
	Entries []atomEntry `xml:"entry"`        // Atom
}

// Unqualified tags such as "title" also match namespaced elements like
// <media:title>, so text fields are slices and the first non-empty wins.
type rssItem struct {
	Titles       []string       `xml:"title"`
	Links        []string       `xml:"link"` // an <atom:link/> may sit beside <link>
	Descriptions []string       `xml:"description"`
	PubDate      string         `xml:"pubDate"`
	Enclosures   []mediaElement `xml:"enclosure"`
	mediaElements
}

type atomEntry struct {
	Titles []string `xml:"title"`
	Links  []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
		Type string `xml:"type,attr"`
	} `xml:"link"`
	Summaries []string `xml:"summary"`
	Published string   `xml:"published"`
	mediaElements
}

// mediaElements are the Media RSS (http://search.yahoo.com/mrss/) image
// elements, valid in RSS items and Atom entries.
type mediaElements struct {
	Thumbnails []mediaElement `xml:"http://search.yahoo.com/mrss/ thumbnail"`
	Contents   []mediaElement `xml:"http://search.yahoo.com/mrss/ content"`
}

type mediaElement struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Medium string `xml:"medium,attr"`
	Width  string `xml:"width,attr"`
	Height string `xml:"height,attr"`
}

func isImageType(mimeType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/")
}

// images returns the Media RSS images: every <media:thumbnail>, and each
// <media:content> whose type is image/* (or, without a type, medium="image").
func (m mediaElements) images() []rawMedia {
	var out []rawMedia
	for _, t := range m.Thumbnails {
		out = append(out, rawMedia{t.URL, t.Width, t.Height})
	}
	for _, c := range m.Contents {
		if isImageType(c.Type) || (c.Type == "" && c.Medium == "image") {
			out = append(out, rawMedia{c.URL, c.Width, c.Height})
		}
	}
	return out
}

// parseFeed reads an RSS 2.0 (<rss>) or Atom (<feed> in the Atom namespace)
// document. Anything else, including HTML, is ErrInvalidFeed; there is no
// fallback to HTML. encoding/xml does not resolve external entities.
func parseFeed(body []byte) ([]item, error) {
	var doc feedDoc
	if err := xml.NewDecoder(bytes.NewReader(body)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFeed, err)
	}

	var items []item
	switch {
	case doc.XMLName.Local == "rss" && doc.XMLName.Space == "":
		for _, it := range doc.Items {
			media := it.images()
			for _, enc := range it.Enclosures {
				if isImageType(enc.Type) { // audio/video/other enclosures are not media here
					media = append(media, rawMedia{URL: enc.URL})
				}
			}
			items = append(items, item{Title: firstNonEmpty(it.Titles), Link: firstNonEmpty(it.Links),
				Summary: firstNonEmpty(it.Descriptions), Published: it.PubDate, Media: media})
		}
	case doc.XMLName.Local == "feed" && doc.XMLName.Space == atomNS:
		for _, e := range doc.Entries {
			var link string
			media := e.images()
			for _, l := range e.Links {
				if link == "" && (l.Rel == "" || l.Rel == "alternate") {
					link = l.Href
				}
				if l.Rel == "enclosure" && isImageType(l.Type) {
					media = append(media, rawMedia{URL: l.Href})
				}
			}
			// Atom <updated> is a modification time, not a publication
			// time, so it is never used as published_at.
			items = append(items, item{Title: firstNonEmpty(e.Titles), Link: link,
				Summary: firstNonEmpty(e.Summaries), Published: e.Published, Media: media})
		}
	default:
		return nil, fmt.Errorf("%w: root element <%s>", ErrInvalidFeed, doc.XMLName.Local)
	}
	return items, nil
}

func firstNonEmpty(values []string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// Date layouts seen in feeds: RFC 3339 for Atom, RFC 822/1123 variants for RSS.
var dateLayouts = []string{
	time.RFC3339,
	time.RFC1123Z,
	time.RFC1123,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700",
	"Mon, 02 Jan 2006 15:04 -0700",
}

// parseDate returns the time in UTC, or nil when the text is empty or not in
// a recognised layout: an unreadable date stays unknown, never "now".
func parseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

// plainText turns a feed summary into bounded plain text: tags removed,
// entities decoded, whitespace collapsed, cut to the Article summary limit.
// ponytail: regex tag stripping; switch to an HTML tokenizer if summaries need
// more fidelity.
func plainText(s string) string {
	s = strings.Join(strings.Fields(html.UnescapeString(tagPattern.ReplaceAllString(s, " "))), " ")
	if utf8.RuneCountInString(s) > domain.MaxArticleSummaryLength {
		s = string([]rune(s)[:domain.MaxArticleSummaryLength])
	}
	return s
}

// normalize validates an item as an Article of the Source. The title is
// trimmed and the link only has surrounding whitespace removed: the URL is
// otherwise kept exactly as the feed gives it.
func normalize(it item, sourceID string) (domain.Article, error) {
	return domain.NewArticle(sourceID, it.Title, strings.TrimSpace(it.Link), plainText(it.Summary), parseDate(it.Published))
}

// normalizeMedia validates a stated image as media of the Article. The URL is
// kept exactly as given; dimensions are used only when stated as integers,
// never inferred. Malformed metadata is an error, not silently repaired.
func normalizeMedia(m rawMedia, articleID string) (domain.ArticleMedia, error) {
	width, err := parseDimension("width", m.Width)
	if err != nil {
		return domain.ArticleMedia{}, err
	}
	height, err := parseDimension("height", m.Height)
	if err != nil {
		return domain.ArticleMedia{}, err
	}
	return domain.NewArticleMedia(articleID, m.URL, "image", width, height)
}

func parseDimension(field, s string) (*int, error) {
	if s == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil, &domain.ValidationError{Field: field, Message: "must be an integer"}
	}
	return &n, nil
}
