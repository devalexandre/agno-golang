// FieldLabeledCSVReader: every CSV row becomes a document of "Label: value"
// lines (headers like movie_title render as "Movie Title"), optionally
// prefixed with a rotating title. Empty fields are skipped by default.
//
// Offline: the sample CSV is written to a temp dir. No env vars needed.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	path := filepath.Join(os.TempDir(), "agno-movies.csv")
	sample := "movie_title,release_year,director,notes\nThe Matrix,1999,Wachowskis,\nArrival,2016,Denis Villeneuve,\"first contact,\nlinguistics\"\n"
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		log.Fatal(err)
	}
	defer os.Remove(path)

	r := reader.NewFieldLabeledCSVReader()
	r.ChunkTitles = []string{"Movie Information"}
	docs, err := r.Read(context.Background(), path)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range docs {
		fmt.Printf("--- %s\n%s\n", d.ID, d.Content)
	}
}
