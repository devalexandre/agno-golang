package reader

import (
	"context"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/document"
)

// TextReader reads plain-text files (.txt, .md, .log, etc.).
type TextReader struct {
	BaseReader
}

// NewTextReader creates a text reader.
func NewTextReader() *TextReader {
	return &TextReader{BaseReader{ID: "text", Extensions: []string{".txt", ".text", ".log", ".md", ".markdown"}}}
}

// Read reads a text file from disk and returns it as a single document.
func (r *TextReader) Read(_ context.Context, source string) ([]*document.Document, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}
	doc := document.NewDocument(string(data))
	doc.Name = filepath.Base(source)
	doc.Source = source
	doc.ContentType = "text/plain"
	doc.Metadata["source"] = source
	return []*document.Document{doc}, nil
}

var _ Reader = (*TextReader)(nil)
