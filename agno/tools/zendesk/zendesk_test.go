package zendesk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchZendesk(t *testing.T) {
	var user, pass, path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ = r.BasicAuth()
		path, query = r.URL.Path, r.URL.Query().Get("query")
		_, _ = w.Write([]byte(`{"results":[{"body":"<p>Reset your <b>password</b></p>"}]}`))
	}))
	defer srv.Close()

	z := New(WithUsername("me@x.com/token"), WithPassword("secret"), WithBaseURL(srv.URL))
	out, err := z.SearchZendesk(SearchParams{SearchString: "reset password"})
	if err != nil {
		t.Fatal(err)
	}
	if user != "me@x.com/token" || pass != "secret" || path != "/api/v2/help_center/articles/search.json" || query != "reset password" {
		t.Fatalf("bad request: %s %s %s %s", user, pass, path, query)
	}
	if out != `["Reset your password"]` {
		t.Fatalf("bad output: %v", out)
	}
}

func TestSearchZendeskMissingCreds(t *testing.T) {
	t.Setenv("ZENDESK_USERNAME", "")
	t.Setenv("ZENDESK_PASSWORD", "")
	out, _ := New(WithEnableSearchZendesk(false)).SearchZendesk(SearchParams{SearchString: "x"})
	if out != "Username, password, or company name not provided." {
		t.Fatalf("got %v", out)
	}
	if len(New(WithEnableSearchZendesk(false)).GetMethods()) != 0 || len(New(WithEnableSearchZendesk(false), WithAll()).GetMethods()) != 1 {
		t.Fatal("bad enable flags")
	}
}
