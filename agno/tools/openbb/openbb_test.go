package openbb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenBB(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		switch r.URL.Path {
		case "/api/v1/equity/price/quote":
			_, _ = w.Write([]byte(`{"results":[{"symbol":"AAPL","last_price":200.5,"extra":"x"}]}`))
		case "/api/v1/news/company":
			_, _ = w.Write([]byte(`{"results":[{"title":"a","images":[1]},{"title":"b"},{"title":"c"}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	o := NewOpenBBTools(WithBaseURL(srv.URL+"/api/v1"), WithAll())
	if len(o.GetMethods()) != 5 {
		t.Fatal(len(o.GetMethods()))
	}
	out, _ := o.GetStockPrice(SymbolParams{Symbol: "AAPL"})
	s := out.(string)
	if !strings.Contains(s, `"last_price": 200.5`) || strings.Contains(s, "extra") || !strings.Contains(gotQuery, "provider=yfinance") {
		t.Fatal(s, gotQuery)
	}
	out, _ = o.GetCompanyNews(NewsParams{Symbol: "AAPL", NumStories: 2})
	if s := out.(string); strings.Contains(s, "images") || strings.Contains(s, `"c"`) {
		t.Fatal(s)
	}
	out, _ = o.GetCompanyProfile(SymbolParams{Symbol: "X"})
	if !strings.HasPrefix(out.(string), "Error fetching company profile") {
		t.Fatal(out)
	}
}
