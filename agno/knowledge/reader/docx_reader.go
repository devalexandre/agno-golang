package reader

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// DocxReader extracts text from .docx files (Office Open XML) using only the
// standard library: a .docx is a zip whose word/document.xml holds the text.
type DocxReader struct {
	BaseReader
}

// NewDocxReader creates a DOCX reader.
func NewDocxReader() *DocxReader {
	return &DocxReader{BaseReader{ID: "docx", Extensions: []string{".docx"}}}
}

var (
	docxParagraph = regexp.MustCompile(`(?s)<w:p[ >].*?</w:p>`)
	docxTextRun   = regexp.MustCompile(`(?s)<w:t[^>]*>(.*?)</w:t>`)
	xmlTag        = regexp.MustCompile(`<[^>]+>`)
)

// Read extracts plain text from the .docx file at source.
func (r *DocxReader) Read(_ context.Context, source string) ([]*document.Document, error) {
	zr, err := zip.OpenReader(source)
	if err != nil {
		return nil, fmt.Errorf("docx: open: %w", err)
	}
	defer zr.Close()

	var docXML string
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("docx: open document.xml: %w", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("docx: read document.xml: %w", err)
			}
			docXML = string(data)
			break
		}
	}
	if docXML == "" {
		return nil, fmt.Errorf("docx: word/document.xml not found in %s", source)
	}

	var paragraphs []string
	for _, p := range docxParagraph.FindAllString(docXML, -1) {
		var b strings.Builder
		for _, m := range docxTextRun.FindAllStringSubmatch(p, -1) {
			b.WriteString(xmlTag.ReplaceAllString(m[1], ""))
		}
		if line := strings.TrimSpace(b.String()); line != "" {
			paragraphs = append(paragraphs, line)
		}
	}

	doc := document.NewDocument(strings.Join(paragraphs, "\n\n"))
	doc.Name = filepath.Base(source)
	doc.Source = source
	doc.ContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	doc.Metadata["source"] = source
	return []*document.Document{doc}, nil
}

var _ Reader = (*DocxReader)(nil)
