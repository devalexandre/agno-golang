package reranker

import (
	"context"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

func result(content string, score float64) *vectordb.SearchResult {
	return &vectordb.SearchResult{Document: document.NewDocument(content), Score: score}
}

func TestRerankersImplementInterface(t *testing.T) {
	var rerankers = []Reranker{
		NewLexicalReranker(0),
		NewCohereReranker("k", "", 0),
		NewInfinityReranker("http://localhost:7997", "", 0),
	}
	for _, r := range rerankers {
		if r.GetID() == "" {
			t.Errorf("reranker has empty ID")
		}
	}
}

func TestLexicalRerankerReorders(t *testing.T) {
	results := []*vectordb.SearchResult{
		result("the cat sat on the mat", 0.1),
		result("quantum physics and relativity", 0.9), // high vector score, low lexical
		result("a happy dog runs in the park dog", 0.2),
	}
	query := "dog park"

	out, err := NewLexicalReranker(0).Rerank(context.Background(), query, results, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The dog/park document should now rank first despite a lower original score.
	if !contains(out[0].Document.Content, "dog") {
		t.Fatalf("expected dog document first, got %q (score %.2f)", out[0].Document.Content, out[0].Score)
	}
	if out[0].Score <= out[len(out)-1].Score {
		t.Fatalf("results not sorted by descending score: %v", out)
	}
}

func TestLexicalRerankerTopN(t *testing.T) {
	results := []*vectordb.SearchResult{
		result("dog park walk", 0),
		result("dog", 0),
		result("unrelated text", 0),
	}
	out, _ := NewLexicalReranker(2).Rerank(context.Background(), "dog park", results, 0)
	if len(out) != 2 {
		t.Fatalf("expected topN=2 results, got %d", len(out))
	}
}

func TestLexicalRerankerEmpty(t *testing.T) {
	out, err := NewLexicalReranker(0).Rerank(context.Background(), "q", nil, 0)
	if err != nil || len(out) != 0 {
		t.Fatalf("empty input: got %d, err=%v", len(out), err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
