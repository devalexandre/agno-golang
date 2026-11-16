package knowledge

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge/querytransformer"
	"github.com/devalexandre/agno-golang/agno/knowledge/reranker"
	"github.com/devalexandre/agno-golang/agno/storage"
)

// fakeKB returns n results ("doc0".."docN-1") in descending score order and
// records the query/limit it was searched with.
type fakeKB struct {
	lastQuery string
	lastLimit int
}

func (f *fakeKB) Search(_ context.Context, query string, n int) ([]*SearchResult, error) {
	f.lastQuery, f.lastLimit = query, n
	out := make([]*SearchResult, n)
	for i := range out {
		out[i] = &SearchResult{Document: document.NewDocument(fmt.Sprintf("doc%d", i)), Score: 1 - float64(i)/100}
	}
	return out, nil
}
func (f *fakeKB) Load(context.Context, bool) error                      { return nil }
func (f *fakeKB) LoadDocument(context.Context, document.Document) error { return nil }
func (f *fakeKB) Drop(context.Context) error                            { return nil }
func (f *fakeKB) Exists(context.Context) (bool, error)                  { return true, nil }
func (f *fakeKB) GetCount(context.Context) (int64, error)               { return 0, nil }
func (f *fakeKB) GetInfo() KnowledgeInfo                                { return KnowledgeInfo{} }
func (f *fakeKB) GetContentsDB() storage.Storage                        { return nil }

// reverseReranker reverses the candidates and records what it saw.
type reverseReranker struct {
	reranker.BaseReranker
	query string
	seen  int
	limit int
	err   error
}

func (r *reverseReranker) Rerank(_ context.Context, q string, results []*SearchResult, limit int) ([]*SearchResult, error) {
	r.query, r.seen, r.limit = q, len(results), limit
	if r.err != nil {
		return nil, r.err
	}
	out := make([]*SearchResult, len(results))
	for i, res := range results {
		out[len(results)-1-i] = res
	}
	return out, nil
}

func TestRAGPipelineNoRerankerUnchanged(t *testing.T) {
	kb := &fakeKB{}
	res, err := NewRAGPipeline(kb, 4).Query(context.Background(), "q")
	if err != nil {
		t.Fatal(err)
	}
	if kb.lastLimit != 4 || kb.lastQuery != "q" || len(res.Documents) != 4 || res.Documents[0].Document.Content != "doc0" {
		t.Fatalf("unexpected: limit=%d query=%q docs=%d", kb.lastLimit, kb.lastQuery, len(res.Documents))
	}
}

func TestRAGPipelineTransformRerankTrim(t *testing.T) {
	kb := &fakeKB{}
	rr := &reverseReranker{}
	p := NewRAGPipeline(kb, 4)
	p.SetRetrievalReranker(rr)
	p.SetQueryTransformer(querytransformer.Func(func(_ context.Context, q string) (string, error) {
		return "hypothetical answer to " + q, nil
	}))
	res, err := p.Query(context.Background(), "q")
	if err != nil {
		t.Fatal(err)
	}
	if kb.lastQuery != "hypothetical answer to q" {
		t.Fatalf("vector db should see the transformed query, got %q", kb.lastQuery)
	}
	if rr.query != "q" {
		t.Fatalf("reranker should see the original query, got %q", rr.query)
	}
	if kb.lastLimit != 12 || rr.seen != 12 || rr.limit != 4 {
		t.Fatalf("expected 12 candidates and limit 4, got fetch=%d seen=%d limit=%d", kb.lastLimit, rr.seen, rr.limit)
	}
	if len(res.Documents) != 4 || res.Documents[0].Document.Content != "doc11" {
		t.Fatalf("expected reranked+trimmed docs, got %d first=%s", len(res.Documents), res.Documents[0].Document.Content)
	}
}

func TestRetrieveFailureModes(t *testing.T) {
	kb := &fakeKB{}
	ctx := context.Background()

	// Transform failure: search with the original query.
	failing := querytransformer.Func(func(context.Context, string) (string, error) { return "", errors.New("boom") })
	if _, err := retrieve(ctx, kb.Search, "q", 3, nil, failing, nil); err != nil || kb.lastQuery != "q" {
		t.Fatalf("transform failure: err=%v query=%q", err, kb.lastQuery)
	}

	// Generic reranker failure: vector db order, trimmed.
	out, err := retrieve(ctx, kb.Search, "q", 3, &reverseReranker{err: errors.New("provider down")}, nil, nil)
	if err != nil || len(out) != 3 || out[0].Document.Content != "doc0" {
		t.Fatalf("degraded rerank: err=%v n=%d", err, len(out))
	}

	// Misconfiguration propagates.
	bad := &reverseReranker{err: fmt.Errorf("%w: no embedder", reranker.ErrInvalidInput)}
	if _, err := retrieve(ctx, kb.Search, "q", 3, bad, nil, nil); !errors.Is(err, reranker.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

type legacy struct{ called bool }

func (l *legacy) Rerank(_ context.Context, _ string, results []*SearchResult) ([]*SearchResult, error) {
	l.called = true
	return results, nil
}

func TestLegacyRerankerStillWorks(t *testing.T) {
	kb := &fakeKB{}
	l := &legacy{}
	p := NewRAGPipeline(kb, 2)
	p.SetReranker(l)
	res, err := p.Query(context.Background(), "q")
	if err != nil {
		t.Fatal(err)
	}
	if !l.called || kb.lastLimit != 6 || len(res.Documents) != 2 {
		t.Fatalf("legacy: called=%v fetch=%d docs=%d", l.called, kb.lastLimit, len(res.Documents))
	}
}
