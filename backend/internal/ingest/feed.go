package ingest

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"regexp"
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
	Published string // raw date text; "" when the feed gives none
}

type feedDoc struct {
	XMLName xml.Name
	Items   []rssItem   `xml:"channel>item"` // RSS 2.0
	Entries []atomEntry `xml:"entry"`        // Atom
}

type rssItem struct {
	Title       string   `xml:"title"`
	Links       []string `xml:"link"` // a slice: an <atom:link/> may sit beside <link>
	Description string   `xml:"description"`
	PubDate     string   `xml:"pubDate"`
}

type atomEntry struct {
	Title string `xml:"title"`
	Links []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
	} `xml:"link"`
	Summary   string `xml:"summary"`
	Published string `xml:"published"`
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
			items = append(items, item{Title: it.Title, Link: firstNonEmpty(it.Links), Summary: it.Description, Published: it.PubDate})
		}
	case doc.XMLName.Local == "feed" && doc.XMLName.Space == atomNS:
		for _, e := range doc.Entries {
			var link string
			for _, l := range e.Links {
				if l.Rel == "" || l.Rel == "alternate" {
					link = l.Href
					break
				}
			}
			// Atom <updated> is a modification time, not a publication
			// time, so it is never used as published_at.
			items = append(items, item{Title: e.Title, Link: link, Summary: e.Summary, Published: e.Published})
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
