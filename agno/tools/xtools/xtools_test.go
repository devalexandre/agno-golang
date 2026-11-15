package xtools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Vector from X's "Creating a signature" documentation.
func TestOAuth1Signature(t *testing.T) {
	c := oauth1Credentials{
		consumerKey:    "xvz1evFS4wEEPTGEFPHBog",
		consumerSecret: "kAcSOqF21Fu85e7zjz7ZN2U4ZRhfV3WpwPAoE3Z7kBw",
		token:          "370773112-GmHxMAgYyLbNEtIKZeRNFsMKPR9EyMZeS9weJAEb",
		tokenSecret:    "LswwdoUaIvS8ltyTt5jkRh4J50vUPVVHtR2YPi5kE",
	}
	auth, err := c.authorization("POST", "https://api.twitter.com/1.1/statuses/update.json?include_entities=true",
		url.Values{"status": {"Hello Ladies + Gentlemen, a signed OAuth request!"}},
		"kYjzVBB8Y0ZFabxSWbWovY3uYSQ2pTgmZeNu2VS4cg", time.Unix(1318622958, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(auth, `oauth_signature="hCtSmYh%2BiHYCEqBWrE7C7hYmtUk%3D"`) {
		t.Fatalf("bad signature: %s", auth)
	}
}

func newTestTools(srv *httptest.Server, extra ...Option) *XTools {
	opts := append([]Option{
		WithBaseURL(srv.URL), WithBearerToken("bt"), WithConsumerKey("ck"), WithConsumerSecret("cs"),
		WithAccessToken("at"), WithAccessTokenSecret("ats"),
	}, extra...)
	return New(opts...)
}

func TestCreatePostAndSearch(t *testing.T) {
	auths := map[string]string{}
	var tweetBody map[string]interface{}
	var searchQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths[r.Method+" "+r.URL.Path] = r.Header.Get("Authorization")
		switch r.Method + " " + r.URL.Path {
		case "POST /tweets":
			data, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(data, &tweetBody)
			_, _ = w.Write([]byte(`{"data":{"id":"111","text":"hi"}}`))
		case "GET /users/me":
			_, _ = w.Write([]byte(`{"data":{"id":"9","username":"gopher"}}`))
		case "GET /tweets/search/recent":
			searchQuery = r.URL.Query()
			_, _ = w.Write([]byte(`{"data":[{"id":"5","text":"go!","author_id":"9","created_at":"2024-01-02T03:04:05.000Z","public_metrics":{"like_count":3}}],"includes":{"users":[{"id":"9","name":"Gopher","username":"gopher","verified":true}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	x := newTestTools(srv, WithIncludePostMetrics(true))

	out, _ := x.ReplyToPost(ReplyToPostParams{PostID: "100", Text: "hi"})
	if !strings.Contains(out.(string), "https://twitter.com/gopher/status/111") {
		t.Fatalf("got %v", out)
	}
	if reply, _ := tweetBody["reply"].(map[string]interface{}); reply["in_reply_to_tweet_id"] != "100" || tweetBody["text"] != "hi" {
		t.Fatalf("bad body %v", tweetBody)
	}
	for _, k := range []string{"POST /tweets", "GET /users/me"} {
		if a := auths[k]; !strings.HasPrefix(a, "OAuth ") || !strings.Contains(a, `oauth_consumer_key="ck"`) || !strings.Contains(a, `oauth_token="at"`) || !strings.Contains(a, "oauth_signature=") {
			t.Fatalf("%s: bad auth %q", k, a)
		}
	}

	out, _ = x.SearchPosts(SearchPostsParams{Query: "golang", MaxResults: 500})
	if auths["GET /tweets/search/recent"] != "Bearer bt" || searchQuery.Get("max_results") != "100" || searchQuery.Get("query") != "golang" {
		t.Fatalf("bad search request %v %v", auths, searchQuery)
	}
	var res struct {
		Count int `json:"count"`
		Posts []struct {
			ID        int64                  `json:"id"`
			CreatedAt string                 `json:"created_at"`
			URL       string                 `json:"url"`
			Author    map[string]interface{} `json:"author"`
			Metrics   map[string]float64     `json:"metrics"`
		} `json:"posts"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	p := res.Posts[0]
	if res.Count != 1 || p.ID != 5 || p.CreatedAt != "2024-01-02 03:04:05" || p.URL != "https://x.com/gopher/status/5" || p.Author["name"] != "Gopher" || p.Metrics["like_count"] != 3 {
		t.Fatalf("bad result %s", out)
	}
}

func TestSendDmByUsername(t *testing.T) {
	var dmPath, dmAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/users/by/username/alice":
			_, _ = w.Write([]byte(`{"data":{"id":"42","username":"alice"}}`))
		case strings.HasPrefix(r.URL.Path, "/dm_conversations/"):
			dmPath, dmAuth = r.URL.Path, r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"data":{"dm_conversation_id":"c","dm_event_id":"e1"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	out, _ := newTestTools(srv).SendDm(SendDmParams{Recipient: "alice", Text: "yo"})
	if dmPath != "/dm_conversations/with/42/messages" || !strings.HasPrefix(dmAuth, "OAuth ") || !strings.Contains(out.(string), `"dm_id": "e1"`) || !strings.Contains(out.(string), `"recipient_username": "alice"`) {
		t.Fatalf("path=%s out=%v", dmPath, out)
	}
}

func TestMissingUserCredentials(t *testing.T) {
	for _, k := range []string{"X_CONSUMER_KEY", "X_CONSUMER_SECRET", "X_ACCESS_TOKEN", "X_ACCESS_TOKEN_SECRET"} {
		t.Setenv(k, "")
	}
	out, _ := New(WithBearerToken("bt"), WithBaseURL("http://127.0.0.1:1")).CreatePost(CreatePostParams{Text: "x"})
	if !strings.Contains(out.(string), "X_CONSUMER_KEY") {
		t.Fatalf("got %v", out)
	}
}
