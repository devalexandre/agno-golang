package website

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
)

type fakeKB struct{ docs []document.Document }

func (f *fakeKB) LoadDocument(_ context.Context, d document.Document) error {
	f.docs = append(f.docs, d)
	return nil
}

func TestWebsite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><main><p>Home page</p><a href="/about">About</a><a href="/img.png">img</a></main></body></html>`))
		case "/about":
			_, _ = w.Write([]byte(`<html><body><p>About us</p></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tk := New()
	out, err := tk.ReadUrl(URLParams{URL: srv.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	var docs []struct {
		Name     string            `json:"name"`
		MetaData map[string]string `json:"meta_data"`
		Content  string            `json:"content"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || docs[1].MetaData["url"] != srv.URL+"/about" || docs[1].Content != "About us" {
		t.Fatalf("unexpected docs: %s", out)
	}

	kb := &fakeKB{}
	k := New(WithKnowledge(kb), WithMaxLinks(1))
	if res, err := k.AddWebsiteToKnowledge(URLParams{URL: srv.URL + "/"}); err != nil || res != "Success" || len(kb.docs) != 1 {
		t.Fatalf("unexpected: %v %v %d", res, err, len(kb.docs))
	}
	if _, ok := k.GetMethods()["website_tools_AddWebsiteToKnowledge"]; !ok || len(k.GetMethods()) != 1 {
		t.Fatalf("unexpected methods: %v", k.GetMethods())
	}
}
