package reranker

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// fakeEmbedder returns a fixed vector per text.
type fakeEmbedder struct {
	vectors map[string][]float64
	calls   int
}

func (f *fakeEmbedder) GetEmbedding(text string) ([]float64, error) {
	f.calls++
	if v, ok := f.vectors[text]; ok {
		return v, nil
	}
	return nil, errors.New("unknown text")
}
func (f *fakeEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	v, err := f.GetEmbedding(text)
	return v, nil, err
}
func (f *fakeEmbedder) GetDimensions() int { return 2 }
func (f *fakeEmbedder) GetID() string      { return "fake" }

func embedded(content string, emb []float64, score float64) *vectordb.SearchResult {
	d := document.NewDocument(content)
	d.Embeddings = emb
	return &vectordb.SearchResult{Document: d, Score: score}
}

func contents(rs []*vectordb.SearchResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Document.Content
	}
	return out
}

func TestSearchLimit(t *testing.T) {
	b := &BaseReranker{}
	cases := map[int]int{1: 3, 10: 30, 40: 100, 200: 200, 0: 0}
	for in, want := range cases {
		if got := b.SearchLimit(in); got != want {
			t.Errorf("SearchLimit(%d)=%d want %d", in, got, want)
		}
	}
	if got := (&BaseReranker{CandidateMultiplier: 1}).SearchLimit(7); got != 7 {
		t.Errorf("multiplier 1: got %d", got)
	}
	if got := NewMMRReranker(nil, 0.5, 0).SearchLimit(4); got != 20 {
		t.Errorf("mmr default x5: got %d", got)
	}
	if got := SearchLimit(nil, 4); got != 4 {
		t.Errorf("nil reranker: got %d", got)
	}
}

func mmrFixture() (*fakeEmbedder, []*vectordb.SearchResult) {
	emb := &fakeEmbedder{vectors: map[string][]float64{"q": {1, 0.3}}}
	results := []*vectordb.SearchResult{
		embedded("a", []float64{1, 0}, 0.9),             // most relevant
		embedded("a-dup", []float64{0.99, -0.01}, 0.89), // near-duplicate of a
		embedded("b", []float64{0.6, 0.8}, 0.5),         // relevant-ish, different
	}
	return emb, results
}

func TestMMRPrefersDiversity(t *testing.T) {
	emb, results := mmrFixture()
	out, err := NewMMRReranker(emb, 0.5, 0).Rerank(context.Background(), "q", results, 2)
	if err != nil {
		t.Fatal(err)
	}
	got := contents(out)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("expected [a b], got %v", got)
	}
	if results[0].Score != 0.9 {
		t.Fatal("input results were mutated")
	}
	// Lambda 1 is pure relevance.
	out, _ = NewMMRReranker(emb, 1, 0).Rerank(context.Background(), "q", results, 2)
	if got := contents(out); got[1] != "a-dup" {
		t.Fatalf("lambda=1 expected a-dup second, got %v", got)
	}
}

func TestMMRTopNAndEmbedderFromContext(t *testing.T) {
	emb, results := mmrFixture()
	r := NewMMRReranker(nil, 0.5, 1)
	ctx := ContextWithEmbedder(context.Background(), emb)
	out, err := r.Rerank(ctx, "q", results, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Document.Content != "a" {
		t.Fatalf("topN=1: got %v", contents(out))
	}
	if emb.calls != 1 {
		t.Fatalf("expected the context embedder to be used once, got %d", emb.calls)
	}
}

func TestMMRErrors(t *testing.T) {
	emb, results := mmrFixture()
	ctx := context.Background()
	if _, err := NewMMRReranker(nil, 0.5, 0).Rerank(ctx, "q", results, 2); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("missing embedder: got %v", err)
	}
	if _, err := NewMMRReranker(emb, 1.5, 0).Rerank(ctx, "q", results, 2); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad lambda: got %v", err)
	}
	noEmb := append([]*vectordb.SearchResult{embedded("x", []float64{0, 0}, 1)}, results...)
	if _, err := NewMMRReranker(emb, 0.5, 0).Rerank(ctx, "q", noEmb, 2); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("zero embedding: got %v", err)
	}
	mixed := append([]*vectordb.SearchResult{embedded("x", []float64{1, 0, 0}, 1)}, results...)
	if _, err := NewMMRReranker(emb, 0.5, 0).Rerank(ctx, "q", mixed, 2); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("dimension mismatch: got %v", err)
	}
	if out, err := NewMMRReranker(emb, 0.5, 0).Rerank(ctx, "q", nil, 2); err != nil || len(out) != 0 {
		t.Errorf("empty: %v %v", out, err)
	}
}

func dated(content string, score float64, ts interface{}) *vectordb.SearchResult {
	d := document.NewDocument(content)
	if ts != nil {
		d.Metadata[RecencyMetadataKey] = ts
	}
	return &vectordb.SearchResult{Document: d, Score: score}
}

func TestRecencyPromotesFreshDocuments(t *testing.T) {
	now := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	r := NewRecencyReranker()
	r.Now = func() time.Time { return now }
	results := []*vectordb.SearchResult{
		dated("old", 0.80, now.AddDate(-1, 0, 0).Format(time.RFC3339)),
		dated("new", 0.75, now.Unix()*1000), // epoch milliseconds
		dated("undated", 0.70, nil),
	}
	out, err := r.Rerank(context.Background(), "q", results, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := contents(out); got[0] != "new" {
		t.Fatalf("expected new first, got %v", got)
	}
	// new: 0.7*0.75 + 0.3*1
	if math.Abs(out[0].Score-(0.7*0.75+0.3)) > 1e-9 {
		t.Fatalf("unexpected score %v", out[0].Score)
	}
	if results[1].Score != 0.75 {
		t.Fatal("input results were mutated")
	}
}

func TestRecencyHalfLife(t *testing.T) {
	now := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	r := NewRecencyReranker()
	r.Weight = 1
	r.Now = func() time.Time { return now }
	out, _ := r.Rerank(context.Background(), "q", []*vectordb.SearchResult{
		dated("d", 0.5, now.Add(-30*24*time.Hour)),
	}, 0)
	if math.Abs(out[0].Score-0.5) > 1e-9 {
		t.Fatalf("expected 0.5 at one half-life, got %v", out[0].Score)
	}
}

func TestRecencyFallbacks(t *testing.T) {
	r := NewRecencyReranker()
	// No timestamps and no scores: rank stands in for relevance, order unchanged.
	results := []*vectordb.SearchResult{dated("a", 0, nil), dated("b", 0, nil), dated("c", 0, nil)}
	out, err := r.Rerank(context.Background(), "q", results, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := contents(out); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("expected [a b], got %v", got)
	}
	// Store fallback key and naive ISO strings are read as UTC.
	d := dated("s", 0.5, nil)
	d.Document.Metadata[StoreRecencyMetadataKey] = "2026-01-01T00:00:00"
	if ts, ok := r.timestamp(d); !ok || int64(ts) != time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("store fallback not read: %v %v", ts, ok)
	}
	// Unbounded scores are rescaled by the max.
	got := rescaled([]float64{2, 4, -1})
	if got[0] != 0.5 || got[1] != 1 || got[2] != 0 {
		t.Fatalf("rescaled: %v", got)
	}
	r.Weight = 2
	if _, err := r.Rerank(context.Background(), "q", results, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid weight error, got %v", err)
	}
}
