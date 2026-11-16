package reader

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// PDFReader extracts text from PDF files, one document per page (metadata
// "page", 1-based), like agno-python knowledge/reader/pdf_reader.py.
//
// Extraction uses poppler's `pdftotext` when it is on PATH (pages are split
// on form feeds). Without it, a dependency-free fallback decodes the
// content streams (FlateDecode or raw) and collects the literal strings shown
// by Tj/TJ/'/" operators; it handles simple PDFs only (no CID fonts, no
// per-page split) and returns a single document.
type PDFReader struct {
	BaseReader
	// Password for encrypted PDFs (passed to pdftotext as -upw).
	Password string
	// PdftotextPath overrides the pdftotext binary (default: looked up on PATH).
	PdftotextPath string
	// DisablePdftotext forces the built-in fallback extractor.
	DisablePdftotext bool
}

// NewPDFReader creates a PDF reader.
func NewPDFReader() *PDFReader {
	return &PDFReader{BaseReader: BaseReader{ID: "pdf", Extensions: []string{".pdf"}}}
}

// Read reads the PDF at source (a file path).
func (r *PDFReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	pages, err := r.extract(ctx, source)
	if err != nil {
		return nil, err
	}
	var docs []*document.Document
	for i, text := range pages {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		doc := document.NewDocument(text)
		doc.Name = name
		doc.Source = source
		doc.ContentType = "application/pdf"
		doc.Metadata["source"] = source
		doc.Metadata["page"] = i + 1
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("pdf: no text content found in %s", source)
	}
	return docs, nil
}

// ReadBytes reads an in-memory PDF (written to a temp file for extraction).
func (r *PDFReader) ReadBytes(ctx context.Context, data []byte, name string) ([]*document.Document, error) {
	f, err := os.CreateTemp("", "agno-pdf-*.pdf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()
	docs, err := r.Read(ctx, f.Name())
	if err != nil {
		return nil, err
	}
	stem := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	for _, d := range docs {
		d.Name = stem
		d.Source = name
		d.Metadata["source"] = name
	}
	return docs, nil
}

func (r *PDFReader) extract(ctx context.Context, path string) ([]string, error) {
	if !r.DisablePdftotext {
		bin := r.PdftotextPath
		if bin == "" {
			bin, _ = exec.LookPath("pdftotext")
		}
		if bin != "" {
			args := []string{"-layout", "-enc", "UTF-8"}
			if r.Password != "" {
				args = append(args, "-upw", r.Password)
			}
			args = append(args, path, "-")
			out, err := exec.CommandContext(ctx, bin, args...).Output()
			// An empty result (e.g. a damaged xref) falls through to the built-in extractor.
			if err == nil && strings.TrimSpace(string(out)) != "" {
				pages := strings.Split(string(out), "\f")
				if len(pages) > 0 && strings.TrimSpace(pages[len(pages)-1]) == "" {
					pages = pages[:len(pages)-1]
				}
				return pages, nil
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(bytes.TrimLeft(data, " \r\n\t"), []byte("%PDF-")) {
		return nil, fmt.Errorf("pdf: %s is not a PDF file", path)
	}
	return []string{basicPDFText(data)}, nil
}

var (
	pdfStream   = regexp.MustCompile(`(?s)<<(.*?)>>\s*stream\r?\n`)
	pdfTextOps  = regexp.MustCompile(`(?s)\((?:\\.|[^\\)])*\)|\[(?:[^\]\\]|\\.)*\]\s*TJ|T\*|Td|TD|ET`)
	pdfTJString = regexp.MustCompile(`(?s)\((?:\\.|[^\\)])*\)|-?\d+(?:\.\d+)?`)
)

// basicPDFText is the dependency-free fallback extractor.
func basicPDFText(data []byte) string {
	var out strings.Builder
	for _, loc := range pdfStream.FindAllSubmatchIndex(data, -1) {
		dict := string(data[loc[2]:loc[3]])
		start := loc[1]
		end := bytes.Index(data[start:], []byte("endstream"))
		if end < 0 {
			continue
		}
		raw := data[start : start+end]
		content := raw
		if strings.Contains(dict, "/FlateDecode") {
			zr, err := zlib.NewReader(bytes.NewReader(raw))
			if err != nil {
				continue
			}
			content, err = io.ReadAll(io.LimitReader(zr, 64<<20))
			zr.Close()
			if err != nil && len(content) == 0 {
				continue
			}
		} else if strings.Contains(dict, "/Filter") {
			continue // other filters (images, fonts) are not text
		}
		if !bytes.Contains(content, []byte("BT")) {
			continue
		}
		extractPDFContentText(string(content), &out)
	}
	return collapseBlankLines(out.String())
}

func extractPDFContentText(content string, out *strings.Builder) {
	for _, tok := range pdfTextOps.FindAllString(content, -1) {
		switch {
		case tok == "T*" || tok == "Td" || tok == "TD" || tok == "ET":
			out.WriteString("\n")
		case strings.HasPrefix(tok, "["):
			for _, part := range pdfTJString.FindAllString(tok, -1) {
				if strings.HasPrefix(part, "(") {
					out.WriteString(unescapePDFString(part[1 : len(part)-1]))
				} else if len(part) > 0 && part[0] == '-' && len(part) >= 4 {
					out.WriteString(" ") // large negative kerning separates words
				}
			}
		default:
			out.WriteString(unescapePDFString(tok[1 : len(tok)-1]))
		}
	}
}

func unescapePDFString(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b', 'f':
		case '\n', '\r':
		default:
			if s[i] >= '0' && s[i] <= '7' {
				v := 0
				j := 0
				for ; j < 3 && i+j < len(s) && s[i+j] >= '0' && s[i+j] <= '7'; j++ {
					v = v*8 + int(s[i+j]-'0')
				}
				i += j - 1
				b.WriteRune(rune(v))
			} else {
				b.WriteByte(s[i])
			}
		}
	}
	return b.String()
}

func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	var keep []string
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			keep = append(keep, t)
		}
	}
	return strings.Join(keep, "\n")
}

var _ Reader = (*PDFReader)(nil)
