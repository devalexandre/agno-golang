package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// KeenableDefaultBaseURL is the Keenable Search API base URL.
const KeenableDefaultBaseURL = "https://api.keenable.ai"

// keenableSearchMode is Keenable's documented search mode used by this toolkit.
const keenableSearchMode = "pro"

// keenableInternalSuffixes are special-use names that never resolve publicly.
var keenableInternalSuffixes = []string{".localhost", ".local", ".internal", ".home.arpa"}

// keenableLookupIP resolves hostnames for the fetch target check (overridable in tests).
var keenableLookupIP = func(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// KeenableTools provides web search and page extraction through the Keenable
// Search API. It is keyless by default: with no API key it calls Keenable's
// public endpoints; setting KEENABLE_API_KEY (or WithKeenableAPIKey) switches
// to the authenticated endpoints, which lift rate limits.
type KeenableTools struct {
	toolkit.Toolkit
	apiKey          string
	baseURL         string
	maxResults      int
	maxSnippetChars int
	format          string
	httpClient      *http.Client

	enableSearch bool
	enableFetch  bool
}

// KeenableOption configures KeenableTools.
type KeenableOption func(*KeenableTools)

// WithKeenableAPIKey sets the API key (default: KEENABLE_API_KEY, else keyless).
func WithKeenableAPIKey(key string) KeenableOption {
	return func(k *KeenableTools) { k.apiKey = key }
}

// WithKeenableBaseURL overrides the API base URL (default: KEENABLE_API_URL, else
// https://api.keenable.ai). Must be https, or http on a loopback host.
func WithKeenableBaseURL(u string) KeenableOption {
	return func(k *KeenableTools) { k.baseURL = u }
}

// WithKeenableSearch enables or disables the WebSearch method (default enabled).
func WithKeenableSearch(enabled bool) KeenableOption {
	return func(k *KeenableTools) { k.enableSearch = enabled }
}

// WithKeenableFetch enables or disables the FetchURLContent method (default disabled).
func WithKeenableFetch(enabled bool) KeenableOption {
	return func(k *KeenableTools) { k.enableFetch = enabled }
}

// WithKeenableAll enables every method.
func WithKeenableAll() KeenableOption {
	return func(k *KeenableTools) { k.enableSearch, k.enableFetch = true, true }
}

// WithKeenableMaxResults sets the default number of search results (default 5).
func WithKeenableMaxResults(n int) KeenableOption {
	return func(k *KeenableTools) { k.maxResults = n }
}

// WithKeenableMaxSnippetChars caps each result's content (default 500, 0 = no cap).
func WithKeenableMaxSnippetChars(n int) KeenableOption {
	return func(k *KeenableTools) { k.maxSnippetChars = n }
}

// WithKeenableFormat sets the search output format: "markdown" (default) or "json".
func WithKeenableFormat(format string) KeenableOption {
	return func(k *KeenableTools) { k.format = format }
}

// WithKeenableTimeout sets the request timeout (default toolkit.DefaultHTTPTimeout).
func WithKeenableTimeout(d time.Duration) KeenableOption {
	return func(k *KeenableTools) { k.Toolkit.Timeout = d }
}

// WithKeenableHTTPClient replaces the HTTP client (e.g. for tests or proxies).
func WithKeenableHTTPClient(c *http.Client) KeenableOption {
	return func(k *KeenableTools) { k.httpClient = c }
}

// KeenableSearchParams are the parameters of WebSearch.
type KeenableSearchParams struct {
	Query      string `json:"query" description:"Query to search for." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of results to return. Defaults to the toolkit setting."`
}

// KeenableFetchParams are the parameters of FetchURLContent.
type KeenableFetchParams struct {
	URL string `json:"url" description:"URL of the page to fetch." required:"true"`
}

// NewKeenableTools creates the Keenable toolkit.
func NewKeenableTools(opts ...KeenableOption) *KeenableTools {
	k := &KeenableTools{
		apiKey:          os.Getenv("KEENABLE_API_KEY"),
		baseURL:         os.Getenv("KEENABLE_API_URL"),
		maxResults:      5,
		maxSnippetChars: 500,
		format:          "markdown",
		enableSearch:    true,
	}
	k.Toolkit = toolkit.NewToolkit()
	k.Toolkit.Name = "KeenableTools"
	k.Toolkit.Description = "Keyless web search and page content extraction powered by the Keenable Search API."
	for _, opt := range opts {
		opt(k)
	}
	if k.baseURL == "" {
		k.baseURL = KeenableDefaultBaseURL
	}
	k.baseURL = strings.TrimRight(k.baseURL, "/")
	if k.httpClient == nil {
		k.httpClient = k.Toolkit.HTTPClient()
	}

	if k.enableSearch {
		k.Toolkit.Register("WebSearch", "Search the live web for a query and return ranked results (title, url, content).", k, k.WebSearch, KeenableSearchParams{})
	}
	if k.enableFetch {
		k.Toolkit.Register("FetchURLContent", "Read a web page and return its main content as clean markdown, prefixed with its title.", k, k.FetchURLContent, KeenableFetchParams{})
	}
	return k
}

// WebSearch searches the web through Keenable.
func (k *KeenableTools) WebSearch(params KeenableSearchParams) (interface{}, error) {
	limit := params.MaxResults
	if limit <= 0 {
		limit = k.maxResults
	}
	if strings.TrimSpace(params.Query) == "" {
		return "Error: a non-empty search query is required.", nil
	}
	endpoint, err := k.endpoint("/v1/search/public", "/v1/search")
	if err != nil {
		return fmt.Sprintf("Error performing search: %v", err), nil
	}

	body, _ := json.Marshal(map[string]string{"query": params.Query, "mode": keenableSearchMode})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Sprintf("Error performing search: %v", err), nil
	}
	status, data, err := k.do(req)
	if err != nil {
		return fmt.Sprintf("Error performing search: %v", err), nil
	}
	if msg := keenableErrorMessage(status, data); msg != "" {
		return msg, nil
	}

	var parsed struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "Error performing search: the Keenable API returned a non-JSON response.", nil
	}
	if parsed.Results == nil {
		return "Error performing search: unexpected response from the Keenable API.", nil
	}

	type result struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	}
	var results []result
	for i, r := range parsed.Results {
		if i >= limit {
			break
		}
		u, _ := r["url"].(string)
		if u == "" {
			continue
		}
		title, _ := r["title"].(string)
		results = append(results, result{Title: title, URL: u, Content: k.resultContent(r)})
	}
	if len(results) == 0 {
		return "No results found.", nil
	}

	if k.format == "json" {
		out, _ := json.MarshalIndent(map[string]interface{}{"query": params.Query, "results": results}, "", "  ")
		return string(out), nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", params.Query)
	for _, r := range results {
		fmt.Fprintf(&sb, "### [%s](%s)\n%s\n\n", r.Title, r.URL, r.Content)
	}
	return sb.String(), nil
}

// FetchURLContent fetches a page's main content through Keenable. URLs that
// point to private/internal hosts are refused so they are not disclosed.
func (k *KeenableTools) FetchURLContent(params KeenableFetchParams) (interface{}, error) {
	if msg := keenableFetchTargetError(params.URL); msg != "" {
		return msg, nil
	}
	endpoint, err := k.endpoint("/v1/fetch/public", "/v1/fetch")
	if err != nil {
		return fmt.Sprintf("Error fetching page content: %v", err), nil
	}
	req, err := http.NewRequest(http.MethodGet, endpoint+"?"+url.Values{"url": {params.URL}}.Encode(), nil)
	if err != nil {
		return fmt.Sprintf("Error fetching page content: %v", err), nil
	}
	status, data, err := k.do(req)
	if err != nil {
		return fmt.Sprintf("Error fetching page content: %v", err), nil
	}
	if msg := keenableErrorMessage(status, data); msg != "" {
		return msg, nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "Error fetching page content: the Keenable API returned a non-JSON response.", nil
	}
	content, _ := parsed["content"].(string)
	if content == "" {
		return fmt.Sprintf("No content could be extracted from %s.", params.URL), nil
	}
	title, _ := parsed["title"].(string)
	if title == "" {
		title, _ = parsed["url"].(string)
	}
	if title == "" {
		title = params.URL
	}
	return fmt.Sprintf("## %s\n\n%s", title, content), nil
}

func (k *KeenableTools) hasKey() bool { return strings.TrimSpace(k.apiKey) != "" }

// endpoint returns the keyless or authenticated endpoint, enforcing https
// outside of loopback hosts.
func (k *KeenableTools) endpoint(publicPath, keyedPath string) (string, error) {
	u, err := url.Parse(k.baseURL)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("Keenable base URL must be an https:// URL with a host, got %q", k.baseURL)
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"):
	default:
		return "", fmt.Errorf("Keenable base URL must be an https:// URL with a host, got %q", k.baseURL)
	}
	if k.hasKey() {
		return k.baseURL + keyedPath, nil
	}
	return k.baseURL + publicPath, nil
}

func (k *KeenableTools) do(req *http.Request) (int, []byte, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "keenable-agno")
	req.Header.Set("X-Keenable-Title", "Agno")
	if k.hasKey() {
		req.Header.Set("X-API-Key", strings.TrimSpace(k.apiKey))
	}
	resp, err := k.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	return resp.StatusCode, data, err
}

// resultContent returns snippet (or description), whitespace-collapsed and capped.
func (k *KeenableTools) resultContent(r map[string]interface{}) string {
	text, _ := r["snippet"].(string)
	if text == "" {
		text, _ = r["description"].(string)
	}
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); k.maxSnippetChars > 0 && len(runes) > k.maxSnippetChars {
		return strings.TrimRight(string(runes[:k.maxSnippetChars]), " ") + "…"
	}
	return text
}

func keenableErrorMessage(status int, body []byte) string {
	if status >= 200 && status < 300 {
		return ""
	}
	detail := ""
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err == nil {
		for _, key := range []string{"message", "error", "detail"} {
			if v, ok := obj[key]; ok && v != nil && fmt.Sprint(v) != "" {
				detail = fmt.Sprint(v)
				break
			}
		}
	} else {
		detail = strings.TrimSpace(string(body))
	}
	label := fmt.Sprintf("Keenable API error (%d)", status)
	switch status {
	case 401:
		label = "Keenable authentication failed (401)"
	case 402:
		label = "Keenable: insufficient credits (402)"
	case 429:
		label = "Keenable rate limit exceeded (429)"
	}
	if detail != "" {
		return label + ": " + detail
	}
	return label
}

// keenableFetchTargetError returns why rawURL must not be sent to the fetch API.
func keenableFetchTargetError(rawURL string) string {
	lower := strings.ToLower(rawURL)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return fmt.Sprintf("Error: refusing to fetch a non-http(s) URL: %q", rawURL)
	}
	u, err := url.Parse(rawURL)
	host := ""
	if err == nil {
		host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(u.Hostname())), ".")
	}
	if host == "" {
		return fmt.Sprintf("Error: refusing to fetch a URL without a valid host: %q", rawURL)
	}
	if host == "localhost" {
		return fmt.Sprintf("Error: refusing to fetch a private/internal host: %q", host)
	}
	for _, s := range keenableInternalSuffixes {
		if strings.HasSuffix(host, s) {
			return fmt.Sprintf("Error: refusing to fetch a private/internal host: %q", host)
		}
	}

	var ips []net.IP
	if ip := net.ParseIP(strings.SplitN(host, "%", 2)[0]); ip != nil {
		ips = []net.IP{ip}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ips, err = keenableLookupIP(ctx, host)
		if err != nil {
			return fmt.Sprintf("Error: could not resolve host %q, so the URL was not sent for fetching: %v", host, err)
		}
	}
	if len(ips) == 0 {
		return fmt.Sprintf("Error: refusing to fetch a private/internal host: %q", host)
	}
	for _, ip := range ips {
		if isNonPublicIP(ip) {
			return fmt.Sprintf("Error: refusing to fetch a private/internal host: %q", host)
		}
	}
	return ""
}

// nonGlobalNets are special-purpose ranges not covered by net.IP helpers.
var nonGlobalNets = func() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "255.255.255.255/32",
		"64:ff9b:1::/48", "100::/64", "2001::/23", "2001:db8::/32",
	} {
		_, n, _ := net.ParseCIDR(c)
		out = append(out, n)
	}
	return out
}()

// isNonPublicIP reports whether ip is not a public unicast address.
func isNonPublicIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4 // also unwraps IPv4-mapped IPv6
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || !ip.IsGlobalUnicast() {
		return true
	}
	for _, n := range nonGlobalNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
