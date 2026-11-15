package couchbase

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
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/internal/testembed"
)

type recorded struct {
	method, path string
	body         map[string]interface{}
	user         string
}

type fakeCluster struct {
	mu       sync.Mutex
	reqs     []recorded
	indexHit bool // GET index returns 200
}

func (f *fakeCluster) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("bad json: %s", raw)
			}
		}
		user, _, _ := r.BasicAuth()
		f.mu.Lock()
		f.reqs = append(f.reqs, recorded{r.Method, r.URL.Path, body, user})
		f.mu.Unlock()

		switch {
		case r.URL.Path == "/query/service":
			stmt, _ := body["statement"].(string)
			switch {
			case strings.Contains(stmt, "USE KEYS $ids") && strings.HasPrefix(stmt, "SELECT"):
				_, _ = w.Write([]byte(`{"status":"success","results":[{"id":"d1","name":"n1","content":"hello","meta_data":{"k":"v"},"embedding":[0.1],"source":"src"}]}`))
			case strings.Contains(stmt, "COUNT(*)"):
				_, _ = w.Write([]byte(`{"status":"success","results":[3]}`))
			case strings.Contains(stmt, "system:keyspaces"):
				_, _ = w.Write([]byte(`{"status":"success","results":["docs"]}`))
			default:
				_, _ = w.Write([]byte(`{"status":"success","results":[]}`))
			}
		case strings.HasSuffix(r.URL.Path, "/query"):
			_, _ = w.Write([]byte(`{"hits":[{"id":"d1","score":0.8},{"id":"missing","score":0.1}]}`))
		case r.Method == http.MethodGet:
			if f.indexHit {
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			} else {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"index not found"}`))
			}
		case r.Method == http.MethodPut:
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func setup(t *testing.T, st vectordb.SearchType) (*Couchbase, *fakeCluster) {
	t.Helper()
	f := &fakeCluster{}
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	db, err := New(Config{
		QueryURL: srv.URL, SearchURL: srv.URL, Username: "admin", Password: "pw",
		Bucket: "b", Scope: "s", Collection: "docs", Embedder: testembed.New(4), SearchType: st,
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, f
}

func TestCreate(t *testing.T) {
	db, f := setup(t, "")
	if err := db.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	var stmts []string
	var put *recorded
	for i, r := range f.reqs {
		if r.path == "/query/service" {
			stmts = append(stmts, r.body["statement"].(string))
		}
		if r.method == http.MethodPut {
			put = &f.reqs[i]
		}
		if r.user != "admin" {
			t.Errorf("missing basic auth on %s", r.path)
		}
	}
	want := []string{
		"CREATE SCOPE `b`.`s` IF NOT EXISTS",
		"CREATE COLLECTION `b`.`s`.`docs` IF NOT EXISTS",
		"CREATE PRIMARY INDEX IF NOT EXISTS ON `b`.`s`.`docs`",
	}
	if strings.Join(stmts, "|") != strings.Join(want, "|") {
		t.Fatalf("statements: %v", stmts)
	}
	if put == nil || put.path != "/api/bucket/b/scope/s/index/docs_vector_index" {
		t.Fatalf("index PUT missing: %+v", put)
	}
	b, _ := json.Marshal(put.body)
	for _, s := range []string{`"type":"vector"`, `"dims":4`, `"similarity":"dot_product"`, `"s.docs"`} {
		if !strings.Contains(string(b), s) {
			t.Errorf("index def missing %s: %s", s, b)
		}
	}
}

func TestCreateSkipsExistingIndex(t *testing.T) {
	db, f := setup(t, "")
	f.indexHit = true
	if err := db.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, r := range f.reqs {
		if r.method == http.MethodPut {
			t.Fatal("index should not be recreated")
		}
	}
}

func TestInsert(t *testing.T) {
	db, f := setup(t, "")
	docs := []*document.Document{
		{ID: "d1", Name: "n1", Content: "hello", Metadata: map[string]interface{}{"k": "v"}},
		{Name: "n2", Content: "world"},
	}
	if err := db.Insert(context.Background(), docs, map[string]interface{}{"tenant": "a"}); err != nil {
		t.Fatal(err)
	}
	body := f.reqs[0].body
	if body["statement"] != "UPSERT INTO `b`.`s`.`docs` (KEY, VALUE) VALUES ($k0, $v0), VALUES ($k1, $v1)" {
		t.Fatalf("statement: %v", body["statement"])
	}
	if body["$k0"] != "d1" || body["$k1"] != "7d793037a0760186574b0282f2f435e7" {
		t.Errorf("keys: %v %v", body["$k0"], body["$k1"])
	}
	v0 := body["$v0"].(map[string]interface{})
	md := v0["meta_data"].(map[string]interface{})
	if v0["content"] != "hello" || md["k"] != "v" || md["tenant"] != "a" || len(v0["embedding"].([]interface{})) != 4 {
		t.Errorf("value: %v", v0)
	}
}

func TestVectorSearch(t *testing.T) {
	db, f := setup(t, "")
	res, err := db.Search(context.Background(), "hello", 2, map[string]interface{}{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	fts := f.reqs[0]
	if fts.path != "/api/bucket/b/scope/s/index/docs_vector_index/query" {
		t.Fatalf("path: %s", fts.path)
	}
	knn := fts.body["knn"].([]interface{})[0].(map[string]interface{})
	if knn["field"] != "embedding" || knn["k"].(float64) != 2 || len(knn["vector"].([]interface{})) != 4 {
		t.Errorf("knn: %v", knn)
	}
	if _, ok := knn["filter"].(map[string]interface{})["conjuncts"]; !ok {
		t.Errorf("filter: %v", knn["filter"])
	}
	if _, ok := fts.body["query"].(map[string]interface{})["match_none"]; !ok {
		t.Errorf("query: %v", fts.body["query"])
	}
	fetch := f.reqs[1].body
	if !strings.Contains(fetch["statement"].(string), "USE KEYS $ids") {
		t.Errorf("fetch: %v", fetch)
	}
	if len(res) != 1 || res[0].Document.ID != "d1" || res[0].Score != 0.8 || res[0].Document.Source != "src" {
		t.Errorf("results: %+v", res)
	}
}

func TestKeywordAndHybrid(t *testing.T) {
	db, f := setup(t, vectordb.SearchTypeHybrid)
	if _, err := db.KeywordSearch(context.Background(), "hello", 3, nil); err != nil {
		t.Fatal(err)
	}
	q := f.reqs[0].body["query"].(map[string]interface{})
	if q["match"] != "hello" || q["field"] != "content" || f.reqs[0].body["knn"] != nil {
		t.Errorf("keyword body: %v", f.reqs[0].body)
	}
	f.reqs = nil
	if _, err := db.Search(context.Background(), "hello", 3, nil); err != nil {
		t.Fatal(err)
	}
	if f.reqs[0].body["knn"] == nil || f.reqs[0].body["query"].(map[string]interface{})["match"] != "hello" {
		t.Errorf("hybrid body: %v", f.reqs[0].body)
	}
}

func TestCountsAndExists(t *testing.T) {
	db, f := setup(t, "")
	ctx := context.Background()
	if n, err := db.GetCount(ctx); err != nil || n != 3 {
		t.Fatalf("count %d %v", n, err)
	}
	if ok, err := db.NameExists(ctx, "n1"); err != nil || !ok {
		t.Fatalf("name exists %v %v", ok, err)
	}
	if f.reqs[1].body["$name"] != "n1" || f.reqs[1].body["scan_consistency"] != "request_plus" {
		t.Errorf("params: %v", f.reqs[1].body)
	}
	if ok, err := db.Exists(ctx); err != nil || !ok {
		t.Fatalf("exists %v %v", ok, err)
	}
}

func TestQueryError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"fatal","errors":[{"code":12003,"msg":"Keyspace not found"}]}`))
	}))
	defer srv.Close()
	db, _ := New(Config{QueryURL: srv.URL, Bucket: "b", Embedder: testembed.New(4)})
	if _, err := db.GetCount(context.Background()); err == nil || !strings.Contains(err.Error(), "Keyspace not found") {
		t.Fatalf("err: %v", err)
	}
}
