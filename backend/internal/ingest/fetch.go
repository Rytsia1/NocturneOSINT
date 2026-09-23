// Package ingest turns a Source's RSS/Atom feed into Articles:
// fetch → parse → normalize → deduplicate → store. It does not crawl, scrape
// HTML, render JavaScript, or create Events, Locations or Evidence.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Ingestion limits. Feeds are small documents; anything larger is refused.
const (
	FetchTimeout    = 15 * time.Second // whole request, including reading the body
	MaxFeedBytes    = 2 << 20          // 2 MiB response body, and so XML document
	MaxFeedItems    = 200              // items considered per ingestion; the rest are ignored
	MaxMediaPerItem = 10               // distinct image URLs recorded per new Article
	MaxRedirects    = 5
	UserAgent       = "Nocturne/0.1 (RSS/Atom feed ingestion)"
	maxErrReports   = 10 // per-item errors returned in a result
)

var (
	ErrBlockedURL      = errors.New("feed URL is not allowed")
	ErrUpstream        = errors.New("feed server request failed")
	ErrUpstreamTimeout = errors.New("feed server timed out")
	ErrFeedTooLarge    = errors.New("feed exceeds the size limit")
)

// Fetcher retrieves exactly one feed URL with a timeout, a size limit, a
// redirect limit and SSRF checks. It never follows links inside the feed.
type Fetcher struct {
	client   *http.Client
	maxBytes int64
	allowIP  func(netip.Addr) bool
}

// NewFetcher builds a Fetcher. allowIP decides which addresses may be
// connected to; production uses IsPublicIP. It is checked on the address
// actually dialed (after DNS resolution, on every redirect), so a hostname
// that resolves or re-resolves to a private address is still refused.
func NewFetcher(timeout time.Duration, maxBytes int64, allowIP func(netip.Addr) bool) *Fetcher {
	f := &Fetcher{maxBytes: maxBytes, allowIP: allowIP}
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil || !allowIP(ap.Addr().Unmap()) {
				return fmt.Errorf("%w: address %s is not public", ErrBlockedURL, address)
			}
			return nil
		},
	}
	f.client = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               nil, // via a proxy the dialed address would be the proxy's, bypassing the check
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: timeout,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > MaxRedirects {
				return fmt.Errorf("%w: more than %d redirects", ErrUpstream, MaxRedirects)
			}
			return f.checkURL(req.URL)
		},
	}
	return f
}

// NewDefaultFetcher is the production Fetcher: public addresses only.
func NewDefaultFetcher() *Fetcher {
	return NewFetcher(FetchTimeout, MaxFeedBytes, IsPublicIP)
}

// Fetch GETs rawURL and returns its body.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBlockedURL, err)
	}
	if err := f.checkURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBlockedURL, err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml;q=0.9, text/xml;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, classify(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%w: status %d", ErrUpstream, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return nil, classify(ctx, err)
	}
	if int64(len(body)) > f.maxBytes {
		return nil, fmt.Errorf("%w of %d bytes", ErrFeedTooLarge, f.maxBytes)
	}
	return body, nil
}

// checkURL allows only http(s) URLs that do not name a local host. Resolved
// addresses are checked again when dialed.
func (f *Fetcher) checkURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme %q", ErrBlockedURL, u.Scheme)
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("%w: host %q", ErrBlockedURL, host)
	}
	if addr, err := netip.ParseAddr(host); err == nil && !f.allowIP(addr.Unmap()) {
		return fmt.Errorf("%w: address %s is not public", ErrBlockedURL, addr)
	}
	return nil
}

func classify(ctx context.Context, err error) error {
	if errors.Is(err, ErrBlockedURL) || errors.Is(err, ErrUpstream) {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err() // the caller gave up; not the feed server's fault
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("%w: %v", ErrUpstreamTimeout, err)
	}
	return fmt.Errorf("%w: %v", ErrUpstream, err)
}

// nonPublic lists ranges the netip predicates below do not cover.
var nonPublic = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this" network
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
}

// IsPublicIP reports whether addr may be fetched from: not loopback, private
// (incl. IPv6 fc00::/7), link-local, CGNAT, multicast or unspecified.
// ponytail: not every IANA special-purpose range (e.g. 198.18.0.0/15); add
// prefixes to nonPublic if one matters.
func IsPublicIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() ||
		addr.IsInterfaceLocalMulticast() {
		return false
	}
	for _, p := range nonPublic {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}
