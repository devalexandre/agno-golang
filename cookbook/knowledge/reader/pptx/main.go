// PPTXReader: extract slide text from a .pptx presentation.
//
// Offline: a two-slide deck is generated with archive/zip in a temp dir.
// The output is one document of "Slide N:\n<text>" blocks. No env vars needed.
package main

import (
	"archive/zip"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	path := filepath.Join(os.TempDir(), "agno-deck.pptx")
	if err := writeSamplePPTX(path); err != nil {
		log.Fatal(err)
	}
	defer os.Remove(path)

	docs, err := reader.NewPPTXReader().Read(context.Background(), path)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s (%v slides)\n\n%s\n", docs[0].Name, docs[0].Metadata["slide_count"], docs[0].Content)
}

func slide(texts ...string) string {
	body := ""
	for _, t := range texts {
		body += `<p:sp><p:txBody><a:p><a:r><a:t>` + t + `</a:t></a:r></a:p></p:txBody></p:sp>`
	}
	return `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree>` +
		body + `</p:spTree></p:cSld></p:sld>`
}

func writeSamplePPTX(path string) error {
	files := map[string]string{
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<p:sldIdLst><p:sldId id="256" r:id="rId1"/><p:sldId id="257" r:id="rId2"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Target="slides/slide1.xml"/><Relationship Id="rId2" Target="slides/slide2.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": slide("Agno Go", "Readers for every format"),
		"ppt/slides/slide2.xml": slide("Roadmap", "Excel, PPTX, YouTube, Sitemap"),
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return err
		}
	}
	return zw.Close()
}
