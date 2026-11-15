package csvtools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCsvTools(t *testing.T) {
	path := filepath.Join(t.TempDir(), "movies.csv")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbfTitle,Year,Rating\nAlien,1979,8.5\nHeat,1995,8.3\nUp,2009,\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCsvTools([]string{path})
	if out, _ := c.ListCsvFiles(NoParams{}); out != `["movies"]` {
		t.Fatal(out)
	}
	if out, _ := c.GetColumns(ColumnsParams{CsvName: "movies"}); out != `["Title","Year","Rating"]` {
		t.Fatal(out)
	}
	one := 1
	if out, _ := c.ReadCsvFile(ReadParams{CsvName: "movies", RowLimit: &one}); out != `[{"Rating":"8.5","Title":"Alien","Year":"1979"}]` {
		t.Fatal(out)
	}
	out, _ := c.QueryCsvFile(QueryParams{CsvName: "movies", SQLQuery: "SELECT Title, Year FROM movies WHERE Year > 1990 ORDER BY Year; DROP TABLE movies"})
	if out != "Title,Year\nHeat,1995\nUp,2009" {
		t.Fatalf("%q", out)
	}
	out, _ = c.QueryCsvFile(QueryParams{CsvName: "movies", SQLQuery: "SELECT AVG(Rating) AS r FROM movies"})
	if out != "r\n8.4" {
		t.Fatalf("%q", out)
	}
	if out, _ := c.GetColumns(ColumnsParams{CsvName: "nope"}); !strings.Contains(out.(string), `please use one of ["movies"]`) {
		t.Fatal(out)
	}
}
