package reader

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// SitemapReader loads a website page by page from its sitemap and returns one
// whole-page document per page. Port of agno-python
// knowledge/reader/sitemap_reader.py.
//
// Discovery follows the sitemap protocol: the URL itself when it looks like a
// sitemap, the "Sitemap:" lines in robots.txt, then /sitemap.xml,
// /sitemap_index.xml and <dir>/sitemap.xml. Sitemap indexes are followed
// (FollowIndex), gzip bodies are decoded, entries are de-duplicated on their
// canonical URL and restricted to the allowed hosts (default: the URL's host
// plus its www twin). Without any sitemap the URL itself is read as a single
// page. Pages are fetched through PageFetcher (default HTTPPageFetcher).
type SitemapReader struct {
	BaseReader
	NetOptions
	// MaxPages bounds the number of pages read (default 50).
	MaxPages int
	// FollowIndex follows <sitemapindex> children (default true).
	FollowIndex bool
	// PageFetcher fetches the pages (default: HTTPPageFetcher sharing NetOptions).
	PageFetcher PageFetcher
	// SourceHeader prepends "# <title>\nSource: <url>\n\n" to every page.
	SourceHeader bool
}

// NewSitemapReader creates a sitemap reader.
func NewSitemapReader() *SitemapReader {
	return &SitemapReader{
		BaseReader:  BaseReader{ID: "sitemap", Extensions: []string{"http", "https"}},
		MaxPages:    50,
		FollowIndex: true,
	}
}

// WithPublicOnly enables the SSRF-safe client for discovery and page fetches.
func (r *SitemapReader) WithPublicOnly(enabled bool) *SitemapReader {
	r.PublicOnly = enabled
	return r
}

func defaultAllowedHosts(host string) []string {
	host = strings.ToLower(host)
	bare := strings.TrimPrefix(host, "www.")
	set := map[string]bool{host: true, bare: true, "www." + bare: true}
	out := make([]string, 0, len(set))
	for h := range set {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

func (r *SitemapReader) hostsFor(rawURL string) []string {
	if r.AllowedHosts != nil {
		return r.AllowedHosts
	}
	u, _ := url.Parse(rawURL)
	if u == nil {
		return []string{}
	}
	return defaultAllowedHosts(u.Hostname())
}

func decodeSitemapBytes(raw []byte) []byte {
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return raw
		}
		defer zr.Close()
		out, err := io.ReadAll(io.LimitReader(zr, 64<<20))
		if err != nil {
			return raw
		}
		return out
	}
	return raw
}

// parseSitemap returns (isIndex, locs, ok). Only <loc> elements that are
// direct children of <url>/<sitemap> count (image:loc etc. are deeper).
func parseSitemap(raw []byte) (bool, []string, bool) {
	dec := xml.NewDecoder(bytes.NewReader(decodeSitemapBytes(raw)))
	dec.Strict = false
	var stack []string
	var root string
	var locs []string
	entryHasLoc := false
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, nil, false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if len(stack) == 0 {
				root = name
				if root != "urlset" && root != "sitemapindex" {
					return false, nil, false
				}
			}
			if len(stack) == 1 {
				entryHasLoc = false
			}
			stack = append(stack, name)
			text.Reset()
		case xml.CharData:
			if len(stack) == 3 {
				text.Write(t)
			}
		case xml.EndElement:
			if len(stack) == 3 && stack[2] == "loc" && (stack[1] == "url" || stack[1] == "sitemap") && !entryHasLoc {
				if loc := strings.TrimSpace(text.String()); loc != "" {
					locs = append(locs, loc)
					entryHasLoc = true
				}
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == "" {
		return false, nil, false
	}
	return root == "sitemapindex", locs, true
}

func robotsSitemapLines(robots string) []string {
	var out []string
	for _, line := range strings.Split(robots, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "sitemap") && strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}

func sitemapCandidates(rawURL string, robots []string) []string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	root := u.Scheme + "://" + u.Host
	var c []string
	last := strings.ToLower(u.Path[strings.LastIndex(u.Path, "/")+1:])
	if strings.Contains(last, "sitemap") || strings.Contains(last, ".xml") {
		c = append(c, rawURL)
	}
	c = append(c, robots...)
	c = append(c, root+"/sitemap.xml", root+"/sitemap_index.xml")
	if i := strings.LastIndex(u.Path, "/"); i > 0 {
		c = append(c, root+u.Path[:i]+"/sitemap.xml")
	}
	seen := map[string]bool{}
	var out []string
	for _, x := range c {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func (r *SitemapReader) maxPages() int {
	if r.MaxPages <= 0 {
		return 50
	}
	return r.MaxPages
}

// Discover finds the sitemap for rawURL and returns its page URLs, the
// sitemap URL ("" when none was found and rawURL is read as a single page)
// and whether discovery was incomplete (an index child failed, the cap
// truncated the read, or no sitemap was found).
func (r *SitemapReader) Discover(ctx context.Context, rawURL string) (pages []string, sitemapURL string, incomplete bool) {
	allowed := r.hostsFor(rawURL)
	opts := r.NetOptions
	opts.AllowedHosts = allowed
	fetch := func(u string) []byte {
		res, err := opts.get(ctx, u, nil)
		if err != nil {
			return nil
		}
		return res.Body
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return []string{rawURL}, "", true
	}
	var robots []string
	if body := fetch(u.Scheme + "://" + u.Host + "/robots.txt"); body != nil {
		robots = robotsSitemapLines(string(body))
	}
	max := r.maxPages()
	for _, candidate := range sitemapCandidates(rawURL, robots) {
		if !IsHostAllowed(candidate, allowed) {
			continue
		}
		raw := fetch(candidate)
		if raw == nil {
			continue
		}
		isIndex, locs, ok := parseSitemap(raw)
		if !ok {
			continue
		}
		var pending, pageLocs []string
		if isIndex {
			if r.FollowIndex {
				pending = append(pending, locs...)
			}
		} else {
			pageLocs = append(pageLocs, locs...)
		}
		visited := map[string]bool{CanonicalPageURL(candidate): true}
		for len(pending) > 0 && len(pageLocs) < max {
			child := pending[0]
			pending = pending[1:]
			key := CanonicalPageURL(child)
			if visited[key] {
				continue
			}
			visited[key] = true
			if !IsHostAllowed(child, allowed) {
				continue
			}
			childRaw := fetch(child)
			if childRaw == nil {
				incomplete = true
				continue
			}
			childIndex, childLocs, ok := parseSitemap(childRaw)
			if !ok {
				incomplete = true
				continue
			}
			if childIndex {
				pending = append(pending, childLocs...)
			} else {
				pageLocs = append(pageLocs, childLocs...)
			}
		}
		if len(pending) > 0 {
			incomplete = true
		}
		seen := map[string]bool{}
		for i, loc := range pageLocs {
			key := CanonicalPageURL(loc)
			if seen[key] {
				continue
			}
			seen[key] = true
			if !IsHostAllowed(loc, allowed) {
				continue
			}
			pages = append(pages, loc)
			if len(pages) >= max {
				if i+1 < len(pageLocs) {
					incomplete = true
				}
				break
			}
		}
		return pages, candidate, incomplete
	}
	return []string{rawURL}, "", true
}

// Read discovers the sitemap of source and returns one document per page.
func (r *SitemapReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	pageURLs, sitemapURL, incomplete := r.Discover(ctx, source)
	allowed := r.hostsFor(source)
	fetcher := r.PageFetcher
	if fetcher == nil {
		fetcher = &HTTPPageFetcher{NetOptions: r.NetOptions, Concurrency: 8}
	}
	pages := fetcher.FetchMany(ctx, pageURLs, allowed)
	kind := "page"
	if sitemapURL != "" {
		kind = "sitemap"
	}
	docs := make([]*document.Document, 0, len(pages))
	for _, p := range pages {
		key := CanonicalPageURL(p.URL)
		name := CanonicalPageName(p.URL)
		title := p.Title
		if title == "" {
			title = name
		}
		host := ""
		if pu, err := url.Parse(p.URL); err == nil {
			host = pu.Hostname()
		}
		content := ""
		if p.OK() {
			content = p.Content
			if r.SourceHeader {
				content = "# " + title + "\nSource: " + key + "\n\n" + content
			}
		}
		doc := document.NewDocument(content)
		doc.ID = key
		doc.Name = name
		doc.Source = key
		doc.ContentType = "text/plain"
		doc.Metadata["url"] = key
		doc.Metadata["title"] = title
		doc.Metadata["host"] = host
		doc.Metadata["extractor"] = p.Extractor
		doc.Metadata["source"] = kind
		if sitemapURL != "" {
			doc.Metadata["sitemap_url"] = sitemapURL
		}
		if len(p.Attempts) > 0 {
			doc.Metadata["attempts"] = p.Attempts
		}
		if incomplete {
			doc.Metadata["discovery_incomplete"] = true
		}
		if !p.OK() {
			e := p.Error
			if e == "" {
				e = "empty"
			}
			doc.Metadata["error"] = e
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

var _ Reader = (*SitemapReader)(nil)
