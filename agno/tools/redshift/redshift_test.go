package redshift

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// fakeRedshift emulates information_schema and a "public" schema in SQLite.
func fakeRedshift(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	for _, s := range []string{
		`ATTACH ':memory:' AS information_schema`,
		`ATTACH ':memory:' AS public`,
		`CREATE TABLE information_schema.tables (table_schema TEXT, table_name TEXT)`,
		`CREATE TABLE information_schema.columns (table_schema TEXT, table_name TEXT, column_name TEXT, data_type TEXT, is_nullable TEXT)`,
		`INSERT INTO information_schema.tables VALUES ('public','sales')`,
		`INSERT INTO information_schema.columns VALUES ('public','sales','id','integer','NO'),('public','sales','region','varchar','YES')`,
		`CREATE TABLE public.sales (id INTEGER, region TEXT)`,
		`INSERT INTO public.sales VALUES (1,'eu'),(2,NULL)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(s, err)
		}
	}
	return db
}

func TestRedshiftTools(t *testing.T) {
	r := NewRedshiftTools(WithDB(fakeRedshift(t)))
	if out, _ := r.ShowTables(NoParams{}); out != "table_name\nsales" {
		t.Fatalf("%q", out)
	}
	out, _ := r.DescribeTable(TableParams{Table: "sales"})
	if !strings.Contains(out.(string), "id,integer,NO") {
		t.Fatal(out)
	}
	out, _ = r.RunQuery(QueryParams{Query: "SELECT * FROM public.sales WHERE id > 5"})
	if out != "Query returned no results.\nColumns: id, region" {
		t.Fatalf("%q", out)
	}
	if out, _ := r.SummarizeTable(TableParams{Table: "missing"}); !strings.Contains(out.(string), "not found in schema 'public'") {
		t.Fatal(out)
	}
	path := filepath.Join(t.TempDir(), "sales.csv")
	if out, _ := r.ExportTableToPath(ExportParams{Table: "sales", Path: path}); !strings.HasPrefix(out.(string), "Successfully") {
		t.Fatal(out)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "id,region\n1,eu\n2,\n" {
		t.Fatalf("%q", data)
	}
	if dsn := NewRedshiftTools(WithHost("h"), WithDatabase("d"), WithCredentials("u", "p@ss")).DSN(); dsn != "postgres://u:p%40ss@h:5439/d?sslmode=require" {
		t.Fatal(dsn)
	}
}
