// Package csvtools ports Agno's CsvTools: list, read, inspect and query CSV
// files. Deviation from Python: QueryCsvFile loads the CSV into an in-memory
// SQLite database (modernc.org/sqlite, pure Go) instead of DuckDB, so queries
// must be valid SQLite SQL. Column types are auto-detected (INTEGER, REAL or TEXT).
package csvtools

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// CsvTools exposes a fixed set of CSV files to an agent.
type CsvTools struct {
	toolkit.Toolkit
	csvs     []string
	rowLimit *int

	enableRead, enableList, enableColumns, enableQuery bool
}

// Option configures CsvTools.
type Option func(*CsvTools)

// WithRowLimit sets the default row limit for ReadCsvFile.
func WithRowLimit(n int) Option { return func(c *CsvTools) { c.rowLimit = &n } }

// WithReadCsvFile toggles ReadCsvFile (default true).
func WithReadCsvFile(b bool) Option { return func(c *CsvTools) { c.enableRead = b } }

// WithListCsvFiles toggles ListCsvFiles (default true).
func WithListCsvFiles(b bool) Option { return func(c *CsvTools) { c.enableList = b } }

// WithGetColumns toggles GetColumns (default true).
func WithGetColumns(b bool) Option { return func(c *CsvTools) { c.enableColumns = b } }

// WithQueryCsvFile toggles QueryCsvFile (default true).
func WithQueryCsvFile(b bool) Option { return func(c *CsvTools) { c.enableQuery = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(c *CsvTools) { c.enableRead, c.enableList, c.enableColumns, c.enableQuery = true, true, true, true }
}

// NoParams is used by parameterless methods.
type NoParams struct{}

// ReadParams for ReadCsvFile.
type ReadParams struct {
	CsvName  string `json:"csv_name" description:"The name of the csv file to read without the extension." required:"true"`
	RowLimit *int   `json:"row_limit,omitempty" description:"The number of rows to return. Omit to return all rows (or the toolkit default)."`
}

// ColumnsParams for GetColumns.
type ColumnsParams struct {
	CsvName string `json:"csv_name" description:"The name of the csv file to get the columns from without the extension." required:"true"`
}

// QueryParams for QueryCsvFile.
type QueryParams struct {
	CsvName  string `json:"csv_name" description:"The name of the csv file to query" required:"true"`
	SQLQuery string `json:"sql_query" description:"The SQL Query to run on the csv file. The table name is the csv file name without the extension. Must be valid SQLite SQL; wrap column names with double quotes if they contain spaces or special characters; use single quotes for string values." required:"true"`
}

// NewCsvTools creates the toolkit for the given csv paths.
func NewCsvTools(csvs []string, opts ...Option) *CsvTools {
	c := &CsvTools{csvs: csvs, enableRead: true, enableList: true, enableColumns: true, enableQuery: true}
	c.Toolkit = toolkit.NewToolkit()
	c.Toolkit.Name = "csv_tools"
	c.Toolkit.Description = "Read, inspect and query CSV files."
	for _, o := range opts {
		o(c)
	}
	if c.enableRead {
		c.Toolkit.Register("ReadCsvFile", "Use this function to read the contents of a csv file `name` without the extension.", c, c.ReadCsvFile, ReadParams{})
	}
	if c.enableList {
		c.Toolkit.Register("ListCsvFiles", "Returns a list of available csv files", c, c.ListCsvFiles, NoParams{})
	}
	if c.enableColumns {
		c.Toolkit.Register("GetColumns", "Use this function to get the columns of the csv file `csv_name` without the extension.", c, c.GetColumns, ColumnsParams{})
	}
	if c.enableQuery {
		c.Toolkit.Register("QueryCsvFile", "Use this function to run a SQL query on csv file `csv_name` without the extension. The table name is the name of the csv file without the extension. The SQL query should be valid SQLite SQL. Always wrap column names with double quotes if they contain spaces or special characters. Use single quotes for string values.", c, c.QueryCsvFile, QueryParams{})
	}
	return c
}

func stem(p string) string {
	b := filepath.Base(p)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

func (c *CsvTools) names() []string {
	out := make([]string, 0, len(c.csvs))
	for _, p := range c.csvs {
		out = append(out, stem(p))
	}
	return out
}

func (c *CsvTools) find(name string) (string, string) {
	for _, p := range c.csvs {
		if stem(p) == name {
			return p, ""
		}
	}
	list, _ := c.ListCsvFiles(NoParams{})
	return "", fmt.Sprintf("File: %s not found, please use one of %s", name, list)
}

func readAll(path string) ([][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

// ListCsvFiles returns the available csv names (without extension) as JSON.
func (c *CsvTools) ListCsvFiles(NoParams) (interface{}, error) {
	b, _ := json.Marshal(c.names())
	return string(b), nil
}

// ReadCsvFile returns rows as a JSON list of objects.
func (c *CsvTools) ReadCsvFile(p ReadParams) (interface{}, error) {
	path, msg := c.find(p.CsvName)
	if msg != "" {
		return msg, nil
	}
	recs, err := readAll(path)
	if err != nil {
		return fmt.Sprintf("Error reading csv: %v", err), nil
	}
	limit := -1
	if p.RowLimit != nil {
		limit = *p.RowLimit
	} else if c.rowLimit != nil {
		limit = *c.rowLimit
	}
	rows := []map[string]string{}
	if len(recs) > 0 {
		header := recs[0]
		for _, rec := range recs[1:] {
			if limit >= 0 && len(rows) >= limit {
				break
			}
			m := make(map[string]string, len(header))
			for i, h := range header {
				if i < len(rec) {
					m[h] = rec[i]
				}
			}
			rows = append(rows, m)
		}
	}
	b, _ := json.Marshal(rows)
	return string(b), nil
}

// GetColumns returns the header row as JSON.
func (c *CsvTools) GetColumns(p ColumnsParams) (interface{}, error) {
	path, msg := c.find(p.CsvName)
	if msg != "" {
		return msg, nil
	}
	recs, err := readAll(path)
	if err != nil {
		return fmt.Sprintf("Error getting columns: %v", err), nil
	}
	var cols []string
	if len(recs) > 0 {
		cols = recs[0]
	}
	b, _ := json.Marshal(cols)
	return string(b), nil
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func columnType(recs [][]string, col int) string {
	typ := "INTEGER"
	seen := false
	for _, rec := range recs {
		if col >= len(rec) || rec[col] == "" {
			continue
		}
		seen = true
		v := rec[col]
		if typ == "INTEGER" {
			if _, err := strconv.ParseInt(v, 10, 64); err == nil {
				continue
			}
			typ = "REAL"
		}
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "TEXT"
		}
	}
	if !seen {
		return "TEXT"
	}
	return typ
}

// QueryCsvFile loads the csv into SQLite and runs the query.
func (c *CsvTools) QueryCsvFile(p QueryParams) (interface{}, error) {
	path, msg := c.find(p.CsvName)
	if msg != "" {
		return msg, nil
	}
	recs, err := readAll(path)
	if err != nil || len(recs) == 0 {
		if err == nil {
			err = fmt.Errorf("empty csv")
		}
		return fmt.Sprintf("Error querying csv: %v", err), nil
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return fmt.Sprintf("Error querying csv: %v", err), nil
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	header, body := recs[0], recs[1:]
	defs := make([]string, len(header))
	marks := make([]string, len(header))
	types := make([]string, len(header))
	for i, h := range header {
		types[i] = columnType(body, i)
		defs[i] = quote(h) + " " + types[i]
		marks[i] = "?"
	}
	table := quote(p.CsvName)
	if _, err := db.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", table, strings.Join(defs, ", "))); err != nil {
		return fmt.Sprintf("Error querying csv: %v", err), nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Sprintf("Error querying csv: %v", err), nil
	}
	stmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO %s VALUES (%s)", table, strings.Join(marks, ", ")))
	if err != nil {
		_ = tx.Rollback()
		return fmt.Sprintf("Error querying csv: %v", err), nil
	}
	for _, rec := range body {
		args := make([]interface{}, len(header))
		for i := range header {
			if i >= len(rec) || (rec[i] == "" && types[i] != "TEXT") {
				args[i] = nil
				continue
			}
			args[i] = rec[i]
		}
		if _, err := stmt.Exec(args...); err != nil {
			_ = tx.Rollback()
			return fmt.Sprintf("Error querying csv: %v", err), nil
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		return fmt.Sprintf("Error querying csv: %v", err), nil
	}

	query := strings.ReplaceAll(p.SQLQuery, "`", "")
	query = strings.Split(query, ";")[0]
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Sprintf("Error querying csv: %v", err), nil
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var lines []string
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Sprintf("Error querying csv: %v", err), nil
		}
		parts := make([]string, len(vals))
		for i, v := range vals {
			switch t := v.(type) {
			case nil:
				parts[i] = "None"
			case []byte:
				parts[i] = string(t)
			default:
				parts[i] = fmt.Sprint(t)
			}
		}
		lines = append(lines, strings.Join(parts, ","))
	}
	if err := rows.Err(); err != nil {
		return fmt.Sprintf("Error querying csv: %v", err), nil
	}
	return strings.Join(cols, ",") + "\n" + strings.Join(lines, "\n"), nil
}
