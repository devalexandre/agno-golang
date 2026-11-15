// Package filegeneration ports Agno's FileGenerationTools: generate JSON,
// CSV, PDF, DOCX, text, HTML and source-code files as in-memory artifacts,
// optionally saving them to an output directory.
//
// PDF and DOCX are produced by small pure-Go writers (no reportlab /
// python-docx equivalent needed): PDF uses the standard Helvetica font with
// WinAnsi (Latin-1) text, word wrapping and pagination; DOCX is a minimal
// WordprocessingML package.
package filegeneration

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// CodeLanguageMap maps language names to (extension, mime type).
var CodeLanguageMap = map[string][2]string{
	"python": {"py", "text/x-python"}, "py": {"py", "text/x-python"},
	"javascript": {"js", "text/javascript"}, "js": {"js", "text/javascript"}, "node": {"js", "text/javascript"},
	"typescript": {"ts", "text/plain"}, "ts": {"ts", "text/plain"}, "jsx": {"jsx", "text/plain"}, "tsx": {"tsx", "text/plain"},
	"go": {"go", "text/plain"}, "golang": {"go", "text/plain"}, "rust": {"rs", "text/plain"}, "rs": {"rs", "text/plain"},
	"java": {"java", "text/plain"}, "kotlin": {"kt", "text/plain"}, "kt": {"kt", "text/plain"}, "scala": {"scala", "text/plain"},
	"c": {"c", "text/plain"}, "cpp": {"cpp", "text/plain"}, "c++": {"cpp", "text/plain"},
	"csharp": {"cs", "text/plain"}, "c#": {"cs", "text/plain"}, "cs": {"cs", "text/plain"}, "objectivec": {"m", "text/plain"},
	"ruby": {"rb", "text/plain"}, "rb": {"rb", "text/plain"}, "php": {"php", "text/plain"}, "perl": {"pl", "text/plain"},
	"shell": {"sh", "text/plain"}, "sh": {"sh", "text/plain"}, "bash": {"sh", "text/plain"}, "zsh": {"sh", "text/plain"},
	"powershell": {"ps1", "text/plain"}, "sql": {"sql", "text/plain"}, "r": {"r", "text/plain"}, "swift": {"swift", "text/plain"},
	"dart": {"dart", "text/plain"}, "lua": {"lua", "text/plain"}, "haskell": {"hs", "text/plain"}, "elixir": {"ex", "text/plain"},
	"clojure": {"clj", "text/plain"}, "yaml": {"yaml", "text/plain"}, "yml": {"yaml", "text/plain"}, "toml": {"toml", "text/plain"},
	"dockerfile": {"dockerfile", "text/plain"},
}

// File is a generated file artifact.
type File struct {
	ID       string `json:"id"`
	Content  []byte `json:"-"`
	MimeType string `json:"mime_type"`
	FileType string `json:"file_type"`
	Filename string `json:"filename"`
	Size     int    `json:"size"`
	Filepath string `json:"filepath,omitempty"`
}

// ToolResult carries the message for the model plus the generated files.
// It renders as Content when formatted with %v or marshalled to JSON.
type ToolResult struct {
	Content string
	Files   []File
}

func (r *ToolResult) String() string { return r.Content }

// MarshalJSON renders the message only (file bytes are not sent to the model).
func (r *ToolResult) MarshalJSON() ([]byte, error) { return json.Marshal(r.Content) }

// FileGenerationTools generates files.
type FileGenerationTools struct {
	toolkit.Toolkit
	outputDirectory string
	saveFiles       bool

	enableJSON, enableCSV, enablePDF, enableDOCX, enableTXT, enableHTML, enableCode bool
}

// Option configures FileGenerationTools.
type Option func(*FileGenerationTools)

// WithOutputDirectory saves files into dir (implies save_files).
func WithOutputDirectory(dir string) Option {
	return func(f *FileGenerationTools) { f.outputDirectory, f.saveFiles = dir, true }
}

// WithSaveFiles saves files to disk (current directory unless WithOutputDirectory).
func WithSaveFiles(b bool) Option { return func(f *FileGenerationTools) { f.saveFiles = b } }

// WithJSON toggles GenerateJSONFile (default true).
func WithJSON(b bool) Option { return func(f *FileGenerationTools) { f.enableJSON = b } }

// WithCSV toggles GenerateCSVFile (default true).
func WithCSV(b bool) Option { return func(f *FileGenerationTools) { f.enableCSV = b } }

// WithPDF toggles GeneratePDFFile (default true).
func WithPDF(b bool) Option { return func(f *FileGenerationTools) { f.enablePDF = b } }

// WithDOCX toggles GenerateDocxFile (default true).
func WithDOCX(b bool) Option { return func(f *FileGenerationTools) { f.enableDOCX = b } }

// WithText toggles GenerateTextFile (default true).
func WithText(b bool) Option { return func(f *FileGenerationTools) { f.enableTXT = b } }

// WithHTML toggles GenerateHTMLFile (default true).
func WithHTML(b bool) Option { return func(f *FileGenerationTools) { f.enableHTML = b } }

// WithCode toggles GenerateCodeFile (default true).
func WithCode(b bool) Option { return func(f *FileGenerationTools) { f.enableCode = b } }

// JSONParams for GenerateJSONFile.
type JSONParams struct {
	Data     interface{} `json:"data" description:"The data to write to the JSON file. Can be a dictionary, list, or JSON string." required:"true"`
	Filename string      `json:"filename,omitempty" description:"Optional filename for the generated file. If not provided, a UUID will be used."`
}

// CSVParams for GenerateCSVFile.
type CSVParams struct {
	Data     interface{} `json:"data" description:"The data to write to the CSV file. Can be a list of lists, list of dictionaries, or CSV string." required:"true"`
	Filename string      `json:"filename,omitempty" description:"Optional filename for the generated file. If not provided, a UUID will be used."`
	Headers  []string    `json:"headers,omitempty" description:"Optional headers for the CSV. Used when data is a list of lists."`
}

// DocParams for GeneratePDFFile / GenerateDocxFile.
type DocParams struct {
	Content  string `json:"content" description:"The text content to write to the document. Paragraphs are separated by blank lines." required:"true"`
	Filename string `json:"filename,omitempty" description:"Optional filename for the generated file. If not provided, a UUID will be used."`
	Title    string `json:"title,omitempty" description:"Optional title for the document."`
}

// TextParams for GenerateTextFile.
type TextParams struct {
	Content  string `json:"content" description:"The text content to write to the file." required:"true"`
	Filename string `json:"filename,omitempty" description:"Optional filename for the generated file. If not provided, a UUID will be used."`
}

// HTMLParams for GenerateHTMLFile.
type HTMLParams struct {
	Content  string `json:"content" description:"A complete, valid HTML5 document (including doctype, html, head, and body tags)." required:"true"`
	Filename string `json:"filename,omitempty" description:"Optional filename for the generated file. If not provided, a UUID will be used."`
}

// CodeParams for GenerateCodeFile.
type CodeParams struct {
	Code     string `json:"code" description:"The source code to write to the file." required:"true"`
	Language string `json:"language,omitempty" description:"Programming language (e.g. \"python\", \"typescript\", \"go\"). Determines the file extension."`
	Filename string `json:"filename,omitempty" description:"Optional filename. If it has no extension, the language's extension is appended. If not provided, a UUID-based name is used."`
}

// NewFileGenerationTools creates the toolkit.
func NewFileGenerationTools(opts ...Option) (*FileGenerationTools, error) {
	f := &FileGenerationTools{enableJSON: true, enableCSV: true, enablePDF: true, enableDOCX: true, enableTXT: true, enableHTML: true, enableCode: true}
	f.Toolkit = toolkit.NewToolkit()
	f.Toolkit.Name = "file_generation"
	f.Toolkit.Description = "Generate JSON, CSV, PDF, DOCX, text, HTML and source-code files."
	for _, o := range opts {
		o(f)
	}
	if f.saveFiles {
		dir := f.outputDirectory
		if dir == "" {
			dir = "."
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return nil, err
		}
		f.outputDirectory = abs
	}
	r := f.Toolkit.Register
	if f.enableJSON {
		r("GenerateJSONFile", "Generate a JSON file from the provided data.", f, f.GenerateJSONFile, JSONParams{})
	}
	if f.enableCSV {
		r("GenerateCSVFile", "Generate a CSV file from the provided data.", f, f.GenerateCSVFile, CSVParams{})
	}
	if f.enablePDF {
		r("GeneratePDFFile", "Generate a PDF file from the provided content.", f, f.GeneratePDFFile, DocParams{})
	}
	if f.enableDOCX {
		r("GenerateDocxFile", "Generate a DOCX file from the provided content.", f, f.GenerateDocxFile, DocParams{})
	}
	if f.enableTXT {
		r("GenerateTextFile", "Generate a text file from the provided content.", f, f.GenerateTextFile, TextParams{})
	}
	if f.enableHTML {
		r("GenerateHTMLFile", "Generate an HTML file from the provided content.", f, f.GenerateHTMLFile, HTMLParams{})
	}
	if f.enableCode {
		r("GenerateCodeFile", "Generate a source code file for any programming language.", f, f.GenerateCodeFile, CodeParams{})
	}
	return f, nil
}

func uuid4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// SanitizeFilename returns a safe basename (path components discarded).
func SanitizeFilename(name string) (string, error) {
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("Invalid filename: %q", name)
		}
	}
	if len(name) >= 2 && name[1] == ':' {
		return "", fmt.Errorf("Invalid filename: %q", name)
	}
	name = strings.ReplaceAll(name, "\\", "/")
	safe := strings.TrimRight(filepath.Base(name), ". ")
	if safe == "" || strings.Trim(safe, ".") == "" || safe == "/" {
		return "", fmt.Errorf("Invalid filename: %q", name)
	}
	return safe, nil
}

func (f *FileGenerationTools) artifact(content []byte, isText bool, filename, fileType, mime, display string) (interface{}, error) {
	if filename == "" {
		filename = fmt.Sprintf("generated_file_%s.%s", uuid4()[:8], fileType)
	} else if !strings.HasSuffix(strings.ToLower(filename), "."+strings.ToLower(fileType)) {
		filename += "." + fileType
	}
	name, err := SanitizeFilename(filename)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("Error generating %s file: %v", display, err)}, nil
	}
	path, saveErr := "", ""
	if f.saveFiles {
		p := filepath.Join(f.outputDirectory, name)
		if err := os.WriteFile(p, content, 0o644); err != nil {
			saveErr = err.Error()
		} else {
			path = p
		}
	}
	count, unit := len(content), "bytes"
	if isText {
		count, unit = utf8.RuneCount(content), "characters"
	}
	msg := fmt.Sprintf("%s file '%s' generated (%d %s)", display, name, count, unit)
	if path != "" {
		msg += " → " + path
	} else if saveErr != "" {
		msg += " (save failed: " + saveErr + ")"
	}
	return &ToolResult{Content: msg, Files: []File{{ID: uuid4(), Content: content, MimeType: mime, FileType: fileType, Filename: name, Size: len(content), Filepath: path}}}, nil
}

// GenerateJSONFile writes data as JSON.
func (f *FileGenerationTools) GenerateJSONFile(p JSONParams) (interface{}, error) {
	var content string
	if s, ok := p.Data.(string); ok {
		if json.Valid([]byte(s)) {
			content = s
		} else {
			b, _ := json.MarshalIndent(map[string]string{"content": s}, "", "  ")
			content = string(b)
		}
	} else {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(p.Data); err != nil {
			return &ToolResult{Content: fmt.Sprintf("Error generating JSON file: %v", err)}, nil
		}
		content = strings.TrimRight(buf.String(), "\n")
	}
	return f.artifact([]byte(content), true, p.Filename, "json", "application/json", "JSON")
}

// GenerateCSVFile writes data as CSV.
func (f *FileGenerationTools) GenerateCSVFile(p CSVParams) (interface{}, error) {
	var content string
	switch d := p.Data.(type) {
	case string:
		content = d
	case []interface{}:
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if len(d) > 0 {
			if first, ok := d[0].(map[string]interface{}); ok {
				fields := make([]string, 0, len(first))
				for k := range first {
					fields = append(fields, k)
				}
				sort.Strings(fields)
				_ = w.Write(fields)
				for _, row := range d {
					rec := make([]string, len(fields))
					if m, ok := row.(map[string]interface{}); ok {
						for i, k := range fields {
							rec[i] = cell(m[k])
						}
					} else if len(rec) > 0 {
						rec[0] = cell(row)
					}
					_ = w.Write(rec)
				}
			} else {
				if len(p.Headers) > 0 {
					_ = w.Write(p.Headers)
				}
				for _, row := range d {
					if list, ok := row.([]interface{}); ok {
						rec := make([]string, len(list))
						for i, v := range list {
							rec[i] = cell(v)
						}
						_ = w.Write(rec)
					} else {
						_ = w.Write([]string{cell(row)})
					}
				}
			}
		}
		w.Flush()
		content = buf.String()
	}
	return f.artifact([]byte(content), true, p.Filename, "csv", "text/csv", "CSV")
}

func cell(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprint(t)
	case bool:
		if t {
			return "True"
		}
		return "False"
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// GeneratePDFFile renders text paragraphs into a PDF.
func (f *FileGenerationTools) GeneratePDFFile(p DocParams) (interface{}, error) {
	return f.artifact(RenderPDF(p.Title, p.Content), false, p.Filename, "pdf", "application/pdf", "PDF")
}

// GenerateDocxFile renders text paragraphs into a DOCX.
func (f *FileGenerationTools) GenerateDocxFile(p DocParams) (interface{}, error) {
	data, err := RenderDOCX(p.Title, p.Content)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("Error generating DOCX file: %v", err)}, nil
	}
	return f.artifact(data, false, p.Filename, "docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "DOCX")
}

// GenerateTextFile writes a plain-text file.
func (f *FileGenerationTools) GenerateTextFile(p TextParams) (interface{}, error) {
	return f.artifact([]byte(p.Content), true, p.Filename, "txt", "text/plain", "Text")
}

// GenerateHTMLFile writes an HTML file.
func (f *FileGenerationTools) GenerateHTMLFile(p HTMLParams) (interface{}, error) {
	return f.artifact([]byte(p.Content), true, p.Filename, "html", "text/html", "HTML")
}

// GenerateCodeFile writes a source file with an extension derived from language/filename.
func (f *FileGenerationTools) GenerateCodeFile(p CodeParams) (interface{}, error) {
	fileType, mime := "", ""
	if p.Language != "" {
		if m, ok := CodeLanguageMap[strings.ToLower(strings.TrimSpace(p.Language))]; ok {
			fileType, mime = m[0], m[1]
		}
	}
	if p.Filename != "" {
		base := strings.ReplaceAll(p.Filename, "\\", "/")
		base = base[strings.LastIndex(base, "/")+1:]
		if i := strings.LastIndex(base, "."); i >= 0 {
			ext := strings.ToLower(strings.TrimSpace(base[i+1:]))
			if ext != "" && ext != fileType {
				fileType, mime = ext, "text/plain"
			}
		}
	}
	if fileType == "" {
		fileType, mime = "txt", "text/plain"
	}
	display := p.Language
	if display == "" {
		display = fileType
	}
	return f.artifact([]byte(p.Code), true, p.Filename, fileType, mime, display+" code")
}

// ---- PDF writer ----

// helveticaWidths are glyph widths (1/1000 em) for ASCII 32..126.
var helveticaWidths = [95]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
}

func textWidth(s string, size float64) float64 {
	w := 0
	for _, r := range s {
		if r >= 32 && r <= 126 {
			w += helveticaWidths[r-32]
		} else {
			w += 556
		}
	}
	return float64(w) * size / 1000
}

func pdfEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '(' || r == ')' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 32:
			b.WriteByte(' ')
		case r < 128:
			b.WriteRune(r)
		case r < 256:
			fmt.Fprintf(&b, "\\%03o", r)
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

func wrap(text string, size, maxWidth float64) []string {
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		words := strings.Fields(raw)
		line := ""
		for _, w := range words {
			cand := w
			if line != "" {
				cand = line + " " + w
			}
			if textWidth(cand, size) <= maxWidth || line == "" {
				line = cand
				continue
			}
			lines = append(lines, line)
			line = w
		}
		lines = append(lines, line)
	}
	return lines
}

// RenderPDF builds a US-Letter PDF with an optional title and paragraphs
// separated by blank lines.
func RenderPDF(title, content string) []byte {
	const pageW, pageH, margin = 612.0, 792.0, 72.0
	type line struct {
		text string
		size float64
		bold bool
		x    float64
	}
	var flow []line
	var gaps []float64
	add := func(l line, gapAfter float64) { flow = append(flow, l); gaps = append(gaps, gapAfter) }
	if strings.TrimSpace(title) != "" {
		tl := wrap(title, 18, pageW-2*margin)
		for i, t := range tl {
			gap := 4.0
			if i == len(tl)-1 {
				gap = 20
			}
			add(line{t, 18, true, (pageW - textWidth(t, 18)) / 2}, gap)
		}
	}
	for _, para := range strings.Split(content, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		pl := wrap(para, 10, pageW-2*margin)
		for i, t := range pl {
			gap := 2.0
			if i == len(pl)-1 {
				gap = 10
			}
			add(line{t, 10, false, margin}, gap)
		}
	}
	var pages []string
	var cur strings.Builder
	y := pageH - margin
	for i, l := range flow {
		lead := l.size * 1.2
		if y-lead < margin && cur.Len() > 0 {
			pages = append(pages, cur.String())
			cur.Reset()
			y = pageH - margin
		}
		y -= lead
		font := "F1"
		if l.bold {
			font = "F2"
		}
		fmt.Fprintf(&cur, "BT /%s %.0f Tf %.2f %.2f Td (%s) Tj ET\n", font, l.size, l.x, y, pdfEscape(l.text))
		y -= gaps[i]
	}
	pages = append(pages, cur.String())

	var buf bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	n := len(pages)
	// 1 catalog, 2 pages, 3 F1, 4 F2, then page/content pairs.
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	kids := make([]string, n)
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 5+2*i)
	}
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	for i, content := range pages {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>", pageW, pageH, 6+2*i))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content))
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return buf.Bytes()
}

// ---- DOCX writer ----

// RenderDOCX builds a minimal DOCX with an optional title paragraph.
func RenderDOCX(title, content string) ([]byte, error) {
	var body strings.Builder
	para := func(text string) {
		body.WriteString(`<w:p><w:r><w:t xml:space="preserve">`)
		body.WriteString(html.EscapeString(text))
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	if title != "" {
		para(title)
	}
	for _, p := range strings.Split(content, "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			para(p)
		}
	}
	files := []struct{ name, data string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`},
		{"word/document.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() + `</w:body></w:document>`},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, fl := range files {
		w, err := zw.Create(fl.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(fl.data)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
