package reader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
)

// WikipediaReader fetches the plain-text extract of a Wikipedia article by title.
type WikipediaReader struct {
	BaseReader
	Language   string // e.g. "en", "pt"
	HTTPClient *http.Client
}

// NewWikipediaReader creates a Wikipedia reader for the given language (default "en").
func NewWikipediaReader(language string) *WikipediaReader {
	if language == "" {
		language = "en"
	}
	return &WikipediaReader{
		BaseReader: BaseReader{ID: "wikipedia"},
		Language:   language,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Read fetches the article whose title is source.
func (r *WikipediaReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	endpoint := fmt.Sprintf("https://%s.wikipedia.org/w/api.php", r.Language)
	q := url.Values{}
	q.Set("action", "query")
	q.Set("prop", "extracts")
	q.Set("explaintext", "1")
	q.Set("format", "json")
	q.Set("redirects", "1")
	q.Set("titles", source)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "agno-golang/1.0 (+https://github.com/devalexandre/agno-golang)")

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikipedia: status %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Query struct {
			Pages map[string]struct {
				Title   string `json:"title"`
				Extract string `json:"extract"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("wikipedia: parse: %w", err)
	}

	var docs []*document.Document
	for _, page := range out.Query.Pages {
		if page.Extract == "" {
			continue
		}
		doc := document.NewDocument(page.Extract)
		doc.Name = page.Title
		doc.Source = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", r.Language, url.PathEscape(page.Title))
		doc.ContentType = "text/plain"
		doc.Metadata["source"] = "wikipedia"
		doc.Metadata["title"] = page.Title
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("wikipedia: no article found for %q", source)
	}
	return docs, nil
}

var _ Reader = (*WikipediaReader)(nil)
