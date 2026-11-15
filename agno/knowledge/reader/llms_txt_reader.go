package reader

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/devalexandre/agno-golang/agno/document"
)

// LLMsTxtReader reads an llms.txt file (https://llmstxt.org), then fetches the
// documentation pages it links to. Port of agno-python
// knowledge/reader/llms_txt_reader.py.
//
// The text before the first "## Section" becomes an overview document
// (metadata type "llms_txt_overview"); every "- [Title](url): description"
// entry under a section is fetched (up to MaxURLs) and becomes a document
// (type "llms_txt_linked_doc") carrying url/section/description metadata.
// HTML pages are reduced to their main content; text/markdown is kept as-is.
type LLMsTxtReader struct {
	BaseReader
	NetOptions
	// MaxURLs bounds how many linked pages are fetched (default 20).
	MaxURLs int
	// SkipOptional ignores the "## Optional" section.
	SkipOptional bool
	// Concurrency bounds parallel page fetches (default 8).
	Concurrency int
}

// NewLLMsTxtReader creates an llms.txt reader.
func NewLLMsTxtReader() *LLMsTxtReader {
	return &LLMsTxtReader{
		BaseReader:  BaseReader{ID: "llms_txt", Extensions: []string{"http", "https"}},
		MaxURLs:     20,
		Concurrency: 8,
	}
}

// WithPublicOnly enables the SSRF-safe client.
func (r *LLMsTxtReader) WithPublicOnly(enabled bool) *LLMsTxtReader {
	r.PublicOnly = enabled
	return r
}

// LLMsTxtEntry is one link listed in an llms.txt file.
type LLMsTxtEntry struct {
	Title       string
	URL         string
	Description string
	Section     string
}

var (
	llmsLinkPattern    = regexp.MustCompile(`^-\s+\[([^\]]+)\]\(([^)]+)\)(?::\s*(.+))?`)
	llmsSectionPattern = regexp.MustCompile(`^##\s+(.+)$`)
)

// ParseLLMsTxt splits llms.txt content into its overview and linked entries,
// resolving relative links against baseURL.
func (r *LLMsTxtReader) ParseLLMsTxt(content, baseURL string) (string, []LLMsTxtEntry) {
	var entries []LLMsTxtEntry
	var overview []string
	section := ""
	base, _ := url.Parse(baseURL)
	for _, line := range strings.Split(content, "\n") {
		if m := llmsSectionPattern.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil {
			section = strings.TrimSpace(m[1])
			continue
		}
		if section == "" {
			overview = append(overview, line)
			continue
		}
		if r.SkipOptional && strings.EqualFold(section, "optional") {
			continue
		}
		m := llmsLinkPattern.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		link := strings.TrimSpace(m[2])
		if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") && base != nil {
			if ref, err := url.Parse(link); err == nil {
				link = base.ResolveReference(ref).String()
			}
		}
		entries = append(entries, LLMsTxtEntry{
			Title:       strings.TrimSpace(m[1]),
			URL:         link,
			Description: strings.TrimSpace(m[3]),
			Section:     section,
		})
	}
	return strings.TrimSpace(strings.Join(overview, "\n")), entries
}

// fetchText fetches u and returns its text ("" on failure).
func (r *LLMsTxtReader) fetchText(ctx context.Context, u string) (string, error) {
	res, err := r.get(ctx, u, nil)
	if err != nil {
		return "", err
	}
	ct := strings.ToLower(res.ContentType)
	body := string(res.Body)
	if strings.Contains(ct, "text/plain") || strings.Contains(ct, "text/markdown") {
		return body, nil
	}
	if strings.Contains(ct, "text/html") || strings.HasPrefix(strings.TrimSpace(body), "<!DOCTYPE") ||
		strings.HasPrefix(strings.TrimSpace(body), "<html") || strings.HasPrefix(strings.TrimSpace(body), "<HTML") {
		return ExtractPageText(body), nil
	}
	return body, nil
}

// Read fetches the llms.txt at source and the pages it links to.
func (r *LLMsTxtReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	content, err := r.fetchText(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("llms_txt: fetch %s: %w", source, err)
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("llms_txt: empty llms.txt at %s", source)
	}
	overview, entries := r.ParseLLMsTxt(content, source)
	max := r.MaxURLs
	if max <= 0 {
		max = 20
	}
	if len(entries) > max {
		entries = entries[:max]
	}
	n := r.Concurrency
	if n <= 0 {
		n = 8
	}
	fetched := make([]string, len(entries))
	sem := make(chan struct{}, n)
	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fetched[i], _ = r.fetchText(ctx, u)
		}(i, e.URL)
	}
	wg.Wait()

	var docs []*document.Document
	if overview != "" {
		d := document.NewDocument(overview)
		d.Name = source
		d.Source = source
		d.ContentType = "text/markdown"
		d.Metadata["url"] = source
		d.Metadata["type"] = "llms_txt_overview"
		docs = append(docs, d)
	}
	for i, e := range entries {
		if fetched[i] == "" {
			continue
		}
		d := document.NewDocument(fetched[i])
		d.Name = e.Title
		d.Source = e.URL
		d.ContentType = "text/plain"
		d.Metadata["url"] = e.URL
		d.Metadata["section"] = e.Section
		d.Metadata["description"] = e.Description
		d.Metadata["type"] = "llms_txt_linked_doc"
		docs = append(docs, d)
	}
	return docs, nil
}

var _ Reader = (*LLMsTxtReader)(nil)
