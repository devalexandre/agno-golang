package cassandra

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/internal/testembed"
)

type call struct {
	stmt string
	args []interface{}
}

type fakeRows struct {
	rows []map[string]interface{}
	i    int
	err  error
}

func (f *fakeRows) MapScan(m map[string]interface{}) bool {
	if f.i >= len(f.rows) {
		return false
	}
	for k, v := range f.rows[f.i] {
		m[k] = v
	}
	f.i++
	return true
}
func (f *fakeRows) Close() error { return f.err }

type fakeSession struct {
	execs   []call
	queries []call
	// respond returns the rows for a query statement.
	respond func(stmt string) *fakeRows
}

func (f *fakeSession) Exec(_ context.Context, stmt string, args ...interface{}) error {
	f.execs = append(f.execs, call{stmt, args})
	return nil
}

func (f *fakeSession) Query(_ context.Context, stmt string, args ...interface{}) Rows {
	f.queries = append(f.queries, call{stmt, args})
	if f.respond != nil {
		return f.respond(stmt)
	}
	return &fakeRows{}
}

func newDB(t *testing.T, s Session, d vectordb.Distance) *Cassandra {
	t.Helper()
	db, err := New(s, Config{Keyspace: "ks", TableName: "docs", Embedder: testembed.New(4), Distance: d})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestNewValidation(t *testing.T) {
	if _, err := New(&fakeSession{}, Config{Keyspace: "ks; DROP", TableName: "t", Embedder: testembed.New(4)}); err == nil {
		t.Fatal("expected invalid identifier error")
	}
	if _, err := New(&fakeSession{}, Config{Keyspace: "ks", TableName: "t"}); err == nil {
		t.Fatal("expected missing embedder error")
	}
}

func TestCreate(t *testing.T) {
	s := &fakeSession{}
	if err := newDB(t, s, vectordb.DistanceDot).Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(s.execs) != 3 {
		t.Fatalf("want 3 statements, got %d", len(s.execs))
	}
	if !strings.Contains(s.execs[0].stmt, "CREATE TABLE IF NOT EXISTS ks.docs (row_id text PRIMARY KEY, body_blob text, vector vector<float, 4>, metadata_s map<text, text>, document_name text)") {
		t.Errorf("table stmt: %s", s.execs[0].stmt)
	}
	if !strings.Contains(s.execs[1].stmt, "'similarity_function': 'dot_product'") || !strings.Contains(s.execs[1].stmt, "StorageAttachedIndex") {
		t.Errorf("index stmt: %s", s.execs[1].stmt)
	}
}

func TestInsert(t *testing.T) {
	s := &fakeSession{}
	db := newDB(t, s, "")
	doc := &document.Document{ID: "d1", Name: "n1", Content: "hello", Source: "src", Metadata: map[string]interface{}{"page": 2}}
	if err := db.Insert(context.Background(), []*document.Document{doc}, map[string]interface{}{"tenant": "a"}); err != nil {
		t.Fatal(err)
	}
	c := s.execs[0]
	if c.stmt != "INSERT INTO ks.docs (row_id, body_blob, vector, metadata_s, document_name) VALUES (?, ?, ?, ?, ?)" {
		t.Fatalf("stmt: %s", c.stmt)
	}
	if c.args[0] != "d1" || c.args[1] != "hello" || c.args[4] != "n1" {
		t.Errorf("args: %v", c.args)
	}
	if v, ok := c.args[2].([]float32); !ok || len(v) != 4 {
		t.Errorf("vector arg: %#v", c.args[2])
	}
	md := c.args[3].(map[string]string)
	if md["page"] != "2" || md["tenant"] != "a" || md["source"] != "src" {
		t.Errorf("metadata: %v", md)
	}
}

func TestVectorSearch(t *testing.T) {
	s := &fakeSession{respond: func(string) *fakeRows {
		return &fakeRows{rows: []map[string]interface{}{
			{"row_id": "d1", "body_blob": "hello", "document_name": "n1", "metadata_s": map[string]string{"source": "src"}, "score": float32(0.9)},
		}}
	}}
	db := newDB(t, s, "")
	res, err := db.Search(context.Background(), "hello", 3, map[string]interface{}{"tenant": "a"})
	if err != nil {
		t.Fatal(err)
	}
	q := s.queries[0]
	want := "SELECT row_id, body_blob, metadata_s, document_name, similarity_cosine(vector, ?) AS score FROM ks.docs WHERE metadata_s['tenant'] = ? ORDER BY vector ANN OF ? LIMIT ?"
	if q.stmt != want {
		t.Fatalf("stmt:\n got %s\nwant %s", q.stmt, want)
	}
	if len(q.args) != 4 || q.args[1] != "a" || q.args[3] != 3 {
		t.Errorf("args: %v", q.args)
	}
	if len(res) != 1 || res[0].Document.ID != "d1" || res[0].Document.Source != "src" || res[0].Score < 0.89 {
		t.Errorf("results: %+v", res[0])
	}
}

func TestUnsupportedAndCounts(t *testing.T) {
	s := &fakeSession{respond: func(stmt string) *fakeRows {
		if strings.Contains(stmt, "COUNT") {
			return &fakeRows{rows: []map[string]interface{}{{"count": int64(2)}}}
		}
		return &fakeRows{err: errors.New("boom")}
	}}
	db := newDB(t, s, "")
	ctx := context.Background()
	if _, err := db.KeywordSearch(ctx, "x", 1, nil); !errors.Is(err, ErrUnsupportedSearch) {
		t.Errorf("keyword: %v", err)
	}
	if _, err := db.HybridSearch(ctx, "x", 1, nil); !errors.Is(err, ErrUnsupportedSearch) {
		t.Errorf("hybrid: %v", err)
	}
	if n, err := db.GetCount(ctx); err != nil || n != 2 {
		t.Errorf("count: %d %v", n, err)
	}
	if ok, err := db.IDExists(ctx, "d1"); err != nil || !ok {
		t.Errorf("id exists: %v %v", ok, err)
	}
	if ok, _ := db.NameExists(ctx, "n1"); !ok {
		t.Error("name exists")
	}
	if _, err := db.Exists(ctx); err == nil {
		t.Error("expected exists error")
	}
}
