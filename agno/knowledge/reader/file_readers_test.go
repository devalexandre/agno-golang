package reader

import (
	"context"
	"strings"
	"testing"
)

func TestExcelReaderXLSX(t *testing.T) {
	path := writeBytes(t, "report.xlsx", sampleXLSX(t))
	docs, err := NewExcelReader().Read(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs (empty sheet skipped), got %d", len(docs))
	}
	want := "name, age, , joined\nAlice, 30, True, 2024-01-15T00:00:00\nline1 line2, 2.5"
	if docs[0].Content != want {
		t.Fatalf("sheet content:\n%q\nwant\n%q", docs[0].Content, want)
	}
	if docs[0].Name != "report" || docs[0].Metadata["sheet_name"] != "People" || docs[0].Metadata["sheet_index"] != 1 {
		t.Fatalf("bad metadata: name=%q %v", docs[0].Name, docs[0].Metadata)
	}
	if docs[1].Content != "hello & bye" || docs[1].Metadata["sheet_index"] != 3 {
		t.Fatalf("inline string sheet: %q %v", docs[1].Content, docs[1].Metadata)
	}

	filtered, err := NewExcelReader().WithSheets("notes").Read(context.Background(), path)
	if err != nil || len(filtered) != 1 || filtered[0].Metadata["sheet_name"] != "Notes" {
		t.Fatalf("name filter: %v %v", filtered, err)
	}
	byIndex, err := NewExcelReader().WithSheets(1).ReadBytes(context.Background(), sampleXLSX(t), "x.xlsx")
	if err != nil || len(byIndex) != 1 || byIndex[0].Metadata["sheet_name"] != "People" {
		t.Fatalf("index filter: %v %v", byIndex, err)
	}
	if _, err := NewExcelReader().Read(context.Background(), writeBytes(t, "old.xls", []byte("x"))); err == nil {
		t.Fatal("expected .xls to be rejected")
	}
}

func TestPPTXReader(t *testing.T) {
	path := writeBytes(t, "deck.pptx", samplePPTX(t))
	docs, err := NewPPTXReader().Read(context.Background(), path)
	if err != nil || len(docs) != 1 {
		t.Fatalf("got %d docs, err=%v", len(docs), err)
	}
	want := "Slide 1:\nQuarterly Review\nRevenue up\nCosts & risks\n\nSlide 2:\nInside group\n\nSlide 3:\n(No text content)"
	if docs[0].Content != want {
		t.Fatalf("content:\n%q\nwant\n%q", docs[0].Content, want)
	}
	if docs[0].Name != "deck" {
		t.Fatalf("name = %q", docs[0].Name)
	}
}

func TestJSONReader(t *testing.T) {
	path := writeFile(t, "items.json", `[{"a":1,"html":"<b>"},{"b":[1,2]}]`)
	docs, err := NewJSONReader().Read(context.Background(), path)
	if err != nil || len(docs) != 2 {
		t.Fatalf("got %d docs, err=%v", len(docs), err)
	}
	if docs[0].Content != `{"a":1,"html":"<b>"}` || docs[1].Metadata["page"] != 2 || docs[0].Name != "items" {
		t.Fatalf("unexpected: %q %v", docs[0].Content, docs[1].Metadata)
	}
	single, err := NewJSONReader().ReadBytes(context.Background(), []byte(`{"k":"v"}`), "one.json")
	if err != nil || len(single) != 1 || single[0].Metadata["page"] != 1 {
		t.Fatalf("object: %v %v", single, err)
	}
	if _, err := NewJSONReader().ReadBytes(context.Background(), []byte(`{bad`), "x.json"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFieldLabeledCSVReader(t *testing.T) {
	path := writeFile(t, "movies.csv", "first_name,movie_title,notes\nAlice,the matrix,\nBob,\"multi\nline\",ok\n,,\n")
	r := NewFieldLabeledCSVReader()
	r.ChunkTitles = []string{"Movie"}
	docs, err := r.Read(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs (title-only row kept like Python), got %d", len(docs))
	}
	if docs[0].Content != "Movie\nFirst Name: Alice\nMovie Title: the matrix" {
		t.Fatalf("row 1: %q", docs[0].Content)
	}
	if docs[1].Content != "Movie\nFirst Name: Bob\nMovie Title: multi line\nNotes: ok" {
		t.Fatalf("row 2: %q", docs[1].Content)
	}
	if docs[0].ID != "movies_row_1" || docs[0].Metadata["total_rows"] != 3 {
		t.Fatalf("metadata: %s %v", docs[0].ID, docs[0].Metadata)
	}

	r2 := NewFieldLabeledCSVReader()
	r2.FieldNames = []string{"Who"}
	r2.SkipEmptyFields = false
	r2.FormatHeaders = false
	docs, _ = r2.ReadBytes(context.Background(), []byte("a_b,c\nx,\n"), "t.csv")
	if len(docs) != 1 || docs[0].Content != "Who: x\nc: " {
		t.Fatalf("overrides: %q", docs[0].Content)
	}
}

func TestPDFReaderFallback(t *testing.T) {
	path := writeBytes(t, "paper.pdf", samplePDF(t, "Hello PDF", `Escaped \(parens\)`))
	r := NewPDFReader()
	r.DisablePdftotext = true
	docs, err := r.Read(context.Background(), path)
	if err != nil || len(docs) != 1 {
		t.Fatalf("got %d docs, err=%v", len(docs), err)
	}
	c := docs[0].Content
	for _, want := range []string{"Hello PDF", "Escaped (parens)", "Kern ed"} {
		if !strings.Contains(c, want) {
			t.Fatalf("missing %q in %q", want, c)
		}
	}
	if docs[0].Metadata["page"] != 1 || docs[0].Name != "paper" {
		t.Fatalf("metadata: %v", docs[0].Metadata)
	}
	if _, err := r.Read(context.Background(), writeFile(t, "fake.pdf", "not a pdf")); err == nil {
		t.Fatal("expected error for non-PDF")
	}
}

func TestFactoryExtended(t *testing.T) {
	cases := map[string]string{
		".pdf": "pdf", "xlsx": "excel", ".XLS": "excel", ".pptx": "pptx", ".json": "json",
		"application/pdf": "pdf", "text/csv": "csv", ".md": "markdown",
	}
	for ext, want := range cases {
		if r := ReaderForExtension(ext); r == nil || r.Name() != want {
			t.Errorf("ReaderForExtension(%q) = %v, want %s", ext, r, want)
		}
	}
	urls := map[string]string{
		"https://www.youtube.com/watch?v=abc":  "youtube",
		"https://youtu.be/abc":                 "youtube",
		"https://example.com/sitemap.xml":      "sitemap",
		"https://example.com/sitemap-1.xml.gz": "sitemap",
		"https://example.com/docs":             "website",
		"s3://bucket/key.txt":                  "s3",
	}
	for u, want := range urls {
		if r := ReaderForURL(u); r.Name() != want {
			t.Errorf("ReaderForURL(%q) = %s, want %s", u, r.Name(), want)
		}
	}
	keys := ReaderKeys()
	if len(keys) < 20 || keys[0] != "website" || keys[1] != "firecrawl" || keys[2] != "tavily" || keys[3] != "youtube" {
		t.Fatalf("keys order: %v", keys)
	}
	for _, k := range keys {
		r, err := CreateReader(k)
		if err != nil || r == nil {
			t.Fatalf("CreateReader(%q): %v", k, err)
		}
	}
	if _, err := CreateReader("nope"); err == nil {
		t.Fatal("expected unknown key error")
	}
	RegisterReader(ReaderInfo{Key: "custom", New: func() Reader { return NewTextReader() }})
	if r, err := CreateReader("custom"); err != nil || r.Name() != "text" {
		t.Fatalf("custom: %v %v", r, err)
	}
}

func TestCanonicalURLs(t *testing.T) {
	cases := map[string][2]string{
		"https://Example.com/docs/index.html#x": {"https://example.com/docs", "example.com/docs"},
		"https://example.com/":                  {"https://example.com", "example.com"},
		"https://example.com/a%20b/?v=1":        {"https://example.com/a%20b?v=1", "example.com/a b?v=1"},
	}
	for in, want := range cases {
		if got := CanonicalPageURL(in); got != want[0] {
			t.Errorf("CanonicalPageURL(%q) = %q want %q", in, got, want[0])
		}
		if got := CanonicalPageName(in); got != want[1] {
			t.Errorf("CanonicalPageName(%q) = %q want %q", in, got, want[1])
		}
	}
	long := "https://example.com/" + strings.Repeat("a", 400)
	if n := CanonicalPageName(long); len(n) != MaxPageNameLength {
		t.Fatalf("long name len %d", len(n))
	}
	if !IsHostAllowed("https://Docs.Example.com/x", []string{"docs.example.com"}) || IsHostAllowed("https://evil.com", []string{"docs.example.com"}) {
		t.Fatal("IsHostAllowed")
	}
	if !IsHostAllowed("https://any.com", nil) {
		t.Fatal("nil allowlist must allow")
	}
}

func TestYouTubeVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=1": "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ":                    "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":      "dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ":       "dQw4w9WgXcQ",
		"dQw4w9WgXcQ": "dQw4w9WgXcQ",
	}
	for in, want := range cases {
		if got, err := YouTubeVideoID(in); err != nil || got != want {
			t.Errorf("YouTubeVideoID(%q) = %q, %v", in, got, err)
		}
	}
}
