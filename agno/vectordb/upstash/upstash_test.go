package upstash

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb/internal/testembed"
)

type recorded struct {
	Method, Path, Auth string
	Body               []byte
}

type fake struct {
	mu      sync.Mutex
	reqs    []recorded
	srv     *httptest.Server
	handler func(r recorded) (int, string)
}

func newFake(t *testing.T, h func(r recorded) (int, string)) *fake {
	f := &fake{handler: h}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec := recorded{Method: r.Method, Path: r.URL.Path, Auth: r.Header.Get("Authorization"), Body: b}
		f.mu.Lock()
		f.reqs = append(f.reqs, rec)
		f.mu.Unlock()
		status, body := f.handler(rec)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fake) last() recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[len(f.reqs)-1]
}

func TestNewRequiresCredentials(t *testing.T) {
	t.Setenv("UPSTASH_VECTOR_REST_URL", "")
	t.Setenv("UPSTASH_VECTOR_REST_TOKEN", "")
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error")
	}
	t.Setenv("UPSTASH_VECTOR_REST_URL", "http://x")
	t.Setenv("UPSTASH_VECTOR_REST_TOKEN", "tok")
	db, err := New(Config{})
	if err != nil || db.cfg.URL != "http://x" || !db.cfg.UseUpstashEmbeddings {
		t.Fatalf("env fallback failed: %v %+v", err, db)
	}
}

func TestUpsertWithEmbedderAndNamespace(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) { return 200, `{"result":"Success"}` })
	db, err := New(Config{URL: f.srv.URL, Token: "tok", Namespace: "ns1", Embedder: testembed.New(4)})
	if err != nil {
		t.Fatal(err)
	}
	docs := []*document.Document{{ID: "a", Content: "hello", Name: "n1", Metadata: map[string]interface{}{"content_hash": "h"}}}
	if err := db.Upsert(context.Background(), docs, map[string]interface{}{"tag": "x"}); err != nil {
		t.Fatal(err)
	}
	r := f.last()
	if r.Method != http.MethodPost || r.Path != "/upsert/ns1" || r.Auth != "Bearer tok" {
		t.Fatalf("unexpected %s %s %s", r.Method, r.Path, r.Auth)
	}
	var recs []map[string]interface{}
	if err := json.Unmarshal(r.Body, &recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0]["id"] != "a" || len(recs[0]["vector"].([]interface{})) != 4 || recs[0]["data"] != "hello" {
		t.Fatalf("bad record %v", recs)
	}
	meta := recs[0]["metadata"].(map[string]interface{})
	if meta["tag"] != "x" || meta["text"] != "hello" || meta["name"] != "n1" || meta["content_hash"] != "h" {
		t.Fatalf("bad metadata %v", meta)
	}
}

func TestUpsertDataHosted(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) { return 200, `{"result":"Success"}` })
	db, _ := New(Config{URL: f.srv.URL, Token: "tok"})
	docs := []*document.Document{{Content: "hello"}}
	if err := db.Insert(context.Background(), docs, nil); err != nil {
		t.Fatal(err)
	}
	r := f.last()
	if r.Path != "/upsert-data" {
		t.Fatalf("path %s", r.Path)
	}
	if strings.Contains(string(r.Body), `"vector"`) {
		t.Fatal("hosted mode must not send vectors")
	}
	if docs[0].ID == "" {
		t.Fatal("expected md5 id")
	}
}

func TestQueryWithFilter(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) {
		return 200, `{"result":[{"id":"a","score":0.8,"data":"hello","metadata":{"name":"n1","text":"hello"},"vector":[1,0]}]}`
	})
	db, _ := New(Config{URL: f.srv.URL, Token: "tok", Embedder: testembed.New(4)})
	res, err := db.Search(context.Background(), "hello", 3, map[string]interface{}{"genre": "drama", "year": map[string]interface{}{"gte": 2000}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Document.ID != "a" || res[0].Document.Content != "hello" || res[0].Document.Name != "n1" || res[0].Score != 0.8 {
		t.Fatalf("bad result %+v", res[0].Document)
	}
	r := f.last()
	if r.Path != "/query" {
		t.Fatalf("path %s", r.Path)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(r.Body, &body)
	if body["topK"].(float64) != 3 || body["includeMetadata"] != true || body["includeData"] != true {
		t.Fatalf("bad body %v", body)
	}
	if body["filter"] != `genre = "drama" AND year >= 2000` {
		t.Fatalf("bad filter %q", body["filter"])
	}
	if len(body["vector"].([]interface{})) != 4 {
		t.Fatal("missing vector")
	}
}

func TestQueryDataHosted(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) { return 200, `{"result":[]}` })
	db, _ := New(Config{URL: f.srv.URL, Token: "tok", Namespace: "n"})
	if _, err := db.HybridSearch(context.Background(), "q", 2, nil); err != nil {
		t.Fatal(err)
	}
	r := f.last()
	var body map[string]interface{}
	_ = json.Unmarshal(r.Body, &body)
	if r.Path != "/query-data/n" || body["data"] != "q" {
		t.Fatalf("bad %s %v", r.Path, body)
	}
	if _, err := db.KeywordSearch(context.Background(), "q", 2, nil); err == nil {
		t.Fatal("keyword search should be unsupported")
	}
}

func TestFetchInfoResetDelete(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) {
		switch r.Path {
		case "/fetch/ns":
			if strings.Contains(string(r.Body), `"a"`) {
				return 200, `{"result":[{"id":"a"}]}`
			}
			return 200, `{"result":[null]}`
		case "/info":
			return 200, `{"result":{"vectorCount":10,"dimension":4,"namespaces":{"":{"vectorCount":4},"ns":{"vectorCount":6}}}}`
		case "/reset/ns", "/delete/ns":
			return 200, `{"result":"Success"}`
		}
		return 404, `{"error":"not found"}`
	})
	db, _ := New(Config{URL: f.srv.URL, Token: "tok", Namespace: "ns", Embedder: testembed.New(4)})
	ctx := context.Background()
	if ok, err := db.IDExists(ctx, "a"); err != nil || !ok {
		t.Fatalf("a should exist: %v", err)
	}
	if ok, _ := db.IDExists(ctx, "b"); ok {
		t.Fatal("b should not exist")
	}
	if n, err := db.GetCount(ctx); err != nil || n != 6 {
		t.Fatalf("count %d %v", n, err)
	}
	if ok, err := db.Exists(ctx); err != nil || !ok {
		t.Fatalf("exists %v %v", ok, err)
	}
	if err := db.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	if r := f.last(); r.Method != http.MethodDelete || r.Path != "/reset/ns" {
		t.Fatalf("bad drop %s %s", r.Method, r.Path)
	}
	if err := db.DeleteByFilter(ctx, map[string]interface{}{"name": "x"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(f.last().Body), `"filter":"name = \"x\""`) {
		t.Fatalf("bad delete body %s", f.last().Body)
	}
}

func TestBuildFilter(t *testing.T) {
	cases := map[string]map[string]interface{}{
		`a IN ("x", "y")`:                   {"a": []string{"x", "y"}},
		`a IN (1, 2)`:                       {"a": map[string]interface{}{"$in": []int{1, 2}}},
		`b = true`:                          {"b": true},
		`c = 'say "hi"'`:                    {"c": `say "hi"`},
		`(HAS FIELD d AND HAS NOT FIELD d)`: {"d": `"'`},
		`e > 1 AND e < 5`:                   {"e": map[string]interface{}{"gt": 1, "lt": 5}},
		``:                                  nil,
	}
	for want, in := range cases {
		if got := BuildFilter(in); got != want {
			t.Errorf("BuildFilter(%v) = %q, want %q", in, got, want)
		}
	}
}
