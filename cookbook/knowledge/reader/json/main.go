// JSONReader: a top-level JSON array becomes one document per element
// (metadata "page" = 1-based position); any other value becomes one document.
//
// Offline: the sample file is written to a temp dir. No env vars needed.
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
	path := filepath.Join(os.TempDir(), "agno-recipes.json")
	sample := `[{"name":"Pad Thai","time":30},{"name":"Tom Yum","time":25,"spicy":true}]`
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		log.Fatal(err)
	}
	defer os.Remove(path)

	docs, err := reader.NewJSONReader().Read(context.Background(), path)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range docs {
		fmt.Printf("%s page %v: %s\n", d.Name, d.Metadata["page"], d.Content)
	}
}
