// Package llmstxt ports Agno's LLMsTxtTools: read an llms.txt index
// (https://llmstxt.org) and fetch the documentation pages it links to, so an
// agent can pick which pages to read. Only the agentic mode (no knowledge
// base) is ported: GetLlmsTxtIndex and ReadLlmsTxtUrl.
package llmstxt

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/trafilatura"
	"golang.org/x/net/html"
)

// DefaultTimeout mirrors Python's timeout=60.
const DefaultTimeout = 60 * time.Second

// LLMsTxtTools reads llms.txt indexes and their linked pages.
type LLMsTxtTools struct {
	toolkit.Toolkit
	maxURLs      int
	skipOptional bool
	allowedHosts []string
	httpClient   *http.Client
}

// Option configures LLMsTxtTools.
type Option func(*LLMsTxtTools)

// WithMaxURLs sets the maximum number of linked URLs (default 20; kept for
// parity, used by the knowledge mode in Python).
func WithMaxURLs(n int) Option { return func(l *LLMsTxtTools) { l.maxURLs = n } }

// WithSkipOptional skips entries of the "Optional" section.
func WithSkipOptional(b bool) Option { return func(l *LLMsTxtTools) { l.skipOptional = b } }

// WithAllowedHosts restricts fetching (including redirects) to these hostnames
// (case-insensitive exact match). Nil allows every host.
func WithAllowedHosts(hosts ...string) Option {
	return func(l *LLMsTxtTools) { l.allowedHosts = hosts }
}

// WithTimeout sets the HTTP timeout (default 60s).
func WithTimeout(d time.Duration) Option { return func(l *LLMsTxtTools) { l.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(l *LLMsTxtTools) { l.httpClient = c } }

// URLParams carries a URL.
type URLParams struct {
	URL string `json:"url" description:"The URL to read." required:"true"`
}

// Entry is a link listed in an llms.txt file.
type Entry struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Section     string `json:"section"`
}

// New creates the llms.txt toolkit.
func New(opts ...Option) *LLMsTxtTools {
	l := &LLMsTxtTools{maxURLs: 20}
	l.Toolkit = toolkit.NewToolkit()
	l.Toolkit.Name = "llms_txt_tools"
	l.Toolkit.Description = "Discover and read documentation through llms.txt files."
	l.Toolkit.Timeout = DefaultTimeout
	for _, o := range opts {
		o(l)
	}
	if l.httpClient == nil {
		l.httpClient = l.Toolkit.HTTPClient()
	}
	if l.allowedHosts != nil {
		guarded := *l.httpClient
		prev := guarded.CheckRedirect
		guarded.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if !l.hostAllowed(req.URL.String()) {
				return fmt.Errorf("redirect to disallowed host %s", req.URL.Hostname())
			}
			if prev != nil {
				return prev(req, via)
			}
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		}
		l.httpClient = &guarded
	}
	l.Toolkit.Register("GetLlmsTxtIndex", "Read an llms.txt file and return the index of available documentation pages (JSON). Use ReadLlmsTxtUrl afterwards to fetch specific pages.", l, l.GetLlmsTxtIndex, URLParams{})
	l.Toolkit.Register("ReadLlmsTxtUrl", "Fetch and return the text content of a documentation page. Use after GetLlmsTxtIndex.", l, l.ReadLlmsTxtUrl, URLParams{})
	return l
}

func (l *LLMsTxtTools) hostAllowed(raw string) bool {
	if l.allowedHosts == nil {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return false
	}
	for _, h := range l.allowedHosts {
		if strings.EqualFold(u.Hostname(), h) {
			return true
		}
	}
	return false
}

// fetch returns the page text: plain text / markdown as-is, HTML reduced to
// its main content.
func (l *LLMsTxtTools) fetch(raw string) (string, error) {
	body, contentType, err := trafilatura.FetchPage(l.httpClient, raw)
	if err != nil {
		return "", err
	}
	return processResponse(contentType, string(body)), nil
}

func processResponse(contentType, text string) string {
	if strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "text/markdown") {
		return text
	}
	trimmed := strings.TrimSpace(text)
	if strings.Contains(contentType, "text/html") || strings.HasPrefix(trimmed, "<!DOCTYPE") || strings.HasPrefix(trimmed, "<html") || strings.HasPrefix(trimmed, "<HTML") {
		doc, err := html.Parse(strings.NewReader(text))
		if err != nil {
			return text
		}
		return trafilatura.ExtractText(trafilatura.MainNode(doc), trafilatura.ExtractOptions{IncludeTables: true, IncludeComments: true})
	}
	return text
}

var (
	linkPattern    = regexp.MustCompile(`^-\s+\[([^\]]+)\]\(([^)]+)\)(?::\s*(.+))?`)
	sectionPattern = regexp.MustCompile(`^##\s+(.+)$`)
)

// ParseLlmsTxt splits an llms.txt document into its overview and link entries.
func ParseLlmsTxt(content, baseURL string, skipOptional bool) (string, []Entry) {
	entries := []Entry{}
	section := ""
	var overview []string
	base, _ := url.Parse(baseURL)
	for _, line := range strings.Split(content, "\n") {
		if m := sectionPattern.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil {
			section = strings.TrimSpace(m[1])
			continue
		}
		if section == "" {
			overview = append(overview, line)
			continue
		}
		if skipOptional && strings.EqualFold(section, "optional") {
			continue
		}
		m := linkPattern.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		link := strings.TrimSpace(m[2])
		if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") && base != nil {
			if ref, err := base.Parse(link); err == nil {
				link = ref.String()
			}
		}
		entries = append(entries, Entry{
			Title:       strings.TrimSpace(m[1]),
			URL:         link,
			Description: strings.TrimSpace(m[3]),
			Section:     section,
		})
	}
	return strings.TrimSpace(strings.Join(overview, "\n")), entries
}

// GetLlmsTxtIndex returns {overview, pages, total_pages} for an llms.txt URL.
func (l *LLMsTxtTools) GetLlmsTxtIndex(params URLParams) (interface{}, error) {
	if !l.hostAllowed(params.URL) {
		return fmt.Sprintf("Host is not in allowed_hosts; refusing to fetch %s", params.URL), nil
	}
	content, err := l.fetch(params.URL)
	if err != nil || content == "" {
		return fmt.Sprintf("Failed to fetch llms.txt from %s", params.URL), nil
	}
	overview, entries := ParseLlmsTxt(content, params.URL, l.skipOptional)
	return restkit.CompactJSON(map[string]interface{}{
		"overview":    overview,
		"pages":       entries,
		"total_pages": len(entries),
	}), nil
}

// ReadLlmsTxtUrl returns the text content of a documentation page.
func (l *LLMsTxtTools) ReadLlmsTxtUrl(params URLParams) (interface{}, error) {
	if !l.hostAllowed(params.URL) {
		return fmt.Sprintf("Host is not in allowed_hosts; refusing to fetch %s", params.URL), nil
	}
	content, err := l.fetch(params.URL)
	if err != nil || content == "" {
		return fmt.Sprintf("Failed to fetch content from %s", params.URL), nil
	}
	return content, nil
}

// Execute implements toolkit.Tool.
func (l *LLMsTxtTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return l.Toolkit.Execute(methodName, input)
}
