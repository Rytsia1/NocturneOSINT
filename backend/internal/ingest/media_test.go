package ingest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"nocturne-backend/internal/domain"
)

const mediaRSS = `<?xml version="1.0"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
<channel><title>Media</title>
  <item>
    <title>With images</title>
    <media:title>Media title must not replace the item title</media:title>
    <link>https://example.test/a</link>
    <description>Item text</description>
    <media:description>Media description</media:description>
    <media:thumbnail url="https://img.example.test/thumb.jpg" width="1200" height="800"/>
    <media:content url="https://img.example.test/content.jpg" type="image/jpeg"/>
    <media:content url="https://img.example.test/clip.mp4" type="video/mp4" width="640" height="360"/>
    <media:content url="https://img.example.test/medium.png" medium="image"/>
    <enclosure url="https://img.example.test/enclosure.png" type="image/png" length="12345"/>
    <enclosure url="https://img.example.test/podcast.mp3" type="audio/mpeg" length="999"/>
  </item>
  <item>
    <title>No image</title>
    <link>https://example.test/b</link>
  </item>
</channel></rss>`

const mediaAtom = `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/">
  <entry>
    <title>Atom with images</title>
    <link href="https://example.test/atom"/>
    <link rel="enclosure" type="image/jpeg" href="https://img.example.test/atom.jpg"/>
    <link rel="enclosure" type="audio/mpeg" href="https://img.example.test/atom.mp3"/>
    <link rel="related" type="image/png" href="https://img.example.test/related.png"/>
    <media:thumbnail url="https://img.example.test/atom-thumb.jpg"/>
  </entry>
</feed>`

func TestParseMedia(t *testing.T) {
	items, err := parseFeed([]byte(mediaRSS))
	if err != nil {
		t.Fatalf("parseFeed: %v", err)
	}
	if items[0].Title != "With images" || items[0].Summary != "Item text" {
		t.Errorf("title/summary = %q/%q, want the item's own, not <media:*>", items[0].Title, items[0].Summary)
	}
	want := []rawMedia{
		{"https://img.example.test/thumb.jpg", "1200", "800"},
		{"https://img.example.test/content.jpg", "", ""},
		{"https://img.example.test/medium.png", "", ""},
		{"https://img.example.test/enclosure.png", "", ""},
	}
	if !reflect.DeepEqual(items[0].Media, want) {
		t.Errorf("RSS media = %v, want %v (no video/audio)", items[0].Media, want)
	}
	if len(items[1].Media) != 0 {
		t.Errorf("item without images has media %v", items[1].Media)
	}

	items, err = parseFeed([]byte(mediaAtom))
	if err != nil {
		t.Fatalf("parseFeed atom: %v", err)
	}
	want = []rawMedia{{"https://img.example.test/atom-thumb.jpg", "", ""}, {"https://img.example.test/atom.jpg", "", ""}}
	if !reflect.DeepEqual(items[0].Media, want) || items[0].Link != "https://example.test/atom" {
		t.Errorf("Atom media = %v link %q, want %v and the alternate link (only image enclosures)", items[0].Media, items[0].Link, want)
	}
}

func TestNormalizeMedia(t *testing.T) {
	article := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	m, err := normalizeMedia(rawMedia{"https://img.example.test/1.jpg", "1200", "800"}, article)
	if err != nil || *m.Width != 1200 || *m.Height != 800 || m.MediaType != "image" {
		t.Fatalf("normalizeMedia = %+v, %v", m, err)
	}
	if m, err := normalizeMedia(rawMedia{URL: "https://img.example.test/1.jpg"}, article); err != nil || m.Width != nil || m.Height != nil {
		t.Errorf("unstated dimensions = %v×%v, %v; want nil (never inferred)", m.Width, m.Height, err)
	}
	for _, tt := range []struct {
		raw   rawMedia
		field string
	}{
		{rawMedia{"https://img.example.test/1.jpg", "wide", ""}, "width"},
		{rawMedia{"https://img.example.test/1.jpg", "12.5", ""}, "width"},
		{rawMedia{"https://img.example.test/1.jpg", "0", ""}, "width"},
		{rawMedia{"https://img.example.test/1.jpg", "100", "-1"}, "height"},
		{rawMedia{"https://img.example.test/1.jpg", "", "99999"}, "height"},
		{rawMedia{"", "", ""}, "url"},
		{rawMedia{"javascript:alert(1)", "", ""}, "url"},
		{rawMedia{"/relative.jpg", "", ""}, "url"},
	} {
		_, err := normalizeMedia(tt.raw, article)
		var verr *domain.ValidationError
		if !errors.As(err, &verr) || verr.Field != tt.field {
			t.Errorf("normalizeMedia(%+v) err = %v, want validation error on %s", tt.raw, err, tt.field)
		}
	}
}

// Media is recorded from feed metadata only: the image URLs point at a
// counting server and at private/loopback addresses, and none is requested.
func TestIntegration_IngestMediaWithoutFetchingImages(t *testing.T) {
	f := newFixture(t)
	var imageHits atomic.Int64
	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { imageHits.Add(1) }))
	defer images.Close()

	r := run()
	link := func(s string) string { return "https://example.test/" + r + "/" + s }
	thumb := images.URL + "/thumb.jpg"
	src := f.source(t, "/media", `<?xml version="1.0"?><rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/"><channel><title>T</title>`+
		fmt.Sprintf(`<item><title>Images</title><link>%s</link>
			<media:thumbnail url="%s" width="1200" height="800"/>
			<enclosure url="http://localhost/enclosure.jpg" type="image/jpeg" length="1"/>
			<media:content url="http://10.0.0.1/content.jpg" type="image/jpeg"/>
			<media:content url="http://127.0.0.1/content.jpg" type="image/jpeg"/></item>`, link("images"), thumb)+
		fmt.Sprintf(`<item><title>Plain</title><link>%s</link></item>`, link("plain"))+
		fmt.Sprintf(`<item><title>Bad media</title><link>%s</link>
			<media:thumbnail url="%s/bad.jpg" width="wide"/>
			<enclosure url="javascript:alert(1)" type="image/png" length="1"/>
			<media:content url="%s/good.jpg" type="image/jpeg"/>
			<media:content url="%s/good.jpg" medium="image"/></item>`, link("bad"), images.URL, images.URL, images.URL)+
		fmt.Sprintf(`<item><title>Podcast</title><link>%s</link><enclosure url="%s/ep.mp3" type="audio/mpeg" length="1"/></item>`, link("podcast"), images.URL)+
		`</channel></rss>`)

	res, err := f.service.Ingest(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	assertCounts(t, res, 4, 4, 0, 0)
	if res.Media != 5 || res.InvalidMedia != 2 {
		t.Errorf("media = %d stored, %d invalid; want 5, 2", res.Media, res.InvalidMedia)
	}

	stored := f.stored(t, src.ID)
	media := func(key string) []domain.ArticleMedia {
		t.Helper()
		m, err := f.media.ListByArticle(context.Background(), stored[link(key)].ID, 20)
		if err != nil {
			t.Fatalf("ListByArticle(%s): %v", key, err)
		}
		return m
	}

	// Document order: Media RSS thumbnail, then content, then enclosures.
	var urls []string
	got := media("images")
	for _, m := range got {
		urls = append(urls, m.URL)
	}
	wantURLs := []string{thumb, "http://10.0.0.1/content.jpg", "http://127.0.0.1/content.jpg", "http://localhost/enclosure.jpg"}
	if !reflect.DeepEqual(urls, wantURLs) {
		t.Errorf("images article media URLs = %v, want %v", urls, wantURLs)
	}
	if len(got) == 4 && (got[0].Width == nil || *got[0].Width != 1200 || *got[0].Height != 800 || got[1].Width != nil) {
		t.Errorf("dimensions = %v×%v then %v; want 1200×800 as stated, then none", got[0].Width, got[0].Height, got[1].Width)
	}
	if n := len(media("plain")); n != 0 {
		t.Errorf("article without images has %d media", n)
	}
	if got := media("bad"); len(got) != 1 || got[0].URL != images.URL+"/good.jpg" {
		t.Errorf("bad-media article media = %+v, want only the valid, de-duplicated image", got)
	}
	if n := len(media("podcast")); n != 0 {
		t.Errorf("audio enclosure stored as media (%d rows)", n)
	}

	// Re-ingesting changes nothing: existing Articles and their media stay.
	res, err = f.service.Ingest(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	assertCounts(t, res, 4, 0, 4, 0)
	if res.Media != 0 || len(media("images")) != 4 {
		t.Errorf("re-ingest stored %d media; images article has %d", res.Media, len(media("images")))
	}

	if n := imageHits.Load(); n != 0 {
		t.Errorf("image server received %d requests; ingestion must never fetch images", n)
	}
}
