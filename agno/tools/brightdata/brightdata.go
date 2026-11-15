// Package brightdata ports Agno's BrightDataTools: markdown scraping,
// screenshots, search-engine results and structured web data feeds through the
// Bright Data API (https://api.brightdata.com).
package brightdata

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Bright Data API base URL.
const DefaultBaseURL = "https://api.brightdata.com"

// DefaultTimeout mirrors Python's timeout=600.
const DefaultTimeout = 600 * time.Second

// Datasets maps web_data_feed source types to Bright Data dataset IDs.
var Datasets = map[string]string{
	"amazon_product":                "gd_l7q7dkf244hwjntr0",
	"amazon_product_reviews":        "gd_le8e811kzy4ggddlq",
	"amazon_product_search":         "gd_lwdb4vjm1ehb499uxs",
	"walmart_product":               "gd_l95fol7l1ru6rlo116",
	"walmart_seller":                "gd_m7ke48w81ocyu4hhz0",
	"ebay_product":                  "gd_ltr9mjt81n0zzdk1fb",
	"homedepot_products":            "gd_lmusivh019i7g97q2n",
	"zara_products":                 "gd_lct4vafw1tgx27d4o0",
	"etsy_products":                 "gd_ltppk0jdv1jqz25mz",
	"bestbuy_products":              "gd_ltre1jqe1jfr7cccf",
	"linkedin_person_profile":       "gd_l1viktl72bvl7bjuj0",
	"linkedin_company_profile":      "gd_l1vikfnt1wgvvqz95w",
	"linkedin_job_listings":         "gd_lpfll7v5hcqtkxl6l",
	"linkedin_posts":                "gd_lyy3tktm25m4avu764",
	"linkedin_people_search":        "gd_m8d03he47z8nwb5xc",
	"crunchbase_company":            "gd_l1vijqt9jfj7olije",
	"zoominfo_company_profile":      "gd_m0ci4a4ivx3j5l6nx",
	"instagram_profiles":            "gd_l1vikfch901nx3by4",
	"instagram_posts":               "gd_lk5ns7kz21pck8jpis",
	"instagram_reels":               "gd_lyclm20il4r5helnj",
	"instagram_comments":            "gd_ltppn085pokosxh13",
	"facebook_posts":                "gd_lyclm1571iy3mv57zw",
	"facebook_marketplace_listings": "gd_lvt9iwuh6fbcwmx1a",
	"facebook_company_reviews":      "gd_m0dtqpiu1mbcyc2g86",
	"facebook_events":               "gd_m14sd0to1jz48ppm51",
	"tiktok_profiles":               "gd_l1villgoiiidt09ci",
	"tiktok_posts":                  "gd_lu702nij2f790tmv9h",
	"tiktok_shop":                   "gd_m45m1u911dsa4274pi",
	"tiktok_comments":               "gd_lkf2st302ap89utw5k",
	"google_maps_reviews":           "gd_luzfs1dn2oa0teb81",
	"google_shopping":               "gd_ltppk50q18kdw67omz",
	"google_play_store":             "gd_lsk382l8xei8vzm4u",
	"apple_app_store":               "gd_lsk9ki3u2iishmwrui",
	"reuter_news":                   "gd_lyptx9h74wtlvpnfu",
	"github_repository_file":        "gd_lyrexgxc24b3d4imjt",
	"yahoo_finance_business":        "gd_lmrpz3vxmz972ghd7",
	"x_posts":                       "gd_lwxkxvnf1cynvib9co",
	"zillow_properties_listing":     "gd_lfqkr8wm13ixtbd8f5",
	"booking_hotel_listings":        "gd_m5mbdl081229ln6t4a",
	"youtube_profiles":              "gd_lk538t2k2p1k3oos71",
	"youtube_comments":              "gd_lk9q0ew71spt1mxywf",
	"reddit_posts":                  "gd_lvz8ah06191smkebj4",
	"youtube_videos":                "gd_m5mbdl081229ln6t4a",
}

// datasetOrder keeps Python's ordering for the "valid options" message.
var datasetOrder = []string{
	"amazon_product", "amazon_product_reviews", "amazon_product_search", "walmart_product", "walmart_seller",
	"ebay_product", "homedepot_products", "zara_products", "etsy_products", "bestbuy_products",
	"linkedin_person_profile", "linkedin_company_profile", "linkedin_job_listings", "linkedin_posts",
	"linkedin_people_search", "crunchbase_company", "zoominfo_company_profile", "instagram_profiles",
	"instagram_posts", "instagram_reels", "instagram_comments", "facebook_posts", "facebook_marketplace_listings",
	"facebook_company_reviews", "facebook_events", "tiktok_profiles", "tiktok_posts", "tiktok_shop",
	"tiktok_comments", "google_maps_reviews", "google_shopping", "google_play_store", "apple_app_store",
	"reuter_news", "github_repository_file", "yahoo_finance_business", "x_posts", "zillow_properties_listing",
	"booking_hotel_listings", "youtube_profiles", "youtube_comments", "reddit_posts", "youtube_videos",
}

// BrightDataTools scrapes, screenshots and searches through Bright Data.
type BrightDataTools struct {
	toolkit.Toolkit
	apiKey             string
	baseURL            string
	httpClient         *http.Client
	client             *restkit.Client
	enableScrape       bool
	enableScreenshot   bool
	enableSearchEngine bool
	enableWebDataFeed  bool
	all                bool
	serpZone           string
	webUnlockerZone    string
	screenshotDir      string
	pollInterval       time.Duration
}

// Option configures BrightDataTools.
type Option func(*BrightDataTools)

// WithAPIKey sets the API key (default: BRIGHT_DATA_API_KEY).
func WithAPIKey(key string) Option { return func(b *BrightDataTools) { b.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(b *BrightDataTools) { b.baseURL = u } }

// WithTimeout sets the operation timeout (default 600s). It bounds each
// request and the total web_data_feed polling budget.
func WithTimeout(d time.Duration) Option { return func(b *BrightDataTools) { b.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(b *BrightDataTools) { b.httpClient = c } }

// WithEnableScrapeMarkdown toggles ScrapeAsMarkdown (default true).
func WithEnableScrapeMarkdown(v bool) Option { return func(b *BrightDataTools) { b.enableScrape = v } }

// WithEnableScreenshot toggles GetScreenshot (default true).
func WithEnableScreenshot(v bool) Option { return func(b *BrightDataTools) { b.enableScreenshot = v } }

// WithEnableSearchEngine toggles SearchEngine (default true).
func WithEnableSearchEngine(v bool) Option {
	return func(b *BrightDataTools) { b.enableSearchEngine = v }
}

// WithEnableWebDataFeed toggles WebDataFeed (default true).
func WithEnableWebDataFeed(v bool) Option { return func(b *BrightDataTools) { b.enableWebDataFeed = v } }

// WithAll enables every tool.
func WithAll() Option { return func(b *BrightDataTools) { b.all = true } }

// WithSerpZone sets the SERP zone (default BRIGHT_DATA_SERP_ZONE or "serp_api").
func WithSerpZone(z string) Option { return func(b *BrightDataTools) { b.serpZone = z } }

// WithWebUnlockerZone sets the Web Unlocker zone (default
// BRIGHT_DATA_WEB_UNLOCKER_ZONE or "web_unlocker1").
func WithWebUnlockerZone(z string) Option { return func(b *BrightDataTools) { b.webUnlockerZone = z } }

// WithScreenshotDir sets where screenshots are saved (default: os.TempDir()).
// Go agents have no image-artifact channel, so screenshots are written to disk.
func WithScreenshotDir(dir string) Option { return func(b *BrightDataTools) { b.screenshotDir = dir } }

// WithPollInterval sets the web_data_feed snapshot polling interval (default 1s).
func WithPollInterval(d time.Duration) Option { return func(b *BrightDataTools) { b.pollInterval = d } }

// URLParams carry a single URL.
type URLParams struct {
	URL string `json:"url" description:"URL to process" required:"true"`
}

// SearchEngineParams are the parameters of SearchEngine.
type SearchEngineParams struct {
	Query       string `json:"query" description:"Search query" required:"true"`
	Engine      string `json:"engine,omitempty" description:"Search engine - 'google', 'bing', or 'yandex'. Default google."`
	NumResults  int    `json:"num_results,omitempty" description:"Number of results to return. Default 10."`
	Language    string `json:"language,omitempty" description:"Two-letter language code"`
	CountryCode string `json:"country_code,omitempty" description:"Two-letter country code"`
}

// WebDataFeedParams are the parameters of WebDataFeed.
type WebDataFeedParams struct {
	SourceType   string `json:"source_type" description:"Type of data source (e.g., 'linkedin_person_profile', 'amazon_product')" required:"true"`
	URL          string `json:"url" description:"URL of the web resource to retrieve data from" required:"true"`
	NumOfReviews int    `json:"num_of_reviews,omitempty" description:"Number of reviews to retrieve"`
}

// New creates the Bright Data toolkit.
func New(opts ...Option) *BrightDataTools {
	b := &BrightDataTools{
		baseURL:            DefaultBaseURL,
		enableScrape:       true,
		enableScreenshot:   true,
		enableSearchEngine: true,
		enableWebDataFeed:  true,
		pollInterval:       time.Second,
	}
	b.Toolkit = toolkit.NewToolkit()
	b.Toolkit.Name = "brightdata_tools"
	b.Toolkit.Description = "Scrape pages, take screenshots, search engines and fetch web data feeds with Bright Data."
	b.Toolkit.Timeout = DefaultTimeout
	for _, o := range opts {
		o(b)
	}
	b.apiKey = restkit.Env(b.apiKey, "BRIGHT_DATA_API_KEY")
	// Python: getenv("BRIGHT_DATA_*_ZONE", constructor value).
	b.webUnlockerZone = zone("BRIGHT_DATA_WEB_UNLOCKER_ZONE", b.webUnlockerZone, "web_unlocker1")
	b.serpZone = zone("BRIGHT_DATA_SERP_ZONE", b.serpZone, "serp_api")
	if b.httpClient == nil {
		b.httpClient = b.Toolkit.HTTPClient()
	}
	b.client = &restkit.Client{
		HTTP:    b.httpClient,
		BaseURL: b.baseURL,
		Name:    "Bright Data API",
		Headers: map[string]string{"Authorization": "Bearer " + b.apiKey},
	}
	tk := &b.Toolkit
	if b.all || b.enableScrape {
		tk.Register("ScrapeAsMarkdown", "Scrape a webpage and return content in Markdown format.", b, b.ScrapeAsMarkdown, URLParams{})
	}
	if b.all || b.enableScreenshot {
		tk.Register("GetScreenshot", "Capture a screenshot of a webpage.", b, b.GetScreenshot, URLParams{})
	}
	if b.all || b.enableSearchEngine {
		tk.Register("SearchEngine", "Search using Google, Bing, or Yandex and return results in Markdown.", b, b.SearchEngine, SearchEngineParams{})
	}
	if b.all || b.enableWebDataFeed {
		tk.Register("WebDataFeed", "Retrieve structured web data from various sources like LinkedIn, Amazon, Instagram, etc.", b, b.WebDataFeed, WebDataFeedParams{})
	}
	return b
}

func (b *BrightDataTools) request(payload map[string]interface{}) ([]byte, error) {
	status, data, err := b.client.Raw(http.MethodPost, "/request", nil, payload)
	if err != nil {
		return nil, fmt.Errorf("Request failed: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("Request failed: Failed to scrape: %d - %s", status, string(data))
	}
	return data, nil
}

// ScrapeAsMarkdown scrapes a page as markdown via the Web Unlocker zone.
func (b *BrightDataTools) ScrapeAsMarkdown(params URLParams) (interface{}, error) {
	if b.apiKey == "" {
		return "Please provide a Bright Data API key", nil
	}
	if params.URL == "" {
		return "Please provide a URL to scrape", nil
	}
	data, err := b.request(map[string]interface{}{
		"url": params.URL, "zone": b.webUnlockerZone, "format": "raw", "data_format": "markdown",
	})
	if err != nil {
		return fmt.Sprintf("Error scraping URL %s: %v", params.URL, err), nil
	}
	return string(data), nil
}

// GetScreenshot captures a PNG screenshot and saves it to the screenshot dir.
func (b *BrightDataTools) GetScreenshot(params URLParams) (interface{}, error) {
	if b.apiKey == "" {
		return "Please provide a Bright Data API key", nil
	}
	if params.URL == "" {
		return "Please provide a URL to screenshot", nil
	}
	status, data, err := b.client.Raw(http.MethodPost, "/request", nil, map[string]interface{}{
		"url": params.URL, "zone": b.webUnlockerZone, "format": "raw", "data_format": "screenshot",
	})
	if err == nil && status != http.StatusOK {
		err = fmt.Errorf("Error %d: %s", status, string(data))
	}
	if err != nil {
		return fmt.Sprintf("Error taking screenshot of %s: %v", params.URL, err), nil
	}
	mediaID := newUUID()
	dir := b.screenshotDir
	if dir == "" {
		dir = os.TempDir()
	}
	path := filepath.Join(dir, "brightdata_screenshot_"+mediaID+".png")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Sprintf("Error taking screenshot of %s: %v", params.URL, err), nil
	}
	return fmt.Sprintf("Screenshot captured and added as artifact with ID: %s (saved to %s)", mediaID, path), nil
}

// SearchEngine searches Google, Bing or Yandex through the SERP zone.
func (b *BrightDataTools) SearchEngine(params SearchEngineParams) (interface{}, error) {
	if b.apiKey == "" {
		return "Please provide a Bright Data API key", nil
	}
	if params.Query == "" {
		return "Please provide a query to search for", nil
	}
	engine := params.Engine
	if engine == "" {
		engine = "google"
	}
	num := params.NumResults
	if num == 0 {
		num = 10
	}
	enc := strings.ReplaceAll(url.QueryEscape(params.Query), "+", "%20")
	var searchURL string
	switch engine {
	case "google":
		searchURL = "https://www.google.com/search?q=" + enc
		if params.Language != "" {
			searchURL += "&hl=" + params.Language
		}
		if params.CountryCode != "" {
			searchURL += "&gl=" + params.CountryCode
		}
		if num > 0 {
			searchURL += fmt.Sprintf("&num=%d", num)
		}
	case "bing":
		searchURL = "https://www.bing.com/search?q=" + enc
	case "yandex":
		searchURL = "https://yandex.com/search/?text=" + enc
	default:
		return fmt.Sprintf("Unsupported search engine: %s. Use 'google', 'bing', or 'yandex'", engine), nil
	}
	data, err := b.request(map[string]interface{}{
		"url": searchURL, "zone": b.serpZone, "format": "raw", "data_format": "markdown",
	})
	if err != nil {
		return fmt.Sprintf("Error searching for query %s: %v", params.Query, err), nil
	}
	return string(data), nil
}

// WebDataFeed triggers a dataset collection and polls for the snapshot.
func (b *BrightDataTools) WebDataFeed(params WebDataFeedParams) (interface{}, error) {
	if b.apiKey == "" {
		return "Please provide a Bright Data API key", nil
	}
	if params.URL == "" {
		return "Please provide a URL to retrieve data from", nil
	}
	datasetID, ok := Datasets[params.SourceType]
	if !ok {
		valid := ""
		for i, k := range datasetOrder {
			if i > 0 {
				valid += ", "
			}
			valid += k
		}
		return fmt.Sprintf("Invalid source_type: %s. Valid options are: %s", params.SourceType, valid), nil
	}
	req := map[string]interface{}{"url": params.URL}
	if params.SourceType == "facebook_company_reviews" && params.NumOfReviews > 0 {
		req["num_of_reviews"] = fmt.Sprint(params.NumOfReviews)
	}
	_, data, err := b.client.Raw(http.MethodPost, "/datasets/v3/trigger",
		url.Values{"dataset_id": {datasetID}, "include_errors": {"true"}}, []interface{}{req})
	if err != nil && len(data) == 0 {
		return fmt.Sprintf("Error retrieving %s data from %s: %v", params.SourceType, params.URL, err), nil
	}
	var trig map[string]interface{}
	_ = json.Unmarshal(data, &trig)
	snapshotID := restkit.Str(trig, "snapshot_id")
	if snapshotID == "" {
		return "No snapshot ID returned from trigger request", nil
	}

	timeout := b.Toolkit.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	maxAttempts := int(timeout.Seconds())
	pollClient := *b.client
	pc := *b.httpClient
	pc.Timeout = 30 * time.Second // per-poll cap, as in Python
	pollClient.HTTP = &pc
	for attempts := 0; attempts < maxAttempts; attempts++ {
		_, body, err := pollClient.Raw(http.MethodGet, "/datasets/v3/snapshot/"+url.PathEscape(snapshotID), url.Values{"format": {"json"}}, nil)
		var snap interface{}
		if len(body) > 0 && json.Unmarshal(body, &snap) == nil {
			if m, ok := snap.(map[string]interface{}); ok && restkit.Str(m, "status") == "running" {
				time.Sleep(b.pollInterval)
				continue
			}
			return restkit.CompactJSON(snap), nil
		}
		// Transport error or non-JSON body: retry like Python's bare except.
		_ = err
		time.Sleep(b.pollInterval)
	}
	return fmt.Sprintf("Timeout after %d seconds waiting for %s data", maxAttempts, params.SourceType), nil
}

// zone resolves a zone: the env var wins (as in Python), then the option, then the default.
func zone(env, opt, def string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	if opt != "" {
		return opt
	}
	return def
}

func newUUID() string {
	var u [16]byte
	_, _ = rand.Read(u[:])
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
}

// Execute implements toolkit.Tool.
func (b *BrightDataTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return b.Toolkit.Execute(methodName, input)
}
