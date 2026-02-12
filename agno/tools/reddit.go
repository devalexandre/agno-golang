package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const redditBaseURL = "https://www.reddit.com"

// RedditTool provides search and reading capabilities for Reddit.
type RedditTool struct {
	toolkit.Toolkit
	httpClient *http.Client
}

// RedditSearchParams defines the parameters for the SearchPosts method.
type RedditSearchParams struct {
	Query     string `json:"query" description:"The search query." required:"true"`
	Subreddit string `json:"subreddit,omitempty" description:"Subreddit to search in (without r/ prefix). If empty, searches all of Reddit."`
	Sort      string `json:"sort,omitempty" description:"Sort order: relevance, hot, top, new, comments. Default: relevance."`
	Limit     int    `json:"limit,omitempty" description:"Number of results. Default: 10, max: 25."`
}

// RedditTopPostsParams defines the parameters for the GetTopPosts method.
type RedditTopPostsParams struct {
	Subreddit  string `json:"subreddit" description:"Subreddit to get top posts from (without r/ prefix)." required:"true"`
	TimeFilter string `json:"time_filter,omitempty" description:"Time filter: hour, day, week, month, year, all. Default: day."`
	Limit      int    `json:"limit,omitempty" description:"Number of results. Default: 10, max: 25."`
}

// NewRedditTool creates a new Reddit tool.
func NewRedditTool() *RedditTool {
	t := &RedditTool{
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "RedditTool"
	tk.Description = "Search and read posts from Reddit."

	t.Toolkit = tk
	t.Toolkit.Register("SearchPosts", "Search Reddit for posts matching a query.", t, t.SearchPosts, RedditSearchParams{})
	t.Toolkit.Register("GetTopPosts", "Get top posts from a subreddit.", t, t.GetTopPosts, RedditTopPostsParams{})

	return t
}

func (t *RedditTool) fetchJSON(endpoint string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	// Reddit requires a user-agent for API access
	req.Header.Set("User-Agent", "agno-golang:v1.0 (by /u/agno-bot)")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// SearchPosts searches Reddit for posts.
func (t *RedditTool) SearchPosts(params RedditSearchParams) (interface{}, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	sort := params.Sort
	if sort == "" {
		sort = "relevance"
	}
	limit := params.Limit
	if limit <= 0 || limit > 25 {
		limit = 10
	}

	var endpoint string
	if params.Subreddit != "" {
		endpoint = fmt.Sprintf("%s/r/%s/search.json?q=%s&sort=%s&limit=%d&restrict_sr=on",
			redditBaseURL, url.PathEscape(params.Subreddit), url.QueryEscape(params.Query), sort, limit)
	} else {
		endpoint = fmt.Sprintf("%s/search.json?q=%s&sort=%s&limit=%d",
			redditBaseURL, url.QueryEscape(params.Query), sort, limit)
	}

	return t.fetchJSON(endpoint)
}

// GetTopPosts gets top posts from a subreddit.
func (t *RedditTool) GetTopPosts(params RedditTopPostsParams) (interface{}, error) {
	if params.Subreddit == "" {
		return nil, fmt.Errorf("subreddit is required")
	}

	timeFilter := params.TimeFilter
	if timeFilter == "" {
		timeFilter = "day"
	}
	limit := params.Limit
	if limit <= 0 || limit > 25 {
		limit = 10
	}

	endpoint := fmt.Sprintf("%s/r/%s/top.json?t=%s&limit=%d",
		redditBaseURL, url.PathEscape(params.Subreddit), timeFilter, limit)

	return t.fetchJSON(endpoint)
}

// Execute implements the toolkit.Tool interface.
func (t *RedditTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
