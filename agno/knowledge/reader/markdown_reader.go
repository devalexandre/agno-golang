package reader

import (
	"context"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/document"
)

// MarkdownReader reads Markdown files, preserving the raw markdown content
// (chunking strategies like MarkdownChunking can split on headings later).
type MarkdownReader struct {
	BaseReader
}

// NewMarkdownReader creates a Markdown reader.
func NewMarkdownReader() *MarkdownReader {
	return &MarkdownReader{BaseReader{ID: "markdown", Extensions: []string{".md", ".markdown", ".mdx"}}}
}

// Read reads a Markdown file from disk.
func (r *MarkdownReader) Read(_ context.Context, source string) ([]*document.Document, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}
	doc := document.NewDocument(string(data))
	doc.Name = filepath.Base(source)
	doc.Source = source
	doc.ContentType = "text/markdown"
	doc.Metadata["source"] = source
	return []*document.Document{doc}, nil
}

var _ Reader = (*MarkdownReader)(nil)
