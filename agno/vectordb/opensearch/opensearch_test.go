package opensearch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/internal/testembed"
)

type recorded struct {
	Method, Path, Query, Auth string
	Body                      []byte
}

type fake struct {
	mu   sync.Mutex
	reqs []recorded
	srv  *httptest.Server
	// handler returns status and body for a request.
	handler func(r recorded) (int, string)
}

func newFake(t *testing.T, h func(r recorded) (int, string)) *fake {
	f := &fake{handler: h}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec := recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Auth: r.Header.Get("Authorization"), Body: b}
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

func newDB(t *testing.T, f *fake, mod func(*Config)) *OpenSearch {
	cfg := Config{URL: f.srv.URL, IndexName: "docs", Embedder: testembed.New(4), Username: "admin", Password: "pw"}
	if mod != nil {
		mod(&cfg)
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func decode(t *testing.T, b []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("bad json %s: %v", b, err)
	}
	return m
}

func TestCreateMapping(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) {
		if r.Method == http.MethodHead {
			return 404, ""
		}
		return 200, `{"acknowledged":true}`
	})
	db := newDB(t, f, func(c *Config) {
		c.Engine = EngineFaiss
		c.Distance = vectordb.DistanceL2
		c.Parameters = map[string]interface{}{"ef_search": 100}
	})
	if err := db.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := f.last()
	if r.Method != http.MethodPut || r.Path != "/docs" {
		t.Fatalf("unexpected request %s %s", r.Method, r.Path)
	}
	if !strings.HasPrefix(r.Auth, "Basic ") {
		t.Fatalf("missing basic auth")
	}
	m := decode(t, r.Body)
	idx := m["settings"].(map[string]interface{})["index"].(map[string]interface{})
	if idx["knn"] != true || idx["knn.algo_param.ef_search"].(float64) != 100 {
		t.Fatalf("bad settings %v", idx)
	}
	emb := m["mappings"].(map[string]interface{})["properties"].(map[string]interface{})["embedding"].(map[string]interface{})
	if emb["type"] != "knn_vector" || emb["dimension"].(float64) != 4 {
		t.Fatalf("bad embedding mapping %v", emb)
	}
	method := emb["method"].(map[string]interface{})
	if method["engine"] != "faiss" || method["space_type"] != "l2" || method["name"] != "hnsw" {
		t.Fatalf("bad method %v", method)
	}
	params := method["parameters"].(map[string]interface{})
	if _, ok := params["ef_search"]; ok || params["m"].(float64) != 16 {
		t.Fatalf("bad params %v", params)
	}
}

func TestUnsupportedEngine(t *testing.T) {
	if _, err := New(Config{Engine: "bogus"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpsertBulk(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) { return 200, `{"errors":false,"items":[]}` })
	db := newDB(t, f, nil)
	docs := []*document.Document{
		{ID: "a", Content: "hello", Name: "n1", Metadata: map[string]interface{}{"content_hash": "h1"}},
		{Content: "world"},
	}
	if err := db.Upsert(context.Background(), docs, map[string]interface{}{"tag": "x"}); err != nil {
		t.Fatal(err)
	}
	r := f.last()
	if r.Path != "/_bulk" || r.Query != "refresh=true" {
		t.Fatalf("unexpected %s?%s", r.Path, r.Query)
	}
	sc := bufio.NewScanner(bytes.NewReader(r.Body))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var lines []map[string]interface{}
	for sc.Scan() {
		lines = append(lines, decode(t, sc.Bytes()))
	}
	if len(lines) != 4 {
		t.Fatalf("expected 4 ndjson lines, got %d", len(lines))
	}
	action := lines[0]["update"].(map[string]interface{})
	if action["_id"] != "a" || action["_index"] != "docs" {
		t.Fatalf("bad action %v", action)
	}
	payload := lines[1]
	if payload["doc_as_upsert"] != true {
		t.Fatalf("missing doc_as_upsert")
	}
	src := payload["doc"].(map[string]interface{})
	if src["content_hash"] != "h1" || src["name"] != "n1" || src["meta_data"].(map[string]interface{})["tag"] != "x" {
		t.Fatalf("bad source %v", src)
	}
	if len(src["embedding"].([]interface{})) != 4 {
		t.Fatalf("missing embedding")
	}
	if docs[1].ID == "" {
		t.Fatal("expected generated id")
	}
}

func TestBulkErrors(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) {
		return 200, `{"errors":true,"items":[{"index":{"_id":"a","error":{"type":"mapper_parsing_exception"}}}]}`
	})
	db := newDB(t, f, nil)
	err := db.Insert(context.Background(), []*document.Document{{ID: "a", Content: "x"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "mapper_parsing_exception") {
		t.Fatalf("expected bulk error, got %v", err)
	}
	if !bytes.Contains(f.last().Body, []byte(`"index"`)) {
		t.Fatal("insert should use index action")
	}
}

const hitsResp = `{"hits":{"total":{"value":1},"hits":[{"_id":"a","_score":0.9,"_source":{"content":"hello","name":"n1","meta_data":{"k":"v"}}}]}}`

func TestVectorSearchWithFilter(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) { return 200, hitsResp })
	db := newDB(t, f, nil)
	res, err := db.Search(context.Background(), "hello", 3, map[string]interface{}{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Document.ID != "a" || res[0].Score != 0.9 || res[0].Document.Metadata["k"] != "v" {
		t.Fatalf("bad results %+v", res[0])
	}
	r := f.last()
	if r.Path != "/docs/_search" {
		t.Fatalf("path %s", r.Path)
	}
	m := decode(t, r.Body)
	knn := m["query"].(map[string]interface{})["knn"].(map[string]interface{})["embedding"].(map[string]interface{})
	if knn["k"].(float64) != 3 || len(knn["vector"].([]interface{})) != 4 {
		t.Fatalf("bad knn %v", knn)
	}
	filter, _ := json.Marshal(knn["filter"])
	if !strings.Contains(string(filter), `"meta_data.k.keyword":"v"`) {
		t.Fatalf("filter not pushed into knn: %s", filter)
	}
}

func TestVectorSearchNmslibPostFilter(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) { return 200, hitsResp })
	db := newDB(t, f, func(c *Config) { c.Engine = EngineNmslib })
	if _, err := db.VectorSearch(context.Background(), "q", 2, map[string]interface{}{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	m := decode(t, f.last().Body)
	b := m["query"].(map[string]interface{})["bool"].(map[string]interface{})
	if b["must"] == nil || b["filter"] == nil {
		t.Fatalf("expected bool must+filter, got %v", b)
	}
}

func TestKeywordAndHybrid(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) { return 200, hitsResp })
	db := newDB(t, f, func(c *Config) { c.SearchType = vectordb.SearchTypeHybrid })

	if _, err := db.KeywordSearch(context.Background(), "hello", 2, nil); err != nil {
		t.Fatal(err)
	}
	m := decode(t, f.last().Body)
	mm := m["query"].(map[string]interface{})["multi_match"].(map[string]interface{})
	if mm["query"] != "hello" || mm["type"] != "best_fields" {
		t.Fatalf("bad multi_match %v", mm)
	}

	if _, err := db.Search(context.Background(), "hello", 2, map[string]interface{}{"n": map[string]interface{}{"gte": 1}}); err != nil {
		t.Fatal(err)
	}
	m = decode(t, f.last().Body)
	b := m["query"].(map[string]interface{})["bool"].(map[string]interface{})
	should := b["should"].([]interface{})
	knn := should[0].(map[string]interface{})["knn"].(map[string]interface{})["embedding"].(map[string]interface{})
	if knn["boost"].(float64) != 0.7 || knn["filter"] == nil {
		t.Fatalf("bad hybrid knn %v", knn)
	}
	if should[1].(map[string]interface{})["multi_match"].(map[string]interface{})["boost"].(float64) != 0.3 {
		t.Fatal("bad multi_match boost")
	}
	if b["filter"] == nil {
		t.Fatal("missing outer filter")
	}
}

func TestExistsCountAndChecks(t *testing.T) {
	f := newFake(t, func(r recorded) (int, string) {
		switch {
		case r.Method == http.MethodHead && r.Path == "/docs":
			return 200, ""
		case r.Method == http.MethodHead && r.Path == "/docs/_doc/a":
			return 200, ""
		case r.Method == http.MethodHead:
			return 404, ""
		case strings.HasSuffix(r.Path, "/_count"):
			return 200, `{"count":7}`
		case strings.HasSuffix(r.Path, "/_search"):
			return 200, hitsResp
		case r.Method == http.MethodDelete:
			return 404, `{}`
		}
		return 500, ""
	})
	db := newDB(t, f, nil)
	ctx := context.Background()
	if ok, err := db.Exists(ctx); err != nil || !ok {
		t.Fatalf("exists %v %v", ok, err)
	}
	if n, err := db.GetCount(ctx); err != nil || n != 7 {
		t.Fatalf("count %v %v", n, err)
	}
	if ok, _ := db.IDExists(ctx, "a"); !ok {
		t.Fatal("id a should exist")
	}
	if ok, _ := db.IDExists(ctx, "b"); ok {
		t.Fatal("id b should not exist")
	}
	if ok, _ := db.NameExists(ctx, "n1"); !ok {
		t.Fatal("name should exist")
	}
	if m := decode(t, f.last().Body); !strings.Contains(string(f.last().Body), `"name.keyword":"n1"`) || m["size"].(float64) != 1 {
		t.Fatalf("bad name query %s", f.last().Body)
	}
	if err := db.Drop(ctx); err != nil {
		t.Fatalf("drop of missing index should succeed: %v", err)
	}
}
