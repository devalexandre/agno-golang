package brandfetch

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrandfetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/brands/nike.com":
			if r.Header.Get("Authorization") != "Bearer k" {
				t.Errorf("missing auth")
			}
			_, _ = w.Write([]byte(`{"name":"Nike"}`))
		case "/search/Apple":
			if r.URL.Query().Get("c") != "cid" {
				t.Errorf("missing client id")
			}
			_, _ = w.Write([]byte(`[{"brandId":"id_1"}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	tk := NewBrandfetchTools(WithAPIKey("k"), WithClientID("cid"), WithBaseURL(srv.URL), WithAll())
	if len(tk.GetMethods()) != 2 {
		t.Fatal("expected 2 methods")
	}
	out, _ := tk.SearchByIdentifier(SearchByIdentifierParams{Identifier: "nike.com"})
	if out.(map[string]interface{})["name"] != "Nike" {
		t.Fatalf("got %v", out)
	}
	out, _ = tk.SearchByIdentifier(SearchByIdentifierParams{Identifier: "nope"})
	if out.(map[string]interface{})["error"] != "Brand not found for identifier: nope" {
		t.Fatalf("got %v", out)
	}
	out, _ = tk.SearchByBrand(SearchByBrandParams{Name: "Apple"})
	if len(out.([]interface{})) != 1 {
		t.Fatalf("got %v", out)
	}
	if _, err := NewBrandfetchTools(WithAPIKey(""), WithBaseURL(srv.URL)).SearchByIdentifier(SearchByIdentifierParams{Identifier: "x"}); err == nil {
		t.Fatal("expected error without key")
	}
}
