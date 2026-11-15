// Package shopify ports Agno's ShopifyTools: sales, product, inventory and
// customer analytics through the Shopify Admin GraphQL API.
//
// The main read/analytics methods are ported (shop info, products, orders,
// top sellers, products bought together, sales by date range, customer order
// history, inventory levels and low stock). Python's get_order_analytics,
// get_product_sales_breakdown, get_sales_trends, get_average_order_value and
// get_repeat_customers are not ported.
package shopify

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultAPIVersion is the Shopify Admin API version.
const DefaultAPIVersion = "2025-10"

// ShopifyTools analyzes Shopify store data.
type ShopifyTools struct {
	toolkit.Toolkit
	shopName    string
	accessToken string
	apiVersion  string
	baseURL     string
	httpClient  *http.Client
}

// Option configures ShopifyTools.
type Option func(*ShopifyTools)

// WithShopName sets the store name, e.g. "my-store" (default: SHOPIFY_SHOP_NAME).
func WithShopName(v string) Option { return func(s *ShopifyTools) { s.shopName = v } }

// WithAccessToken sets the Admin API access token (default: SHOPIFY_ACCESS_TOKEN).
func WithAccessToken(v string) Option { return func(s *ShopifyTools) { s.accessToken = v } }

// WithAPIVersion sets the API version (default 2025-10).
func WithAPIVersion(v string) Option { return func(s *ShopifyTools) { s.apiVersion = v } }

// WithBaseURL overrides the store URL (default https://{shop_name}.myshopify.com).
func WithBaseURL(v string) Option { return func(s *ShopifyTools) { s.baseURL = v } }

// WithTimeout sets the HTTP timeout (default 30s).
func WithTimeout(d time.Duration) Option { return func(s *ShopifyTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *ShopifyTools) { s.httpClient = c } }

// EmptyParams is used by methods without arguments.
type EmptyParams struct{}

// GetProductsParams are the parameters of GetProducts.
type GetProductsParams struct {
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of products to return (default 50, max 250)."`
	Status     string `json:"status,omitempty" description:"Filter by status - 'ACTIVE', 'ARCHIVED', or 'DRAFT' (optional)."`
}

// GetOrdersParams are the parameters of GetOrders.
type GetOrdersParams struct {
	MaxResults    int    `json:"max_results,omitempty" description:"Maximum number of orders to return (default 50, max 250)."`
	Status        string `json:"status,omitempty" description:"Filter by financial status - 'paid', 'pending', 'refunded' (optional)."`
	CreatedAfter  string `json:"created_after,omitempty" description:"Only include orders created after this date (YYYY-MM-DD format, optional)."`
	CreatedBefore string `json:"created_before,omitempty" description:"Only include orders created before this date (YYYY-MM-DD format, optional)."`
}

// TopSellingParams are the parameters of GetTopSellingProducts.
type TopSellingParams struct {
	Limit         int    `json:"limit,omitempty" description:"Number of top products to return (default 10)."`
	CreatedAfter  string `json:"created_after,omitempty" description:"Only include orders created after this date (YYYY-MM-DD format, optional)."`
	CreatedBefore string `json:"created_before,omitempty" description:"Only include orders created before this date (YYYY-MM-DD format, optional)."`
}

// BoughtTogetherParams are the parameters of GetProductsBoughtTogether.
type BoughtTogetherParams struct {
	MinOccurrences int    `json:"min_occurrences,omitempty" description:"Minimum times a pair must appear together (default 2)."`
	Limit          int    `json:"limit,omitempty" description:"Number of product pairs to return (default 20)."`
	CreatedAfter   string `json:"created_after,omitempty" description:"Only include orders created after this date (YYYY-MM-DD format, optional)."`
	CreatedBefore  string `json:"created_before,omitempty" description:"Only include orders created before this date (YYYY-MM-DD format, optional)."`
}

// SalesByDateRangeParams are the parameters of GetSalesByDateRange.
type SalesByDateRangeParams struct {
	StartDate string `json:"start_date" description:"Start date in YYYY-MM-DD format." required:"true"`
	EndDate   string `json:"end_date" description:"End date in YYYY-MM-DD format." required:"true"`
}

// CustomerOrderHistoryParams are the parameters of GetCustomerOrderHistory.
type CustomerOrderHistoryParams struct {
	CustomerEmail string `json:"customer_email" description:"The customer's email address." required:"true"`
	MaxOrders     int    `json:"max_orders,omitempty" description:"Maximum number of orders to return (default 50)."`
}

// InventoryLevelsParams are the parameters of GetInventoryLevels.
type InventoryLevelsParams struct {
	MaxResults int `json:"max_results,omitempty" description:"Maximum number of products to return (default 100)."`
}

// LowStockParams are the parameters of GetLowStockProducts.
type LowStockParams struct {
	Threshold  int `json:"threshold,omitempty" description:"Inventory level below which a product is considered low stock (default 10)."`
	MaxResults int `json:"max_results,omitempty" description:"Maximum number of products to return (default 50)."`
}

// New creates the Shopify toolkit.
func New(opts ...Option) *ShopifyTools {
	s := &ShopifyTools{apiVersion: DefaultAPIVersion}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "shopify"
	s.Toolkit.Description = "Analyze Shopify sales data, product performance, inventory and customer insights."
	for _, o := range opts {
		o(s)
	}
	s.shopName = restkit.Env(s.shopName, "SHOPIFY_SHOP_NAME")
	s.accessToken = restkit.Env(s.accessToken, "SHOPIFY_ACCESS_TOKEN")
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	s.Register("GetShopInfo", "Get basic information about the Shopify store (name, email, currency, domain, plan).", s, s.GetShopInfo, EmptyParams{})
	s.Register("GetProducts", "Get products from the store with id, title, status, variants, and pricing.", s, s.GetProducts, GetProductsParams{})
	s.Register("GetOrders", "Get recent orders from the store with id, total, customer, and line items.", s, s.GetOrders, GetOrdersParams{})
	s.Register("GetTopSellingProducts", "Get the top selling products by quantity sold, analyzing paid orders.", s, s.GetTopSellingProducts, TopSellingParams{})
	s.Register("GetProductsBoughtTogether", "Find product pairs that are frequently bought together. Useful for cross-selling and bundle recommendations.", s, s.GetProductsBoughtTogether, BoughtTogetherParams{})
	s.Register("GetSalesByDateRange", "Get sales summary (total revenue, order count, daily breakdown) for a specific date range.", s, s.GetSalesByDateRange, SalesByDateRangeParams{})
	s.Register("GetCustomerOrderHistory", "Get customer info and order history for a specific customer email.", s, s.GetCustomerOrderHistory, CustomerOrderHistoryParams{})
	s.Register("GetInventoryLevels", "Get current inventory levels for all active products.", s, s.GetInventoryLevels, InventoryLevelsParams{})
	s.Register("GetLowStockProducts", "Get products that are running low on stock, sorted by inventory level.", s, s.GetLowStockProducts, LowStockParams{})
	return s
}

// graphql posts a query and returns the data object, or a Python-style
// {"error": ...} result when GraphQL errors are returned.
func (s *ShopifyTools) graphql(query string) (map[string]interface{}, interface{}, error) {
	if s.accessToken == "" {
		return nil, nil, fmt.Errorf("SHOPIFY_ACCESS_TOKEN is required")
	}
	base := s.baseURL
	if base == "" {
		if s.shopName == "" {
			return nil, nil, fmt.Errorf("SHOPIFY_SHOP_NAME is required")
		}
		base = "https://" + s.shopName + ".myshopify.com"
	}
	client := &restkit.Client{
		HTTP:    s.httpClient,
		BaseURL: strings.TrimRight(base, "/") + "/admin/api/" + s.apiVersion,
		Name:    "Shopify API",
		Headers: map[string]string{"X-Shopify-Access-Token": s.accessToken},
	}
	_, data, err := client.Raw(http.MethodPost, "/graphql.json", nil, map[string]interface{}{"query": query})
	var apiErr *restkit.APIError
	if err != nil && !errors.As(err, &apiErr) {
		return nil, nil, err
	}
	var result map[string]interface{}
	if jerr := json.Unmarshal(data, &result); jerr != nil {
		return nil, map[string]interface{}{"error": "Failed to parse response: " + string(data)}, nil
	}
	if errs, ok := result["errors"]; ok {
		return nil, map[string]interface{}{"error": errs}, nil
	}
	d, _ := result["data"].(map[string]interface{})
	if d == nil {
		d = map[string]interface{}{}
	}
	return d, nil, nil
}

// --- small helpers over generic JSON ---

func path(v interface{}, keys ...string) interface{} {
	for _, k := range keys {
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func edges(v interface{}, keys ...string) []map[string]interface{} {
	arr, _ := path(v, append(keys, "edges")...).([]interface{})
	out := make([]map[string]interface{}, 0, len(arr))
	for _, e := range arr {
		if n, ok := path(e, "node").(map[string]interface{}); ok {
			out = append(out, n)
		}
	}
	return out
}

func num(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func quote(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v)
}

func capFirst(n, def, max int) int {
	if n <= 0 {
		n = def
	}
	if n > max {
		n = max
	}
	return n
}

func dateFilter(parts []string, after, before string) []string {
	if after != "" {
		parts = append(parts, "created_at:>="+quote(after))
	}
	if before != "" {
		parts = append(parts, "created_at:<="+quote(before))
	}
	return parts
}

func name(c map[string]interface{}) string {
	return strings.TrimSpace(restkit.Str(c, "firstName") + " " + restkit.Str(c, "lastName"))
}

// GetShopInfo returns basic information about the store.
func (s *ShopifyTools) GetShopInfo(_ EmptyParams) (interface{}, error) {
	data, gqlErr, err := s.graphql(`query { shop { name email currencyCode primaryDomain { url } billingAddress { country city } plan { displayName } } }`)
	if err != nil {
		return nil, err
	}
	if gqlErr != nil {
		return restkit.JSONString(gqlErr), nil
	}
	shop := data["shop"]
	if shop == nil {
		shop = map[string]interface{}{}
	}
	return restkit.JSONString(shop), nil
}

// GetProducts lists products.
func (s *ShopifyTools) GetProducts(params GetProductsParams) (interface{}, error) {
	filter := ""
	if params.Status != "" {
		filter = fmt.Sprintf(`, query: "status:%s"`, quote(params.Status))
	}
	q := fmt.Sprintf(`query { products(first: %d%s) { edges { node { id title status totalInventory createdAt updatedAt
		priceRangeV2 { minVariantPrice { amount currencyCode } maxVariantPrice { amount currencyCode } }
		variants(first: 10) { edges { node { id title sku price inventoryQuantity } } } } } } }`,
		capFirst(params.MaxResults, 50, 250), filter)
	data, gqlErr, err := s.graphql(q)
	if err != nil {
		return nil, err
	}
	if gqlErr != nil {
		return restkit.JSONString(gqlErr), nil
	}
	products := []map[string]interface{}{}
	for _, n := range edges(data, "products") {
		variants := []map[string]interface{}{}
		for _, v := range edges(n, "variants") {
			variants = append(variants, map[string]interface{}{
				"id": v["id"], "title": v["title"], "sku": v["sku"], "price": v["price"], "inventory": v["inventoryQuantity"],
			})
		}
		products = append(products, map[string]interface{}{
			"id":              n["id"],
			"title":           n["title"],
			"status":          n["status"],
			"total_inventory": n["totalInventory"],
			"created_at":      n["createdAt"],
			"price_range": map[string]interface{}{
				"min":      path(n, "priceRangeV2", "minVariantPrice", "amount"),
				"max":      path(n, "priceRangeV2", "maxVariantPrice", "amount"),
				"currency": path(n, "priceRangeV2", "minVariantPrice", "currencyCode"),
			},
			"variants": variants,
		})
	}
	return restkit.JSONString(products), nil
}

// GetOrders lists recent orders.
func (s *ShopifyTools) GetOrders(params GetOrdersParams) (interface{}, error) {
	parts := dateFilter(nil, params.CreatedAfter, params.CreatedBefore)
	if params.Status != "" {
		parts = append(parts, "financial_status:"+quote(params.Status))
	}
	filter := ""
	if len(parts) > 0 {
		filter = fmt.Sprintf(`, query: "%s"`, strings.Join(parts, " AND "))
	}
	q := fmt.Sprintf(`query { orders(first: %d%s, sortKey: CREATED_AT, reverse: true) { edges { node {
		id name createdAt displayFinancialStatus displayFulfillmentStatus
		totalPriceSet { shopMoney { amount currencyCode } } subtotalPriceSet { shopMoney { amount } }
		customer { id email firstName lastName }
		lineItems(first: 50) { edges { node { id title quantity variant { id sku } originalUnitPriceSet { shopMoney { amount } } } } } } } } }`,
		capFirst(params.MaxResults, 50, 250), filter)
	data, gqlErr, err := s.graphql(q)
	if err != nil {
		return nil, err
	}
	if gqlErr != nil {
		return restkit.JSONString(gqlErr), nil
	}
	orders := []map[string]interface{}{}
	for _, n := range edges(data, "orders") {
		var customer interface{}
		if c, ok := n["customer"].(map[string]interface{}); ok && len(c) > 0 {
			customer = map[string]interface{}{"id": c["id"], "email": c["email"], "name": name(c)}
		}
		items := []map[string]interface{}{}
		for _, it := range edges(n, "lineItems") {
			items = append(items, map[string]interface{}{
				"id": it["id"], "title": it["title"], "quantity": it["quantity"],
				"unit_price": path(it, "originalUnitPriceSet", "shopMoney", "amount"),
				"sku":        path(it, "variant", "sku"),
			})
		}
		orders = append(orders, map[string]interface{}{
			"id":                 n["id"],
			"name":               n["name"],
			"created_at":         n["createdAt"],
			"financial_status":   n["displayFinancialStatus"],
			"fulfillment_status": n["displayFulfillmentStatus"],
			"total":              path(n, "totalPriceSet", "shopMoney", "amount"),
			"subtotal":           path(n, "subtotalPriceSet", "shopMoney", "amount"),
			"currency":           path(n, "totalPriceSet", "shopMoney", "currencyCode"),
			"customer":           customer,
			"line_items":         items,
		})
	}
	return restkit.JSONString(orders), nil
}

// GetTopSellingProducts ranks products by quantity sold in paid orders.
func (s *ShopifyTools) GetTopSellingProducts(params TopSellingParams) (interface{}, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}
	filter := strings.Join(dateFilter([]string{"financial_status:paid"}, params.CreatedAfter, params.CreatedBefore), " AND ")
	q := fmt.Sprintf(`query { orders(first: 250, query: "%s", sortKey: CREATED_AT) { edges { node {
		lineItems(first: 100) { edges { node { title quantity variant { id product { id title } } originalUnitPriceSet { shopMoney { amount } } } } } } } } }`, filter)
	data, gqlErr, err := s.graphql(q)
	if err != nil {
		return nil, err
	}
	if gqlErr != nil {
		return restkit.JSONString(gqlErr), nil
	}
	type productSales struct {
		ID            interface{} `json:"id"`
		Title         interface{} `json:"title"`
		TotalQuantity float64     `json:"total_quantity"`
		TotalRevenue  float64     `json:"total_revenue"`
		OrderCount    int         `json:"order_count"`
		Rank          int         `json:"rank"`
	}
	index := map[string]*productSales{}
	ordered := []*productSales{}
	for _, order := range edges(data, "orders") {
		for _, it := range edges(order, "lineItems") {
			product, ok := path(it, "variant", "product").(map[string]interface{})
			if !ok {
				continue
			}
			id := restkit.Str(product, "id")
			p := index[id]
			if p == nil {
				p = &productSales{ID: product["id"], Title: product["title"]}
				index[id] = p
				ordered = append(ordered, p)
			}
			qty := num(it["quantity"])
			p.TotalQuantity += qty
			p.TotalRevenue += qty * num(path(it, "originalUnitPriceSet", "shopMoney", "amount"))
			p.OrderCount++
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].TotalQuantity > ordered[j].TotalQuantity })
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	for i, p := range ordered {
		p.Rank = i + 1
		p.TotalRevenue = round2(p.TotalRevenue)
	}
	return restkit.JSONString(ordered), nil
}

// GetProductsBoughtTogether finds product pairs frequently bought together.
func (s *ShopifyTools) GetProductsBoughtTogether(params BoughtTogetherParams) (interface{}, error) {
	minOcc := params.MinOccurrences
	if minOcc <= 0 {
		minOcc = 2
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	filter := strings.Join(dateFilter([]string{"financial_status:paid"}, params.CreatedAfter, params.CreatedBefore), " AND ")
	q := fmt.Sprintf(`query { orders(first: 250, query: "%s") { edges { node {
		lineItems(first: 100) { edges { node { variant { product { id title } } } } } } } } }`, filter)
	data, gqlErr, err := s.graphql(q)
	if err != nil {
		return nil, err
	}
	if gqlErr != nil {
		return restkit.JSONString(gqlErr), nil
	}
	type pairCount struct {
		a, b  string
		count int
	}
	titles := map[string]interface{}{}
	counts := map[[2]string]*pairCount{}
	var pairs []*pairCount
	for _, order := range edges(data, "orders") {
		seen := map[string]bool{}
		var ids []string
		for _, it := range edges(order, "lineItems") {
			product, ok := path(it, "variant", "product").(map[string]interface{})
			if !ok {
				continue
			}
			id := restkit.Str(product, "id")
			titles[id] = product["title"]
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				key := [2]string{ids[i], ids[j]}
				pc := counts[key]
				if pc == nil {
					pc = &pairCount{a: ids[i], b: ids[j]}
					counts[key] = pc
					pairs = append(pairs, pc)
				}
				pc.count++
			}
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	title := func(id string) interface{} {
		if t, ok := titles[id]; ok {
			return t
		}
		return "Unknown"
	}
	out := []map[string]interface{}{}
	for _, p := range pairs {
		if p.count < minOcc {
			continue
		}
		out = append(out, map[string]interface{}{
			"product_1":             map[string]interface{}{"id": p.a, "title": title(p.a)},
			"product_2":             map[string]interface{}{"id": p.b, "title": title(p.b)},
			"times_bought_together": p.count,
		})
	}
	return restkit.JSONString(out), nil
}

// GetSalesByDateRange summarizes paid sales between two dates.
func (s *ShopifyTools) GetSalesByDateRange(params SalesByDateRangeParams) (interface{}, error) {
	q := fmt.Sprintf(`query { orders(first: 250, query: "created_at:>=%s AND created_at:<=%s AND financial_status:paid", sortKey: CREATED_AT) { edges { node {
		createdAt totalPriceSet { shopMoney { amount currencyCode } } lineItems(first: 100) { edges { node { quantity } } } } } } }`,
		quote(params.StartDate), quote(params.EndDate))
	data, gqlErr, err := s.graphql(q)
	if err != nil {
		return nil, err
	}
	if gqlErr != nil {
		return restkit.JSONString(gqlErr), nil
	}
	type day struct {
		Date    string  `json:"date"`
		Revenue float64 `json:"revenue"`
		Orders  int     `json:"orders"`
		Items   float64 `json:"items"`
	}
	var totalRevenue, totalItems float64
	totalOrders := 0
	var currency interface{}
	days := map[string]*day{}
	for _, o := range edges(data, "orders") {
		amount := num(path(o, "totalPriceSet", "shopMoney", "amount"))
		currency = path(o, "totalPriceSet", "shopMoney", "currencyCode")
		date := restkit.Truncate(restkit.Str(o, "createdAt"), 10)
		var items float64
		for _, it := range edges(o, "lineItems") {
			items += num(it["quantity"])
		}
		totalRevenue += amount
		totalOrders++
		totalItems += items
		d := days[date]
		if d == nil {
			d = &day{Date: date}
			days[date] = d
		}
		d.Revenue += amount
		d.Orders++
		d.Items += items
	}
	daily := make([]*day, 0, len(days))
	for _, d := range days {
		d.Revenue = round2(d.Revenue)
		daily = append(daily, d)
	}
	sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })
	var avg float64
	if totalOrders > 0 {
		avg = round2(totalRevenue / float64(totalOrders))
	}
	return restkit.JSONString(map[string]interface{}{
		"period":              map[string]string{"start": params.StartDate, "end": params.EndDate},
		"total_revenue":       round2(totalRevenue),
		"total_orders":        totalOrders,
		"total_items_sold":    totalItems,
		"average_order_value": avg,
		"currency":            currency,
		"daily_breakdown":     daily,
	}), nil
}

// GetCustomerOrderHistory returns a customer's info and order history.
func (s *ShopifyTools) GetCustomerOrderHistory(params CustomerOrderHistoryParams) (interface{}, error) {
	q := fmt.Sprintf(`query { orders(first: %d, query: "email:%s", sortKey: CREATED_AT, reverse: true) { edges { node {
		id name createdAt displayFinancialStatus displayFulfillmentStatus totalPriceSet { shopMoney { amount currencyCode } }
		customer { id firstName lastName numberOfOrders amountSpent { amount currencyCode } }
		lineItems(first: 20) { edges { node { title quantity } } } } } } }`,
		capFirst(params.MaxOrders, 50, 250), quote(params.CustomerEmail))
	data, gqlErr, err := s.graphql(q)
	if err != nil {
		return nil, err
	}
	if gqlErr != nil {
		return restkit.JSONString(gqlErr), nil
	}
	orders := edges(data, "orders")
	if len(orders) == 0 {
		return restkit.JSONString(map[string]string{"message": "No orders found for " + params.CustomerEmail}), nil
	}
	c, _ := orders[0]["customer"].(map[string]interface{})
	customer := map[string]interface{}{
		"email":        params.CustomerEmail,
		"id":           path(c, "id"),
		"name":         name(c),
		"total_orders": path(c, "numberOfOrders"),
		"total_spent":  path(c, "amountSpent", "amount"),
		"currency":     path(c, "amountSpent", "currencyCode"),
	}
	list := []map[string]interface{}{}
	for _, o := range orders {
		items := []map[string]interface{}{}
		for _, it := range edges(o, "lineItems") {
			items = append(items, map[string]interface{}{"title": it["title"], "quantity": it["quantity"]})
		}
		list = append(list, map[string]interface{}{
			"id":                 o["id"],
			"name":               o["name"],
			"created_at":         o["createdAt"],
			"financial_status":   o["displayFinancialStatus"],
			"fulfillment_status": o["displayFulfillmentStatus"],
			"total":              path(o, "totalPriceSet", "shopMoney", "amount"),
			"items":              items,
		})
	}
	return restkit.JSONString(map[string]interface{}{"customer": customer, "orders": list}), nil
}

// GetInventoryLevels returns inventory levels for active products.
func (s *ShopifyTools) GetInventoryLevels(params InventoryLevelsParams) (interface{}, error) {
	q := fmt.Sprintf(`query { products(first: %d, query: "status:ACTIVE") { edges { node { id title totalInventory tracksInventory
		variants(first: 50) { edges { node { id title sku inventoryQuantity inventoryPolicy } } } } } } }`,
		capFirst(params.MaxResults, 100, 250))
	data, gqlErr, err := s.graphql(q)
	if err != nil {
		return nil, err
	}
	if gqlErr != nil {
		return restkit.JSONString(gqlErr), nil
	}
	products := []map[string]interface{}{}
	for _, n := range edges(data, "products") {
		variants := []map[string]interface{}{}
		for _, v := range edges(n, "variants") {
			variants = append(variants, map[string]interface{}{
				"id": v["id"], "title": v["title"], "sku": v["sku"],
				"inventory_quantity": v["inventoryQuantity"], "inventory_policy": v["inventoryPolicy"],
			})
		}
		products = append(products, map[string]interface{}{
			"id": n["id"], "title": n["title"], "total_inventory": n["totalInventory"],
			"tracks_inventory": n["tracksInventory"], "variants": variants,
		})
	}
	return restkit.JSONString(products), nil
}

// GetLowStockProducts returns active products at or below the threshold.
func (s *ShopifyTools) GetLowStockProducts(params LowStockParams) (interface{}, error) {
	threshold := 10.0
	if params.Threshold > 0 {
		threshold = float64(params.Threshold)
	}
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}
	data, gqlErr, err := s.graphql(`query { products(first: 250, query: "status:ACTIVE") { edges { node { id title totalInventory
		variants(first: 50) { edges { node { id title sku inventoryQuantity } } } } } } }`)
	if err != nil {
		return nil, err
	}
	if gqlErr != nil {
		return restkit.JSONString(gqlErr), nil
	}
	type lowStock struct {
		ID       interface{}              `json:"id"`
		Title    interface{}              `json:"title"`
		Total    float64                  `json:"total_inventory"`
		Variants []map[string]interface{} `json:"low_stock_variants"`
	}
	out := []lowStock{}
	for _, n := range edges(data, "products") {
		if n["totalInventory"] == nil {
			continue
		}
		total := num(n["totalInventory"])
		if total > threshold {
			continue
		}
		var variants []map[string]interface{}
		for _, v := range edges(n, "variants") {
			if v["inventoryQuantity"] != nil && num(v["inventoryQuantity"]) <= threshold {
				variants = append(variants, map[string]interface{}{
					"id": v["id"], "title": v["title"], "sku": v["sku"], "inventory_quantity": v["inventoryQuantity"],
				})
			}
		}
		if len(variants) > 0 {
			out = append(out, lowStock{ID: n["id"], Title: n["title"], Total: total, Variants: variants})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Total < out[j].Total })
	if len(out) > maxResults {
		out = out[:maxResults]
	}
	return restkit.JSONString(out), nil
}

// Execute implements toolkit.Tool.
func (s *ShopifyTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}
