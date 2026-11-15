package functions

import (
	"strings"
	"testing"

	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func prev(content interface{}) *v2.StepInput {
	return &v2.StepInput{Message: "workflow input", PreviousStepContent: content}
}

func run(t *testing.T, fn v2.ExecutorFunc, in *v2.StepInput) *v2.StepOutput {
	t.Helper()
	out, err := fn(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name string
		in   *v2.StepInput
		want string
	}{
		{"object in prose", prev(`Here you go: {"b": 1, "a": [true, null, 1.0]} done`),
			"{\n  \"b\": 1,\n  \"a\": [\n    true,\n    null,\n    1.0\n  ]\n}"},
		{"largest wins over citations", prev(`See [1] and [ ] then [{"x": "é<>"}]`),
			"[\n  {\n    \"x\": \"é<>\"\n  }\n]"},
		{"nested values are skipped", prev(`{"a": {"b": [1, 2]}} [3]`),
			"{\n  \"a\": {\n    \"b\": [\n      1,\n      2\n    ]\n  }\n}"},
		{"empty containers", prev(`{"a": {}, "b": []}`), "{\n  \"a\": {},\n  \"b\": []\n}"},
		{"falls back to workflow input", &v2.StepInput{Message: `[1,2]`}, "[\n  1,\n  2\n]"},
		{"no json", prev("nothing here [unclosed"), "Error: no valid JSON object or array in the previous step's output"},
		{"upstream error passes through", prev("Error: boom {\"a\":1}"), "Error: boom {\"a\":1}"},
		{"numbers like python", prev(`[1E2, -0, 12345678901234567890, 1e-5]`), "[\n  100.0,\n  0,\n  12345678901234567890,\n  1e-05\n]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, ExtractJSON, tt.in).Content; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"dedupe and order", "a https://b.com/x, then http://a.org. again https://b.com/x", "https://b.com/x\nhttp://a.org"},
		{"stops at delimiters", `<a href="https://c.io/p?q=1">(https://d.io/y)</a> [https://e.io]`, "https://c.io/p?q=1\nhttps://d.io/y\nhttps://e.io"},
		{"trims markdown punctuation", "**https://f.io/z**! `https://g.io`", "https://f.io/z\nhttps://g.io"},
		{"no urls", "ftp://nope", "Error: no URLs in the previous step's output"},
		{"upstream error", "Error: x https://a.b", "Error: x https://a.b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, ExtractURLs, prev(tt.in)).Content; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONToCSV(t *testing.T) {
	tests := []struct {
		name      string
		in        interface{}
		want      string
		wantFiles bool
	}{
		{"union of keys in order", `[{"b": 1, "a": "x"}, {"a": "y", "c": true}]`,
			"b,a,c\r\n1,x,\r\n,y,True\r\n", true},
		{"quoting and nested", `[{"t": "a,b", "q": "say \"hi\"", "n": {"k": [1, 2]}, "s": " lead"}]`,
			"t,q,n,s\r\n\"a,b\",\"say \"\"hi\"\"\",\"{\"\"k\"\": [1, 2]}\", lead\r\n", true},
		{"single array wrapper", "rows: {\"rows\": [{\"a\": 1.5}], \"meta\": 1}", "a\r\n1.5\r\n", true},
		{"null cell", `[{"a": null}]`, "a\r\n\"\"\r\n", true},
		{"not objects", `[1, 2]`, "Error: expected a JSON array of objects in the previous step's output", false},
		{"empty array", `[]`, "Error: expected a JSON array of objects in the previous step's output", false},
		{"two arrays not unwrapped", `{"a": [{"x":1}], "b": [{"y":2}]}`, "Error: expected a JSON array of objects in the previous step's output", false},
		{"upstream error", "Error: nope", "Error: nope", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := run(t, JSONToCSV, prev(tt.in))
			if out.Content != tt.want {
				t.Errorf("got %q, want %q", out.Content, tt.want)
			}
			if tt.wantFiles {
				if len(out.Files) != 1 || out.Files[0].Name != "data.csv" || out.Files[0].MimeType != "text/csv" || string(out.Files[0].Data) != tt.want {
					t.Errorf("unexpected files: %+v", out.Files)
				}
			} else if len(out.Files) != 0 {
				t.Errorf("expected no files, got %+v", out.Files)
			}
		})
	}
}

func TestCSVToMarkdownTable(t *testing.T) {
	var many strings.Builder
	many.WriteString("n\n")
	for i := 0; i < 53; i++ {
		many.WriteString("r\n")
	}
	manyWant := "| n |\n| --- |" + strings.Repeat("\n| r |", 50) + "\n... 3 more rows"

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"basic", "a,b\r\n1,2\r\n", "| a | b |\n| --- | --- |\n| 1 | 2 |"},
		{"escapes and newlines", "h\n\"x|y\"\n\" multi\nline \"\n", "| h |\n| --- |\n| x\\|y |\n| multi<br>line |"},
		{"blank lines skipped", "a\n\n1\n", "| a |\n| --- |\n| 1 |"},
		{"bare quote is literal", "a\nsay \"hi\"\n", "| a |\n| --- |\n| say \"hi\" |"},
		{"cap", many.String(), manyWant},
		{"header only", "a,b\n", "Error: expected CSV with a header row and at least one data row"},
		{"empty", "", "Error: expected CSV with a header row and at least one data row"},
		{"stray char after quote", "a\n\"x\"y\n", "Error: invalid CSV or a field exceeds the CSV parser limit"},
		{"unterminated quote", "a\n\"x\n", "Error: invalid CSV or a field exceeds the CSV parser limit"},
		{"field too large", "a\n" + strings.Repeat("x", csvFieldLimit+1), "Error: invalid CSV or a field exceeds the CSV parser limit"},
		{"upstream error", "Error: bad", "Error: bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, CSVToMarkdownTable, prev(tt.in)).Content; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONToCSVThenMarkdown(t *testing.T) {
	csvOut := run(t, JSONToCSV, prev(`[{"name": "a|b", "n": 1}]`))
	md := run(t, CSVToMarkdownTable, prev(csvOut.Content))
	want := "| name | n |\n| --- | --- |\n| a\\|b | 1 |"
	if md.Content != want {
		t.Errorf("got %q, want %q", md.Content, want)
	}
}

func TestContentToFile(t *testing.T) {
	tests := []struct {
		name      string
		in        *v2.StepInput
		want      string
		wantFiles bool
	}{
		{"report", prev("# Report\nbody"), "# Report\nbody", true},
		{"structured content", prev(map[string]interface{}{"a": "<b>"}), "{\n  \"a\": \"<b>\"\n}", true},
		{"workflow input", &v2.StepInput{Message: "hello"}, "hello", true},
		{"blank", prev("  \n\t"), "Error: previous step produced no content to save", false},
		{"upstream error", prev("Error: x"), "Error: x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := run(t, ContentToFile, tt.in)
			if out.Content != tt.want {
				t.Errorf("got %q, want %q", out.Content, tt.want)
			}
			if tt.wantFiles {
				if len(out.Files) != 1 || out.Files[0].Name != "output.md" || out.Files[0].MimeType != "text/markdown" || string(out.Files[0].Data) != tt.want {
					t.Errorf("unexpected files: %+v", out.Files)
				}
			} else if len(out.Files) != 0 {
				t.Errorf("expected no files, got %+v", out.Files)
			}
		})
	}
}

func TestBuiltinsInWorkflow(t *testing.T) {
	extract, _ := v2.NewStep(v2.WithName("extract"), v2.WithExecutor(Builtins["extract_json"]))
	toCSV, _ := v2.NewStep(v2.WithName("csv"), v2.WithExecutor(Builtins["json_to_csv"]))
	table, _ := v2.NewStep(v2.WithName("table"), v2.WithExecutor(Builtins["csv_to_markdown_table"]))
	wf := v2.NewWorkflow(v2.WithWorkflowSteps([]*v2.Step{extract, toCSV, table}))

	resp, err := wf.Run(t.Context(), `result: [{"city": "Lisbon", "pop": 545000}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := "| city | pop |\n| --- | --- |\n| Lisbon | 545000 |"
	if resp.Content != want {
		t.Errorf("got %q, want %q", resp.Content, want)
	}
	if len(resp.Files) != 1 || resp.Files[0].Name != "data.csv" {
		t.Errorf("expected data.csv on the run response, got %+v", resp.Files)
	}
}
