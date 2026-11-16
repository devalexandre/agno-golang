// Package brandfetch ports Agno's BrandfetchTools: brand data lookup (Brand API)
// and brand search by name (Brand Search API).
package brandfetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Brandfetch API root.
const DefaultBaseURL = "https://api.brandfetch.io/v2"

// BrandfetchTools retrieves brand data from Brandfetch.
type BrandfetchTools struct {
	toolkit.Toolkit
	apiKey     string
	clientID   string
	baseURL    string
	httpClient *http.Client

	enableSearchByIdentifier bool
	enableSearchByBrand      bool
}

// Option configures BrandfetchTools.
type Option func(*BrandfetchTools)

// WithAPIKey sets the Brand API key (default BRANDFETCH_API_KEY).
func WithAPIKey(k string) Option { return func(t *BrandfetchTools) { t.apiKey = k } }

// WithClientID sets the Brand Search client id (default BRANDFETCH_CLIENT_ID).
func WithClientID(id string) Option { return func(t *BrandfetchTools) { t.clientID = id } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *BrandfetchTools) { t.baseURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *BrandfetchTools) { t.httpClient = c } }

// WithTimeout sets the request timeout (default 20s).
func WithTimeout(d time.Duration) Option { return func(t *BrandfetchTools) { t.Toolkit.Timeout = d } }

// WithSearchByIdentifier enables/disables SearchByIdentifier (default true).
func WithSearchByIdentifier(on bool) Option {
	return func(t *BrandfetchTools) { t.enableSearchByIdentifier = on }
}

// WithSearchByBrand enables/disables SearchByBrand (default false).
func WithSearchByBrand(on bool) Option {
	return func(t *BrandfetchTools) { t.enableSearchByBrand = on }
}

// WithAll enables every tool.
func WithAll() Option {
	return func(t *BrandfetchTools) { t.enableSearchByIdentifier, t.enableSearchByBrand = true, true }
}

// SearchByIdentifierParams are the parameters of SearchByIdentifier.
type SearchByIdentifierParams struct {
	Identifier string `json:"identifier" description:"Options are you can use: Domain (nike.com), Brand ID (id_0dwKPKT), ISIN (US6541061031), Stock Ticker (NKE)" required:"true"`
}

// SearchByBrandParams are the parameters of SearchByBrand.
type SearchByBrandParams struct {
	Name string `json:"name" description:"Brand name to search for (e.g., 'Google', 'Apple')" required:"true"`
}

// NewBrandfetchTools creates the toolkit.
func NewBrandfetchTools(opts ...Option) *BrandfetchTools {
	t := &BrandfetchTools{
		apiKey: os.Getenv("BRANDFETCH_API_KEY"), clientID: os.Getenv("BRANDFETCH_CLIENT_ID"),
		baseURL: DefaultBaseURL, enableSearchByIdentifier: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "brandfetch_tools"
	t.Toolkit.Description = "Retrieve brand data (logos, colors, fonts) and search brands with Brandfetch."
	t.Toolkit.Timeout = 20 * time.Second
	for _, o := range opts {
		o(t)
	}
	t.baseURL = strings.TrimRight(t.baseURL, "/")
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableSearchByIdentifier {
		t.Toolkit.Register("SearchByIdentifier", "Search for brand data by identifier (domain, brand id, isin, stock ticker). Returns brand data including logos, colors, fonts, and other brand assets.", t, t.SearchByIdentifier, SearchByIdentifierParams{})
	}
	if t.enableSearchByBrand {
		t.Toolkit.Register("SearchByBrand", "Search for brands by name using the Brand Search API - can give you the right brand id to use for the brand api.", t, t.SearchByBrand, SearchByBrandParams{})
	}
	return t
}

func statusError(err error, notFound, unauthorized string) map[string]interface{} {
	var apiErr *restapi.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case 404:
			return map[string]interface{}{"error": notFound}
		case 401:
			return map[string]interface{}{"error": unauthorized}
		case 429:
			return map[string]interface{}{"error": "Rate limit exceeded"}
		}
		return map[string]interface{}{"error": fmt.Sprintf("API error: %d", apiErr.Status)}
	}
	return map[string]interface{}{"error": fmt.Sprintf("Request failed: %v", err)}
}

func (t *BrandfetchTools) get(path string, q url.Values, headers map[string]string) (interface{}, error) {
	c := restapi.New(t.baseURL, t.httpClient, headers)
	data, err := c.Do(context.Background(), http.MethodGet, path, q, nil)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return string(data), nil
	}
	return out, nil
}

// SearchByIdentifier retrieves brand data by identifier.
func (t *BrandfetchTools) SearchByIdentifier(p SearchByIdentifierParams) (interface{}, error) {
	if t.apiKey == "" {
		return nil, fmt.Errorf("API key is required for brand search by identifier")
	}
	out, err := t.get("/brands/"+url.PathEscape(p.Identifier), nil, map[string]string{"Authorization": "Bearer " + t.apiKey})
	if err != nil {
		return statusError(err, "Brand not found for identifier: "+p.Identifier, "Invalid API key"), nil
	}
	return out, nil
}

// SearchByBrand searches brands by name.
func (t *BrandfetchTools) SearchByBrand(p SearchByBrandParams) (interface{}, error) {
	if t.clientID == "" {
		return nil, fmt.Errorf("Client ID is required for brand search by name")
	}
	out, err := t.get("/search/"+url.PathEscape(p.Name), url.Values{"c": {t.clientID}}, nil)
	if err != nil {
		return statusError(err, "No brands found for name: "+p.Name, "Invalid client ID"), nil
	}
	return out, nil
}
