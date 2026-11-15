package elasticsearch

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
	Method, Path, Query, Auth, ContentType string
	Body                                   []byte
}

type fakeES struct {
	mu       sync.Mutex
	reqs     []recorded
	handlers map[string]func(w http.ResponseWriter, r recorded)
}

func newFake(t *testing.T) (*fakeES, *httptest.Server) {
	f := &fakeES{handlers: map[string]func(http.ResponseWriter, recorded){}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec := recorded{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), r.Header.Get("Content-Type"), b}
		f.mu.Lock()
		f.reqs = append(f.reqs, rec)
		h := f.handlers[r.Method+" "+r.URL.Path]
		f.mu.Unlock()
		if h == nil {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"type":"index_not_found_exception"}}`))
			return
		}
		h(w, rec)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeES) on(key string, status int, body string) {
	f.handlers[key] = func(w http.ResponseWriter, _ recorded) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func (f *fakeES) last() recorded { return f.reqs[len(f.reqs)-1] }

func decode(t *testing.T, b []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("bad json %s: %v", b, err)
	}
	return m
}

func newDB(t *testing.T, srv *httptest.Server, cfg Config) *Elasticsearch {
	t.Helper()
	cfg.Addresses = []string{srv.URL}
	if cfg.IndexName == "" {
		cfg.IndexName = "docs"
	}
	if cfg.Embedder == nil {
		cfg.Embedder = testembed.New(4)
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateMappingAndAuth(t *testing.T) {
	f, srv := newFake(t)
	f.on("PUT /docs", 200, `{"acknowledged":true}`)
	db := newDB(t, srv, Config{APIKey: "abc", Distance: vectordb.DistanceL2})

	if err := db.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.reqs[0].Method != "HEAD" || f.reqs[0].Auth != "ApiKey abc" {
		t.Fatalf("expected HEAD with ApiKey auth, got %+v", f.reqs[0])
	}
	body := decode(t, f.last().Body)
	emb := body["mappings"].(map[string]interface{})["properties"].(map[string]interface{})["embedding"].(map[string]interface{})
	if emb["type"] != "dense_vector" || emb["dims"].(float64) != 4 || emb["similarity"] != "l2_norm" || emb["index"] != true {
		t.Fatalf("bad embedding mapping %v", emb)
	}
	if body["settings"].(map[string]interface{})["index"].(map[string]interface{})["number_of_replicas"].(float64) != 0 {
		t.Fatalf("bad settings %v", body["settings"])
	}
}

func TestBasicAuthExistsDropCount(t *testing.T) {
	f, srv := newFake(t)
	f.on("HEAD /docs", 200, "")
	f.on("DELETE /docs", 200, `{}`)
	f.on("GET /docs/_count", 200, `{"count":42}`)
	db := newDB(t, srv, Config{Username: "elastic", Password: "pw"})
	ctx := context.Background()

	if ok, err := db.Exists(ctx); err != nil || !ok {
		t.Fatalf("exists = %v %v", ok, err)
	}
	if f.last().Auth != "Basic ZWxhc3RpYzpwdw==" {
		t.Fatalf("auth = %q", f.last().Auth)
	}
	if n, err := db.GetCount(ctx); err != nil || n != 42 {
		t.Fatalf("count = %d %v", n, err)
	}
	if err := db.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	delete(f.handlers, "DELETE /docs")
	if err := db.Drop(ctx); err != nil {
		t.Fatalf("dropping a missing index should not fail: %v", err)
	}
	if ok, _ := db.IDExists(ctx, "nope"); ok {
		t.Fatal("expected missing id")
	}
}

func readBulk(t *testing.T, b []byte) []map[string]interface{} {
	var lines []map[string]interface{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		lines = append(lines, decode(t, sc.Bytes()))
	}
	return lines
}

func TestUpsertAndInsertBulkBodies(t *testing.T) {
	f, srv := newFake(t)
	f.on("POST /_bulk", 200, `{"errors":false,"items":[]}`)
	db := newDB(t, srv, Config{})
	ctx := context.Background()
	docs := []*document.Document{
		{ID: "a", Content: "alpha", Name: "doc-a", Metadata: map[string]interface{}{"lang": "en"}},
		{Content: "beta"},
	}

	if err := db.Upsert(ctx, docs, map[string]interface{}{"tenant": "t1"}); err != nil {
		t.Fatal(err)
	}
	r := f.last()
	if r.ContentType != "application/x-ndjson" || r.Query != "refresh=true" {
		t.Fatalf("bad bulk request %+v", r)
	}
	lines := readBulk(t, r.Body)
	if len(lines) != 4 {
		t.Fatalf("expected 4 ndjson lines, got %d", len(lines))
	}
	meta := lines[0]["update"].(map[string]interface{})
	if meta["_id"] != "a" || meta["_index"] != "docs" {
		t.Fatalf("bad action %v", lines[0])
	}
	src := lines[1]["doc"].(map[string]interface{})
	if lines[1]["doc_as_upsert"] != true || src["content"] != "alpha" || src["name"] != "doc-a" ||
		len(src["embedding"].([]interface{})) != 4 {
		t.Fatalf("bad upsert doc %v", lines[1])
	}
	md := src["meta_data"].(map[string]interface{})
	if md["lang"] != "en" || md["tenant"] != "t1" {
		t.Fatalf("filters not merged into meta_data: %v", md)
	}
	if id := lines[2]["update"].(map[string]interface{})["_id"]; id != DocumentID(&document.Document{Content: "beta"}) || docs[1].ID != id {
		t.Fatalf("content-derived id not used: %v / %q", id, docs[1].ID)
	}

	if err := db.Insert(ctx, docs[:1], nil); err != nil {
		t.Fatal(err)
	}
	lines = readBulk(t, f.last().Body)
	if _, ok := lines[0]["index"]; !ok || lines[1]["content"] != "alpha" {
		t.Fatalf("insert should use bulk index action: %v", lines)
	}

	f.on("POST /_bulk", 200, `{"errors":true,"items":[{"index":{"_id":"a","status":400,"error":{"type":"mapper_parsing_exception"}}}]}`)
	if err := db.Insert(ctx, docs[:1], nil); err == nil || !strings.Contains(err.Error(), "mapper_parsing_exception") {
		t.Fatalf("expected bulk item error, got %v", err)
	}
}

const hitsBody = `{"hits":{"total":{"value":1},"hits":[{"_id":"a","_score":0.9,"_source":{"content":"alpha","name":"doc-a","meta_data":{"lang":"en"}}}]}}`

func TestVectorKeywordHybridQueries(t *testing.T) {
	f, srv := newFake(t)
	f.on("POST /docs/_search", 200, hitsBody)
	db := newDB(t, srv, Config{})
	ctx := context.Background()
	filters := map[string]interface{}{"lang": "en"}

	res, err := db.VectorSearch(ctx, "alpha", 3, filters)
	if err != nil || len(res) != 1 || res[0].Document.ID != "a" || res[0].Score != 0.9 || res[0].Document.Metadata["lang"] != "en" {
		t.Fatalf("vector search = %+v %v", res, err)
	}
	body := decode(t, f.last().Body)
	knn := body["knn"].(map[string]interface{})
	if knn["field"] != "embedding" || knn["k"].(float64) != 3 || knn["num_candidates"].(float64) != 50 || len(knn["query_vector"].([]interface{})) != 4 {
		t.Fatalf("bad knn %v", knn)
	}
	term := knn["filter"].([]interface{})[0].(map[string]interface{})["term"].(map[string]interface{})
	if term["meta_data.lang.keyword"] != "en" {
		t.Fatalf("bad pre-filter %v", knn["filter"])
	}

	if _, err := db.KeywordSearch(ctx, "alpha", 2, nil); err != nil {
		t.Fatal(err)
	}
	body = decode(t, f.last().Body)
	mm := body["query"].(map[string]interface{})["multi_match"].(map[string]interface{})
	if mm["query"] != "alpha" || mm["type"] != "best_fields" || body["size"].(float64) != 2 {
		t.Fatalf("bad keyword query %v", body)
	}

	if _, err := db.HybridSearch(ctx, "alpha", 2, nil); err != nil {
		t.Fatal(err)
	}
	body = decode(t, f.last().Body)
	if body["knn"].(map[string]interface{})["boost"].(float64) != 0.7 ||
		body["query"].(map[string]interface{})["multi_match"].(map[string]interface{})["boost"].(float64) != 0.3 {
		t.Fatalf("bad boosted hybrid %v", body)
	}

	db.SearchType = vectordb.SearchTypeKeyword
	if _, err := db.Search(ctx, "x", 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := decode(t, f.last().Body)["knn"]; ok {
		t.Fatal("Search should dispatch to keyword search")
	}
}

func TestHybridRRFFallsBackOnLicenceError(t *testing.T) {
	f, srv := newFake(t)
	calls := 0
	f.handlers["POST /docs/_search"] = func(w http.ResponseWriter, r recorded) {
		calls++
		if _, ok := decode(t, r.Body)["retriever"]; ok {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":{"type":"security_exception","reason":"current license is non-compliant for [rrf]"}}`))
			return
		}
		_, _ = w.Write([]byte(hitsBody))
	}
	db := newDB(t, srv, Config{HybridStrategy: HybridRRF})
	res, err := db.HybridSearch(context.Background(), "alpha", 3, nil)
	if err != nil || len(res) != 1 || calls != 2 {
		t.Fatalf("fallback failed: res=%v err=%v calls=%d", res, err, calls)
	}
	first := decode(t, f.reqs[0].Body)["retriever"].(map[string]interface{})["rrf"].(map[string]interface{})
	if first["rank_window_size"].(float64) != 10 || len(first["retrievers"].([]interface{})) != 2 {
		t.Fatalf("bad rrf body %v", first)
	}
	if db.currentStrategy() != HybridBoost {
		t.Fatal("strategy should stick to boost after licence refusal")
	}
}

func TestSearchMissingIndexAndNameExists(t *testing.T) {
	f, srv := newFake(t)
	db := newDB(t, srv, Config{})
	res, err := db.KeywordSearch(context.Background(), "x", 1, nil)
	if err != nil || len(res) != 0 {
		t.Fatalf("missing index should yield no results: %v %v", res, err)
	}
	f.on("POST /docs/_search", 200, hitsBody)
	ok, err := db.NameExists(context.Background(), "doc-a")
	if err != nil || !ok {
		t.Fatalf("name exists = %v %v", ok, err)
	}
	q := decode(t, f.last().Body)["query"].(map[string]interface{})["term"].(map[string]interface{})
	if q["name.keyword"] != "doc-a" {
		t.Fatalf("bad name query %v", q)
	}
}

func TestBuildFilterConditions(t *testing.T) {
	got := BuildFilterConditions(map[string]interface{}{
		"tags": []string{"a", "b"},
		"year": map[string]interface{}{"gte": 2020, "lt": 2025},
		"page": 3,
		"kind": map[string]interface{}{"$in": []interface{}{"x"}},
	})
	b, _ := json.Marshal(got)
	s := string(b)
	for _, want := range []string{
		`{"terms":{"meta_data.tags.keyword":["a","b"]}}`,
		`{"range":{"meta_data.year":{"gte":2020,"lt":2025}}}`,
		`{"term":{"meta_data.page":3}}`,
		`{"terms":{"meta_data.kind.keyword":["x"]}}`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
}

func TestAddressFailover(t *testing.T) {
	f, srv := newFake(t)
	f.on("HEAD /docs", 200, "")
	db, _ := New(Config{Addresses: []string{"http://127.0.0.1:1", srv.URL}, IndexName: "docs"})
	if ok, err := db.Exists(context.Background()); err != nil || !ok {
		t.Fatalf("failover failed: %v %v", ok, err)
	}
}
