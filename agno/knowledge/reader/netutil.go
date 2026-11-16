package reader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// Shared networking helpers for URL readers. Ports of agno-python
// knowledge/reader/utils/url_validation.py and utils/urls.py plus the HTML
// main-content extraction used by page_fetcher.py / llms_txt_reader.py.

// ErrHostNotAllowed is returned when a URL's host is not in AllowedHosts.
var ErrHostNotAllowed = errors.New("host not in allowed_hosts")

const defaultUserAgent = "agno-golang/1.0 (+https://github.com/devalexandre/agno-golang)"

// defaultMaxBytes caps response bodies when PublicOnly is off (64 MiB).
const defaultMaxBytes = 64 * 1024 * 1024

// IsHostAllowed reports whether rawURL's hostname is permitted by allowedHosts.
// A nil allowlist permits every host; otherwise matching is exact and
// case-insensitive (no implicit subdomains), as in Python's is_host_allowed.
func IsHostAllowed(rawURL string, allowedHosts []string) bool {
	if allowedHosts == nil {
		return true
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range allowedHosts {
		if host == strings.ToLower(h) {
			return true
		}
	}
	return false
}

// NetOptions are the network settings shared by URL readers.
type NetOptions struct {
	// HTTPClient is used when PublicOnly is false (default: 30s timeout client).
	HTTPClient *http.Client
	// PublicOnly fetches user-provided URLs through the SSRF-safe client
	// (see NewPublicHTTPClient). Turn it on whenever URLs come from untrusted
	// sources (users, LLM tool calls).
	PublicOnly bool
	// AllowedHosts, when non-nil, restricts every request (and every redirect
	// hop) to these exact hostnames.
	AllowedHosts []string
	// MaxBytes caps the response body (default 4 MiB in PublicOnly mode, 64 MiB otherwise).
	MaxBytes int64
	// UserAgent sent with every request.
	UserAgent string

	policy *publicPolicy // test hook for PublicOnly
}

type fetchResult struct {
	URL         string
	ContentType string
	Body        []byte
}

// get fetches rawURL honoring PublicOnly / AllowedHosts / MaxBytes.
func (o *NetOptions) get(ctx context.Context, rawURL string, header http.Header) (*fetchResult, error) {
	if !IsHostAllowed(rawURL, o.AllowedHosts) {
		return nil, fmt.Errorf("%w: %s", ErrHostNotAllowed, rawURL)
	}
	maxBytes := o.MaxBytes
	var client *http.Client
	if o.PublicOnly {
		p := defaultPublicPolicy
		if o.policy != nil {
			p = *o.policy
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNonPublicURL, err)
		}
		if p.validateURL != nil {
			if err := p.validateURL(u); err != nil {
				return nil, err
			}
		}
		client = newPublicClient(p)
		if maxBytes <= 0 {
			maxBytes = PublicFetchMaxBytes
		}
	} else {
		base := o.HTTPClient
		if base == nil {
			base = &http.Client{Timeout: 30 * time.Second}
		}
		c := *base
		allowed := o.AllowedHosts
		prev := base.CheckRedirect
		c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if !IsHostAllowed(req.URL.String(), allowed) {
				return fmt.Errorf("%w: redirect to %s", ErrHostNotAllowed, req.URL.Host)
			}
			if prev != nil {
				return prev(req, via)
			}
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		}
		client = &c
		if maxBytes <= 0 {
			maxBytes = defaultMaxBytes
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	ua := o.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &HTTPStatusError{URL: rawURL, StatusCode: resp.StatusCode}
	}
	if resp.ContentLength > maxBytes {
		return nil, ErrBodyTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, ErrBodyTooLarge
	}
	return &fetchResult{URL: resp.Request.URL.String(), ContentType: resp.Header.Get("Content-Type"), Body: body}, nil
}

// HTTPStatusError reports a non-2xx response.
type HTTPStatusError struct {
	URL        string
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("GET %s returned HTTP %d", e.URL, e.StatusCode)
}

// mediaType returns the lowercased media type of a Content-Type header.
func mediaType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
}

func looksLikeHTML(body string) bool {
	head := strings.ToLower(strings.TrimSpace(body))
	if len(head) > 15 {
		head = head[:15]
	}
	return strings.HasPrefix(head, "<!doctype") || strings.HasPrefix(head, "<html")
}

// ExtractPageText returns the main-content text of an HTML page: script,
// style, nav, header, footer and aside are removed, <main>/<article>/
// [role=main] (else <body>) is selected and its text nodes are joined with
// newlines. Port of page_fetcher.extract_page_text.
func ExtractPageText(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	doc.Find("script, style, nav, header, footer, aside, noscript").Remove()
	sel := doc.Find("main").First()
	if sel.Length() == 0 {
		sel = doc.Find("article").First()
	}
	if sel.Length() == 0 {
		sel = doc.Find("[role=main]").First()
	}
	if sel.Length() == 0 {
		sel = doc.Find("body").First()
	}
	if sel.Length() == 0 {
		sel = doc.Selection
	}
	var lines []string
	for _, n := range sel.Nodes {
		lines = appendTextNodes(n, lines)
	}
	return strings.Join(lines, "\n")
}

func appendTextNodes(n *html.Node, lines []string) []string {
	if n.Type == html.TextNode {
		if t := strings.TrimSpace(n.Data); t != "" {
			lines = append(lines, t)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		lines = appendTextNodes(c, lines)
	}
	return lines
}

// ExtractPageTitle returns the trimmed <title> of an HTML page, or "".
func ExtractPageTitle(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Find("title").First().Text())
}

// MaxPageNameLength bounds CanonicalPageName (MySQL String(255) column).
const MaxPageNameLength = 255

func stripIndexSuffix(path string) string {
	for _, suffix := range []string{"index.html", "index.htm"} {
		if strings.HasSuffix(path, "/"+suffix) {
			return path[:len(path)-len(suffix)]
		}
	}
	return path
}

// CanonicalPageName returns a stable "<host>/<path>" name for one page:
// fragment dropped, trailing slash and index.html stripped, path decoded,
// query kept; overlong names are truncated with a short hash suffix.
func CanonicalPageName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := strings.ToLower(u.Hostname())
	path := stripIndexSuffix(u.Path)
	path = strings.Trim(path, "/")
	name := host
	if path != "" {
		name = host + "/" + path
	}
	if u.RawQuery != "" {
		name += "?" + u.RawQuery
	}
	if len(name) > MaxPageNameLength {
		sum := sha256.Sum256([]byte(name))
		name = name[:MaxPageNameLength-9] + "-" + hex.EncodeToString(sum[:])[:8]
	}
	return name
}

// CanonicalPageURL returns the canonical form of a page URL for identity and
// de-duplication (scheme/host lowercased, fragment dropped, trailing slash and
// index.html stripped, query kept, path encoding preserved).
func CanonicalPageURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	path := u.EscapedPath()
	path = strings.TrimRight(stripIndexSuffix(path), "/")
	scheme := strings.ToLower(u.Scheme)
	if scheme == "" {
		scheme = "https"
	}
	out := scheme + "://" + strings.ToLower(u.Host) + path
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	return out
}

// IsSitemapURL reports whether rawURL names a sitemap file (sitemap*.xml[.gz]).
func IsSitemapURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := strings.ToLower(u.Path)
	seg := path[strings.LastIndex(path, "/")+1:]
	return strings.HasPrefix(seg, "sitemap") && (strings.HasSuffix(seg, ".xml") || strings.HasSuffix(seg, ".xml.gz"))
}

// bytesTrimBOM drops a UTF-8 byte-order mark.
func bytesTrimBOM(b []byte) []byte {
	return bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
}
