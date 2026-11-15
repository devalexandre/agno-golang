package webtools

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExpandUrl(t *testing.T) {
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.URL.Path == "/short" {
			http.Redirect(w, r, "/final/page", http.StatusMovedPermanently)
		}
	}))
	defer srv.Close()

	out, err := New().ExpandUrl(ExpandURLParams{URL: srv.URL + "/short"})
	if err != nil {
		t.Fatal(err)
	}
	if out != srv.URL+"/final/page" {
		t.Fatalf("unexpected: %v", out)
	}
	if len(methods) != 2 || methods[0] != http.MethodHead || methods[1] != http.MethodHead {
		t.Fatalf("expected two HEAD requests, got %v", methods)
	}
	bad := "http://127.0.0.1:1/unreachable"
	if out, _ := New(WithRetries(2)).ExpandUrl(ExpandURLParams{URL: bad}); out != bad {
		t.Fatalf("expected original URL, got %v", out)
	}
}
