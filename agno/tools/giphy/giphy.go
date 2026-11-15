// Package giphy ports Agno's GiphyTools: GIF search through the GIPHY API.
package giphy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/uuid"
)

// DefaultBaseURL is the GIPHY API base URL.
const DefaultBaseURL = "https://api.giphy.com"

// Image is a found GIF artifact.
type Image struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	AltText       string `json:"alt_text"`
	RevisedPrompt string `json:"revised_prompt"`
}

// GiphyTools wraps the GIPHY search API.
type GiphyTools struct {
	toolkit.Toolkit
	apiKey     string
	baseURL    string
	limit      int
	httpClient *http.Client

	enableSearchGifs bool

	mu     sync.Mutex
	images []Image
}

// Option configures GiphyTools.
type Option func(*GiphyTools)

// WithAPIKey sets the API key (default GIPHY_API_KEY).
func WithAPIKey(k string) Option { return func(t *GiphyTools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *GiphyTools) { t.baseURL = u } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *GiphyTools) { t.httpClient = c } }

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option { return func(t *GiphyTools) { t.Toolkit.Timeout = d } }

// WithLimit sets the number of GIFs to return (default 1).
func WithLimit(n int) Option { return func(t *GiphyTools) { t.limit = n } }

// WithSearchGifs toggles SearchGifs (default true).
func WithSearchGifs(b bool) Option { return func(t *GiphyTools) { t.enableSearchGifs = b } }

// SearchGifsParams are the SearchGifs parameters.
type SearchGifsParams struct {
	Query string `json:"query" description:"A text description of the required gif." required:"true"`
}

// NewGiphyTools creates the toolkit.
func NewGiphyTools(opts ...Option) *GiphyTools {
	t := &GiphyTools{apiKey: os.Getenv("GIPHY_API_KEY"), baseURL: DefaultBaseURL, limit: 1, enableSearchGifs: true}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "giphy_tools"
	t.Toolkit.Description = "Find GIFs on GIPHY."
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableSearchGifs {
		t.Toolkit.Register("SearchGifs", "Find a GIPHY gif", t, t.SearchGifs, SearchGifsParams{})
	}
	return t
}

// Images returns the GIFs found so far.
func (t *GiphyTools) Images() []Image {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Image(nil), t.images...)
}

// SearchGifs searches GIPHY.
func (t *GiphyTools) SearchGifs(p SearchGifsParams) (interface{}, error) {
	q := url.Values{"api_key": {t.apiKey}, "q": {p.Query}, "limit": {strconv.Itoa(t.limit)}}
	var resp struct {
		Data []struct {
			AltText string `json:"alt_text"`
			Images  struct {
				Original struct {
					URL string `json:"url"`
				} `json:"original"`
			} `json:"images"`
		} `json:"data"`
	}
	err := restapi.New(t.baseURL, t.httpClient, nil).JSON(context.Background(), http.MethodGet, "/v1/gifs/search", q, nil, &resp)
	if err != nil {
		var apiErr *restapi.APIError
		if errors.As(err, &apiErr) {
			return fmt.Sprintf("HTTP error occurred: %d", apiErr.Status), nil
		}
		return fmt.Sprintf("An error occurred: %v", err), nil
	}
	var urls []string
	var found []Image
	for _, g := range resp.Data {
		urls = append(urls, g.Images.Original.URL)
		found = append(found, Image{ID: uuid.NewString(), URL: g.Images.Original.URL, AltText: g.AltText, RevisedPrompt: p.Query})
	}
	if len(found) == 0 {
		return "No gifs found", nil
	}
	t.mu.Lock()
	t.images = append(t.images, found...)
	t.mu.Unlock()
	return fmt.Sprintf("Found %d GIF(s): ['%s']", len(urls), strings.Join(urls, "', '")), nil
}
