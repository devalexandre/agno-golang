package trafilatura

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// MaxPageBytes caps the size of fetched pages.
const MaxPageBytes = 20 << 20

// UserAgent is sent with every page fetch.
const UserAgent = "Mozilla/5.0 (compatible; agno-golang/trafilatura)"

// FetchPage downloads rawURL and returns its body and Content-Type. Non-2xx
// statuses are errors.
func FetchPage(client *http.Client, rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, "", fmt.Errorf("invalid URL: %s", rawURL)
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8")
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxPageBytes))
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// ExtractOptions controls main-text extraction.
type ExtractOptions struct {
	// Markdown renders headings, list items, links and images as Markdown.
	Markdown        bool
	IncludeComments bool
	IncludeTables   bool
	IncludeImages   bool
	IncludeLinks    bool
	Deduplicate     bool
}

// boilerplate tags are always dropped from the main text.
var boilerplate = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true, "svg": true,
	"iframe": true, "nav": true, "header": true, "footer": true, "aside": true,
	"form": true, "button": true, "select": true, "input": true, "textarea": true,
	"head": true, "object": true, "embed": true, "canvas": true,
}

var blockTags = map[string]bool{
	"p": true, "div": true, "section": true, "article": true, "main": true, "blockquote": true,
	"pre": true, "ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"table": true, "tr": true, "thead": true, "tbody": true, "figure": true, "figcaption": true,
	"hr": true, "address": true, "details": true, "summary": true, "body": true, "html": true,
}

// Attr returns the value of attribute key on n.
func Attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

// Find returns the first element node matching pred (depth first).
func Find(n *html.Node, pred func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && pred(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := Find(c, pred); f != nil {
			return f
		}
	}
	return nil
}

func byTag(tag string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Data == tag }
}

// MainNode picks the main content container: <main>, <article>,
// [role=main], <body>, else the document itself.
func MainNode(doc *html.Node) *html.Node {
	if n := Find(doc, byTag("main")); n != nil {
		return n
	}
	if n := Find(doc, byTag("article")); n != nil {
		return n
	}
	if n := Find(doc, func(n *html.Node) bool { return strings.EqualFold(Attr(n, "role"), "main") }); n != nil {
		return n
	}
	if n := Find(doc, byTag("body")); n != nil {
		return n
	}
	return doc
}

func isCommentSection(n *html.Node) bool {
	idc := strings.ToLower(Attr(n, "id") + " " + Attr(n, "class"))
	return strings.Contains(idc, "comment")
}

type textBuilder struct {
	opts   ExtractOptions
	lines  []string
	cur    strings.Builder
	prefix string
}

func (b *textBuilder) flush() {
	text := strings.Join(strings.Fields(b.cur.String()), " ")
	b.cur.Reset()
	if text == "" {
		return
	}
	b.lines = append(b.lines, b.prefix+text)
	b.prefix = ""
}

func (b *textBuilder) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		b.cur.WriteString(n.Data)
		b.cur.WriteString(" ")
		return
	case html.CommentNode, html.DoctypeNode:
		return
	case html.ElementNode:
		tag := n.Data
		if boilerplate[tag] || strings.EqualFold(Attr(n, "aria-hidden"), "true") {
			return
		}
		if !b.opts.IncludeTables && tag == "table" {
			return
		}
		if !b.opts.IncludeComments && isCommentSection(n) {
			return
		}
		switch tag {
		case "br":
			b.flush()
			return
		case "img":
			if b.opts.IncludeImages {
				src, alt := Attr(n, "src"), Attr(n, "alt")
				if src != "" {
					b.flush()
					if b.opts.Markdown {
						b.lines = append(b.lines, fmt.Sprintf("![%s](%s)", alt, src))
					} else if alt != "" {
						b.lines = append(b.lines, alt)
					}
				}
			}
			return
		case "a":
			href := Attr(n, "href")
			if b.opts.IncludeLinks && b.opts.Markdown && href != "" && !strings.HasPrefix(href, "#") {
				inner := &textBuilder{opts: b.opts}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					inner.walk(c)
				}
				inner.flush()
				text := strings.Join(inner.lines, " ")
				if text != "" {
					fmt.Fprintf(&b.cur, " [%s](%s) ", text, href)
				}
				return
			}
		case "td", "th":
			b.cur.WriteString(" | ")
		}
		block := blockTags[tag]
		if block {
			b.flush()
			if b.opts.Markdown {
				switch tag {
				case "h1", "h2", "h3", "h4", "h5", "h6":
					b.prefix = strings.Repeat("#", int(tag[1]-'0')) + " "
				case "li":
					b.prefix = "- "
				case "blockquote":
					b.prefix = "> "
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.walk(c)
		}
		if block {
			b.flush()
		}
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.walk(c)
		}
	}
}

// ExtractText returns the readable text of n, one block per line.
func ExtractText(n *html.Node, opts ExtractOptions) string {
	b := &textBuilder{opts: opts}
	b.walk(n)
	b.flush()
	lines := b.lines
	if opts.Deduplicate {
		seen := map[string]bool{}
		out := lines[:0]
		for _, l := range lines {
			if !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
		lines = out
	}
	sep := "\n"
	if opts.Markdown {
		sep = "\n\n"
	}
	return strings.TrimSpace(strings.Join(lines, sep))
}

// ExtractMainText parses page HTML and returns the text of its main content.
func ExtractMainText(page []byte, opts ExtractOptions) (string, error) {
	doc, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return "", err
	}
	return ExtractText(MainNode(doc), opts), nil
}

// HTMLToText converts an HTML fragment to whitespace-normalised plain text
// (trafilatura.html2txt). clean drops scripts, styles and other non-content
// elements.
func HTMLToText(s string, clean bool) (string, error) {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return "", err
	}
	root := Find(doc, byTag("body"))
	if root == nil {
		root = doc
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && clean && boilerplate[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
			sb.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return strings.Join(strings.Fields(sb.String()), " "), nil
}

// ExtractLinks returns the absolute http(s) links of doc (fragments removed).
func ExtractLinks(doc *html.Node, base *url.URL) []string {
	var out []string
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			if href := strings.TrimSpace(Attr(n, "href")); href != "" {
				if u, err := base.Parse(href); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
					u.Fragment = ""
					s := u.String()
					if !seen[s] {
						seen[s] = true
						out = append(out, s)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

// MetadataKeys is the stable key order of Metadata (mirrors trafilatura's
// Document.as_dict).
var MetadataKeys = []string{"title", "author", "url", "hostname", "description", "sitename", "date", "categories", "tags", "image", "pagetype", "language"}

// ExtractMetadata reads title, author, description, dates, tags and similar
// fields from the page's <head> (OpenGraph, standard meta tags, <title>).
// Missing values are nil.
func ExtractMetadata(doc *html.Node, pageURL string) map[string]interface{} {
	metas := map[string]string{}
	var tags []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			key := strings.ToLower(Attr(n, "property"))
			if key == "" {
				key = strings.ToLower(Attr(n, "name"))
			}
			if key == "" {
				key = strings.ToLower(Attr(n, "itemprop"))
			}
			val := strings.TrimSpace(Attr(n, "content"))
			if key != "" && val != "" {
				if key == "article:tag" {
					tags = append(tags, val)
				}
				if _, ok := metas[key]; !ok {
					metas[key] = val
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := metas[k]; v != "" {
				return v
			}
		}
		return ""
	}
	nodeText := func(tag string) string {
		if n := Find(doc, byTag(tag)); n != nil {
			return ExtractText(n, ExtractOptions{IncludeTables: true, IncludeComments: true})
		}
		return ""
	}

	title := first("og:title", "twitter:title", "dc.title")
	if title == "" {
		title = nodeText("title")
	}
	if title == "" {
		title = nodeText("h1")
	}
	canonical := ""
	if n := Find(doc, func(n *html.Node) bool { return n.Data == "link" && strings.EqualFold(Attr(n, "rel"), "canonical") }); n != nil {
		canonical = Attr(n, "href")
	}
	finalURL := first("og:url")
	if canonical != "" {
		finalURL = canonical
	}
	if finalURL == "" {
		finalURL = pageURL
	}
	hostname := ""
	if u, err := url.Parse(finalURL); err == nil {
		hostname = strings.TrimPrefix(u.Hostname(), "www.")
	}
	date := first("article:published_time", "og:published_time", "date", "dc.date", "pubdate", "publishdate", "datepublished", "article:modified_time")
	if date == "" {
		if n := Find(doc, func(n *html.Node) bool { return n.Data == "time" && Attr(n, "datetime") != "" }); n != nil {
			date = Attr(n, "datetime")
		}
	}
	if len(tags) == 0 {
		if kw := first("keywords", "news_keywords"); kw != "" {
			for _, t := range strings.Split(kw, ",") {
				if t = strings.TrimSpace(t); t != "" {
					tags = append(tags, t)
				}
			}
		}
	}
	var categories []string
	if s := first("article:section"); s != "" {
		categories = []string{s}
	}
	language := ""
	if n := Find(doc, byTag("html")); n != nil {
		language = Attr(n, "lang")
	}
	if language == "" {
		language = first("og:locale", "content-language", "language")
	}

	orNil := func(s string) interface{} {
		if s == "" {
			return nil
		}
		return s
	}
	listOrNil := func(l []string) interface{} {
		if len(l) == 0 {
			return nil
		}
		return l
	}
	return map[string]interface{}{
		"title":       orNil(title),
		"author":      orNil(first("author", "article:author", "dc.creator", "twitter:creator", "parsely-author")),
		"url":         orNil(finalURL),
		"hostname":    orNil(hostname),
		"description": orNil(first("description", "og:description", "twitter:description", "dc.description")),
		"sitename":    orNil(first("og:site_name", "application-name")),
		"date":        orNil(date),
		"categories":  listOrNil(categories),
		"tags":        listOrNil(tags),
		"image":       orNil(first("og:image", "twitter:image", "og:image:url")),
		"pagetype":    orNil(first("og:type")),
		"language":    orNil(language),
	}
}
