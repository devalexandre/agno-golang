// Reader factory: pick a reader by extension/MIME type (ReaderForExtension),
// by URL (ReaderForURL: YouTube, sitemap, s3://, website), or by key
// (CreateReader), and register custom readers (RegisterReader).
//
// Offline: sample files are written to a temp dir and read through the
// factory. No env vars needed.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

// upperReader is a toy custom reader registered under the key "upper".
type upperReader struct{ reader.BaseReader }

func (u *upperReader) Read(_ context.Context, source string) ([]*document.Document, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}
	return []*document.Document{document.NewDocument(strings.ToUpper(string(data)))}, nil
}

func main() {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "agno-factory")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	files := map[string]string{
		"notes.txt":  "plain text notes",
		"guide.md":   "# Guide\n\nMarkdown body",
		"data.json":  `[{"id":1},{"id":2}]`,
		"people.csv": "name,role\nAna,dev\nBo,ops",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			log.Fatal(err)
		}
		r := reader.ReaderForExtension(filepath.Ext(name))
		docs, err := r.Read(ctx, path)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%-11s -> %-8s %d doc(s)\n", name, r.Name(), len(docs))
	}

	fmt.Println()
	for _, u := range []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://example.com/sitemap.xml",
		"s3://bucket/docs/",
		"https://example.com/blog/post",
	} {
		fmt.Printf("%-45s -> %s\n", u, reader.ReaderForURL(u).Name())
	}

	fmt.Println("\nregistered keys:", reader.ReaderKeys())

	reader.RegisterReader(reader.ReaderInfo{
		Key: "upper", Name: "UpperReader", Description: "Upper-cases text files",
		New: func() reader.Reader { return &upperReader{reader.BaseReader{ID: "upper"}} },
	})
	r, err := reader.CreateReader("upper")
	if err != nil {
		log.Fatal(err)
	}
	docs, err := r.Read(ctx, filepath.Join(dir, "notes.txt"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("custom reader:", docs[0].Content)
}
