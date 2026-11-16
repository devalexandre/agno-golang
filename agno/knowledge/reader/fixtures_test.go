package reader

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// zipBytes builds an in-memory zip from name→content.
func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeBytes(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const relNS = `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

// sampleXLSX: sheet "People" (strings, numbers, a bool, a date, a multi-line
// cell, a gap column), sheet "Empty", sheet "Notes" (inline string).
func sampleXLSX(t *testing.T) []byte {
	return zipBytes(t, map[string]string{
		"xl/workbook.xml": `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` + relNS + `><sheets>` +
			`<sheet name="People" sheetId="1" r:id="rId1"/><sheet name="Empty" sheetId="2" r:id="rId2"/><sheet name="Notes" sheetId="3" r:id="rId3"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Target="worksheets/sheet2.xml"/><Relationship Id="rId3" Target="/xl/worksheets/sheet3.xml"/></Relationships>`,
		"xl/sharedStrings.xml": `<?xml version="1.0"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
			`<si><t>name</t></si><si><t>age</t></si><si><r><t>Ali</t></r><r><t>ce</t></r></si><si><t xml:space="preserve">line1
line2</t></si><si><t>joined</t></si></sst>`,
		"xl/styles.xml": `<?xml version="1.0"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cellXfs count="2"><xf numFmtId="0"/><xf numFmtId="14"/></cellXfs></styleSheet>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
			`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="D1" t="s"><v>4</v></c></row>` +
			`<row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2"><v>30</v></c><c r="C2" t="b"><v>1</v></c><c r="D2" s="1"><v>45306</v></c></row>` +
			`<row r="3"><c r="A3" t="s"><v>3</v></c><c r="B3"><v>2.5</v></c><c r="C3"/></row>` +
			`<row r="5"></row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml": `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`,
		"xl/worksheets/sheet3.xml": `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
			`<row r="1"><c r="A1" t="inlineStr"><is><t>hello &amp; bye</t></is></c></row></sheetData></worksheet>`,
	})
}

func pptxSlide(body string) string {
	return `<?xml version="1.0"?><p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree>` +
		body + `</p:spTree></p:cSld></p:sld>`
}

func pptxShapeXML(paras ...string) string {
	s := `<p:sp><p:nvSpPr><p:cNvPr id="1" name="x"/></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/>`
	for _, p := range paras {
		s += `<a:p><a:pPr/><a:r><a:rPr lang="en"/><a:t>` + p + `</a:t></a:r></a:p>`
	}
	return s + `</p:txBody></p:sp>`
}

// samplePPTX: slide order comes from presentation.xml (slide2.xml is first).
func samplePPTX(t *testing.T) []byte {
	return zipBytes(t, map[string]string{
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" ` + relNS +
			`><p:sldIdLst><p:sldId id="256" r:id="rId7"/><p:sldId id="257" r:id="rId8"/><p:sldId id="258" r:id="rId9"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId7" Target="slides/slide2.xml"/><Relationship Id="rId8" Target="slides/slide1.xml"/><Relationship Id="rId9" Target="slides/slide3.xml"/></Relationships>`,
		"ppt/slides/slide2.xml": pptxSlide(pptxShapeXML("Quarterly Review") + pptxShapeXML("Revenue up", "Costs &amp; risks")),
		"ppt/slides/slide1.xml": pptxSlide(`<p:grpSp><p:nvGrpSpPr/>` + pptxShapeXML("Inside group") + `</p:grpSp>` + pptxShapeXML("   ")),
		"ppt/slides/slide3.xml": pptxSlide(``),
	})
}

// samplePDF builds a tiny PDF whose content stream is Flate-compressed.
func samplePDF(t *testing.T, lines ...string) []byte {
	var content bytes.Buffer
	content.WriteString("BT /F1 12 Tf 72 720 Td ")
	for i, l := range lines {
		if i > 0 {
			content.WriteString("0 -14 Td ")
		}
		fmt.Fprintf(&content, "(%s) Tj ", l)
	}
	content.WriteString("[(Kern)-300(ed)] TJ ET")
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
