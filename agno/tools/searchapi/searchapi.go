// Package searchapi ports Agno's SearchApiTools: Google, Google News, Google
// Images and YouTube search through SearchAPI.io
// (GET https://www.searchapi.io/api/v1/search).
package searchapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the SearchAPI.io search endpoint.
const DefaultBaseURL = "https://www.searchapi.io/api/v1/search"

// SearchApiTools searches the web with SearchAPI.io.
type SearchApiTools struct {
	toolkit.Toolkit
	apiKey        string
	baseURL       string
	numResults    int
	enableGoogle  bool
	enableNews    bool
	enableImages  bool
	enableYoutube bool
	all           bool
	httpClient    *http.Client
	client        *restkit.Client
}

// Option configures SearchApiTools.
type Option func(*SearchApiTools)

// WithAPIKey sets the API key (default: SEARCHAPI_API_KEY).
func WithAPIKey(key string) Option { return func(s *SearchApiTools) { s.apiKey = key } }

// WithBaseURL overrides the search endpoint URL.
func WithBaseURL(u string) Option { return func(s *SearchApiTools) { s.baseURL = u } }

// WithNumResults sets the default number of results (default 5).
func WithNumResults(n int) Option { return func(s *SearchApiTools) { s.numResults = n } }

// WithEnableSearchGoogle toggles SearchGoogle (default true).
func WithEnableSearchGoogle(b bool) Option { return func(s *SearchApiTools) { s.enableGoogle = b } }

// WithEnableSearchNews toggles SearchNews (default false).
func WithEnableSearchNews(b bool) Option { return func(s *SearchApiTools) { s.enableNews = b } }

// WithEnableSearchImages toggles SearchImages (default false).
func WithEnableSearchImages(b bool) Option { return func(s *SearchApiTools) { s.enableImages = b } }

// WithEnableSearchYoutube toggles SearchYoutube (default false).
func WithEnableSearchYoutube(b bool) Option { return func(s *SearchApiTools) { s.enableYoutube = b } }

// WithAll enables every tool.
func WithAll() Option { return func(s *SearchApiTools) { s.all = true } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(s *SearchApiTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *SearchApiTools) { s.httpClient = c } }

// GoogleParams are the parameters of SearchGoogle.
type GoogleParams struct {
	Query      string `json:"query" description:"The search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Number of results to return. Defaults to instance setting."`
	Location   string `json:"location,omitempty" description:"Location for localized results, e.g. \"New York,United States\"."`
	Language   string `json:"language,omitempty" description:"Language code for results, e.g. \"en\"."`
}

// NewsParams are the parameters of SearchNews.
type NewsParams struct {
	Query      string `json:"query" description:"The news search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Number of results to return. Defaults to instance setting."`
	Language   string `json:"language,omitempty" description:"Language code for results, e.g. \"en\"."`
	Country    string `json:"country,omitempty" description:"Country code for results, e.g. \"us\"."`
}

// ImagesParams are the parameters of SearchImages.
type ImagesParams struct {
	Query      string `json:"query" description:"The image search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Number of results to return. Defaults to instance setting."`
	SafeSearch string `json:"safe_search,omitempty" description:"Safe search setting: \"active\" or \"off\"."`
}

// YoutubeParams are the parameters of SearchYoutube.
type YoutubeParams struct {
	Query      string `json:"query" description:"The YouTube search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Maximum number of results to return (applied client-side)."`
}

// New creates the SearchAPI toolkit.
func New(opts ...Option) *SearchApiTools {
	s := &SearchApiTools{baseURL: DefaultBaseURL, numResults: 5, enableGoogle: true}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "searchapi_tools"
	s.Toolkit.Description = "Search Google, Google News, Google Images and YouTube using SearchAPI.io."
	for _, o := range opts {
		o(s)
	}
	s.apiKey = restkit.Env(s.apiKey, "SEARCHAPI_API_KEY")
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	s.client = &restkit.Client{HTTP: s.httpClient, BaseURL: s.baseURL, Name: "SearchAPI"}
	if s.all || s.enableGoogle {
		s.Toolkit.Register("SearchGoogle", "Search Google for the given query using SearchAPI. Returns organic results, knowledge graph and related questions as JSON.", s, s.SearchGoogle, GoogleParams{})
	}
	if s.all || s.enableNews {
		s.Toolkit.Register("SearchNews", "Search Google News for the given query using SearchAPI. Returns news articles with title, link, source and date as JSON.", s, s.SearchNews, NewsParams{})
	}
	if s.all || s.enableImages {
		s.Toolkit.Register("SearchImages", "Search Google Images for the given query using SearchAPI. Returns image results with title, link and thumbnail as JSON.", s, s.SearchImages, ImagesParams{})
	}
	if s.all || s.enableYoutube {
		s.Toolkit.Register("SearchYoutube", "Search YouTube for the given query using SearchAPI. Returns video results with title, link, channel, length, views and description as JSON.", s, s.SearchYoutube, YoutubeParams{})
	}
	return s
}

func (s *SearchApiTools) num(n int) int {
	if n > 0 {
		return n
	}
	return s.numResults
}

func (s *SearchApiTools) request(q url.Values) (map[string]interface{}, string) {
	if s.apiKey == "" {
		return nil, "No SearchAPI key provided. Set the SEARCHAPI_API_KEY environment variable."
	}
	q.Set("api_key", s.apiKey)
	var data map[string]interface{}
	if err := s.client.Into(http.MethodGet, "", q, nil, &data); err != nil {
		if _, ok := err.(*restkit.APIError); ok {
			return nil, "HTTP error: " + err.Error()
		}
		// Transport errors embed the request URL; never leak the key.
		return nil, strings.ReplaceAll(err.Error(), url.QueryEscape(s.apiKey), "***")
	}
	return data, ""
}

func obj(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func get(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	return m[key]
}

func list(m map[string]interface{}, key string) []interface{} {
	arr, _ := get(m, key).([]interface{})
	return arr
}

// nested returns v[key] when v is an object, else v itself.
func nested(v interface{}, key string) interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m[key]
	}
	return v
}

const emptyQuery = "Please provide a query to search for"

// SearchGoogle searches Google.
func (s *SearchApiTools) SearchGoogle(p GoogleParams) (interface{}, error) {
	if p.Query == "" {
		return restkit.ErrorJSON(emptyQuery), nil
	}
	q := url.Values{"engine": {"google"}, "q": {p.Query}, "num": {strconv.Itoa(s.num(p.NumResults))}}
	if p.Location != "" {
		q.Set("location", p.Location)
	}
	if p.Language != "" {
		q.Set("hl", p.Language)
	}
	data, errMsg := s.request(q)
	if errMsg != "" {
		return restkit.ErrorJSON(errMsg), nil
	}
	type organic struct {
		Position interface{} `json:"position"`
		Title    interface{} `json:"title"`
		Link     interface{} `json:"link"`
		Snippet  interface{} `json:"snippet"`
		Source   interface{} `json:"source"`
	}
	type question struct {
		Question interface{} `json:"question"`
		Snippet  interface{} `json:"snippet"`
	}
	results := []organic{}
	for _, it := range list(data, "organic_results") {
		r := obj(it)
		results = append(results, organic{get(r, "position"), get(r, "title"), get(r, "link"), get(r, "snippet"), get(r, "source")})
	}
	questions := []question{}
	for _, it := range list(data, "related_questions") {
		r := obj(it)
		questions = append(questions, question{get(r, "question"), get(r, "snippet")})
	}
	return restkit.JSONString(struct {
		OrganicResults    []organic   `json:"organic_results"`
		KnowledgeGraph    interface{} `json:"knowledge_graph"`
		RelatedQuestions  []question  `json:"related_questions"`
		SearchInformation interface{} `json:"search_information"`
	}{results, get(data, "knowledge_graph"), questions, get(data, "search_information")}), nil
}

// SearchNews searches Google News.
func (s *SearchApiTools) SearchNews(p NewsParams) (interface{}, error) {
	if p.Query == "" {
		return restkit.ErrorJSON(emptyQuery), nil
	}
	q := url.Values{"engine": {"google_news"}, "q": {p.Query}, "num": {strconv.Itoa(s.num(p.NumResults))}}
	if p.Language != "" {
		q.Set("hl", p.Language)
	}
	if p.Country != "" {
		q.Set("gl", p.Country)
	}
	data, errMsg := s.request(q)
	if errMsg != "" {
		return restkit.ErrorJSON(errMsg), nil
	}
	type item struct {
		Position  interface{} `json:"position"`
		Title     interface{} `json:"title"`
		Link      interface{} `json:"link"`
		Source    interface{} `json:"source"`
		Date      interface{} `json:"date"`
		Snippet   interface{} `json:"snippet"`
		Thumbnail interface{} `json:"thumbnail"`
	}
	results := []item{}
	for _, it := range list(data, "news_results") {
		r := obj(it)
		results = append(results, item{get(r, "position"), get(r, "title"), get(r, "link"), nested(get(r, "source"), "name"),
			get(r, "date"), get(r, "snippet"), get(r, "thumbnail")})
	}
	return restkit.JSONString(map[string]interface{}{"news_results": results}), nil
}

// SearchImages searches Google Images.
func (s *SearchApiTools) SearchImages(p ImagesParams) (interface{}, error) {
	if p.Query == "" {
		return restkit.ErrorJSON(emptyQuery), nil
	}
	q := url.Values{"engine": {"google_images"}, "q": {p.Query}, "num": {strconv.Itoa(s.num(p.NumResults))}}
	if p.SafeSearch != "" {
		q.Set("safe", p.SafeSearch)
	}
	data, errMsg := s.request(q)
	if errMsg != "" {
		return restkit.ErrorJSON(errMsg), nil
	}
	type item struct {
		Position  interface{} `json:"position"`
		Title     interface{} `json:"title"`
		Link      interface{} `json:"link"`
		Original  interface{} `json:"original"`
		Thumbnail interface{} `json:"thumbnail"`
		Source    interface{} `json:"source"`
	}
	results := []item{}
	for _, it := range list(data, "image_results") {
		r := obj(it)
		results = append(results, item{get(r, "position"), get(r, "title"), get(r, "link"), get(r, "original"), get(r, "thumbnail"), get(r, "source")})
	}
	return restkit.JSONString(map[string]interface{}{"image_results": results}), nil
}

// SearchYoutube searches YouTube; num_results is applied client-side.
func (s *SearchApiTools) SearchYoutube(p YoutubeParams) (interface{}, error) {
	if p.Query == "" {
		return restkit.ErrorJSON(emptyQuery), nil
	}
	data, errMsg := s.request(url.Values{"engine": {"youtube"}, "q": {p.Query}})
	if errMsg != "" {
		return restkit.ErrorJSON(errMsg), nil
	}
	type item struct {
		Position      interface{} `json:"position"`
		ID            interface{} `json:"id"`
		Title         interface{} `json:"title"`
		Link          interface{} `json:"link"`
		Channel       interface{} `json:"channel"`
		Length        interface{} `json:"length"`
		Views         interface{} `json:"views"`
		PublishedTime interface{} `json:"published_time"`
		Description   interface{} `json:"description"`
		Thumbnail     interface{} `json:"thumbnail"`
	}
	videos := list(data, "videos")
	if limit := s.num(p.NumResults); len(videos) > limit {
		videos = videos[:limit]
	}
	results := []item{}
	for _, it := range videos {
		r := obj(it)
		results = append(results, item{get(r, "position"), get(r, "id"), get(r, "title"), get(r, "link"),
			nested(get(r, "channel"), "title"), get(r, "length"), get(r, "views"), get(r, "published_time"),
			get(r, "description"), nested(get(r, "thumbnail"), "static")})
	}
	return restkit.JSONString(map[string]interface{}{"video_results": results}), nil
}

// Execute implements toolkit.Tool.
func (s *SearchApiTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}
