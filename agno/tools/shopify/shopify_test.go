package shopify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newServer(t *testing.T, gotQuery *string, resp string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/admin/api/2025-10/graphql.json" || r.Header.Get("X-Shopify-Access-Token") != "tok" {
			t.Errorf("unexpected request %s %s token=%q", r.Method, r.URL.Path, r.Header.Get("X-Shopify-Access-Token"))
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		*gotQuery = body["query"]
		_, _ = w.Write([]byte(resp))
	}))
}

func TestGetOrdersAndTopSelling(t *testing.T) {
	var q string
	srv := newServer(t, &q, `{"data":{"orders":{"edges":[
		{"node":{"id":"o1","name":"#1","createdAt":"2025-01-02T10:00:00Z","totalPriceSet":{"shopMoney":{"amount":"30.00","currencyCode":"USD"}},
		 "customer":{"id":"c1","email":"a@b.com","firstName":"Ann","lastName":"Lee"},
		 "lineItems":{"edges":[{"node":{"id":"l1","title":"Mug","quantity":2,"variant":{"sku":"M","product":{"id":"p1","title":"Mug"}},"originalUnitPriceSet":{"shopMoney":{"amount":"10.00"}}}},
		                       {"node":{"id":"l2","title":"Tea","quantity":1,"variant":{"sku":"T","product":{"id":"p2","title":"Tea"}},"originalUnitPriceSet":{"shopMoney":{"amount":"10.00"}}}}]}}},
		{"node":{"id":"o2","name":"#2","createdAt":"2025-01-03T10:00:00Z","totalPriceSet":{"shopMoney":{"amount":"5.50","currencyCode":"USD"}},"customer":null,
		 "lineItems":{"edges":[{"node":{"id":"l3","title":"Tea","quantity":3,"variant":{"sku":"T","product":{"id":"p2","title":"Tea"}},"originalUnitPriceSet":{"shopMoney":{"amount":"1.833"}}}}]}}}]}}}`)
	defer srv.Close()
	s := New(WithAccessToken("tok"), WithBaseURL(srv.URL))
	if len(s.GetMethods()) != 9 {
		t.Fatalf("expected 9 methods, got %d", len(s.GetMethods()))
	}

	out, err := s.GetOrders(GetOrdersParams{MaxResults: 500, Status: "paid", CreatedAfter: "2025-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, `orders(first: 250, query: "created_at:>=2025-01-01 AND financial_status:paid"`) {
		t.Fatalf("unexpected query %s", q)
	}
	var orders []map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &orders)
	if len(orders) != 2 || orders[0]["customer"].(map[string]interface{})["name"] != "Ann Lee" || orders[1]["customer"] != nil {
		t.Fatalf("unexpected orders %s", out)
	}

	out, _ = s.GetTopSellingProducts(TopSellingParams{})
	var top []map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &top)
	if len(top) != 2 || top[0]["id"] != "p2" || top[0]["total_quantity"] != float64(4) || top[0]["total_revenue"] != 15.5 || top[0]["rank"] != float64(1) {
		t.Fatalf("unexpected top sellers %s", out)
	}

	out, _ = s.GetSalesByDateRange(SalesByDateRangeParams{StartDate: "2025-01-01", EndDate: "2025-01-31"})
	var sales map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &sales)
	if sales["total_revenue"] != 35.5 || sales["total_orders"] != float64(2) || sales["average_order_value"] != 17.75 || len(sales["daily_breakdown"].([]interface{})) != 2 {
		t.Fatalf("unexpected sales %s", out)
	}

	out, _ = s.GetProductsBoughtTogether(BoughtTogetherParams{MinOccurrences: 1})
	if !strings.Contains(out.(string), `"times_bought_together": 1`) {
		t.Fatalf("unexpected pairs %s", out)
	}
}

func TestGraphQLErrorsAndMissingToken(t *testing.T) {
	var q string
	srv := newServer(t, &q, `{"errors":[{"message":"Access denied"}]}`)
	defer srv.Close()
	out, err := New(WithAccessToken("tok"), WithBaseURL(srv.URL)).GetShopInfo(EmptyParams{})
	if err != nil || !strings.Contains(out.(string), "Access denied") || !strings.Contains(out.(string), `"error"`) {
		t.Fatalf("unexpected %v %v", out, err)
	}
	t.Setenv("SHOPIFY_ACCESS_TOKEN", "")
	if _, err := New().GetShopInfo(EmptyParams{}); err == nil {
		t.Fatal("expected missing token error")
	}
}

func TestLowStock(t *testing.T) {
	var q string
	srv := newServer(t, &q, `{"data":{"products":{"edges":[
		{"node":{"id":"p1","title":"A","totalInventory":50,"variants":{"edges":[]}}},
		{"node":{"id":"p2","title":"B","totalInventory":5,"variants":{"edges":[{"node":{"id":"v1","title":"S","sku":"x","inventoryQuantity":2}},{"node":{"id":"v2","title":"L","inventoryQuantity":20}}]}}}]}}}`)
	defer srv.Close()
	out, _ := New(WithAccessToken("tok"), WithBaseURL(srv.URL)).GetLowStockProducts(LowStockParams{})
	var res []map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &res)
	if len(res) != 1 || res[0]["id"] != "p2" || len(res[0]["low_stock_variants"].([]interface{})) != 1 {
		t.Fatalf("unexpected low stock %s", out)
	}
}
