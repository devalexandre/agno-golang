package reader

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTextReader(t *testing.T) {
	path := writeFile(t, "doc.txt", "hello world")
	docs, err := NewTextReader().Read(context.Background(), path)
	if err != nil || len(docs) != 1 {
		t.Fatalf("got %d docs, err=%v", len(docs), err)
	}
	if docs[0].Content != "hello world" {
		t.Fatalf("content = %q", docs[0].Content)
	}
	if docs[0].Name != "doc.txt" {
		t.Fatalf("name = %q", docs[0].Name)
	}
}

func TestCSVReaderRowPerDocument(t *testing.T) {
	path := writeFile(t, "data.csv", "name,age\nalice,30\nbob,25")
	docs, err := NewCSVReader().Read(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 row docs, got %d", len(docs))
	}
	if !strings.Contains(docs[0].Content, "name: alice") || !strings.Contains(docs[0].Content, "age: 30") {
		t.Fatalf("unexpected row content: %q", docs[0].Content)
	}
}

func TestMarkdownReader(t *testing.T) {
	path := writeFile(t, "readme.md", "# Title\n\nbody")
	docs, err := NewMarkdownReader().Read(context.Background(), path)
	if err != nil || len(docs) != 1 {
		t.Fatalf("got %d docs, err=%v", len(docs), err)
	}
	if docs[0].ContentType != "text/markdown" {
		t.Fatalf("content type = %q", docs[0].ContentType)
	}
}

func TestDocxReader(t *testing.T) {
	// Build a minimal valid .docx (zip with word/document.xml).
	path := filepath.Join(t.TempDir(), "doc.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("word/document.xml")
	w.Write([]byte(`<?xml version="1.0"?><w:document xmlns:w="x"><w:body>` +
		`<w:p><w:r><w:t>Hello</w:t></w:r><w:r><w:t> World</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Second paragraph</w:t></w:r></w:p>` +
		`</w:body></w:document>`))
	zw.Close()
	f.Close()

	docs, err := NewDocxReader().Read(context.Background(), path)
	if err != nil || len(docs) != 1 {
		t.Fatalf("got %d docs, err=%v", len(docs), err)
	}
	if !strings.Contains(docs[0].Content, "Hello World") {
		t.Fatalf("docx text not extracted: %q", docs[0].Content)
	}
	if !strings.Contains(docs[0].Content, "Second paragraph") {
		t.Fatalf("second paragraph missing: %q", docs[0].Content)
	}
}

func TestWebsiteExtractHTMLText(t *testing.T) {
	// extractHTMLText is the offline core of the website reader.
	const page = `<html><head><title>My Page</title><style>.x{}</style></head>` +
		`<body><script>ignore()</script><h1>Heading</h1><p>Para one.</p><p>Para two.</p></body></html>`
	node, err := parseHTMLString(page)
	if err != nil {
		t.Fatal(err)
	}
	title, text := extractHTMLText(node)
	if title != "My Page" {
		t.Fatalf("title = %q", title)
	}
	if strings.Contains(text, "ignore") || strings.Contains(text, ".x{") {
		t.Fatalf("script/style leaked into text: %q", text)
	}
	if !strings.Contains(text, "Heading") || !strings.Contains(text, "Para one.") {
		t.Fatalf("visible text missing: %q", text)
	}
}

func TestFactory(t *testing.T) {
	if r := ReaderForExtension(".csv"); r == nil || r.Name() != "csv" {
		t.Fatalf("ReaderForExtension(.csv) = %v", r)
	}
	if r := ReaderForExtension(".txt"); r == nil || r.Name() != "text" {
		t.Fatalf("ReaderForExtension(.txt) = %v", r)
	}
	if r := ReaderForExtension(".docx"); r == nil || r.Name() != "docx" {
		t.Fatalf("ReaderForExtension(.docx) = %v", r)
	}
	if r := ReaderForExtension(".unknown"); r != nil {
		t.Fatalf("expected nil for unknown extension, got %v", r)
	}
}
