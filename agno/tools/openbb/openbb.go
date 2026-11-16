// Package openbb ports Agno's OpenBBTools. Python calls the in-process `obb`
// SDK; Go talks to the OpenBB Platform REST API (`openbb-api`, default
// http://127.0.0.1:6900/api/v1), whose routes mirror the SDK
// (obb.equity.price.quote -> GET /equity/price/quote).
package openbb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the default OpenBB Platform API base URL.
const DefaultBaseURL = "http://127.0.0.1:6900/api/v1"

// OpenBBTools provides stock quotes, symbol search, news, profiles and price targets.
type OpenBBTools struct {
	toolkit.Toolkit
	baseURL    string
	pat        string
	provider   string
	httpClient *http.Client

	enablePrice, enableSearch, enableNews, enableProfile, enableTargets bool
}

// Option configures OpenBBTools.
type Option func(*OpenBBTools)

// WithBaseURL sets the API base URL (default OPENBB_API_URL, else DefaultBaseURL).
func WithBaseURL(u string) Option { return func(o *OpenBBTools) { o.baseURL = u } }

// WithPAT sets a bearer token sent to the API (default OPENBB_PAT).
func WithPAT(p string) Option { return func(o *OpenBBTools) { o.pat = p } }

// WithProvider sets the data provider (default yfinance).
func WithProvider(p string) Option { return func(o *OpenBBTools) { o.provider = p } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(o *OpenBBTools) { o.httpClient = c } }

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option { return func(o *OpenBBTools) { o.Toolkit.Timeout = d } }

// WithStockPrice toggles GetStockPrice (default true).
func WithStockPrice(b bool) Option { return func(o *OpenBBTools) { o.enablePrice = b } }

// WithSearchCompanySymbol toggles SearchCompanySymbol (default false).
func WithSearchCompanySymbol(b bool) Option { return func(o *OpenBBTools) { o.enableSearch = b } }

// WithCompanyNews toggles GetCompanyNews (default false).
func WithCompanyNews(b bool) Option { return func(o *OpenBBTools) { o.enableNews = b } }

// WithCompanyProfile toggles GetCompanyProfile (default false).
func WithCompanyProfile(b bool) Option { return func(o *OpenBBTools) { o.enableProfile = b } }

// WithPriceTargets toggles GetPriceTargets (default false).
func WithPriceTargets(b bool) Option { return func(o *OpenBBTools) { o.enableTargets = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(o *OpenBBTools) {
		o.enablePrice, o.enableSearch, o.enableNews, o.enableProfile, o.enableTargets = true, true, true, true, true
	}
}

// SymbolParams has a symbol.
type SymbolParams struct {
	Symbol string `json:"symbol" description:"The stock symbol or list of stock symbols. Eg: \"AAPL\" or \"AAPL,MSFT,GOOGL\"" required:"true"`
}

// CompanyParams for SearchCompanySymbol.
type CompanyParams struct {
	CompanyName string `json:"company_name" description:"The name of the company." required:"true"`
}

// NewsParams for GetCompanyNews.
type NewsParams struct {
	Symbol     string `json:"symbol" description:"The stock symbol or list of stock symbols. Eg: \"AAPL\" or \"AAPL,MSFT,GOOGL\"" required:"true"`
	NumStories int    `json:"num_stories,omitempty" description:"The number of news stories to return. Defaults to 10."`
}

// NewOpenBBTools creates the toolkit.
func NewOpenBBTools(opts ...Option) *OpenBBTools {
	o := &OpenBBTools{baseURL: os.Getenv("OPENBB_API_URL"), pat: os.Getenv("OPENBB_PAT"), provider: "yfinance", enablePrice: true}
	if o.baseURL == "" {
		o.baseURL = DefaultBaseURL
	}
	o.Toolkit = toolkit.NewToolkit()
	o.Toolkit.Name = "openbb_tools"
	o.Toolkit.Description = "Financial market data through the OpenBB Platform API."
	for _, opt := range opts {
		opt(o)
	}
	if o.httpClient == nil {
		o.httpClient = o.Toolkit.HTTPClient()
	}
	if o.enablePrice {
		o.Toolkit.Register("GetStockPrice", "Use this function to get the current stock price for a stock symbol or list of symbols.", o, o.GetStockPrice, SymbolParams{})
	}
	if o.enableSearch {
		o.Toolkit.Register("SearchCompanySymbol", "Use this function to get a list of ticker symbols for a company.", o, o.SearchCompanySymbol, CompanyParams{})
	}
	if o.enableNews {
		o.Toolkit.Register("GetCompanyNews", "Use this function to get company news for a stock symbol or list of symbols.", o, o.GetCompanyNews, NewsParams{})
	}
	if o.enableProfile {
		o.Toolkit.Register("GetCompanyProfile", "Use this function to get company profile and overview for a stock symbol or list of symbols.", o, o.GetCompanyProfile, SymbolParams{})
	}
	if o.enableTargets {
		o.Toolkit.Register("GetPriceTargets", "Use this function to get consensus price target and recommendations for a stock symbol or list of symbols.", o, o.GetPriceTargets, SymbolParams{})
	}
	return o
}

func (o *OpenBBTools) results(path string, q url.Values) ([]map[string]interface{}, error) {
	headers := map[string]string{}
	if o.pat != "" {
		headers["Authorization"] = "Bearer " + o.pat
	}
	var resp struct {
		Results json.RawMessage `json:"results"`
	}
	if err := restapi.New(o.baseURL, o.httpClient, headers).JSON(context.Background(), http.MethodGet, path, q, nil, &resp); err != nil {
		return nil, err
	}
	var rows []map[string]interface{}
	if len(resp.Results) > 0 && resp.Results[0] == '{' {
		var one map[string]interface{}
		if err := json.Unmarshal(resp.Results, &one); err != nil {
			return nil, err
		}
		return []map[string]interface{}{one}, nil
	}
	if len(resp.Results) > 0 {
		if err := json.Unmarshal(resp.Results, &rows); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// GetStockPrice returns quote data for the symbol(s).
func (o *OpenBBTools) GetStockPrice(p SymbolParams) (interface{}, error) {
	rows, err := o.results("/equity/price/quote", url.Values{"symbol": {p.Symbol}, "provider": {o.provider}})
	if err != nil {
		return fmt.Sprintf("Error fetching current price for %s: %v", p.Symbol, err), nil
	}
	keys := []string{"symbol", "last_price", "currency", "name", "high", "low", "open", "close", "prev_close", "volume", "ma_50d", "ma_200d"}
	clean := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		c := map[string]interface{}{}
		for _, k := range keys {
			c[k] = r[k]
		}
		clean = append(clean, c)
	}
	return restapi.ToJSON(clean), nil
}

// SearchCompanySymbol finds ticker symbols for a company name.
func (o *OpenBBTools) SearchCompanySymbol(p CompanyParams) (interface{}, error) {
	rows, err := o.results("/equity/search", url.Values{"query": {p.CompanyName}})
	if err != nil {
		return fmt.Sprintf("Error searching ticker for %s: %v", p.CompanyName, err), nil
	}
	clean := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		clean = append(clean, map[string]interface{}{"symbol": r["symbol"], "name": r["name"]})
	}
	return restapi.ToJSON(clean), nil
}

// GetPriceTargets returns consensus price targets.
func (o *OpenBBTools) GetPriceTargets(p SymbolParams) (interface{}, error) {
	rows, err := o.results("/equity/estimates/consensus", url.Values{"symbol": {p.Symbol}, "provider": {o.provider}})
	if err != nil {
		return fmt.Sprintf("Error fetching company news for %s: %v", p.Symbol, err), nil
	}
	return restapi.ToJSON(rows), nil
}

// GetCompanyNews returns company news (images stripped).
func (o *OpenBBTools) GetCompanyNews(p NewsParams) (interface{}, error) {
	n := p.NumStories
	if n <= 0 {
		n = 10
	}
	rows, err := o.results("/news/company", url.Values{"symbol": {p.Symbol}, "provider": {o.provider}, "limit": {strconv.Itoa(n)}})
	if err != nil {
		return fmt.Sprintf("Error fetching company news for %s: %v", p.Symbol, err), nil
	}
	for _, r := range rows {
		delete(r, "images")
	}
	if len(rows) > n {
		rows = rows[:n]
	}
	return restapi.ToJSON(rows), nil
}

// GetCompanyProfile returns the company profile.
func (o *OpenBBTools) GetCompanyProfile(p SymbolParams) (interface{}, error) {
	rows, err := o.results("/equity/profile", url.Values{"symbol": {p.Symbol}, "provider": {o.provider}})
	if err != nil {
		return fmt.Sprintf("Error fetching company profile for %s: %v", p.Symbol, err), nil
	}
	return restapi.ToJSON(rows), nil
}
