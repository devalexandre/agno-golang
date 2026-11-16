// PDFReader: one document per page. Uses poppler's `pdftotext` when it is on
// PATH; otherwise a built-in fallback extracts text from simple PDFs.
//
// Usage: go run ./cookbook/knowledge/reader/pdf [file.pdf]
// Without an argument a tiny PDF is generated in a temp dir (offline).
package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	var path string
	if len(os.Args) > 1 {
		path = os.Args[1]
	} else {
		path = filepath.Join(os.TempDir(), "agno-sample.pdf")
		if err := os.WriteFile(path, samplePDF("Agno Go PDF reader", "Second line of text"), 0o644); err != nil {
			log.Fatal(err)
		}
		defer os.Remove(path)
	}
	docs, err := reader.NewPDFReader().Read(context.Background(), path)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range docs {
		fmt.Printf("== %s page %v\n%s\n", d.Name, d.Metadata["page"], d.Content)
	}
}

// samplePDF builds a minimal one-page PDF with a Flate-compressed content stream.
func samplePDF(lines ...string) []byte {
	var content bytes.Buffer
	content.WriteString("BT /F1 12 Tf 72 720 Td ")
	for i, l := range lines {
		if i > 0 {
			content.WriteString("0 -14 Td ")
		}
		fmt.Fprintf(&content, "(%s) Tj ", l)
	}
	content.WriteString("ET")
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(content.Bytes())
	zw.Close()
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n")
	pdf.WriteString("2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj\n")
	pdf.WriteString("3 0 obj << /Type /Page /Parent 2 0 R /Contents 4 0 R >> endobj\n")
	fmt.Fprintf(&pdf, "4 0 obj << /Length %d /Filter /FlateDecode >> stream\n", z.Len())
	pdf.Write(z.Bytes())
	pdf.WriteString("\nendstream endobj\ntrailer << /Root 1 0 R >>\n%%EOF\n")
	return pdf.Bytes()
}
