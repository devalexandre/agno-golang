package linkup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebSearchWithLinkup(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"results":[{"type":"text","name":"Go","url":"https://go.dev","content":"Go"}]}`))
	}))
	defer srv.Close()

	l := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithDepth("deep"))
	out, err := l.WebSearchWithLinkup(WebSearchParams{Query: "golang"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/search" || gotAuth != "Bearer k" {
		t.Fatalf("unexpected request: %s %s %s", gotMethod, gotPath, gotAuth)
	}
	if gotBody["q"] != "golang" || gotBody["depth"] != "deep" || gotBody["outputType"] != "searchResults" {
		t.Fatalf("unexpected body: %v", gotBody)
	}
	if !strings.Contains(out.(string), "https://go.dev") {
		t.Fatalf("unexpected output: %s", out)
	}
	if len(l.GetMethods()) != 1 {
		t.Fatalf("expected 1 method")
	}
}
