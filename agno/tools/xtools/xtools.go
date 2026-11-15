// Package xtools ports Agno's XTools: posting, replying, direct messages,
// user lookups, home timeline and recent search on X (Twitter) API v2.
//
// Like tweepy, write and "me" endpoints use OAuth 1.0a user context (signed
// with HMAC-SHA1) and public read endpoints use the app bearer token.
package xtools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the X API v2 base URL.
const DefaultBaseURL = "https://api.x.com/2"

// maxRateLimitWait bounds the sleep when WithWaitOnRateLimit is enabled.
const maxRateLimitWait = 15 * time.Minute

// XTools interacts with X (Twitter).
type XTools struct {
	toolkit.Toolkit
	bearerToken        string
	oauth              oauth1Credentials
	includePostMetrics bool
	waitOnRateLimit    bool
	baseURL            string
	httpClient         *http.Client

	// test hooks
	now   func() time.Time
	nonce func() string
	sleep func(time.Duration)
}

// Option configures XTools.
type Option func(*XTools)

// WithBearerToken sets the app bearer token (default: X_BEARER_TOKEN).
func WithBearerToken(s string) Option { return func(x *XTools) { x.bearerToken = s } }

// WithConsumerKey sets the consumer key (default: X_CONSUMER_KEY).
func WithConsumerKey(s string) Option { return func(x *XTools) { x.oauth.consumerKey = s } }

// WithConsumerSecret sets the consumer secret (default: X_CONSUMER_SECRET).
func WithConsumerSecret(s string) Option { return func(x *XTools) { x.oauth.consumerSecret = s } }

// WithAccessToken sets the user access token (default: X_ACCESS_TOKEN).
func WithAccessToken(s string) Option { return func(x *XTools) { x.oauth.token = s } }

// WithAccessTokenSecret sets the user access token secret (default: X_ACCESS_TOKEN_SECRET).
func WithAccessTokenSecret(s string) Option { return func(x *XTools) { x.oauth.tokenSecret = s } }

// WithIncludePostMetrics adds engagement metrics to SearchPosts results.
func WithIncludePostMetrics(v bool) Option { return func(x *XTools) { x.includePostMetrics = v } }

// WithWaitOnRateLimit sleeps until the rate-limit reset (max 15m) and retries once on HTTP 429.
func WithWaitOnRateLimit(v bool) Option { return func(x *XTools) { x.waitOnRateLimit = v } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(x *XTools) { x.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(x *XTools) { x.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(x *XTools) { x.httpClient = c } }

// CreatePostParams are the parameters of CreatePost.
type CreatePostParams struct {
	Text string `json:"text" description:"The content of the post to create." required:"true"`
}

// ReplyToPostParams are the parameters of ReplyToPost.
type ReplyToPostParams struct {
	PostID string `json:"post_id" description:"The ID of the post to reply to." required:"true"`
	Text   string `json:"text" description:"The content of the reply post." required:"true"`
}

// SendDmParams are the parameters of SendDm.
type SendDmParams struct {
	Recipient string `json:"recipient" description:"The username or user ID of the recipient." required:"true"`
	Text      string `json:"text" description:"The content of the direct message." required:"true"`
}

// GetUserInfoParams are the parameters of GetUserInfo.
type GetUserInfoParams struct {
	Username string `json:"username" description:"The username of the user to fetch information about." required:"true"`
}

// GetHomeTimelineParams are the parameters of GetHomeTimeline.
type GetHomeTimelineParams struct {
	MaxResults int `json:"max_results,omitempty" description:"The maximum number of tweets to retrieve. Default is 10."`
}

// SearchPostsParams are the parameters of SearchPosts.
type SearchPostsParams struct {
	Query      string `json:"query" description:"The search query." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"The maximum number of posts to retrieve (10-100). Default is 10."`
}

// New creates the X toolkit.
func New(opts ...Option) *XTools {
	x := &XTools{baseURL: DefaultBaseURL, now: time.Now, nonce: newNonce, sleep: time.Sleep}
	x.Toolkit = toolkit.NewToolkit()
	x.Toolkit.Name = "x"
	x.Toolkit.Description = "Post, reply, send direct messages and search on X (Twitter)."
	for _, o := range opts {
		o(x)
	}
	x.bearerToken = restkit.Env(x.bearerToken, "X_BEARER_TOKEN")
	x.oauth.consumerKey = restkit.Env(x.oauth.consumerKey, "X_CONSUMER_KEY")
	x.oauth.consumerSecret = restkit.Env(x.oauth.consumerSecret, "X_CONSUMER_SECRET")
	x.oauth.token = restkit.Env(x.oauth.token, "X_ACCESS_TOKEN")
	x.oauth.tokenSecret = restkit.Env(x.oauth.tokenSecret, "X_ACCESS_TOKEN_SECRET")
	if x.httpClient == nil {
		x.httpClient = x.Toolkit.HTTPClient()
	}
	x.Toolkit.Register("CreatePost", "Create a new X post. Returns JSON with the post URL or an error.", x, x.CreatePost, CreatePostParams{})
	x.Toolkit.Register("ReplyToPost", "Reply to an existing post. Returns JSON with the reply URL or an error.", x, x.ReplyToPost, ReplyToPostParams{})
	x.Toolkit.Register("SendDm", "Send a direct message to a user (username or numeric user ID).", x, x.SendDm, SendDmParams{})
	x.Toolkit.Register("GetUserInfo", "Retrieve profile information (id, name, description, follower counts) about a user.", x, x.GetUserInfo, GetUserInfoParams{})
	x.Toolkit.Register("GetHomeTimeline", "Retrieve the authenticated user's home timeline.", x, x.GetHomeTimeline, GetHomeTimelineParams{})
	x.Toolkit.Register("SearchPosts", "Search for recent posts matching a query.", x, x.SearchPosts, SearchPostsParams{})
	return x
}

// do performs an API call. userAuth selects OAuth 1.0a user context
// (otherwise the bearer token, falling back to user context like tweepy).
func (x *XTools) do(method, path string, query url.Values, body interface{}, userAuth bool) (map[string]interface{}, error) {
	u := strings.TrimRight(x.baseURL, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	useOAuth := userAuth || x.bearerToken == ""
	if useOAuth && (x.oauth.consumerKey == "" || x.oauth.consumerSecret == "" || x.oauth.token == "" || x.oauth.tokenSecret == "") {
		if userAuth {
			return nil, errors.New("X_CONSUMER_KEY, X_CONSUMER_SECRET, X_ACCESS_TOKEN and X_ACCESS_TOKEN_SECRET are required for this action")
		}
		return nil, errors.New("X_BEARER_TOKEN (or OAuth 1.0a user credentials) is required")
	}
	for attempt := 0; ; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequest(method, u, reader)
		if err != nil {
			return nil, err
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if useOAuth {
			auth, err := x.oauth.authorization(method, u, nil, x.nonce(), x.now())
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", auth)
		} else {
			req.Header.Set("Authorization", "Bearer "+x.bearerToken)
		}
		resp, err := x.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("X API request failed: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, restkit.MaxBodyBytes))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests && x.waitOnRateLimit && attempt == 0 {
			wait := time.Second
			if reset, err := strconv.ParseInt(resp.Header.Get("x-rate-limit-reset"), 10, 64); err == nil {
				wait = time.Unix(reset, 0).Sub(x.now()) + time.Second
			}
			if wait > maxRateLimitWait {
				wait = maxRateLimitWait
			}
			if wait > 0 {
				x.sleep(wait)
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &restkit.APIError{Name: "X API", Status: resp.StatusCode, Body: string(data)}
		}
		out, _ := restkit.Decode(data).(map[string]interface{})
		return out, nil
	}
}

func errJSON(msg string) string { return restkit.CompactJSON(map[string]string{"error": msg}) }

func (x *XTools) me(fields string) (map[string]interface{}, error) {
	var q url.Values
	if fields != "" {
		q = url.Values{"user.fields": {fields}}
	}
	resp, err := x.do(http.MethodGet, "/users/me", q, nil, true)
	if err != nil {
		return nil, err
	}
	return restkit.Map(resp, "data"), nil
}

func (x *XTools) tweet(text, replyTo string) (string, error) {
	body := map[string]interface{}{"text": text}
	if replyTo != "" {
		body["reply"] = map[string]string{"in_reply_to_tweet_id": replyTo}
	}
	resp, err := x.do(http.MethodPost, "/tweets", nil, body, true)
	if err != nil {
		return "", err
	}
	return restkit.Str(restkit.Map(resp, "data"), "id"), nil
}

// CreatePost publishes a post and returns {"message", "url"}.
func (x *XTools) CreatePost(params CreatePostParams) (interface{}, error) {
	id, err := x.tweet(params.Text, "")
	if err != nil {
		return errJSON(err.Error()), nil
	}
	user, err := x.me("")
	if err != nil {
		return errJSON(err.Error()), nil
	}
	return restkit.JSONString(map[string]string{
		"message": "Post successfully created!",
		"url":     fmt.Sprintf("https://x.com/%s/status/%s", restkit.Str(user, "username"), id),
	}), nil
}

// ReplyToPost replies to a post and returns {"message", "url"}.
func (x *XTools) ReplyToPost(params ReplyToPostParams) (interface{}, error) {
	id, err := x.tweet(params.Text, params.PostID)
	if err != nil {
		return errJSON(err.Error()), nil
	}
	user, err := x.me("")
	if err != nil {
		return errJSON(err.Error()), nil
	}
	return restkit.JSONString(map[string]string{
		"message": "Reply successfully posted!",
		"url":     fmt.Sprintf("https://twitter.com/%s/status/%s", restkit.Str(user, "username"), id),
	}), nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// SendDm sends a direct message to a username or user ID.
func (x *XTools) SendDm(params SendDmParams) (interface{}, error) {
	numeric := isDigits(params.Recipient)
	fail := func(err error) (interface{}, error) {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "User not found") || strings.Contains(msg, "Could not find user"):
			msg = fmt.Sprintf("User '%s' not found. Please check the username or user ID.", params.Recipient)
		case strings.Contains(msg, "You cannot send messages to this user"):
			msg = fmt.Sprintf("Unable to send message to '%s'. The user may have restricted who can send them messages.", params.Recipient)
		}
		return restkit.JSONString(map[string]string{"error": msg}), nil
	}
	recipientID := params.Recipient
	if !numeric {
		resp, err := x.do(http.MethodGet, "/users/by/username/"+url.PathEscape(params.Recipient), nil, nil, false)
		if err != nil {
			return fail(err)
		}
		recipientID = restkit.Str(restkit.Map(resp, "data"), "id")
		if recipientID == "" {
			return fail(errors.New("User not found"))
		}
	}
	resp, err := x.do(http.MethodPost, "/dm_conversations/with/"+url.PathEscape(recipientID)+"/messages", nil, map[string]string{"text": params.Text}, true)
	if err != nil {
		return fail(err)
	}
	data := restkit.Map(resp, "data")
	dmID := restkit.Str(data, "dm_event_id")
	if dmID == "" {
		dmID = restkit.Str(data, "id")
	}
	var username interface{}
	if !numeric {
		username = params.Recipient
	}
	return restkit.JSONString(map[string]interface{}{
		"message":            "Direct message sent successfully!",
		"dm_id":              dmID,
		"recipient_id":       recipientID,
		"recipient_username": username,
	}), nil
}

func userSummary(u map[string]interface{}) string {
	pm := restkit.Map(u, "public_metrics")
	return restkit.JSONString(map[string]interface{}{
		"id":              u["id"],
		"name":            u["name"],
		"username":        u["username"],
		"description":     u["description"],
		"followers_count": pm["followers_count"],
		"following_count": pm["following_count"],
		"tweet_count":     pm["tweet_count"],
	})
}

// GetMyInfo returns the authenticated user's profile (not registered as a
// tool, matching the Python toolkit).
func (x *XTools) GetMyInfo() (interface{}, error) {
	u, err := x.me("description,public_metrics")
	if err != nil {
		return errJSON(err.Error()), nil
	}
	return userSummary(u), nil
}

// GetUserInfo returns a user's profile summary.
func (x *XTools) GetUserInfo(params GetUserInfoParams) (interface{}, error) {
	resp, err := x.do(http.MethodGet, "/users/by/username/"+url.PathEscape(params.Username), url.Values{"user.fields": {"description,public_metrics"}}, nil, false)
	if err != nil {
		return errJSON(err.Error()), nil
	}
	return userSummary(restkit.Map(resp, "data")), nil
}

// pyID mimics tweepy's integer IDs.
func pyID(v interface{}) interface{} {
	if s, ok := v.(string); ok {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
	}
	return v
}

func pyDate(v interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02 15:04:05")
	}
	return s
}

// GetHomeTimeline returns {"home_timeline": [...]}.
func (x *XTools) GetHomeTimeline(params GetHomeTimelineParams) (interface{}, error) {
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}
	user, err := x.me("")
	if err != nil {
		return errJSON(err.Error()), nil
	}
	q := url.Values{"max_results": {strconv.Itoa(maxResults)}, "tweet.fields": {"created_at,public_metrics,author_id"}}
	resp, err := x.do(http.MethodGet, "/users/"+url.PathEscape(restkit.Str(user, "id"))+"/timelines/reverse_chronological", q, nil, true)
	if err != nil {
		return errJSON(err.Error()), nil
	}
	timeline := []map[string]interface{}{}
	for _, t := range restkit.Slice(resp, "data") {
		timeline = append(timeline, map[string]interface{}{
			"id":         pyID(t["id"]),
			"text":       t["text"],
			"created_at": pyDate(t["created_at"]),
			"author_id":  pyID(t["author_id"]),
		})
	}
	return restkit.JSONString(map[string]interface{}{"home_timeline": timeline}), nil
}

// SearchPosts searches recent posts and returns {"query", "count", "posts"}
// (or {} when nothing matches).
func (x *XTools) SearchPosts(params SearchPostsParams) (interface{}, error) {
	maxResults := params.MaxResults
	if maxResults < 10 {
		maxResults = 10
	}
	if maxResults > 100 {
		maxResults = 100
	}
	q := url.Values{
		"query":        {params.Query},
		"max_results":  {strconv.Itoa(maxResults)},
		"tweet.fields": {"author_id,created_at,id,public_metrics,text"},
		"user.fields":  {"name,username,verified"},
		"expansions":   {"author_id"},
	}
	resp, err := x.do(http.MethodGet, "/tweets/search/recent", q, nil, false)
	if err != nil {
		return restkit.CompactJSON(map[string]string{"error": err.Error(), "query": params.Query}), nil
	}
	users := map[string]map[string]interface{}{}
	for _, u := range restkit.Slice(restkit.Map(resp, "includes"), "users") {
		verified, _ := u["verified"].(bool)
		users[restkit.Str(u, "id")] = map[string]interface{}{
			"id": pyID(u["id"]), "name": u["name"], "username": u["username"], "verified": verified,
		}
	}
	data := restkit.Slice(resp, "data")
	if len(data) == 0 {
		return "{}", nil
	}
	posts := []map[string]interface{}{}
	for _, t := range data {
		authorID := restkit.Str(t, "author_id")
		author, ok := users[authorID]
		if !ok {
			author = map[string]interface{}{"id": pyID(t["author_id"]), "name": "Unknown", "username": "unknown"}
		}
		post := map[string]interface{}{
			"id":         pyID(t["id"]),
			"text":       t["text"],
			"created_at": pyDate(t["created_at"]),
			"author":     author,
			"url":        fmt.Sprintf("https://x.com/%v/status/%s", author["username"], restkit.Str(t, "id")),
		}
		if x.includePostMetrics {
			pm := restkit.Map(t, "public_metrics")
			metric := func(k string) interface{} {
				if v, ok := pm[k]; ok {
					return v
				}
				return 0
			}
			post["metrics"] = map[string]interface{}{
				"retweet_count": metric("retweet_count"),
				"reply_count":   metric("reply_count"),
				"like_count":    metric("like_count"),
				"quote_count":   metric("quote_count"),
			}
		}
		posts = append(posts, post)
	}
	return restkit.JSONString(map[string]interface{}{"query": params.Query, "count": len(posts), "posts": posts}), nil
}

// Execute implements toolkit.Tool.
func (x *XTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return x.Toolkit.Execute(methodName, input)
}
