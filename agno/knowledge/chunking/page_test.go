package chunking

import (
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
)

const samplePage = "# Agents (/docs/agents)\n\nAgents are programs that use tools.\n\n## Overview\n\nSome text here.\n\n### Detail\n\nDeeper stays.\n\n## Example [#example]\n\n```python\n# not a heading\nprint(1)\n```\n"

func TestPageMarkdownChunkingSections(t *testing.T) {
	c := NewPageMarkdownChunking(0, 0)
	pairs := c.ChunkPairs(samplePage)
	if len(pairs) != 3 {
		for _, p := range pairs {
			t.Logf("[%s] %q", p.Breadcrumb, p.Text)
		}
		t.Fatalf("expected 3 chunks, got %d", len(pairs))
	}
	// The intro already starts with the page title, so it is used as-is (no "Agents: " prefix).
	ctx := "Agents are programs that use tools."
	if pairs[0].Breadcrumb != "Agents" || !strings.HasPrefix(pairs[0].Text, ctx+"\n\nAgents\n\n# Agents (/docs/agents)") {
		t.Fatalf("chunk 0: [%s] %q", pairs[0].Breadcrumb, pairs[0].Text)
	}
	if pairs[1].Breadcrumb != "Agents › Overview" || !strings.Contains(pairs[1].Text, "### Detail\n\nDeeper stays.") {
		t.Fatalf("chunk 1: [%s] %q", pairs[1].Breadcrumb, pairs[1].Text)
	}
	if pairs[2].Breadcrumb != "Agents › Example" || !strings.Contains(pairs[2].Text, "# not a heading\nprint(1)\n```") {
		t.Fatalf("chunk 2 (heading inside fence must not split): [%s] %q", pairs[2].Breadcrumb, pairs[2].Text)
	}
}

func TestPageMarkdownChunkingFoldsBareHeadings(t *testing.T) {
	c := NewPageMarkdownChunking(0, 0)
	if got := c.ChunkTexts("# Only a title\n"); len(got) != 0 {
		t.Fatalf("title-only page must yield no chunks: %v", got)
	}
	got := c.ChunkPairs("# Title\n\n## First\n\nbody")
	if len(got) != 1 || !strings.Contains(got[0].Text, "# Title\n\n## First\n\nbody") || got[0].Breadcrumb != "Title › First" {
		t.Fatalf("bare title should fold into next section: %+v", got)
	}
}

func TestPageMarkdownChunkingSplitsLongFence(t *testing.T) {
	var b strings.Builder
	b.WriteString("## Code\n\n```go\n")
	for i := 0; i < 60; i++ {
		b.WriteString("fmt.Println(\"line number xx\")\n")
	}
	b.WriteString("```\n")
	c := NewPageMarkdownChunking(400, 2)
	texts := c.ChunkTexts(b.String())
	if len(texts) < 3 {
		t.Fatalf("expected the fence to be split, got %d chunks", len(texts))
	}
	for i, txt := range texts {
		if strings.Count(txt, "```")%2 != 0 {
			t.Fatalf("chunk %d has an unbalanced fence:\n%s", i, txt)
		}
		if len(txt) > 400+400/4+80 {
			t.Fatalf("chunk %d too large: %d", i, len(txt))
		}
	}
}

func TestPageMarkdownChunkingMetadata(t *testing.T) {
	doc := document.NewDocument(samplePage)
	doc.ID = "page1"
	doc.Metadata["path"] = "/examples/agents/basic"
	chunks, err := NewPageMarkdownChunking(0, 0).Chunk(doc)
	if err != nil || len(chunks) != 3 {
		t.Fatalf("got %d, %v", len(chunks), err)
	}
	m := chunks[1].Metadata
	if chunks[1].ID != "page1_2" || m["chunk"] != 2 || m["section"] != "examples" || m["kind"] != "example" || m["breadcrumb"] != "Agents › Overview" {
		t.Fatalf("metadata: %s %v", chunks[1].ID, m)
	}
	if PageKind("reference/api") != "reference" || PageKind("agents/usage/x") != "example" || PageKind("agents") != "guide" {
		t.Fatal("PageKind")
	}
	anon := document.NewDocument("## A\n\nbody")
	anon.ID = ""
	anon.Name = ""
	out, _ := NewPageMarkdownChunking(0, 0).Chunk(anon)
	if !strings.HasPrefix(out[0].ID, "chunk_") {
		t.Fatalf("hash id: %s", out[0].ID)
	}
}

func TestStrategyFactory(t *testing.T) {
	for in, want := range map[string]StrategyType{
		"FixedSizeChunker": FixedSizeChunker, "fixed_size": FixedSizeChunker, "Markdown": MarkdownChunker,
		"page": PageChunker, "RecursiveChunking": RecursiveChunker,
	} {
		got, err := ParseStrategyType(in)
		if err != nil || got != want {
			t.Errorf("ParseStrategyType(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := ParseStrategyType("bogus"); err == nil {
		t.Fatal("expected error")
	}
	for _, st := range AllStrategyTypes() {
		s, err := NewStrategy(st, StrategyOptions{ChunkSize: 100})
		if st == AgenticChunker || st == SemanticChunker {
			if err == nil {
				t.Errorf("%s without model/embedder must fail", st)
			}
			continue
		}
		if err != nil || s == nil {
			t.Errorf("NewStrategy(%s): %v", st, err)
		}
	}
	s, _ := NewStrategy(PageChunker, StrategyOptions{ChunkSize: 500, SplitLevel: 1})
	if p := s.(*PageMarkdownChunking); p.ChunkSize != 500 || p.SplitLevel != 1 {
		t.Fatalf("page options: %+v", p)
	}
}
