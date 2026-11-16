// Package functions provides deterministic, built-in workflow step functions,
// ported from Agno's agno.workflow.functions.
//
// Every function has the v2.ExecutorFunc signature, so it can be used directly
// as a step executor:
//
//	step, _ := v2.NewStep(v2.WithName("extract_json"), v2.WithExecutor(functions.ExtractJSON))
//
// Each function consumes the previous step's content, falling back to the
// workflow input. Failures are returned as content prefixed with "Error: " so a
// workflow can branch on them; the returned Go error is always nil. An upstream
// "Error: " content passes through unchanged. File-producing functions return
// the data both as content and as a file artifact on StepOutput.Files.
package functions

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/devalexandre/agno-golang/agno/models"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// ErrorPrefix marks a failure returned as step content.
const ErrorPrefix = "Error: "

// TableRowCap is the maximum number of data rows CSVToMarkdownTable renders.
const TableRowCap = 50

// urlPattern mirrors Python's r"https?://[^\s<>\"')\]]+" with Unicode \s.
var urlPattern = regexp.MustCompile(`https?://[^\s\p{Z}\x{85}\x{1c}-\x{1f}<>"')\]]+`)

// Builtins maps each function's upstream name to its executor, for
// applications that let users pick functions by name.
var Builtins = map[string]v2.ExecutorFunc{
	"extract_json":          ExtractJSON,
	"extract_urls":          ExtractURLs,
	"json_to_csv":           JSONToCSV,
	"csv_to_markdown_table": CSVToMarkdownTable,
	"content_to_file":       ContentToFile,
}

func errorText(message string) string { return ErrorPrefix + message }

func text(content string) (*v2.StepOutput, error) {
	return &v2.StepOutput{Content: content}, nil
}

// stepText is the text a function step operates on: the previous step's
// content, else the workflow input.
func stepText(input *v2.StepInput) string {
	if input == nil {
		return ""
	}
	switch c := input.PreviousStepContent.(type) {
	case nil:
		return input.GetMessageAsString()
	case string:
		return c
	case []byte:
		return string(c)
	case fmt.Stringer:
		return c.String()
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(c)
	default:
		if s, err := marshalNoEscape(c, "  "); err == nil {
			return s
		}
		return fmt.Sprint(c)
	}
}

// ExtractJSON returns the largest JSON object or array in the previous step's
// output, validated and pretty-printed (2-space indent, key order preserved).
// It returns "Error: ..." when there is no parseable JSON.
func ExtractJSON(input *v2.StepInput) (*v2.StepOutput, error) {
	t := stepText(input)
	if strings.HasPrefix(t, ErrorPrefix) {
		return text(t)
	}
	value := jsonPayload(t)
	if _, none := value.(noJSON); none {
		return text(errorText("no valid JSON object or array in the previous step's output"))
	}
	return text(dumpJSON(value, true))
}

// ExtractURLs returns the URLs found in the previous step's output,
// deduplicated in order, one per line. Trailing punctuation (".,;:!?`*") is
// trimmed. It returns "Error: ..." when there is no URL.
func ExtractURLs(input *v2.StepInput) (*v2.StepOutput, error) {
	t := stepText(input)
	if strings.HasPrefix(t, ErrorPrefix) {
		return text(t)
	}
	seen := map[string]bool{}
	var urls []string
	for _, match := range urlPattern.FindAllString(t, -1) {
		u := strings.TrimRight(match, ".,;:!?`*")
		if !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		return text(errorText("no URLs in the previous step's output"))
	}
	return text(strings.Join(urls, "\n"))
}

// JSONToCSV converts a JSON array of objects from the previous step's output
// into CSV, returned as content and as a "data.csv" file artifact. Columns are
// the union of the objects' keys in first-appearance order; nested values are
// JSON-encoded in their cell. A single-array wrapper such as {"rows": [...]} is
// unwrapped. It returns "Error: ..." when there is no JSON array of objects.
func JSONToCSV(input *v2.StepInput) (*v2.StepOutput, error) {
	t := stepText(input)
	if strings.HasPrefix(t, ErrorPrefix) {
		return text(t)
	}
	value := jsonPayload(t)
	if obj, ok := value.(*jsonObject); ok {
		var arrays []interface{}
		for _, k := range obj.keys {
			if arr, isArr := obj.vals[k].([]interface{}); isArr {
				arrays = append(arrays, arr)
			}
		}
		if len(arrays) == 1 {
			value = arrays[0]
		}
	}
	arr, ok := value.([]interface{})
	rows := make([]*jsonObject, 0, len(arr))
	for _, item := range arr {
		row, isObj := item.(*jsonObject)
		if !isObj {
			ok = false
			break
		}
		rows = append(rows, row)
	}
	if !ok || len(rows) == 0 {
		return text(errorText("expected a JSON array of objects in the previous step's output"))
	}

	var header []string
	seen := map[string]bool{}
	for _, row := range rows {
		for _, k := range row.keys {
			if !seen[k] {
				seen[k] = true
				header = append(header, k)
			}
		}
	}
	var b strings.Builder
	writeCSVRow(&b, header)
	for _, row := range rows {
		cells := make([]string, len(header))
		for i, k := range header {
			cells[i] = pyStr(row.vals[k])
		}
		writeCSVRow(&b, cells)
	}
	data := b.String()
	return &v2.StepOutput{
		Content: data,
		Files:   []models.File{{Name: "data.csv", MimeType: "text/csv", Data: []byte(data)}},
	}, nil
}

// CSVToMarkdownTable renders CSV text from the previous step's output as a
// markdown table capped at TableRowCap data rows ("... N more rows" follows).
// It returns "Error: ..." when the input is not CSV with a header row and at
// least one data row.
func CSVToMarkdownTable(input *v2.StepInput) (*v2.StepOutput, error) {
	t := stepText(input)
	if strings.HasPrefix(t, ErrorPrefix) {
		return text(t) // a failure upstream stays one failure, not two
	}
	rows, err := readCSV(t)
	if err != nil {
		return text(errorText("invalid CSV or a field exceeds the CSV parser limit"))
	}
	if len(rows) < 2 {
		return text(errorText("expected CSV with a header row and at least one data row"))
	}
	line := func(cells []string) string {
		out := make([]string, len(cells))
		for i, c := range cells {
			out[i] = tableCell(c)
		}
		return "| " + strings.Join(out, " | ") + " |"
	}
	header := rows[0]
	sep := make([]string, len(header))
	for i := range sep {
		sep[i] = "---"
	}
	lines := []string{line(header), "| " + strings.Join(sep, " | ") + " |"}
	data := rows[1:]
	for i, row := range data {
		if i >= TableRowCap {
			break
		}
		lines = append(lines, line(row))
	}
	if len(data) > TableRowCap {
		lines = append(lines, fmt.Sprintf("... %d more rows", len(data)-TableRowCap))
	}
	return text(strings.Join(lines, "\n"))
}

// ContentToFile attaches the previous step's output unchanged as an
// "output.md" file artifact; the content also flows through as the step's
// output. It returns "Error: ..." when there is no content.
func ContentToFile(input *v2.StepInput) (*v2.StepOutput, error) {
	t := stepText(input)
	if strings.HasPrefix(t, ErrorPrefix) {
		return text(t)
	}
	if strings.TrimFunc(t, unicode.IsSpace) == "" {
		return text(errorText("previous step produced no content to save"))
	}
	return &v2.StepOutput{
		Content: t,
		Files:   []models.File{{Name: "output.md", MimeType: "text/markdown", Data: []byte(t)}},
	}, nil
}

func tableCell(cell string) string {
	cell = strings.TrimFunc(cell, unicode.IsSpace)
	cell = strings.ReplaceAll(cell, "|", `\|`)
	cell = strings.ReplaceAll(cell, "\r\n", "\n")
	return strings.ReplaceAll(cell, "\n", "<br>")
}
