// ExcelReader: read an .xlsx workbook, one document per non-empty sheet.
//
// Offline: a small workbook (two sheets, shared strings, a date cell) is
// generated in a temp dir with archive/zip, then read with and without a
// sheet filter. No env vars needed. Legacy .xls (BIFF) is not supported.
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
	path := filepath.Join(os.TempDir(), "agno-sales.xlsx")
	if err := writeSampleXLSX(path); err != nil {
		log.Fatal(err)
	}
	defer os.Remove(path)

	docs, err := reader.NewExcelReader().Read(context.Background(), path)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range docs {
		fmt.Printf("== %s / sheet %v (#%v)\n%s\n\n", d.Name, d.Metadata["sheet_name"], d.Metadata["sheet_index"], d.Content)
	}

	// Only the "Summary" sheet (names are case-insensitive; ints are 1-based indexes).
	only, err := reader.NewExcelReader().WithSheets("summary").Read(context.Background(), path)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("filtered: %d doc(s): %q\n", len(only), only[0].Content)
}

func writeSampleXLSX(path string) error {
	const ns = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`
	files := map[string]string{
		"xl/workbook.xml": `<workbook ` + ns + ` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` +
			`<sheet name="Sales" sheetId="1" r:id="rId1"/><sheet name="Summary" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Target="worksheets/sheet2.xml"/></Relationships>`,
		"xl/sharedStrings.xml": `<sst ` + ns + `><si><t>product</t></si><si><t>units</t></si><si><t>date</t></si><si><t>Widget</t></si></sst>`,
		"xl/styles.xml":        `<styleSheet ` + ns + `><cellXfs><xf numFmtId="0"/><xf numFmtId="14"/></cellXfs></styleSheet>`,
		"xl/worksheets/sheet1.xml": `<worksheet ` + ns + `><sheetData>` +
			`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="s"><v>2</v></c></row>` +
			`<row r="2"><c r="A2" t="s"><v>3</v></c><c r="B2"><v>42</v></c><c r="C2" s="1"><v>45306</v></c></row>` +
			`</sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml": `<worksheet ` + ns + `><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Total units: 42</t></is></c></row></sheetData></worksheet>`,
	}
	return writeZip(path, files)
}

func writeZip(path string, files map[string]string) error {
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
