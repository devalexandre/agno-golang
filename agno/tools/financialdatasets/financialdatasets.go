// Package financialdatasets ports Agno's FinancialDatasetsTools: financial
// statements, prices, filings and news from api.financialdatasets.ai.
package financialdatasets

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Financial Datasets API base URL.
const DefaultBaseURL = "https://api.financialdatasets.ai"

// FinancialDatasetsTools wraps the Financial Datasets REST API.
type FinancialDatasetsTools struct {
	toolkit.Toolkit
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Option configures FinancialDatasetsTools.
type Option func(*FinancialDatasetsTools)

// WithAPIKey sets the API key (default FINANCIAL_DATASETS_API_KEY).
func WithAPIKey(k string) Option { return func(f *FinancialDatasetsTools) { f.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(f *FinancialDatasetsTools) { f.baseURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(f *FinancialDatasetsTools) { f.httpClient = c }
}

// WithTimeout sets the per-request timeout (default 30s).
func WithTimeout(d time.Duration) Option {
	return func(f *FinancialDatasetsTools) { f.Toolkit.Timeout = d }
}

// StatementParams is used by statement endpoints.
type StatementParams struct {
	Ticker string `json:"ticker" description:"Stock ticker symbol" required:"true"`
	Period string `json:"period,omitempty" description:"'annual', 'quarterly', or 'ttm' (default annual)"`
	Limit  int    `json:"limit,omitempty" description:"Number of statements to return (default 10)"`
}

// TickerParams has only a ticker.
type TickerParams struct {
	Ticker string `json:"ticker" description:"Stock ticker symbol" required:"true"`
}

// PriceParams for stock prices.
type PriceParams struct {
	Ticker   string `json:"ticker" description:"Stock ticker symbol" required:"true"`
	Interval string `json:"interval,omitempty" description:"Price interval (e.g., '1d', '1h'). Default 1d"`
	Limit    int    `json:"limit,omitempty" description:"Number of price points to return (default 100)"`
}

// CryptoParams for crypto prices.
type CryptoParams struct {
	Symbol   string `json:"symbol" description:"Cryptocurrency symbol (e.g., 'BTC')" required:"true"`
	Interval string `json:"interval,omitempty" description:"Price interval (e.g., '1d', '1h'). Default 1d"`
	Limit    int    `json:"limit,omitempty" description:"Number of price points to return (default 100)"`
}

// TickerLimitParams is a ticker plus limit.
type TickerLimitParams struct {
	Ticker string `json:"ticker" description:"Stock ticker symbol" required:"true"`
	Limit  int    `json:"limit,omitempty" description:"Number of items to return"`
}

// NewsParams for market news.
type NewsParams struct {
	Ticker string `json:"ticker,omitempty" description:"Stock ticker symbol (optional)"`
	Limit  int    `json:"limit,omitempty" description:"Number of news items to return (default 50)"`
}

// SearchParams for ticker search.
type SearchParams struct {
	Query string `json:"query" description:"Search query" required:"true"`
	Limit int    `json:"limit,omitempty" description:"Maximum number of results to return (default 10)"`
}

// SECFilingsParams for SEC filings.
type SECFilingsParams struct {
	Ticker   string `json:"ticker" description:"Stock ticker symbol" required:"true"`
	FormType string `json:"form_type,omitempty" description:"Type of SEC form (e.g., '10-K', '10-Q')"`
	Limit    int    `json:"limit,omitempty" description:"Number of filings to return (default 50)"`
}

// NewFinancialDatasetsTools creates the toolkit.
func NewFinancialDatasetsTools(opts ...Option) *FinancialDatasetsTools {
	f := &FinancialDatasetsTools{apiKey: os.Getenv("FINANCIAL_DATASETS_API_KEY"), baseURL: DefaultBaseURL}
	f.Toolkit = toolkit.NewToolkit()
	f.Toolkit.Name = "financial_datasets_tools"
	f.Toolkit.Description = "Financial statements, prices, filings, ownership and news from Financial Datasets."
	for _, o := range opts {
		o(f)
	}
	if f.httpClient == nil {
		f.httpClient = f.Toolkit.HTTPClient()
	}
	r := f.Toolkit.Register
	r("GetIncomeStatements", "Get income statements for a ticker.", f, f.GetIncomeStatements, StatementParams{})
	r("GetBalanceSheets", "Get balance sheets for a ticker.", f, f.GetBalanceSheets, StatementParams{})
	r("GetCashFlowStatements", "Get cash flow statements for a ticker.", f, f.GetCashFlowStatements, StatementParams{})
	r("GetSegmentedFinancials", "Get segmented financials for a ticker.", f, f.GetSegmentedFinancials, StatementParams{})
	r("GetFinancialMetrics", "Get financial metrics for a ticker.", f, f.GetFinancialMetrics, TickerParams{})
	r("GetCompanyInfo", "Get company information for a ticker.", f, f.GetCompanyInfo, TickerParams{})
	r("GetStockPrices", "Get stock prices for a ticker.", f, f.GetStockPrices, PriceParams{})
	r("GetEarnings", "Get earnings data for a ticker.", f, f.GetEarnings, TickerLimitParams{})
	r("GetInsiderTrades", "Get insider trades for a ticker.", f, f.GetInsiderTrades, TickerLimitParams{})
	r("GetInstitutionalOwnership", "Get institutional ownership data for a ticker.", f, f.GetInstitutionalOwnership, TickerParams{})
	r("GetNews", "Get market news, optionally filtered by ticker.", f, f.GetNews, NewsParams{})
	r("GetSecFilings", "Get SEC filings for a ticker.", f, f.GetSecFilings, SECFilingsParams{})
	r("GetCryptoPrices", "Get cryptocurrency prices.", f, f.GetCryptoPrices, CryptoParams{})
	r("SearchTickers", "Search for tickers based on a query.", f, f.SearchTickers, SearchParams{})
	return f
}

func (f *FinancialDatasetsTools) request(endpoint string, q url.Values) (interface{}, error) {
	if f.apiKey == "" {
		return "API key not set", nil
	}
	c := restapi.New(f.baseURL, f.httpClient, map[string]string{"X-API-KEY": f.apiKey})
	data, err := c.Do(context.Background(), http.MethodGet, endpoint, q, nil)
	if err != nil {
		return fmt.Sprintf("Error making request to %s: %v", c.URL(endpoint, nil), err), nil
	}
	return string(data), nil
}

func def(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func defInt(n, d int) string {
	if n <= 0 {
		n = d
	}
	return strconv.Itoa(n)
}

func (f *FinancialDatasetsTools) statement(endpoint string, p StatementParams) (interface{}, error) {
	return f.request(endpoint, url.Values{"ticker": {p.Ticker}, "period": {def(p.Period, "annual")}, "limit": {defInt(p.Limit, 10)}})
}

// GetIncomeStatements returns income statements.
func (f *FinancialDatasetsTools) GetIncomeStatements(p StatementParams) (interface{}, error) {
	return f.statement("financials/income-statements", p)
}

// GetBalanceSheets returns balance sheets.
func (f *FinancialDatasetsTools) GetBalanceSheets(p StatementParams) (interface{}, error) {
	return f.statement("financials/balance-sheets", p)
}

// GetCashFlowStatements returns cash flow statements.
func (f *FinancialDatasetsTools) GetCashFlowStatements(p StatementParams) (interface{}, error) {
	return f.statement("financials/cash-flow-statements", p)
}

// GetSegmentedFinancials returns segmented financials.
func (f *FinancialDatasetsTools) GetSegmentedFinancials(p StatementParams) (interface{}, error) {
	return f.statement("financials/segmented", p)
}

// GetFinancialMetrics returns financial metrics.
func (f *FinancialDatasetsTools) GetFinancialMetrics(p TickerParams) (interface{}, error) {
	return f.request("financials/metrics", url.Values{"ticker": {p.Ticker}})
}

// GetCompanyInfo returns company information.
func (f *FinancialDatasetsTools) GetCompanyInfo(p TickerParams) (interface{}, error) {
	return f.request("company", url.Values{"ticker": {p.Ticker}})
}

// GetStockPrices returns stock prices.
func (f *FinancialDatasetsTools) GetStockPrices(p PriceParams) (interface{}, error) {
	return f.request("prices", url.Values{"ticker": {p.Ticker}, "interval": {def(p.Interval, "1d")}, "limit": {defInt(p.Limit, 100)}})
}

// GetCryptoPrices returns crypto prices.
func (f *FinancialDatasetsTools) GetCryptoPrices(p CryptoParams) (interface{}, error) {
	return f.request("crypto/prices", url.Values{"symbol": {p.Symbol}, "interval": {def(p.Interval, "1d")}, "limit": {defInt(p.Limit, 100)}})
}

// GetEarnings returns earnings data (default limit 10).
func (f *FinancialDatasetsTools) GetEarnings(p TickerLimitParams) (interface{}, error) {
	return f.request("earnings", url.Values{"ticker": {p.Ticker}, "limit": {defInt(p.Limit, 10)}})
}

// GetInsiderTrades returns insider trades (default limit 50).
func (f *FinancialDatasetsTools) GetInsiderTrades(p TickerLimitParams) (interface{}, error) {
	return f.request("insider-trades", url.Values{"ticker": {p.Ticker}, "limit": {defInt(p.Limit, 50)}})
}

// GetInstitutionalOwnership returns institutional ownership.
func (f *FinancialDatasetsTools) GetInstitutionalOwnership(p TickerParams) (interface{}, error) {
	return f.request("institutional-ownership", url.Values{"ticker": {p.Ticker}})
}

// GetNews returns market news.
func (f *FinancialDatasetsTools) GetNews(p NewsParams) (interface{}, error) {
	q := url.Values{"limit": {defInt(p.Limit, 50)}}
	if p.Ticker != "" {
		q.Set("ticker", p.Ticker)
	}
	return f.request("news", q)
}

// GetSecFilings returns SEC filings.
func (f *FinancialDatasetsTools) GetSecFilings(p SECFilingsParams) (interface{}, error) {
	q := url.Values{"ticker": {p.Ticker}, "limit": {defInt(p.Limit, 50)}}
	if p.FormType != "" {
		q.Set("form_type", p.FormType)
	}
	return f.request("sec-filings", q)
}

// SearchTickers searches tickers.
func (f *FinancialDatasetsTools) SearchTickers(p SearchParams) (interface{}, error) {
	return f.request("search", url.Values{"query": {p.Query}, "limit": {defInt(p.Limit, 10)}})
}
