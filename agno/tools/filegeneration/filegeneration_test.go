package filegeneration

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileGeneration(t *testing.T) {
	dir := t.TempDir()
	f, err := NewFileGenerationTools(WithOutputDirectory(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.GetMethods()) != 7 {
		t.Fatal(len(f.GetMethods()))
	}
	out, _ := f.Execute("file_generation_GenerateJSONFile", []byte(`{"data":{"a":"<b>"},"filename":"../../evil"}`))
	r := out.(*ToolResult)
	if r.Files[0].Filename != "evil.json" || !strings.Contains(fmt.Sprint(out), "→ "+filepath.Join(dir, "evil.json")) {
		t.Fatal(r.Content, r.Files[0].Filename)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "evil.json")); string(data) != "{\n  \"a\": \"<b>\"\n}" {
		t.Fatalf("%q", data)
	}
	out, _ = f.GenerateCSVFile(CSVParams{Data: []interface{}{[]interface{}{1.0, "x"}}, Headers: []string{"n", "s"}, Filename: "t"})
	if string(out.(*ToolResult).Files[0].Content) != "n,s\n1,x\n" {
		t.Fatal(string(out.(*ToolResult).Files[0].Content))
	}
	out, _ = f.GeneratePDFFile(DocParams{Title: "Report (Q1)", Content: strings.Repeat("Lorem ipsum dolor sit amet. ", 400) + "\n\nCafé"})
	pdf := out.(*ToolResult).Files[0].Content
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) || !bytes.Contains(pdf, []byte(`Report \(Q1\)`)) || !bytes.Contains(pdf, []byte("/Count 3")) {
		t.Fatal(string(pdf[:200]))
	}
	out, _ = f.GenerateDocxFile(DocParams{Content: "a & b\n\nc"})
	zr, err := zip.NewReader(bytes.NewReader(out.(*ToolResult).Files[0].Content), int64(len(out.(*ToolResult).Files[0].Content)))
	if err != nil || len(zr.File) != 3 {
		t.Fatal(err)
	}
	out, _ = f.GenerateCodeFile(CodeParams{Code: "package main", Language: "golang", Filename: "main"})
	if out.(*ToolResult).Files[0].Filename != "main.go" {
		t.Fatal(out)
	}
	out, _ = f.GenerateCodeFile(CodeParams{Code: "x", Language: "python", Filename: "s.sh"})
	if out.(*ToolResult).Files[0].Filename != "s.sh" {
		t.Fatal(out)
	}
	mem, _ := NewFileGenerationTools()
	out, _ = mem.GenerateTextFile(TextParams{Content: "héllo"})
	if r := out.(*ToolResult); r.Files[0].Filepath != "" || !strings.Contains(r.Content, "(5 characters)") {
		t.Fatal(r.Content)
	}
}
