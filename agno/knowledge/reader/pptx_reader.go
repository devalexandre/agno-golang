package reader

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	stdhtml "html"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// PPTXReader extracts slide text from .pptx presentations using only the
// standard library (a .pptx is a zip of PresentationML parts). Port of
// agno-python knowledge/reader/pptx_reader.py: the output is one document
// whose content is "Slide N:\n<shape texts>" blocks separated by blank lines;
// shapes inside groups are included, slides without text read
// "(No text content)".
type PPTXReader struct {
	BaseReader
}

// NewPPTXReader creates a PPTX reader.
func NewPPTXReader() *PPTXReader {
	return &PPTXReader{BaseReader{ID: "pptx", Extensions: []string{".pptx"}}}
}

// Read reads the presentation at source (a file path).
func (r *PPTXReader) Read(_ context.Context, source string) ([]*document.Document, error) {
	zr, err := zip.OpenReader(source)
	if err != nil {
		return nil, fmt.Errorf("pptx: open: %w", err)
	}
	defer zr.Close()
	doc, err := r.read(&zr.Reader, strings.TrimSuffix(filepath.Base(source), filepath.Ext(source)))
	if err != nil {
		return nil, err
	}
	doc.Source = source
	doc.Metadata["source"] = source
	return []*document.Document{doc}, nil
}

// ReadBytes reads an in-memory .pptx; name is used for the document name.
func (r *PPTXReader) ReadBytes(_ context.Context, data []byte, name string) ([]*document.Document, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("pptx: open: %w", err)
	}
	stem := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if stem == "" {
		stem = "pptx_file"
	}
	doc, err := r.read(zr, stem)
	if err != nil {
		return nil, err
	}
	return []*document.Document{doc}, nil
}

type pptxPresentation struct {
	SlideIDs []struct {
		RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	} `xml:"sldIdLst>sldId"`
}

var (
	pptxShape     = regexp.MustCompile(`(?s)<p:sp[ >].*?</p:sp>`)
	pptxParagraph = regexp.MustCompile(`(?s)<a:p[ >].*?</a:p>|<a:p/>`)
	pptxTextRun   = regexp.MustCompile(`(?s)<a:t>(.*?)</a:t>|<a:t [^>]*>(.*?)</a:t>|<a:br/>`)
	pptxSlideNum  = regexp.MustCompile(`slide(\d+)\.xml$`)
)

// slidePaths returns the slide parts in presentation order.
func pptxSlidePaths(zr *zip.Reader) []string {
	var paths []string
	presData, _ := zipFile(zr, "ppt/presentation.xml")
	relData, _ := zipFile(zr, "ppt/_rels/presentation.xml.rels")
	if presData != nil && relData != nil {
		var pres pptxPresentation
		var rels xlsxRels
		if xml.Unmarshal(presData, &pres) == nil && xml.Unmarshal(relData, &rels) == nil {
			targets := map[string]string{}
			for _, rel := range rels.Rels {
				t := rel.Target
				if strings.HasPrefix(t, "/") {
					t = strings.TrimPrefix(t, "/")
				} else {
					t = path.Join("ppt", t)
				}
				targets[rel.ID] = t
			}
			for _, s := range pres.SlideIDs {
				if t, ok := targets[s.RID]; ok {
					paths = append(paths, t)
				}
			}
		}
	}
	if len(paths) > 0 {
		return paths
	}
	// Fallback: every ppt/slides/slideN.xml in numeric order.
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/slides/") && pptxSlideNum.MatchString(f.Name) {
			paths = append(paths, f.Name)
		}
	}
	num := func(p string) int {
		m := pptxSlideNum.FindStringSubmatch(p)
		n, _ := strconv.Atoi(m[1])
		return n
	}
	sort.Slice(paths, func(i, j int) bool { return num(paths[i]) < num(paths[j]) })
	return paths
}

// pptxShapeTexts returns the stripped, non-empty text of every <p:sp> shape
// (group children included, since <p:grpSp> contains them) in document order.
func pptxShapeTexts(slideXML string) []string {
	var texts []string
	for _, sp := range pptxShape.FindAllString(slideXML, -1) {
		var paras []string
		for _, p := range pptxParagraph.FindAllString(sp, -1) {
			var b strings.Builder
			for _, m := range pptxTextRun.FindAllStringSubmatch(p, -1) {
				switch {
				case m[0] == "<a:br/>":
					b.WriteString("\n")
				case m[1] != "":
					b.WriteString(stdhtml.UnescapeString(m[1]))
				default:
					b.WriteString(stdhtml.UnescapeString(m[2]))
				}
			}
			paras = append(paras, b.String())
		}
		if t := strings.TrimSpace(strings.Join(paras, "\n")); t != "" {
			texts = append(texts, t)
		}
	}
	return texts
}

func (r *PPTXReader) read(zr *zip.Reader, name string) (*document.Document, error) {
	paths := pptxSlidePaths(zr)
	if len(paths) == 0 {
		return nil, fmt.Errorf("pptx: no slides found")
	}
	var slides []string
	for i, p := range paths {
		data, err := zipFile(zr, p)
		if err != nil {
			return nil, fmt.Errorf("pptx: read %s: %w", p, err)
		}
		text := fmt.Sprintf("Slide %d:\n", i+1)
		if texts := pptxShapeTexts(string(data)); len(texts) > 0 {
			text += strings.Join(texts, "\n")
		} else {
			text += "(No text content)"
		}
		slides = append(slides, text)
	}
	doc := document.NewDocument(strings.Join(slides, "\n\n"))
	doc.Name = name
	doc.ContentType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	doc.Metadata["slide_count"] = len(paths)
	return doc, nil
}

var _ Reader = (*PPTXReader)(nil)
