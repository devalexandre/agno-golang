// Package scavio ports Agno's ScavioTools: a unified search API over Google,
// Amazon, Walmart, YouTube and Reddit through the Scavio REST API
// (https://api.scavio.dev). The TikTok and Instagram tools are not ported.
package scavio

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Scavio API base URL.
const DefaultBaseURL = "https://api.scavio.dev"

// ScavioTools searches Google, Amazon, Walmart, YouTube and Reddit with Scavio.
type ScavioTools struct {
	toolkit.Toolkit
	apiKey        string
	baseURL       string
	httpClient    *http.Client
	client        *restkit.Client
	enableGoogle  bool
	enableAmazon  bool
	enableWalmart bool
	enableYouTube bool
	enableReddit  bool
	all           bool
}

// Option configures ScavioTools.
type Option func(*ScavioTools)

// WithAPIKey sets the API key (default: SCAVIO_API_KEY).
func WithAPIKey(key string) Option { return func(s *ScavioTools) { s.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(s *ScavioTools) { s.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(s *ScavioTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *ScavioTools) { s.httpClient = c } }

// WithEnableGoogle toggles GoogleSearch (default true).
func WithEnableGoogle(v bool) Option { return func(s *ScavioTools) { s.enableGoogle = v } }

// WithEnableAmazon toggles the Amazon tools (default true).
func WithEnableAmazon(v bool) Option { return func(s *ScavioTools) { s.enableAmazon = v } }

// WithEnableWalmart toggles the Walmart tools (default true).
func WithEnableWalmart(v bool) Option { return func(s *ScavioTools) { s.enableWalmart = v } }

// WithEnableYouTube toggles the YouTube tools (default true).
func WithEnableYouTube(v bool) Option { return func(s *ScavioTools) { s.enableYouTube = v } }

// WithEnableReddit toggles the Reddit tools (default true).
func WithEnableReddit(v bool) Option { return func(s *ScavioTools) { s.enableReddit = v } }

// WithAll enables every tool.
func WithAll() Option { return func(s *ScavioTools) { s.all = true } }

// GoogleSearchParams are the parameters of GoogleSearch.
type GoogleSearchParams struct {
	Query        string `json:"query" description:"The search query." required:"true"`
	GL           string `json:"gl,omitempty" description:"Two-letter country code to localize results (e.g. \"us\")."`
	HL           string `json:"hl,omitempty" description:"Two-letter UI language code (e.g. \"en\")."`
	Start        int    `json:"start,omitempty" description:"Result offset for pagination (0, 10, 20, ...)."`
	Device       string `json:"device,omitempty" description:"\"desktop\" or \"mobile\"."`
	NFPR         bool   `json:"nfpr,omitempty" description:"Disable auto-correction / spelling suggestions when True."`
	GoogleDomain string `json:"google_domain,omitempty" description:"Regional Google domain (e.g. \"google.co.uk\")."`
	Location     string `json:"location,omitempty" description:"Canonical location to search from (e.g. \"New York,New York,United States\")."`
	Safe         string `json:"safe,omitempty" description:"SafeSearch filter; \"active\" filters adult content."`
	TimePeriod   string `json:"time_period,omitempty" description:"Restrict to a recent window: \"last_hour\", \"last_day\", \"last_week\", \"last_month\", or \"last_year\"."`
}

// AmazonSearchParams are the parameters of AmazonSearch.
type AmazonSearchParams struct {
	Query             string `json:"query" description:"The product search query." required:"true"`
	Domain            string `json:"domain,omitempty" description:"Amazon domain (e.g. \"amazon.com\")."`
	Country           string `json:"country,omitempty" description:"Delivery country code."`
	Language          string `json:"language,omitempty" description:"Language code for results."`
	Currency          string `json:"currency,omitempty" description:"Currency code for prices."`
	Device            string `json:"device,omitempty" description:"\"desktop\" or \"mobile\"."`
	SortBy            string `json:"sort_by,omitempty" description:"Sort order for results."`
	StartPage         int    `json:"start_page,omitempty" description:"First page to fetch."`
	Pages             int    `json:"pages,omitempty" description:"Number of pages to fetch."`
	CategoryID        string `json:"category_id,omitempty" description:"Restrict to an Amazon category."`
	MerchantID        string `json:"merchant_id,omitempty" description:"Restrict to a merchant."`
	ZipCode           string `json:"zip_code,omitempty" description:"Delivery ZIP/postal code."`
	AutoselectVariant bool   `json:"autoselect_variant,omitempty" description:"Auto-select a product variant when True."`
}

// AmazonProductParams are the parameters of AmazonProduct.
type AmazonProductParams struct {
	ASIN              string `json:"asin" description:"The Amazon Standard Identification Number (ASIN)." required:"true"`
	Domain            string `json:"domain,omitempty" description:"Amazon domain (e.g. \"amazon.com\")."`
	Country           string `json:"country,omitempty" description:"Delivery country code."`
	Language          string `json:"language,omitempty" description:"Language code for results."`
	Currency          string `json:"currency,omitempty" description:"Currency code for prices."`
	Device            string `json:"device,omitempty" description:"\"desktop\" or \"mobile\"."`
	ZipCode           string `json:"zip_code,omitempty" description:"Delivery ZIP/postal code."`
	AutoselectVariant bool   `json:"autoselect_variant,omitempty" description:"Auto-select a product variant when True."`
}

// WalmartSearchParams are the parameters of WalmartSearch.
type WalmartSearchParams struct {
	Query            string `json:"query" description:"The product search query." required:"true"`
	Domain           string `json:"domain,omitempty" description:"Walmart domain."`
	Device           string `json:"device,omitempty" description:"\"desktop\" or \"mobile\"."`
	SortBy           string `json:"sort_by,omitempty" description:"Sort order for results."`
	StartPage        int    `json:"start_page,omitempty" description:"First page to fetch."`
	MinPrice         int    `json:"min_price,omitempty" description:"Minimum price filter."`
	MaxPrice         int    `json:"max_price,omitempty" description:"Maximum price filter."`
	FulfillmentSpeed string `json:"fulfillment_speed,omitempty" description:"Fulfillment speed filter."`
	FulfillmentType  string `json:"fulfillment_type,omitempty" description:"Fulfillment type filter."`
	DeliveryZip      string `json:"delivery_zip,omitempty" description:"Delivery ZIP/postal code."`
	StoreID          string `json:"store_id,omitempty" description:"Restrict to a store."`
}

// WalmartProductParams are the parameters of WalmartProduct.
type WalmartProductParams struct {
	ProductID   string `json:"product_id" description:"The Walmart product ID." required:"true"`
	Domain      string `json:"domain,omitempty" description:"Walmart domain."`
	Device      string `json:"device,omitempty" description:"\"desktop\" or \"mobile\"."`
	DeliveryZip string `json:"delivery_zip,omitempty" description:"Delivery ZIP/postal code."`
	StoreID     string `json:"store_id,omitempty" description:"Restrict to a store."`
}

// YouTubeSearchParams are the parameters of YoutubeSearch.
type YouTubeSearchParams struct {
	Query           string `json:"query" description:"The search query." required:"true"`
	UploadDate      string `json:"upload_date,omitempty" description:"Filter by upload date."`
	Type            string `json:"type,omitempty" description:"Result type filter (e.g. \"video\", \"channel\", \"playlist\")."`
	Duration        string `json:"duration,omitempty" description:"Video duration filter."`
	SortBy          string `json:"sort_by,omitempty" description:"Sort order for results."`
	HD              bool   `json:"hd,omitempty" description:"Restrict to HD videos when True."`
	Subtitles       bool   `json:"subtitles,omitempty" description:"Restrict to videos with subtitles when True."`
	CreativeCommons bool   `json:"creative_commons,omitempty" description:"Restrict to Creative Commons videos when True."`
	Live            bool   `json:"live,omitempty" description:"Restrict to live videos when True."`
}

// YouTubeMetadataParams are the parameters of YoutubeMetadata.
type YouTubeMetadataParams struct {
	VideoID string `json:"video_id" description:"The YouTube video ID." required:"true"`
}

// RedditSearchParams are the parameters of RedditSearch.
type RedditSearchParams struct {
	Query  string `json:"query" description:"The search query." required:"true"`
	Type   string `json:"type,omitempty" description:"What to search for (e.g. \"posts\", \"communities\")."`
	Sort   string `json:"sort,omitempty" description:"Sort order for results."`
	Cursor string `json:"cursor,omitempty" description:"Pagination cursor from a previous response."`
}

// RedditPostParams are the parameters of RedditPost.
type RedditPostParams struct {
	URL string `json:"url" description:"The full URL of the Reddit post." required:"true"`
}

// New creates the Scavio toolkit.
func New(opts ...Option) *ScavioTools {
	s := &ScavioTools{
		baseURL:       DefaultBaseURL,
		enableGoogle:  true,
		enableAmazon:  true,
		enableWalmart: true,
		enableYouTube: true,
		enableReddit:  true,
	}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "scavio"
	s.Toolkit.Description = "Unified search over Google, Amazon, Walmart, YouTube and Reddit with Scavio."
	for _, o := range opts {
		o(s)
	}
	s.apiKey = restkit.Env(s.apiKey, "SCAVIO_API_KEY")
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	s.client = &restkit.Client{
		HTTP:    s.httpClient,
		BaseURL: s.baseURL,
		Name:    "Scavio API",
		Headers: map[string]string{"Authorization": "Bearer " + s.apiKey, "X-Client-Source": "agno-golang"},
	}
	tk := &s.Toolkit
	if s.all || s.enableGoogle {
		tk.Register("GoogleSearch", "Search Google for real-time organic web results. Returns JSON with an organic_results list (each item has title and link, and usually a snippet). Costs 1 credit.", s, s.GoogleSearch, GoogleSearchParams{})
	}
	if s.all || s.enableAmazon {
		tk.Register("AmazonSearch", "Search Amazon for products matching a query.", s, s.AmazonSearch, AmazonSearchParams{})
		tk.Register("AmazonProduct", "Get full details for a single Amazon product by ASIN.", s, s.AmazonProduct, AmazonProductParams{})
	}
	if s.all || s.enableWalmart {
		tk.Register("WalmartSearch", "Search Walmart for products matching a query.", s, s.WalmartSearch, WalmartSearchParams{})
		tk.Register("WalmartProduct", "Get full details for a single Walmart product by product ID.", s, s.WalmartProduct, WalmartProductParams{})
	}
	if s.all || s.enableYouTube {
		tk.Register("YoutubeSearch", "Search YouTube for videos matching a query.", s, s.YoutubeSearch, YouTubeSearchParams{})
		tk.Register("YoutubeMetadata", "Get structured metadata for a single YouTube video.", s, s.YoutubeMetadata, YouTubeMetadataParams{})
	}
	if s.all || s.enableReddit {
		tk.Register("RedditSearch", "Search Reddit for posts or communities matching a query.", s, s.RedditSearch, RedditSearchParams{})
		tk.Register("RedditPost", "Fetch a Reddit post with its threaded comments by URL.", s, s.RedditPost, RedditPostParams{})
	}
	return s
}

// call POSTs the params struct (unset fields omitted) to path, renaming
// Python argument names to wire fields where Scavio differs.
func (s *ScavioTools) call(path string, params interface{}, rename map[string]string) (interface{}, error) {
	if s.apiKey == "" {
		return restkit.CompactJSON(map[string]string{"error": "SCAVIO_API_KEY not provided"}), nil
	}
	raw, _ := json.Marshal(params)
	var body map[string]interface{}
	_ = json.Unmarshal(raw, &body)
	for from, to := range rename {
		if v, ok := body[from]; ok {
			delete(body, from)
			body[to] = v
		}
	}
	res, err := s.client.JSON(http.MethodPost, path, nil, body)
	if err != nil {
		return restkit.CompactJSON(map[string]string{"error": err.Error()}), nil
	}
	return restkit.CompactJSON(res), nil
}

// GoogleSearch searches Google.
func (s *ScavioTools) GoogleSearch(p GoogleSearchParams) (interface{}, error) {
	return s.call("/api/v2/google", p, nil)
}

// AmazonSearch searches Amazon products.
func (s *ScavioTools) AmazonSearch(p AmazonSearchParams) (interface{}, error) {
	return s.call("/api/v1/amazon/search", p, nil)
}

// AmazonProduct fetches an Amazon product by ASIN.
func (s *ScavioTools) AmazonProduct(p AmazonProductParams) (interface{}, error) {
	return s.call("/api/v1/amazon/product", p, map[string]string{"asin": "query"})
}

// WalmartSearch searches Walmart products.
func (s *ScavioTools) WalmartSearch(p WalmartSearchParams) (interface{}, error) {
	return s.call("/api/v1/walmart/search", p, nil)
}

// WalmartProduct fetches a Walmart product.
func (s *ScavioTools) WalmartProduct(p WalmartProductParams) (interface{}, error) {
	return s.call("/api/v1/walmart/product", p, nil)
}

// YoutubeSearch searches YouTube videos.
func (s *ScavioTools) YoutubeSearch(p YouTubeSearchParams) (interface{}, error) {
	return s.call("/api/v1/youtube/search", p, map[string]string{"query": "search"})
}

// YoutubeMetadata fetches YouTube video metadata.
func (s *ScavioTools) YoutubeMetadata(p YouTubeMetadataParams) (interface{}, error) {
	return s.call("/api/v1/youtube/video", p, nil)
}

// RedditSearch searches Reddit.
func (s *ScavioTools) RedditSearch(p RedditSearchParams) (interface{}, error) {
	return s.call("/api/v1/reddit/search", p, nil)
}

// RedditPost fetches a Reddit post by URL.
func (s *ScavioTools) RedditPost(p RedditPostParams) (interface{}, error) {
	return s.call("/api/v1/reddit/post", p, nil)
}

// Execute implements toolkit.Tool.
func (s *ScavioTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}
