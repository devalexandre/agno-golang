// Package baidusearch ports Agno's BaiduSearchTools: Baidu web search. Like the
// `baidusearch` Python library it relies on, it fetches Baidu's HTML results
// page (https://www.baidu.com/s?wd=...) and parses the result containers,
// following the "next page" link until enough results are collected.
//
// Baidu may answer automated traffic with a CAPTCHA/verification page; in that
// case no result containers are found and an empty list is returned.
package baidusearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is Baidu's host.
const DefaultBaseURL = "https://www.baidu.com"

// abstractMaxLength mirrors the baidusearch library's ABSTRACT_MAX_LENGTH.
const abstractMaxLength = 300

// maxPages bounds pagination.
const maxPages = 10

var defaultHeaders = map[string]string{
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36",
	"Referer":         "https://www.baidu.com/",
}

// languageNames maps common language names to ISO 639-1 codes (stand-in for pycountry).
var languageNames = map[string]string{
	"chinese": "zh", "english": "en", "japanese": "ja", "korean": "ko", "french": "fr",
	"german": "de", "spanish": "es", "portuguese": "pt", "russian": "ru", "arabic": "ar", "italian": "it",
}

// BaiduSearchTools searches Baidu.
type BaiduSearchTools struct {
	toolkit.Toolkit
	baseURL         string
	fixedMaxResults int
	fixedLanguage   string
	headers         map[string]string
	enableSearch    bool
	all             bool
	httpClient      *http.Client
}

// Option configures BaiduSearchTools.
type Option func(*BaiduSearchTools)

// WithBaseURL overrides the Baidu host (useful for tests).
func WithBaseURL(u string) Option { return func(b *BaiduSearchTools) { b.baseURL = u } }

// WithFixedMaxResults forces the number of results regardless of the call argument.
func WithFixedMaxResults(n int) Option { return func(b *BaiduSearchTools) { b.fixedMaxResults = n } }

// WithFixedLanguage forces the search language regardless of the call argument.
func WithFixedLanguage(l string) Option { return func(b *BaiduSearchTools) { b.fixedLanguage = l } }

// WithHeaders overrides/adds request headers.
func WithHeaders(h map[string]string) Option { return func(b *BaiduSearchTools) { b.headers = h } }

// WithEnableBaiduSearch toggles BaiduSearch (default true).
func WithEnableBaiduSearch(v bool) Option { return func(b *BaiduSearchTools) { b.enableSearch = v } }

// WithAll enables every tool.
func WithAll() Option { return func(b *BaiduSearchTools) { b.all = true } }

// WithTimeout sets the HTTP timeout (Python default 10s).
func WithTimeout(d time.Duration) Option { return func(b *BaiduSearchTools) { b.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(b *BaiduSearchTools) { b.httpClient = c } }

// SearchParams are the parameters of BaiduSearch.
type SearchParams struct {
	Query      string `json:"query" description:"Search keyword" required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of results to return, default 5"`
	Language   string `json:"language,omitempty" description:"Search language, default Chinese (zh)"`
}

// New creates the Baidu search toolkit.
func New(opts ...Option) *BaiduSearchTools {
	b := &BaiduSearchTools{baseURL: DefaultBaseURL, enableSearch: true}
	b.Toolkit = toolkit.NewToolkit()
	b.Toolkit.Name = "baidusearch"
	b.Toolkit.Description = "Search the web using Baidu."
	b.Toolkit.Timeout = 10 * time.Second
	for _, o := range opts {
		o(b)
	}
	b.baseURL = strings.TrimRight(b.baseURL, "/")
	if b.httpClient == nil {
		b.httpClient = b.Toolkit.HTTPClient()
	}
	if b.all || b.enableSearch {
		b.Toolkit.Register("BaiduSearch", "Execute Baidu search and return results as a JSON formatted string.", b, b.BaiduSearch, SearchParams{})
	}
	return b
}

type result struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Abstract string `json:"abstract"`
	Rank     string `json:"rank"`
}

// BaiduSearch returns a JSON array of {title, url, abstract, rank}.
func (b *BaiduSearchTools) BaiduSearch(p SearchParams) (interface{}, error) {
	maxResults := p.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	if b.fixedMaxResults > 0 {
		maxResults = b.fixedMaxResults
	}
	// Language is resolved for parity with Python, which only logs it.
	language := p.Language
	if b.fixedLanguage != "" {
		language = b.fixedLanguage
	}
	if len(language) != 2 {
		if code, ok := languageNames[strings.ToLower(language)]; ok {
			language = code
		} else {
			language = "zh"
		}
	}
	_ = language

	results := []result{}
	next := b.baseURL + "/s?" + url.Values{"ie": {"utf-8"}, "tn": {"baidu"}, "wd": {p.Query}}.Encode()
	for page := 0; next != "" && len(results) < maxResults && page < maxPages; page++ {
		items, nextURL, err := b.parsePage(next)
		if err != nil {
			if page == 0 {
				return nil, err
			}
			break
		}
		for _, it := range items {
			if len(results) >= maxResults {
				break
			}
			it.Rank = strconv.Itoa(len(results) + 1)
			results = append(results, it)
		}
		if len(items) == 0 {
			break
		}
		next = nextURL
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(results)
	return strings.TrimRight(buf.String(), "\n"), nil
}

func (b *BaiduSearchTools) parsePage(pageURL string) ([]result, string, error) {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", err
	}
	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range b.headers {
		req.Header.Set(k, v)
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("baidu request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, restkit.MaxBodyBytes))
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", &restkit.APIError{Name: "Baidu", Status: resp.StatusCode, Body: string(body)}
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse Baidu HTML: %w", err)
	}
	var items []result
	doc.Find("#content_left > div.c-container, #content_left > div.result, #content_left > div.result-op").Each(func(_ int, s *goquery.Selection) {
		titleSel := s.Find("h3").First()
		title := clean(titleSel.Text())
		link, _ := titleSel.Find("a").First().Attr("href")
		if link == "" {
			link, _ = s.Find("a").First().Attr("href")
		}
		if link == "" {
			link, _ = s.Attr("mu")
		}
		abstract := ""
		for _, sel := range []string{".c-abstract", "[class*='content-right']", "[class*='c-span-last']", ".c-gap-top-small", "span[class*='content']"} {
			if t := clean(s.Find(sel).First().Text()); t != "" {
				abstract = t
				break
			}
		}
		if abstract == "" {
			abstract = strings.TrimSpace(strings.TrimPrefix(clean(s.Text()), title))
		}
		if title == "" && link == "" {
			return
		}
		items = append(items, result{Title: title, URL: strings.TrimSpace(link), Abstract: restkit.Truncate(abstract, abstractMaxLength)})
	})

	next := ""
	nextBtns := doc.Find("#page a.n, a.n")
	if nextBtns.Length() > 0 {
		last := nextBtns.Last()
		if !strings.Contains(last.Text(), "上一页") {
			if href, ok := last.Attr("href"); ok && href != "" {
				if u, err := url.Parse(href); err == nil {
					base, _ := url.Parse(pageURL)
					next = base.ResolveReference(u).String()
				}
			}
		}
	}
	return items, next, nil
}

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

// Execute implements toolkit.Tool.
func (b *BaiduSearchTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return b.Toolkit.Execute(methodName, input)
}
