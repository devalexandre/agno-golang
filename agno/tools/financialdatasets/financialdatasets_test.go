package financialdatasets

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFinancialDatasets(t *testing.T) {
	var path, query, key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query, key = r.URL.Path, r.URL.RawQuery, r.Header.Get("X-API-KEY")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	f := NewFinancialDatasetsTools(WithAPIKey("k"), WithBaseURL(srv.URL))
	if len(f.GetMethods()) != 14 {
		t.Fatalf("methods=%d", len(f.GetMethods()))
	}
	out, _ := f.GetIncomeStatements(StatementParams{Ticker: "AAPL"})
	if out != `{"ok":true}` || path != "/financials/income-statements" || key != "k" ||
		!strings.Contains(query, "period=annual") || !strings.Contains(query, "limit=10") {
		t.Fatalf("out=%v path=%s q=%s", out, path, query)
	}
	_, _ = f.GetSecFilings(SECFilingsParams{Ticker: "AAPL", FormType: "10-K"})
	if path != "/sec-filings" || !strings.Contains(query, "form_type=10-K") {
		t.Fatal(path, query)
	}
	_, _ = f.Execute("financial_datasets_tools_GetNews", []byte(`{}`))
	if path != "/news" || strings.Contains(query, "ticker") {
		t.Fatal(path, query)
	}
	if out, _ := NewFinancialDatasetsTools(WithAPIKey(""), WithBaseURL(srv.URL)).GetEarnings(TickerLimitParams{Ticker: "X"}); out != "API key not set" {
		t.Fatal(out)
	}
}
