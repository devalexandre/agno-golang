package reader

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/devalexandre/agno-golang/agno/document"
)

// WebsiteReader fetches a URL and extracts its visible text content.
type WebsiteReader struct {
	BaseReader
	HTTPClient *http.Client
	UserAgent  string
}

// NewWebsiteReader creates a website reader.
func NewWebsiteReader() *WebsiteReader {
	return &WebsiteReader{
		BaseReader: BaseReader{ID: "website", Extensions: []string{"http", "https"}},
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  "agno-golang/1.0 (+https://github.com/devalexandre/agno-golang)",
	}
}

// Read fetches source (an http/https URL) and returns its extracted text.
func (r *WebsiteReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", r.UserAgent)

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("website: GET %s returned %d", source, resp.StatusCode)
	}

	root, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("website: parse HTML: %w", err)
	}

	title, text := extractHTMLText(root)
	doc := document.NewDocument(strings.TrimSpace(text))
	doc.Name = title
	if doc.Name == "" {
		doc.Name = source
	}
	doc.Source = source
	doc.ContentType = "text/plain"
	doc.Metadata["source"] = source
	doc.Metadata["url"] = source
	if title != "" {
		doc.Metadata["title"] = title
	}
	return []*document.Document{doc}, nil
}

// extractHTMLText walks the node tree, returning the <title> and visible text,
// skipping script/style/noscript content.
func extractHTMLText(n *html.Node) (title, text string) {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "script", "style", "noscript":
				return
			case "title":
				if node.FirstChild != nil && title == "" {
					title = strings.TrimSpace(node.FirstChild.Data)
				}
			}
		}
		if node.Type == html.TextNode {
			if t := strings.TrimSpace(node.Data); t != "" {
				sb.WriteString(t)
				sb.WriteString(" ")
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return title, collapseWhitespace(sb.String())
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

var _ Reader = (*WebsiteReader)(nil)
