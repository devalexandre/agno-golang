package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// YFinanceTool provides access to stock market data via Yahoo Finance
type YFinanceTool struct {
	toolkit.Toolkit
}

// NewYFinanceTool creates a new YFinance tool
func NewYFinanceTool() *YFinanceTool {
	t := &YFinanceTool{}

	tk := toolkit.NewToolkit()
	tk.Name = "YFinance"
	tk.Description = "Get stock market data from Yahoo Finance"

	t.Toolkit = tk
	t.Toolkit.Register("GetStockPrice", "Get the current stock price for a symbol", t, t.GetStockPrice, StockPriceParams{})
	t.Toolkit.Register("GetCompanyInfo", "Get company information", t, t.GetCompanyInfo, StockPriceParams{})

	return t
}

type StockPriceParams struct {
	Symbol string `json:"symbol" jsonschema:"description=The stock symbol (e.g. AAPL, MSFT),required=true"`
}

// Yahoo Finance API response structures (simplified)
type YahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency             string  `json:"currency"`
				Symbol               string  `json:"symbol"`
				RegularMarketPrice   float64 `json:"regularMarketPrice"`
				RegularMarketTime    int64   `json:"regularMarketTime"`
				PreviousClose        float64 `json:"previousClose"`
				RegularMarketDayHigh float64 `json:"regularMarketDayHigh"`
				RegularMarketDayLow  float64 `json:"regularMarketDayLow"`
			} `json:"meta"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}

// GetStockPrice gets the current price for a stock
func (t *YFinanceTool) GetStockPrice(params StockPriceParams) (string, error) {
	symbol := strings.ToUpper(params.Symbol)
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch stock data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch data: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var data YahooChartResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(data.Chart.Result) == 0 {
		return fmt.Sprintf("No data found for symbol %s", symbol), nil
	}

	meta := data.Chart.Result[0].Meta

	change := meta.RegularMarketPrice - meta.PreviousClose
	changePercent := (change / meta.PreviousClose) * 100

	return fmt.Sprintf("Stock Data for %s:\n"+
		"Current Price: %.2f %s\n"+
		"Change: %.2f (%.2f%%)\n"+
		"High: %.2f\n"+
		"Low: %.2f\n"+
		"Previous Close: %.2f",
		meta.Symbol, meta.RegularMarketPrice, meta.Currency,
		change, changePercent,
		meta.RegularMarketDayHigh, meta.RegularMarketDayLow, meta.PreviousClose), nil
}

// GetCompanyInfo gets basic company info (simulated for now as the API is complex)
func (t *YFinanceTool) GetCompanyInfo(params StockPriceParams) (string, error) {
	// Note: The full company profile API often requires cookies/crumb which is complex to implement
	// without a library. For this demo, we'll return the price data which contains some meta info.
	return t.GetStockPrice(params)
}

// Execute implements the Tool interface
func (t *YFinanceTool) Execute(methodName string, args json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, args)
}
