// Package oxylabs ports Agno's OxylabsTools: Google search, Amazon product
// lookup and search, and universal website scraping through the Oxylabs Web
// Scraper API realtime endpoint (https://realtime.oxylabs.io/v1/queries).
package oxylabs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Oxylabs realtime queries endpoint.
const DefaultBaseURL = "https://realtime.oxylabs.io/v1/queries"

// DefaultTimeout covers the SDK's 165s request timeout.
const DefaultTimeout = 180 * time.Second

// OxylabsTools scrapes Google, Amazon and arbitrary websites with Oxylabs.
type OxylabsTools struct {
	toolkit.Toolkit
	username   string
	password   string
	markdown   bool
	baseURL    string
	httpClient *http.Client
	client     *restkit.Client
}

// Option configures OxylabsTools.
type Option func(*OxylabsTools)

// WithUsername sets the Oxylabs username (default: OXYLABS_USERNAME).
func WithUsername(u string) Option { return func(o *OxylabsTools) { o.username = u } }

// WithPassword sets the Oxylabs password (default: OXYLABS_PASSWORD).
func WithPassword(p string) Option { return func(o *OxylabsTools) { o.password = p } }

// WithMarkdown makes ScrapeWebsite return Markdown by default.
func WithMarkdown(v bool) Option { return func(o *OxylabsTools) { o.markdown = v } }

// WithBaseURL overrides the realtime endpoint URL.
func WithBaseURL(u string) Option { return func(o *OxylabsTools) { o.baseURL = u } }

// WithTimeout sets the HTTP timeout (default 180s).
func WithTimeout(d time.Duration) Option { return func(o *OxylabsTools) { o.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(o *OxylabsTools) { o.httpClient = c } }

// SearchGoogleParams are the parameters of SearchGoogle.
type SearchGoogleParams struct {
	Query      string `json:"query" description:"Search query" required:"true"`
	DomainCode string `json:"domain_code,omitempty" description:"Google domain to search (e.g., \"com\", \"co.uk\", \"de\", default: \"com\")"`
}

// AmazonProductParams are the parameters of GetAmazonProduct.
type AmazonProductParams struct {
	ASIN       string `json:"asin" description:"Amazon Standard Identification Number (10 alphanumeric characters, e.g., \"B07FZ8S74R\")" required:"true"`
	DomainCode string `json:"domain_code,omitempty" description:"Amazon domain (e.g., \"com\", \"co.uk\", \"de\", default: \"com\")"`
}

// SearchAmazonParams are the parameters of SearchAmazonProducts.
type SearchAmazonParams struct {
	Query      string `json:"query" description:"Product search query" required:"true"`
	DomainCode string `json:"domain_code,omitempty" description:"Amazon domain (e.g., \"com\", \"co.uk\", \"de\", default: \"com\")"`
}

// ScrapeWebsiteParams are the parameters of ScrapeWebsite.
type ScrapeWebsiteParams struct {
	URL              string `json:"url" description:"Website URL to scrape (must start with http:// or https://)" required:"true"`
	RenderJavascript bool   `json:"render_javascript,omitempty" description:"Whether to enable JavaScript rendering for dynamic content (default: False)"`
	// Markdown is optional and tri-state (unset = constructor setting); it
	// accepts a JSON boolean or a "true"/"false" string.
	Markdown interface{} `json:"markdown,omitempty" description:"\"true\" to return content as Markdown instead of parsed HTML, \"false\" for parsed HTML (default: uses constructor setting)"`
}

func optBool(v interface{}) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case *bool:
		if b != nil {
			return *b, true
		}
	case string:
		if p, err := strconv.ParseBool(strings.TrimSpace(b)); err == nil {
			return p, true
		}
	}
	return false, false
}

// New creates the Oxylabs toolkit.
func New(opts ...Option) *OxylabsTools {
	o := &OxylabsTools{baseURL: DefaultBaseURL}
	o.Toolkit = toolkit.NewToolkit()
	o.Toolkit.Name = "oxylabs_web_scraping"
	o.Toolkit.Description = "Scrape Google, Amazon and websites with the Oxylabs Web Scraper API."
	o.Toolkit.Timeout = DefaultTimeout
	for _, opt := range opts {
		opt(o)
	}
	o.username = restkit.Env(o.username, "OXYLABS_USERNAME")
	o.password = restkit.Env(o.password, "OXYLABS_PASSWORD")
	if o.httpClient == nil {
		o.httpClient = o.Toolkit.HTTPClient()
	}
	o.client = &restkit.Client{
		HTTP:      o.httpClient,
		BaseURL:   o.baseURL,
		Name:      "Oxylabs API",
		BasicUser: o.username,
		BasicPass: o.password,
	}
	tk := &o.Toolkit
	tk.Register("SearchGoogle", "Search Google for a query. Returns JSON of search results.", o, o.SearchGoogle, SearchGoogleParams{})
	tk.Register("GetAmazonProduct", "Get detailed information about an Amazon product by ASIN. Returns JSON of product details.", o, o.GetAmazonProduct, AmazonProductParams{})
	tk.Register("SearchAmazonProducts", "Search Amazon for products and return search results as JSON.", o, o.SearchAmazonProducts, SearchAmazonParams{})
	tk.Register("ScrapeWebsite", "Scrape content from any website URL. Returns JSON containing scraped content (Markdown if markdown=True, otherwise parsed HTML).", o, o.ScrapeWebsite, ScrapeWebsiteParams{})
	return o
}

type obj = map[string]interface{}

func errorResponse(tool, msg string, ctx obj) string {
	if ctx == nil {
		ctx = obj{}
	}
	return restkit.JSONString(obj{"tool": tool, "error": msg, "context": ctx})
}

// query posts a realtime job and returns the first result.
func (o *OxylabsTools) query(payload obj) (obj, error) {
	if o.username == "" || o.password == "" {
		return nil, fmt.Errorf("no Oxylabs credentials provided. Please set the OXYLABS_USERNAME and OXYLABS_PASSWORD environment variables or pass them to the OxylabsTools constructor")
	}
	var resp struct {
		Results []obj `json:"results"`
	}
	if err := o.client.Into(http.MethodPost, "", nil, payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, nil
	}
	return resp.Results[0], nil
}

func trim(m obj, k string) string { return strings.TrimSpace(restkit.Str(m, k)) }

func orDefault(m obj, k string, def interface{}) interface{} {
	if v, ok := m[k]; ok && v != nil {
		return v
	}
	return def
}

func validDomain(d string) bool { return len(d) <= 10 }

// SearchGoogle searches Google and returns organic results.
func (o *OxylabsTools) SearchGoogle(params SearchGoogleParams) (interface{}, error) {
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return errorResponse("search_google", "Query cannot be empty", obj{"query": params.Query}), nil
	}
	domain := params.DomainCode
	if domain == "" {
		domain = "com"
	}
	if !validDomain(domain) {
		return errorResponse("search_google", "Domain must be a valid string (e.g., 'com', 'co.uk')", nil), nil
	}
	res, err := o.query(obj{"source": "google_search", "query": query, "domain": domain, "parse": true})
	if err != nil {
		return errorResponse("search_google", "Google search failed: "+err.Error(), obj{"query": query}), nil
	}
	results := []obj{}
	content := restkit.Map(res, "content")
	for _, item := range restkit.Slice(restkit.Map(content, "results"), "organic") {
		results = append(results, obj{
			"title":       trim(item, "title"),
			"url":         trim(item, "url"),
			"description": trim(item, "desc"),
			"position":    orDefault(item, "pos", 0),
		})
	}
	return restkit.JSONString(obj{"tool": "search_google", "query": query, "results": results}), nil
}

// GetAmazonProduct looks up an Amazon product by ASIN.
func (o *OxylabsTools) GetAmazonProduct(params AmazonProductParams) (interface{}, error) {
	if params.ASIN == "" {
		return errorResponse("get_amazon_product", "ASIN is required and must be a string", nil), nil
	}
	asin := strings.ToUpper(strings.TrimSpace(params.ASIN))
	alnum := len(asin) == 10
	for _, r := range asin {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			alnum = false
		}
	}
	if !alnum {
		return errorResponse("get_amazon_product", fmt.Sprintf("Invalid ASIN format: %s. Must be 10 alphanumeric characters (e.g., 'B07FZ8S74R')", asin), nil), nil
	}
	domain := params.DomainCode
	if domain == "" {
		domain = "com"
	}
	if !validDomain(domain) {
		return errorResponse("get_amazon_product", "Domain must be a valid string (e.g., 'com', 'co.uk')", nil), nil
	}
	res, err := o.query(obj{"source": "amazon_product", "query": asin, "domain": domain, "parse": true})
	if err != nil {
		return errorResponse("get_amazon_product", "Amazon product lookup failed: "+err.Error(), obj{"asin": asin}), nil
	}
	info := obj{"found": false, "asin": asin, "domain": "amazon." + domain}
	if c := restkit.Map(res, "content"); len(c) > 0 {
		images, _ := c["images"].([]interface{})
		if len(images) > 3 {
			images = images[:3]
		}
		bullets := []interface{}{}
		switch bp := c["bullet_points"].(type) {
		case []interface{}:
			bullets = bp
			if len(bullets) > 5 {
				bullets = bullets[:5]
			}
		case string:
			if bp != "" {
				bullets = []interface{}{bp}
			}
		}
		if images == nil {
			images = []interface{}{}
		}
		info["found"] = true
		info["title"] = trim(c, "title")
		info["price"] = orDefault(c, "price", 0)
		info["currency"] = restkit.Str(c, "currency")
		info["rating"] = orDefault(c, "rating", 0)
		info["reviews_count"] = orDefault(c, "reviews_count", 0)
		info["url"] = restkit.Str(c, "url")
		info["description"] = trim(c, "description")
		info["stock_status"] = trim(c, "stock")
		info["brand"] = trim(c, "brand")
		info["images"] = images
		info["bullet_points"] = bullets
	}
	return restkit.JSONString(obj{"tool": "get_amazon_product", "asin": asin, "product_info": info}), nil
}

// SearchAmazonProducts searches Amazon for products.
func (o *OxylabsTools) SearchAmazonProducts(params SearchAmazonParams) (interface{}, error) {
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return errorResponse("search_amazon_products", "Query cannot be empty", nil), nil
	}
	domain := params.DomainCode
	if domain == "" {
		domain = "com"
	}
	if !validDomain(domain) {
		return errorResponse("search_amazon_products", "Domain must be a valid string (e.g., 'com', 'co.uk')", nil), nil
	}
	res, err := o.query(obj{"source": "amazon_search", "query": query, "domain": domain, "parse": true})
	if err != nil {
		return errorResponse("search_amazon_products", "Amazon search failed: "+err.Error(), obj{"query": query}), nil
	}
	products := []obj{}
	for _, item := range restkit.Slice(restkit.Map(restkit.Map(res, "content"), "results"), "organic") {
		products = append(products, obj{
			"title":         trim(item, "title"),
			"asin":          trim(item, "asin"),
			"price":         orDefault(item, "price", 0),
			"currency":      restkit.Str(item, "currency"),
			"rating":        orDefault(item, "rating", 0),
			"reviews_count": orDefault(item, "reviews_count", 0),
			"url":           trim(item, "url"),
			"position":      orDefault(item, "pos", 0),
			"image":         trim(item, "image"),
		})
	}
	return restkit.JSONString(obj{"tool": "search_amazon_products", "query": query, "products": products}), nil
}

// ScrapeWebsite scrapes any URL, as Markdown or parsed content.
func (o *OxylabsTools) ScrapeWebsite(params ScrapeWebsiteParams) (interface{}, error) {
	if params.URL == "" {
		return errorResponse("scrape_website", "URL is required and must be a string", nil), nil
	}
	u := strings.TrimSpace(params.URL)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return errorResponse("scrape_website", fmt.Sprintf("Invalid URL format: %s. Must start with http:// or https://", u), nil), nil
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return errorResponse("scrape_website", fmt.Sprintf("Invalid URL format: %s", u), nil), nil
	}
	if parsed.Host == "" {
		return errorResponse("scrape_website", fmt.Sprintf("Invalid URL format: %s. Missing domain name", u), nil), nil
	}
	useMarkdown := o.markdown
	if v, ok := optBool(params.Markdown); ok {
		useMarkdown = v
	}
	payload := obj{"source": "universal", "url": u}
	if params.RenderJavascript {
		payload["render"] = "html"
	}
	// Oxylabs options are mutually exclusive: markdown=True OR parse=True.
	if useMarkdown {
		payload["markdown"] = true
	} else {
		payload["parse"] = true
	}
	res, err := o.query(payload)
	if err != nil {
		return errorResponse("scrape_website", "Website scraping failed: "+err.Error(), obj{"url": u}), nil
	}
	info := obj{"url": u, "javascript_rendered": params.RenderJavascript, "markdown": useMarkdown}
	if res != nil {
		var contentStr string
		switch c := res["content"].(type) {
		case nil:
		case string:
			contentStr = c
		default:
			contentStr = restkit.CompactJSON(c)
		}
		length := len([]rune(contentStr))
		scraped := ""
		if contentStr != "" {
			if useMarkdown {
				scraped = contentStr
			} else {
				scraped = strings.TrimSpace(restkit.Truncate(contentStr, 1000))
			}
			info["scraped"] = true
		}
		info["status_code"] = res["status_code"]
		info["content_length"] = length
		info["content"] = scraped
		info["has_content"] = length > 0
	}
	return restkit.JSONString(obj{"tool": "scrape_website", "url": u, "content_info": info}), nil
}

// Execute implements toolkit.Tool.
func (o *OxylabsTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return o.Toolkit.Execute(methodName, input)
}
