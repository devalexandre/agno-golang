package reader

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
)

// ArxivReader searches the arXiv API and returns matching papers' abstracts.
type ArxivReader struct {
	BaseReader
	MaxResults int
	HTTPClient *http.Client
}

// NewArxivReader creates an arXiv reader (default 5 results).
func NewArxivReader(maxResults int) *ArxivReader {
	if maxResults <= 0 {
		maxResults = 5
	}
	return &ArxivReader{
		BaseReader: BaseReader{ID: "arxiv"},
		MaxResults: maxResults,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type arxivFeed struct {
	Entries []struct {
		Title     string `xml:"title"`
		Summary   string `xml:"summary"`
		ID        string `xml:"id"`
		Published string `xml:"published"`
		Authors   []struct {
			Name string `xml:"name"`
		} `xml:"author"`
	} `xml:"entry"`
}

// Read runs source as an arXiv search query and returns one document per paper.
func (r *ArxivReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	q := url.Values{}
	q.Set("search_query", "all:"+source)
	q.Set("start", "0")
	q.Set("max_results", fmt.Sprintf("%d", r.MaxResults))

	endpoint := "http://export.arxiv.org/api/query?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("arxiv: status %d", resp.StatusCode)
	}

	var feed arxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("arxiv: parse: %w", err)
	}

	var docs []*document.Document
	for _, e := range feed.Entries {
		var authors []string
		for _, a := range e.Authors {
			authors = append(authors, strings.TrimSpace(a.Name))
		}
		title := strings.Join(strings.Fields(e.Title), " ")
		content := fmt.Sprintf("Title: %s\nAuthors: %s\n\n%s",
			title, strings.Join(authors, ", "), strings.TrimSpace(e.Summary))

		doc := document.NewDocument(content)
		doc.Name = title
		doc.Source = strings.TrimSpace(e.ID)
		doc.ContentType = "text/plain"
		doc.Metadata["source"] = "arxiv"
		doc.Metadata["title"] = title
		doc.Metadata["authors"] = authors
		doc.Metadata["published"] = strings.TrimSpace(e.Published)
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("arxiv: no results for %q", source)
	}
	return docs, nil
}

var _ Reader = (*ArxivReader)(nil)
