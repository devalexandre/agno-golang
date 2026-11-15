package sqltools

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLToolsSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT 'x');
		INSERT INTO users(name) VALUES ('a'),('b'),('c');`); err != nil {
		t.Fatal(err)
	}
	s := NewSQLTools(db)
	if s.dialect != DialectSQLite {
		t.Fatal(s.dialect)
	}
	out, _ := s.ListTables(NoParams{})
	if out != `["users"]` {
		t.Fatal(out)
	}
	out, _ = s.DescribeTable(DescribeTableParams{TableName: "users"})
	if !strings.Contains(out.(string), `"name":"name","type":"TEXT","nullable":false`) {
		t.Fatal(out)
	}
	two := 2
	out, _ = s.RunSQLQuery(RunSQLQueryParams{Query: "SELECT name FROM users ORDER BY id", Limit: &two})
	if out != `[{"name":"a"},{"name":"b"}]` {
		t.Fatal(out)
	}
	out, _ = s.Execute("sql_tools_RunSQLQuery", []byte(`{"query":"SELECT count(*) AS n FROM users"}`))
	if out != `[{"n":3}]` {
		t.Fatal(out)
	}
	out, _ = s.RunSQLQuery(RunSQLQueryParams{Query: "SELECT * FROM nope"})
	if !strings.HasPrefix(out.(string), "Error running query") {
		t.Fatal(out)
	}
}
