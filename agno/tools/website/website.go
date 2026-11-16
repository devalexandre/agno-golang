// Package website ports Agno's WebsiteTools: read a website (crawling a few
// same-domain pages, like Agno's WebsiteReader) and return its main content,
// or, when a knowledge base is configured, add the crawled pages to it.
package website

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/trafilatura"
	"golang.org/x/net/html"
)

// DocumentLoader is the part of a knowledge base used by
// AddWebsiteToKnowledge; knowledge.Knowledge implementations satisfy it.
type DocumentLoader interface {
	LoadDocument(ctx context.Context, doc document.Document) error
}

// WebsiteTools reads websites.
type WebsiteTools struct {
	toolkit.Toolkit
	knowledge  DocumentLoader
	maxDepth   int
	maxLinks   int
	httpClient *http.Client
}

// Option configures WebsiteTools.
type Option func(*WebsiteTools)

// WithKnowledge registers AddWebsiteToKnowledge instead of ReadUrl.
func WithKnowledge(k DocumentLoader) Option { return func(w *WebsiteTools) { w.knowledge = k } }

// WithMaxDepth sets the crawl depth (default 3).
func WithMaxDepth(n int) Option { return func(w *WebsiteTools) { w.maxDepth = n } }

// WithMaxLinks sets the maximum number of pages read (default 10).
func WithMaxLinks(n int) Option { return func(w *WebsiteTools) { w.maxLinks = n } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(w *WebsiteTools) { w.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(w *WebsiteTools) { w.httpClient = c } }

// URLParams carries a URL.
type URLParams struct {
	URL string `json:"url" description:"The url of the website." required:"true"`
}

// New creates the website toolkit.
func New(opts ...Option) *WebsiteTools {
	w := &WebsiteTools{maxDepth: 3, maxLinks: 10}
	w.Toolkit = toolkit.NewToolkit()
	w.Toolkit.Name = "website_tools"
	w.Toolkit.Description = "Read the content of websites."
	for _, o := range opts {
		o(w)
	}
	if w.httpClient == nil {
		w.httpClient = w.Toolkit.HTTPClient()
	}
	if w.knowledge != nil {
		w.Toolkit.Register("AddWebsiteToKnowledge", "Add a website's content to the knowledge base. The website must start with https:// and be a valid website. Use this to get information about products from the internet.", w, w.AddWebsiteToKnowledge, URLParams{})
	} else {
		w.Toolkit.Register("ReadUrl", "Read a url and return its content as a JSON list of documents.", w, w.ReadUrl, URLParams{})
	}
	return w
}

// Page is one crawled page.
type Page struct {
	URL     string
	Content string
}

func primaryDomain(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return host
}

// Crawl reads startURL and same-domain pages reachable from it (breadth
// first, up to maxDepth/maxLinks), returning each page's main text.
func (w *WebsiteTools) Crawl(startURL string) ([]Page, error) {
	start, err := url.Parse(startURL)
	if err != nil || start.Host == "" {
		return nil, &url.Error{Op: "parse", URL: startURL, Err: err}
	}
	domain := primaryDomain(start.Host)
	type item struct {
		u     string
		depth int
	}
	queue := []item{{startURL, 1}}
	queued := map[string]bool{startURL: true}
	visited := map[string]bool{}
	var pages []Page
	var firstErr error
	for len(queue) > 0 && len(pages) < w.maxLinks {
		cur := queue[0]
		queue = queue[1:]
		cu, err := url.Parse(cur.u)
		if err != nil || visited[cur.u] || !strings.HasSuffix(cu.Host, domain) || (cur.depth > w.maxDepth && cur.u != startURL) {
			continue
		}
		visited[cur.u] = true
		body, _, err := trafilatura.FetchPage(w.httpClient, cur.u)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		doc, err := html.Parse(strings.NewReader(string(body)))
		if err != nil {
			continue
		}
		if text := trafilatura.ExtractText(trafilatura.MainNode(doc), trafilatura.ExtractOptions{IncludeTables: true, IncludeComments: true}); text != "" {
			pages = append(pages, Page{URL: cur.u, Content: text})
		}
		for _, link := range trafilatura.ExtractLinks(doc, cu) {
			lu, err := url.Parse(link)
			if err != nil || !strings.HasSuffix(lu.Host, domain) || queued[link] {
				continue
			}
			p := strings.ToLower(lu.Path)
			if strings.HasSuffix(p, ".pdf") || strings.HasSuffix(p, ".jpg") || strings.HasSuffix(p, ".png") {
				continue
			}
			queued[link] = true
			queue = append(queue, item{link, cur.depth + 1})
		}
	}
	if len(pages) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return pages, nil
}

// ReadUrl crawls the website and returns its documents as JSON
// ([{"name","meta_data":{"url"},"content"}]).
func (w *WebsiteTools) ReadUrl(params URLParams) (interface{}, error) {
	pages, err := w.Crawl(params.URL)
	if err != nil {
		return nil, err
	}
	docs := make([]map[string]interface{}, 0, len(pages))
	for _, p := range pages {
		docs = append(docs, map[string]interface{}{
			"name":      params.URL,
			"meta_data": map[string]string{"url": p.URL},
			"content":   p.Content,
		})
	}
	return restkit.CompactJSON(docs), nil
}

// AddWebsiteToKnowledge crawls the website and loads each page into the
// knowledge base.
func (w *WebsiteTools) AddWebsiteToKnowledge(params URLParams) (interface{}, error) {
	if w.knowledge == nil {
		return "Knowledge base not provided", nil
	}
	pages, err := w.Crawl(params.URL)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, p := range pages {
		doc := document.Document{
			ID:          p.URL,
			Name:        params.URL,
			Content:     p.Content,
			ContentType: "text/plain",
			Metadata:    map[string]interface{}{"url": p.URL},
			Source:      p.URL,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := w.knowledge.LoadDocument(context.Background(), doc); err != nil {
			return nil, err
		}
	}
	return "Success", nil
}

// Execute implements toolkit.Tool.
func (w *WebsiteTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return w.Toolkit.Execute(methodName, input)
}
