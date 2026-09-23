package ingest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// Fetcher tests use in-process servers on 127.0.0.1, so they allow loopback;
// the production policy is exercised in TestFetchBlocksNonPublicTargets.
func allowAll(netip.Addr) bool { return true }

func testFetcher(timeout time.Duration, maxBytes int64) *Fetcher {
	return NewFetcher(timeout, maxBytes, allowAll)
}

func TestFetch(t *testing.T) {
	var gotUA string
	mux := http.NewServeMux()
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html") // Content-Type is not trusted either way
		w.Write([]byte(rssFixture))
	})
	mux.HandleFunc("/missing", http.NotFound)
	mux.HandleFunc("/broken", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) })
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", 101))) })
	mux.HandleFunc("/exact", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", 100))) })
	mux.HandleFunc("/html", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html><body>not a feed</body></html>")) })
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/feed", http.StatusFound) })
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	mux.HandleFunc("/to-file", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
	})
	mux.HandleFunc("/to-private", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://10.0.0.1/feed", http.StatusFound)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := testFetcher(300*time.Millisecond, 100)
	ctx := context.Background()

	t.Run("200 feed", func(t *testing.T) {
		body, err := testFetcher(time.Second, MaxFeedBytes).Fetch(ctx, srv.URL+"/feed")
		if err != nil || string(body) != rssFixture {
			t.Fatalf("Fetch = %d bytes, %v", len(body), err)
		}
		if gotUA != UserAgent {
			t.Errorf("User-Agent = %q, want %q", gotUA, UserAgent)
		}
	})
	t.Run("redirect followed", func(t *testing.T) {
		if body, err := testFetcher(time.Second, MaxFeedBytes).Fetch(ctx, srv.URL+"/moved"); err != nil || string(body) != rssFixture {
			t.Errorf("Fetch via redirect = %d bytes, %v", len(body), err)
		}
	})
	t.Run("body at the limit", func(t *testing.T) {
		if body, err := f.Fetch(ctx, srv.URL+"/exact"); err != nil || len(body) != 100 {
			t.Errorf("Fetch = %d bytes, %v", len(body), err)
		}
	})
	t.Run("invalid content is fetched, then rejected by the parser", func(t *testing.T) {
		body, err := f.Fetch(ctx, srv.URL+"/html")
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if _, err := parseFeed(body); !errors.Is(err, ErrInvalidFeed) {
			t.Errorf("parseFeed(HTML) err = %v, want ErrInvalidFeed", err)
		}
	})

	for _, tt := range []struct {
		name, path string
		want       error
	}{
		{"404", "/missing", ErrUpstream},
		{"500", "/broken", ErrUpstream},
		{"oversized response", "/big", ErrFeedTooLarge},
		{"timeout", "/slow", ErrUpstreamTimeout},
		{"redirect loop", "/loop", ErrUpstream},
		{"redirect to file scheme", "/to-file", ErrBlockedURL},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.Fetch(ctx, srv.URL+tt.path); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}

	t.Run("redirect to a private address", func(t *testing.T) {
		loopbackOnly := NewFetcher(time.Second, MaxFeedBytes, func(a netip.Addr) bool { return a.IsLoopback() })
		if _, err := loopbackOnly.Fetch(ctx, srv.URL+"/to-private"); !errors.Is(err, ErrBlockedURL) {
			t.Errorf("err = %v, want ErrBlockedURL", err)
		}
	})
	t.Run("context cancellation", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(50*time.Millisecond, cancel)
		if _, err := testFetcher(5*time.Second, MaxFeedBytes).Fetch(cctx, srv.URL+"/slow"); !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	})
}

// The production policy, with literal addresses and reserved names only, so
// no test depends on the machine's DNS.
func TestFetchBlocksNonPublicTargets(t *testing.T) {
	f := NewDefaultFetcher()
	for _, u := range []string{
		"http://localhost/feed",
		"http://LOCALHOST./feed",
		"http://feed.localhost/feed",
		"http://127.0.0.1/feed",
		"http://127.9.9.9:8080/feed",
		"http://[::1]/feed",
		"http://[::ffff:127.0.0.1]/feed",
		"http://0.0.0.0/feed",
		"http://10.0.0.1/feed",
		"http://172.16.5.4/feed",
		"http://192.168.1.1/feed",
		"http://100.64.0.1/feed",
		"http://169.254.169.254/latest/meta-data",
		"http://[fc00::1]/feed",
		"http://[fd12:3456::1]/feed",
		"http://[fe80::1]/feed",
		"ftp://example.test/feed",
		"file:///etc/passwd",
		"gopher://example.test/",
		"/relative/feed",
	} {
		t.Run(u, func(t *testing.T) {
			if _, err := f.Fetch(context.Background(), u); !errors.Is(err, ErrBlockedURL) {
				t.Errorf("err = %v, want ErrBlockedURL", err)
			}
		})
	}
}

// A hostname passes the URL check; its resolved address is checked when
// dialing. Dialing a loopback address directly exercises that guard.
func TestDialRejectsNonPublicAddress(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	transport := NewDefaultFetcher().client.Transport.(*http.Transport)
	conn, err := transport.DialContext(context.Background(), "tcp", srv.Listener.Addr().String())
	if conn != nil {
		conn.Close()
	}
	if !errors.Is(err, ErrBlockedURL) {
		t.Errorf("dial %s err = %v, want ErrBlockedURL", srv.Listener.Addr(), err)
	}
}

func TestIsPublicIP(t *testing.T) {
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:4700:4700::1111"} {
		if !IsPublicIP(netip.MustParseAddr(s)) {
			t.Errorf("%s rejected, want public", s)
		}
	}
	for _, s := range []string{
		"127.0.0.1", "10.1.2.3", "172.31.255.255", "192.168.0.1", "169.254.1.1", "100.100.0.1",
		"0.0.0.0", "0.1.2.3", "224.0.0.1", "::1", "::", "fc00::1", "fe80::1", "ff02::1", "::ffff:10.0.0.1",
	} {
		if IsPublicIP(netip.MustParseAddr(s)) {
			t.Errorf("%s allowed, want rejected", s)
		}
	}
}
