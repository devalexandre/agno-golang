package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// JSONReader reads JSON files. A top-level array yields one document per
// element (metadata "page" is the 1-based position); any other value yields a
// single document. Each document's content is the compact JSON encoding of
// its value. Port of agno-python knowledge/reader/json_reader.py.
type JSONReader struct {
	BaseReader
}

// NewJSONReader creates a JSON reader.
func NewJSONReader() *JSONReader {
	return &JSONReader{BaseReader{ID: "json", Extensions: []string{".json"}}}
}

// Read reads the JSON file at source.
func (r *JSONReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}
	docs, err := r.ReadBytes(ctx, data, filepath.Base(source))
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		d.Source = source
		d.Metadata["source"] = source
	}
	return docs, nil
}

// ReadBytes parses in-memory JSON; name is used for the document name.
func (r *JSONReader) ReadBytes(_ context.Context, data []byte, name string) ([]*document.Document, error) {
	dec := json.NewDecoder(bytes.NewReader(bytesTrimBOM(data)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("json: parse: %w", err)
	}
	items, ok := v.([]any)
	if !ok {
		items = []any{v}
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if stem == "" {
		stem = "json_file"
	}
	docs := make([]*document.Document, 0, len(items))
	for i, item := range items {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(item); err != nil {
			return nil, err
		}
		doc := document.NewDocument(strings.TrimRight(buf.String(), "\n"))
		doc.Name = stem
		doc.ContentType = "application/json"
		doc.Metadata["page"] = i + 1
		docs = append(docs, doc)
	}
	return docs, nil
}

var _ Reader = (*JSONReader)(nil)
