// Package trafilatura ports Agno's TrafilaturaTools: fetch web pages and
// extract their main text and metadata. The Python toolkit wraps the
// trafilatura library; this port implements the same tools with
// golang.org/x/net/html (main-content selection, boilerplate removal,
// OpenGraph/meta metadata and a small same-site crawler).
package trafilatura

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"golang.org/x/net/html"
)

// TrafilaturaTools extracts text and metadata from web pages.
type TrafilaturaTools struct {
	toolkit.Toolkit
	outputFormat    string
	includeComments bool
	includeTables   bool
	includeImages   bool
	includeLinks    bool
	withMetadata    bool
	deduplicate     bool
	maxCrawlURLs    int
	maxKnownURLs    int

	enableExtractText     bool
	enableExtractMetadata bool
	enableHTMLToText      bool
	enableExtractBatch    bool
	enableCrawlWebsite    bool
	all                   bool

	httpClient *http.Client
}

// Option configures TrafilaturaTools.
type Option func(*TrafilaturaTools)

// WithOutputFormat sets the default output format: txt (default), markdown, json or html.
func WithOutputFormat(f string) Option { return func(t *TrafilaturaTools) { t.outputFormat = f } }

// WithIncludeComments keeps comment sections in the text (default true).
func WithIncludeComments(b bool) Option { return func(t *TrafilaturaTools) { t.includeComments = b } }

// WithIncludeTables keeps table content (default true).
func WithIncludeTables(b bool) Option { return func(t *TrafilaturaTools) { t.includeTables = b } }

// WithIncludeImages includes image references (default false).
func WithIncludeImages(b bool) Option { return func(t *TrafilaturaTools) { t.includeImages = b } }

// WithIncludeLinks keeps links in markdown output (default false).
func WithIncludeLinks(b bool) Option { return func(t *TrafilaturaTools) { t.includeLinks = b } }

// WithMetadata prepends page metadata to extractions (default false).
func WithMetadata(b bool) Option { return func(t *TrafilaturaTools) { t.withMetadata = b } }

// WithDeduplicate removes duplicate text segments (default false).
func WithDeduplicate(b bool) Option { return func(t *TrafilaturaTools) { t.deduplicate = b } }

// WithMaxCrawlURLs sets the maximum number of pages visited by CrawlWebsite (default 10).
func WithMaxCrawlURLs(n int) Option { return func(t *TrafilaturaTools) { t.maxCrawlURLs = n } }

// WithMaxKnownURLs sets the maximum number of known URLs during crawling (default 100000).
func WithMaxKnownURLs(n int) Option { return func(t *TrafilaturaTools) { t.maxKnownURLs = n } }

// WithEnableExtractText toggles ExtractText (default true).
func WithEnableExtractText(b bool) Option {
	return func(t *TrafilaturaTools) { t.enableExtractText = b }
}

// WithEnableExtractMetadataOnly toggles ExtractMetadataOnly (default true).
func WithEnableExtractMetadataOnly(b bool) Option {
	return func(t *TrafilaturaTools) { t.enableExtractMetadata = b }
}

// WithEnableHTMLToText toggles HtmlToText (default true).
func WithEnableHTMLToText(b bool) Option { return func(t *TrafilaturaTools) { t.enableHTMLToText = b } }

// WithEnableExtractBatch toggles ExtractBatch (default true).
func WithEnableExtractBatch(b bool) Option {
	return func(t *TrafilaturaTools) { t.enableExtractBatch = b }
}

// WithEnableCrawlWebsite toggles CrawlWebsite (default true).
func WithEnableCrawlWebsite(b bool) Option {
	return func(t *TrafilaturaTools) { t.enableCrawlWebsite = b }
}

// WithAll enables every tool.
func WithAll() Option { return func(t *TrafilaturaTools) { t.all = true } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(t *TrafilaturaTools) { t.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *TrafilaturaTools) { t.httpClient = c } }

// ExtractTextParams are the parameters of ExtractText.
type ExtractTextParams struct {
	URL          string `json:"url" description:"The URL to extract content from." required:"true"`
	OutputFormat string `json:"output_format,omitempty" description:"Output format. Options: 'txt', 'json', 'markdown', 'html'."`
}

// ExtractMetadataParams are the parameters of ExtractMetadataOnly.
type ExtractMetadataParams struct {
	URL string `json:"url" description:"The URL to extract metadata from." required:"true"`
	// AsJSON defaults to true when the tool is called through Execute and the
	// argument is omitted (Python default).
	AsJSON bool `json:"as_json" description:"Whether to return metadata as JSON string. Default is true."`
}

// HTMLToTextParams are the parameters of HtmlToText.
type HTMLToTextParams struct {
	HTMLContent string `json:"html_content" description:"The HTML content to convert." required:"true"`
	// Clean defaults to true when the tool is called through Execute and the
	// argument is omitted (Python default).
	Clean bool `json:"clean" description:"Whether to remove potentially undesirable elements. Default is true."`
}

// ExtractBatchParams are the parameters of ExtractBatch.
type ExtractBatchParams struct {
	URLs []string `json:"urls" description:"List of URLs to extract content from." required:"true"`
}

// CrawlWebsiteParams are the parameters of CrawlWebsite.
type CrawlWebsiteParams struct {
	HomepageURL    string `json:"homepage_url" description:"The starting URL (preferably homepage) to crawl from." required:"true"`
	ExtractContent bool   `json:"extract_content,omitempty" description:"Whether to extract content from discovered URLs."`
}

// New creates the Trafilatura toolkit.
func New(opts ...Option) *TrafilaturaTools {
	t := &TrafilaturaTools{
		outputFormat:          "txt",
		includeComments:       true,
		includeTables:         true,
		maxCrawlURLs:          10,
		maxKnownURLs:          100000,
		enableExtractText:     true,
		enableExtractMetadata: true,
		enableHTMLToText:      true,
		enableExtractBatch:    true,
		enableCrawlWebsite:    true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "trafilatura_tools"
	t.Toolkit.Description = "Web scraping and main-text extraction from web pages."
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.all || t.enableExtractText {
		t.Toolkit.Register("ExtractText", "Extract main text content from a web page URL.", t, t.ExtractText, ExtractTextParams{})
	}
	if t.all || t.enableExtractMetadata {
		t.Toolkit.Register("ExtractMetadataOnly", "Extract only metadata (title, author, date, description, tags...) from a web page URL.", t, t.ExtractMetadataOnly, ExtractMetadataParams{})
	}
	if t.all || t.enableHTMLToText {
		t.Toolkit.Register("HtmlToText", "Convert HTML content to plain text.", t, t.HtmlToText, HTMLToTextParams{})
	}
	if t.all || t.enableExtractBatch {
		t.Toolkit.Register("ExtractBatch", "Extract content from multiple URLs in batch. Returns JSON with batch extraction results.", t, t.ExtractBatch, ExtractBatchParams{})
	}
	if t.all || t.enableCrawlWebsite {
		t.Toolkit.Register("CrawlWebsite", "Crawl a website and optionally extract content from discovered pages. Returns JSON with crawl results.", t, t.CrawlWebsite, CrawlWebsiteParams{})
	}
	return t
}

func (t *TrafilaturaTools) extractOptions(markdown bool) ExtractOptions {
	return ExtractOptions{
		Markdown:        markdown,
		IncludeComments: t.includeComments,
		IncludeTables:   t.includeTables,
		IncludeImages:   t.includeImages,
		IncludeLinks:    t.includeLinks,
		Deduplicate:     t.deduplicate,
	}
}

// extract fetches pageURL and renders it in format. ok=false means the page
// could not be fetched (fetched=false) or had no readable content.
func (t *TrafilaturaTools) extract(pageURL, format string) (result string, fetched bool, err error) {
	page, _, err := FetchPage(t.httpClient, pageURL)
	if err != nil || len(page) == 0 {
		return "", false, err
	}
	doc, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return "", true, err
	}
	main := MainNode(doc)
	format = strings.ToLower(format)
	switch format {
	case "html":
		var sb strings.Builder
		if err := html.Render(&sb, main); err != nil {
			return "", true, err
		}
		return sb.String(), true, nil
	case "json":
		text := ExtractText(main, t.extractOptions(false))
		if text == "" {
			return "", true, nil
		}
		out := ExtractMetadata(doc, pageURL)
		out["source"] = pageURL
		out["text"] = text
		out["raw_text"] = strings.Join(strings.Fields(text), " ")
		return restkit.CompactJSON(out), true, nil
	default:
		text := ExtractText(main, t.extractOptions(format == "markdown" || format == "md"))
		if text == "" {
			return "", true, nil
		}
		if t.withMetadata {
			meta := ExtractMetadata(doc, pageURL)
			var sb strings.Builder
			sb.WriteString("---\n")
			for _, k := range MetadataKeys {
				if v := meta[k]; v != nil {
					fmt.Fprintf(&sb, "%s: %s\n", k, formatValue(v))
				}
			}
			sb.WriteString("---\n")
			text = sb.String() + text
		}
		return text, true, nil
	}
}

func formatValue(v interface{}) string {
	if l, ok := v.([]string); ok {
		return strings.Join(l, ", ")
	}
	return fmt.Sprint(v)
}

// ExtractText extracts the main text content of a URL.
func (t *TrafilaturaTools) ExtractText(params ExtractTextParams) (interface{}, error) {
	format := params.OutputFormat
	if format == "" {
		format = t.outputFormat
	}
	result, fetched, err := t.extract(params.URL, format)
	if !fetched {
		return fmt.Sprintf("Error: Could not fetch content from URL: %s", params.URL), nil
	}
	if err != nil {
		return fmt.Sprintf("Error extracting text from %s: %v", params.URL, err), nil
	}
	if result == "" {
		return fmt.Sprintf("Error: Could not extract readable content from URL: %s", params.URL), nil
	}
	return result, nil
}

// ExtractMetadataOnly extracts page metadata only.
func (t *TrafilaturaTools) ExtractMetadataOnly(params ExtractMetadataParams) (interface{}, error) {
	page, _, err := FetchPage(t.httpClient, params.URL)
	if err != nil || len(page) == 0 {
		return fmt.Sprintf("Error: Could not fetch content from URL: %s", params.URL), nil
	}
	doc, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		return fmt.Sprintf("Error extracting metadata from %s: %v", params.URL, err), nil
	}
	meta := ExtractMetadata(doc, params.URL)
	if params.AsJSON {
		return restkit.JSONString(meta), nil
	}
	lines := make([]string, 0, len(MetadataKeys))
	for _, k := range MetadataKeys {
		v := meta[k]
		if v == nil {
			lines = append(lines, k+": None")
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", k, formatValue(v)))
	}
	return strings.Join(lines, "\n"), nil
}

// HtmlToText converts HTML content to plain text.
func (t *TrafilaturaTools) HtmlToText(params HTMLToTextParams) (interface{}, error) {
	result, err := HTMLToText(params.HTMLContent, params.Clean)
	if err != nil {
		return fmt.Sprintf("Error converting HTML to text: %v", err), nil
	}
	if result == "" {
		return "Error: Could not extract text from HTML content", nil
	}
	return result, nil
}

// ExtractBatch extracts content from several URLs.
func (t *TrafilaturaTools) ExtractBatch(params ExtractBatchParams) (interface{}, error) {
	results := map[string]string{}
	failed := []string{}
	errored := 0
	for _, u := range params.URLs {
		content, fetched, err := t.extract(u, t.outputFormat)
		switch {
		case err != nil && fetched:
			failed = append(failed, u)
			results[u] = fmt.Sprintf("Error: %v", err)
			errored++
		case content == "":
			failed = append(failed, u)
		default:
			results[u] = content
		}
	}
	return restkit.JSONString(map[string]interface{}{
		"successful_extractions": len(results) - errored,
		"failed_extractions":     len(failed),
		"total_urls":             len(params.URLs),
		"results":                results,
		"failed_urls":            failed,
	}), nil
}

// CrawlWebsite discovers same-site links starting at the homepage
// (trafilatura's focused_crawler) and optionally extracts their content.
func (t *TrafilaturaTools) CrawlWebsite(params CrawlWebsiteParams) (interface{}, error) {
	home, err := url.Parse(params.HomepageURL)
	if err != nil || home.Host == "" {
		return fmt.Sprintf("Error crawling website %s: invalid URL", params.HomepageURL), nil
	}
	known := []string{}
	knownSet := map[string]bool{}
	visited := map[string]bool{}
	addKnown := func(u string) {
		if !knownSet[u] && len(known) < t.maxKnownURLs {
			knownSet[u] = true
			known = append(known, u)
		}
	}
	queue := []string{home.String()}
	addKnown(home.String())
	seen := 0
	for len(queue) > 0 && seen < t.maxCrawlURLs {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		seen++
		page, _, err := FetchPage(t.httpClient, cur)
		if err != nil {
			if cur == home.String() {
				return fmt.Sprintf("Error crawling website %s: %v", params.HomepageURL, err), nil
			}
			continue
		}
		doc, err := html.Parse(strings.NewReader(string(page)))
		if err != nil {
			continue
		}
		base, _ := url.Parse(cur)
		for _, link := range ExtractLinks(doc, base) {
			lu, err := url.Parse(link)
			if err != nil || lu.Host != home.Host {
				continue
			}
			addKnown(link)
			if !visited[link] {
				queue = append(queue, link)
			}
		}
	}
	toVisit := []string{}
	for _, u := range known {
		if !visited[u] {
			toVisit = append(toVisit, u)
		}
	}
	out := map[string]interface{}{
		"homepage":    params.HomepageURL,
		"to_visit":    toVisit,
		"known_links": known,
		"stats": map[string]int{
			"urls_to_visit":     len(toVisit),
			"known_links_count": len(known),
		},
	}
	if params.ExtractContent && len(known) > 0 {
		extracted := map[string]string{}
		limit := len(known)
		if limit > 10 {
			limit = 10
		}
		for _, u := range known[:limit] {
			content, _, err := t.extract(u, t.outputFormat)
			if err != nil {
				extracted[u] = fmt.Sprintf("Error extracting content: %v", err)
			} else if content != "" {
				extracted[u] = content
			}
		}
		out["extracted_content"] = extracted
	}
	return restkit.JSONString(out), nil
}

// Execute implements toolkit.Tool.
// Omitted boolean arguments whose Python default is true are filled in first.
func (t *TrafilaturaTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	switch {
	case strings.HasSuffix(methodName, "ExtractMetadataOnly"):
		input = withDefault(input, "as_json", true)
	case strings.HasSuffix(methodName, "HtmlToText"):
		input = withDefault(input, "clean", true)
	}
	return t.Toolkit.Execute(methodName, input)
}

// withDefault sets key to val in the JSON object input when it is absent.
func withDefault(input json.RawMessage, key string, val interface{}) json.RawMessage {
	var m map[string]interface{}
	if err := json.Unmarshal(input, &m); err != nil || m == nil {
		return input
	}
	if _, ok := m[key]; ok {
		return input
	}
	m[key] = val
	out, err := json.Marshal(m)
	if err != nil {
		return input
	}
	return out
}
