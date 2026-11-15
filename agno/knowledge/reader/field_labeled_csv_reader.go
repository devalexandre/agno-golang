package reader

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// FieldLabeledCSVReader converts each CSV row into a field-labeled document:
//
//	Optional Title
//	First Name: Alice
//	Age: 30
//
// Port of agno-python knowledge/reader/field_labeled_csv_reader.py. Each row
// is already a logical unit, so no chunking is expected downstream.
type FieldLabeledCSVReader struct {
	BaseReader
	// ChunkTitles, when set, prefixes each row; titles rotate by row index
	// (a single entry is used for every row).
	ChunkTitles []string
	// FieldNames overrides the labels positionally (falls back to headers).
	FieldNames []string
	// FormatHeaders turns "first_name" into "First Name" (default true).
	FormatHeaders bool
	// SkipEmptyFields omits fields whose value is empty (default true).
	SkipEmptyFields bool
	// Delimiter (default ',').
	Delimiter rune
}

// NewFieldLabeledCSVReader creates a field-labeled CSV reader.
func NewFieldLabeledCSVReader() *FieldLabeledCSVReader {
	return &FieldLabeledCSVReader{
		BaseReader:      BaseReader{ID: "field_labeled_csv", Extensions: []string{".csv"}},
		FormatHeaders:   true,
		SkipEmptyFields: true,
		Delimiter:       ',',
	}
}

func (r *FieldLabeledCSVReader) formatFieldName(name string) string {
	if !r.FormatHeaders {
		return strings.TrimSpace(name)
	}
	return pyTitle(strings.TrimSpace(strings.ReplaceAll(name, "_", " ")))
}

// pyTitle mimics Python's str.title(): uppercase the first letter of every
// run of letters, lowercase the rest.
func pyTitle(s string) string {
	var b strings.Builder
	prevLetter := false
	for _, ch := range s {
		isLetter := strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", ch) ||
			(ch > 127 && strings.ToUpper(string(ch)) != strings.ToLower(string(ch)))
		switch {
		case isLetter && !prevLetter:
			b.WriteString(strings.ToUpper(string(ch)))
		case isLetter:
			b.WriteString(strings.ToLower(string(ch)))
		default:
			b.WriteRune(ch)
		}
		prevLetter = isLetter
	}
	return b.String()
}

func (r *FieldLabeledCSVReader) title(i int) string {
	if len(r.ChunkTitles) == 0 {
		return ""
	}
	return r.ChunkTitles[i%len(r.ChunkTitles)]
}

// Read parses the CSV file at source.
func (r *FieldLabeledCSVReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
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
	}
	return docs, nil
}

// ReadBytes parses in-memory CSV; name is used for document names/IDs.
func (r *FieldLabeledCSVReader) ReadBytes(_ context.Context, data []byte, name string) ([]*document.Document, error) {
	cr := csv.NewReader(bytes.NewReader(bytesTrimBOM(data)))
	cr.FieldsPerRecord = -1
	if r.Delimiter != 0 {
		cr.Comma = r.Delimiter
	}
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("field_labeled_csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	csvName := strings.TrimSuffix(name, filepath.Ext(name))
	if csvName == "" {
		csvName = "csv_file"
	}
	headers := make([]string, len(rows[0]))
	for i, h := range rows[0] {
		headers[i] = strings.TrimSpace(h)
	}
	dataRows := rows[1:]
	var docs []*document.Document
	for ri, row := range dataRows {
		var lines []string
		if t := r.title(ri); t != "" {
			lines = append(lines, t)
		}
		for i, header := range headers {
			value := ""
			if i < len(row) {
				value = strings.TrimSpace(stringifyCell(row[i]))
			}
			if r.SkipEmptyFields && value == "" {
				continue
			}
			label := r.formatFieldName(header)
			if i < len(r.FieldNames) {
				label = r.FieldNames[i]
			}
			lines = append(lines, label+": "+value)
		}
		text := strings.Join(lines, "\n")
		if strings.TrimSpace(text) == "" {
			continue
		}
		doc := document.NewDocument(text)
		doc.ID = fmt.Sprintf("%s_row_%d", csvName, ri+1)
		doc.Name = csvName
		doc.ContentType = "text/plain"
		doc.Metadata["row_index"] = ri
		doc.Metadata["headers"] = headers
		doc.Metadata["total_rows"] = len(dataRows)
		doc.Metadata["source"] = "field_labeled_csv_reader"
		docs = append(docs, doc)
	}
	return docs, nil
}

var _ Reader = (*FieldLabeledCSVReader)(nil)
