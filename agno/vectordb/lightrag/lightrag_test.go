package lightrag

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
)

type call struct {
	method, path, auth string
	body               map[string]interface{}
}

func newServer(t *testing.T, calls *[]call, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := call{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization")}
		b, _ := io.ReadAll(r.Body)
		if len(b) > 0 {
			_ = json.Unmarshal(b, &c.body)
		}
		*calls = append(*calls, c)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestInsertAndSearch(t *testing.T) {
	var calls []call
	srv := newServer(t, &calls, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/documents/text":
			_, _ = w.Write([]byte(`{"status":"success","track_id":"trk-1"}`))
		case "/query":
			_, _ = w.Write([]byte(`{"response":"Paris is the capital","references":[{"reference_id":"1","file_path":"geo.txt"}]}`))
		}
	})
	l := New(Config{ServerURL: srv.URL, APIKey: "k", AuthHeaderName: "Authorization", AuthHeaderFormat: "Bearer {api_key}"})
	ctx := context.Background()
	doc := &document.Document{Name: "geo.txt", Content: "Paris is the capital of France"}
	if err := l.Insert(ctx, []*document.Document{doc, {Content: "  "}}, nil); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].body["file_source"] != "geo.txt" || calls[0].body["text"] != doc.Content {
		t.Fatalf("unexpected insert calls %+v", calls)
	}
	if calls[0].auth != "Bearer k" {
		t.Errorf("auth header = %q", calls[0].auth)
	}
	if doc.Metadata["lightrag_track_id"] != "trk-1" {
		t.Errorf("track id not recorded: %v", doc.Metadata)
	}

	res, err := l.Search(ctx, "capital?", 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	q := calls[1]
	if q.path != "/query" || q.body["mode"] != "hybrid" || q.body["query"] != "capital?" || q.body["include_references"] != true {
		t.Fatalf("unexpected query body %+v", q)
	}
	if len(res) != 1 || res[0].Document.Content != "Paris is the capital" || res[0].Document.Metadata["references"] == nil {
		t.Fatalf("unexpected results %+v", res[0].Document)
	}

	if _, err := l.VectorSearch(ctx, "x", 1, nil); err != nil {
		t.Fatal(err)
	}
	if calls[2].body["mode"] != "naive" {
		t.Errorf("vector search mode = %v", calls[2].body["mode"])
	}
}

func TestListResponseAndDrop(t *testing.T) {
	var calls []call
	srv := newServer(t, &calls, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/query" {
			_, _ = w.Write([]byte(`[{"content":"a","metadata":{"k":"v"}},"b","c"]`))
		}
	})
	l := New(Config{ServerURL: srv.URL + "/", APIKey: "secret"})
	ctx := context.Background()
	res, err := l.Search(ctx, "q", 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Document.Metadata["k"] != "v" || res[1].Document.Content != "b" {
		t.Fatalf("unexpected list results %+v", res)
	}
	if err := l.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	if calls[1].method != http.MethodDelete || calls[1].path != "/documents" || calls[2].path != "/documents/clear_cache" {
		t.Fatalf("unexpected drop calls %+v", calls[1:])
	}
	if err := l.DeleteByExternalID(ctx, "doc-1"); err != nil {
		t.Fatal(err)
	}
	last := calls[len(calls)-1]
	if last.path != "/documents/delete_document" || last.body["delete_file"] != false {
		t.Fatalf("unexpected delete call %+v", last)
	}
}

func TestErrorStatus(t *testing.T) {
	var calls []call
	srv := newServer(t, &calls, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "secret" {
			t.Errorf("missing X-API-KEY header")
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	l := New(Config{ServerURL: srv.URL, APIKey: "secret"})
	if _, err := l.Search(context.Background(), "q", 1, nil); err == nil {
		t.Fatal("expected error")
	}
}
