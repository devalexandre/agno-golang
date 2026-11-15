package reader

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
)

// ExcelReader reads Excel workbooks, emitting one document per non-empty
// sheet whose content is one CSV-like line per non-empty row ("a, b, c").
// Port of agno-python knowledge/reader/excel_reader.py.
//
// .xlsx/.xlsm (Office Open XML) are parsed with the standard library only
// (zip + XML: shared strings, inline strings, booleans, and date-styled
// numbers rendered as ISO-8601 like openpyxl's datetime.isoformat()).
// Legacy binary .xls (BIFF8) is not supported: convert it to .xlsx or use
// DoclingReader.
type ExcelReader struct {
	BaseReader
	// Sheets filters the sheets to read. Each entry is either a sheet name
	// (string, case-insensitive) or a 1-based sheet index (int). Empty = all.
	Sheets []any
}

// NewExcelReader creates an Excel reader.
func NewExcelReader() *ExcelReader {
	return &ExcelReader{BaseReader: BaseReader{ID: "excel", Extensions: []string{".xlsx", ".xlsm", ".xls"}}}
}

// WithSheets sets the sheet filter (names and/or 1-based indexes).
func (r *ExcelReader) WithSheets(sheets ...any) *ExcelReader {
	r.Sheets = sheets
	return r
}

func (r *ExcelReader) includeSheet(name string, index int) bool {
	if len(r.Sheets) == 0 {
		return true
	}
	for _, f := range r.Sheets {
		switch v := f.(type) {
		case int:
			if v == index {
				return true
			}
		case string:
			if strings.EqualFold(v, name) {
				return true
			}
		}
	}
	return false
}

// Read reads the workbook at source (a file path).
func (r *ExcelReader) Read(_ context.Context, source string) ([]*document.Document, error) {
	ext := strings.ToLower(filepath.Ext(source))
	switch ext {
	case ".xlsx", ".xlsm":
	case ".xls":
		return nil, fmt.Errorf("excel: legacy .xls (BIFF) files are not supported; convert %s to .xlsx or use DoclingReader", source)
	default:
		return nil, fmt.Errorf("excel: unsupported file extension %q, expected .xlsx or .xls", ext)
	}
	zr, err := zip.OpenReader(source)
	if err != nil {
		return nil, fmt.Errorf("excel: open: %w", err)
	}
	defer zr.Close()
	name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	docs, err := r.readXLSX(&zr.Reader, name)
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		d.Source = source
		d.Metadata["source"] = source
	}
	return docs, nil
}

// ReadBytes reads an in-memory .xlsx workbook; name is used for the document name.
func (r *ExcelReader) ReadBytes(_ context.Context, data []byte, name string) ([]*document.Document, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("excel: open: %w", err)
	}
	stem := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if stem == "" {
		stem = "workbook"
	}
	return r.readXLSX(zr, stem)
}

type xlsxWorkbook struct {
	WorkbookPr struct {
		Date1904 string `xml:"date1904,attr"`
	} `xml:"workbookPr"`
	Sheets []struct {
		Name string `xml:"name,attr"`
		RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	} `xml:"sheets>sheet"`
}

type xlsxRels struct {
	Rels []struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

type xlsxRichText struct {
	T *string `xml:"t"`
	R []struct {
		T string `xml:"t"`
	} `xml:"r"`
}

func (s xlsxRichText) text() string {
	if s.T != nil {
		return *s.T
	}
	var b strings.Builder
	for _, r := range s.R {
		b.WriteString(r.T)
	}
	return b.String()
}

type xlsxSST struct {
	SI []xlsxRichText `xml:"si"`
}

type xlsxStyles struct {
	NumFmts []struct {
		ID   int    `xml:"numFmtId,attr"`
		Code string `xml:"formatCode,attr"`
	} `xml:"numFmts>numFmt"`
	CellXfs []struct {
		NumFmtID int `xml:"numFmtId,attr"`
	} `xml:"cellXfs>xf"`
}

type xlsxSheet struct {
	Rows []struct {
		Cells []struct {
			Ref  string       `xml:"r,attr"`
			Type string       `xml:"t,attr"`
			S    int          `xml:"s,attr"`
			V    *string      `xml:"v"`
			Is   xlsxRichText `xml:"is"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

func zipFile(zr *zip.Reader, name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "/")
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, nil
}

var dateCodeStrip = regexp.MustCompile(`"[^"]*"|\[[^\]]*\]|\\.`)

func isDateFormat(id int, code string) bool {
	if (id >= 14 && id <= 22) || (id >= 45 && id <= 47) || (id >= 27 && id <= 36) || (id >= 50 && id <= 58) {
		return true
	}
	if code == "" {
		return false
	}
	c := strings.ToLower(dateCodeStrip.ReplaceAllString(code, ""))
	return strings.ContainsAny(c, "dmyhs")
}

func (r *ExcelReader) readXLSX(zr *zip.Reader, workbookName string) ([]*document.Document, error) {
	wbData, err := zipFile(zr, "xl/workbook.xml")
	if err != nil || wbData == nil {
		return nil, fmt.Errorf("excel: xl/workbook.xml not found or unreadable: %v", err)
	}
	var wb xlsxWorkbook
	if err := xml.Unmarshal(wbData, &wb); err != nil {
		return nil, fmt.Errorf("excel: parse workbook.xml: %w", err)
	}
	targets := map[string]string{}
	if relData, _ := zipFile(zr, "xl/_rels/workbook.xml.rels"); relData != nil {
		var rels xlsxRels
		if err := xml.Unmarshal(relData, &rels); err == nil {
			for _, rel := range rels.Rels {
				t := rel.Target
				if strings.HasPrefix(t, "/") {
					t = strings.TrimPrefix(t, "/")
				} else {
					t = path.Join("xl", t)
				}
				targets[rel.ID] = t
			}
		}
	}
	var shared []string
	if sstData, _ := zipFile(zr, "xl/sharedStrings.xml"); sstData != nil {
		var sst xlsxSST
		if err := xml.Unmarshal(sstData, &sst); err == nil {
			for _, si := range sst.SI {
				shared = append(shared, si.text())
			}
		}
	}
	var dateStyles []bool
	if stData, _ := zipFile(zr, "xl/styles.xml"); stData != nil {
		var st xlsxStyles
		if err := xml.Unmarshal(stData, &st); err == nil {
			custom := map[int]string{}
			for _, f := range st.NumFmts {
				custom[f.ID] = f.Code
			}
			for _, xf := range st.CellXfs {
				dateStyles = append(dateStyles, isDateFormat(xf.NumFmtID, custom[xf.NumFmtID]))
			}
		}
	}
	date1904 := wb.WorkbookPr.Date1904 == "1" || wb.WorkbookPr.Date1904 == "true"

	var docs []*document.Document
	for i, sh := range wb.Sheets {
		index := i + 1
		if !r.includeSheet(sh.Name, index) {
			continue
		}
		target, ok := targets[sh.RID]
		if !ok {
			target = fmt.Sprintf("xl/worksheets/sheet%d.xml", index)
		}
		data, err := zipFile(zr, target)
		if err != nil || data == nil {
			continue
		}
		var sheet xlsxSheet
		if err := xml.Unmarshal(data, &sheet); err != nil {
			return nil, fmt.Errorf("excel: parse sheet %q: %w", sh.Name, err)
		}
		var lines []string
		for _, row := range sheet.Rows {
			var values []string
			for ci, c := range row.Cells {
				col := ci
				if c.Ref != "" {
					if idx := columnIndex(c.Ref); idx >= 0 {
						col = idx
					}
				}
				for len(values) < col {
					values = append(values, "")
				}
				isDate := c.S >= 0 && c.S < len(dateStyles) && dateStyles[c.S]
				v := xlsxCellValue(c.Type, c.V, c.Is, shared, isDate, date1904)
				if col < len(values) {
					values[col] = v
				} else {
					values = append(values, v)
				}
			}
			if line := rowToCSVLine(values); line != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) == 0 {
			continue
		}
		doc := document.NewDocument(strings.Join(lines, "\n"))
		doc.Name = workbookName
		doc.ContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		doc.Metadata["sheet_name"] = sh.Name
		doc.Metadata["sheet_index"] = index
		docs = append(docs, doc)
	}
	return docs, nil
}

// columnIndex converts the letters of a cell reference ("C7") to a 0-based column.
func columnIndex(ref string) int {
	n := 0
	seen := false
	for _, ch := range ref {
		if ch >= 'A' && ch <= 'Z' {
			n = n*26 + int(ch-'A'+1)
			seen = true
		} else if ch >= 'a' && ch <= 'z' {
			n = n*26 + int(ch-'a'+1)
			seen = true
		} else {
			break
		}
	}
	if !seen {
		return -1
	}
	return n - 1
}

func xlsxCellValue(typ string, v *string, is xlsxRichText, shared []string, isDate, date1904 bool) string {
	switch typ {
	case "inlineStr":
		return stringifyCell(is.text())
	case "s":
		if v == nil {
			return ""
		}
		i, err := strconv.Atoi(strings.TrimSpace(*v))
		if err != nil || i < 0 || i >= len(shared) {
			return ""
		}
		return stringifyCell(shared[i])
	case "b":
		if v != nil && strings.TrimSpace(*v) == "1" {
			return "True"
		}
		return "False"
	case "str", "e":
		if v == nil {
			return ""
		}
		return stringifyCell(*v)
	}
	if v == nil {
		return ""
	}
	raw := strings.TrimSpace(*v)
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return stringifyCell(raw)
	}
	if isDate {
		return excelSerialToTime(f, date1904).Format("2006-01-02T15:04:05")
	}
	return formatNumber(f)
}

func excelSerialToTime(serial float64, date1904 bool) time.Time {
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	if date1904 {
		epoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	days := math.Floor(serial)
	secs := math.Round((serial - days) * 86400)
	return epoch.AddDate(0, 0, int(days)).Add(time.Duration(secs) * time.Second)
}

// formatNumber renders integral floats without a decimal part, like Python's
// stringify_cell_value.
func formatNumber(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// stringifyCell normalizes line endings to spaces so a cell never breaks a row.
func stringifyCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\n", " ")
}

// rowToCSVLine joins values with ", " after trimming trailing empty cells.
func rowToCSVLine(values []string) string {
	for len(values) > 0 && values[len(values)-1] == "" {
		values = values[:len(values)-1]
	}
	return strings.Join(values, ", ")
}

var _ Reader = (*ExcelReader)(nil)
