package reader

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// CSVReader reads CSV/TSV files, emitting one document per row with the header
// fields labeling each value (e.g. "name: alice | age: 30").
type CSVReader struct {
	BaseReader
	Delimiter rune
	// RowPerDocument emits one document per data row when true (default);
	// when false the whole file becomes a single document.
	RowPerDocument bool
}

// NewCSVReader creates a CSV reader.
func NewCSVReader() *CSVReader {
	return &CSVReader{
		BaseReader:     BaseReader{ID: "csv", Extensions: []string{".csv", ".tsv"}},
		Delimiter:      ',',
		RowPerDocument: true,
	}
}

// Read parses the CSV file at source.
func (r *CSVReader) Read(_ context.Context, source string) ([]*document.Document, error) {
	f, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cr := csv.NewReader(f)
	cr.FieldsPerRecord = -1
	if strings.EqualFold(filepath.Ext(source), ".tsv") {
		cr.Comma = '\t'
	} else if r.Delimiter != 0 {
		cr.Comma = r.Delimiter
	}

	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	name := filepath.Base(source)
	header := rows[0]

	if !r.RowPerDocument {
		var b strings.Builder
		for _, row := range rows {
			b.WriteString(strings.Join(row, ", "))
			b.WriteString("\n")
		}
		doc := document.NewDocument(b.String())
		doc.Name = name
		doc.Source = source
		doc.ContentType = "text/csv"
		doc.Metadata["source"] = source
		return []*document.Document{doc}, nil
	}

	var docs []*document.Document
	for i, row := range rows[1:] {
		var parts []string
		for j, val := range row {
			label := fmt.Sprintf("col_%d", j)
			if j < len(header) {
				label = header[j]
			}
			parts = append(parts, fmt.Sprintf("%s: %s", label, val))
		}
		doc := document.NewDocument(strings.Join(parts, " | "))
		doc.Name = fmt.Sprintf("%s row %d", name, i+1)
		doc.Source = source
		doc.ContentType = "text/csv"
		doc.Metadata["source"] = source
		doc.Metadata["row"] = i + 1
		docs = append(docs, doc)
	}
	return docs, nil
}

var _ Reader = (*CSVReader)(nil)
