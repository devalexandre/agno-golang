package knowledgetools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge"
)

type fakeKB struct {
	docs []*knowledge.SearchResult
	err  error
	n    int
}

func (f *fakeKB) Search(_ context.Context, _ string, n int) ([]*knowledge.SearchResult, error) {
	f.n = n
	return f.docs, f.err
}

func TestKnowledgeTools(t *testing.T) {
	if _, err := NewKnowledgeTools(nil); err == nil {
		t.Fatal("expected error for nil knowledge")
	}
	kb := &fakeKB{docs: []*knowledge.SearchResult{{Document: &document.Document{Content: "Go is fast"}}}}
	k, err := NewKnowledgeTools(kb, WithNumDocuments(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(k.GetMethods()) != 3 || !k.AddInstructions {
		t.Fatal("expected 3 tools and add_instructions=true")
	}
	out, _ := k.Think(ThinkParams{Thought: "a"})
	out, _ = k.Think(ThinkParams{Thought: "b"})
	if out != "Thoughts:\n- a\n- b" {
		t.Fatalf("think: %q", out)
	}
	res, _ := k.SearchKnowledge(SearchParams{Query: "go"})
	if !strings.Contains(res.(string), "Go is fast") || kb.n != 3 {
		t.Fatalf("search: %v (n=%d)", res, kb.n)
	}
	kb.docs = nil
	if res, _ := k.SearchKnowledge(SearchParams{Query: "x"}); res != "No documents found" {
		t.Fatalf("empty search: %v", res)
	}
	kb.err = errors.New("boom")
	if res, _ := k.SearchKnowledge(SearchParams{Query: "x"}); !strings.HasPrefix(res.(string), "Error searching knowledge base") {
		t.Fatalf("error search: %v", res)
	}
	if a, _ := k.Analyze(AnalyzeParams{Analysis: "ok"}); a != "Analysis:\n- ok" {
		t.Fatalf("analyze: %q", a)
	}
}
