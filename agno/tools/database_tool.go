package tools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DatabaseConfig holds configuration for database operations
type DatabaseConfig struct {
	Type         string // postgres, mysql, sqlite3
	ReadOnly     bool   // If true, only SELECT queries are allowed
	MaxRows      int    // Maximum number of rows to return (default: 1000)
	MaxOpenConns int    // Maximum number of open connections (default: 10)
	MaxIdleConns int    // Maximum number of idle connections (default: 5)
}

// DatabaseTool is the Tool wrapper to use in Agent
type DatabaseTool struct {
	db       *sql.DB
	dbType   string
	readOnly bool
	maxRows  int
	toolkit.Toolkit
}

// NewDatabaseTool initializes the tool and registers the methods
func NewDatabaseTool(sqlDB *sql.DB, config DatabaseConfig) toolkit.Tool {
	if sqlDB == nil {
		panic("database connection is nil")
	}

	// Apply defaults
	dbType := config.Type
	if dbType == "" {
		dbType = "postgres"
	}

	maxRows := config.MaxRows
	if maxRows == 0 {
		maxRows = 1000
	}

	// Optional: configure connection pool
	if config.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	}

	dbTool := &DatabaseTool{
		db:       sqlDB,
		dbType:   dbType,
		readOnly: config.ReadOnly,
		maxRows:  maxRows,
	}
	dbTool.Toolkit = toolkit.NewToolkit()
	dbTool.Toolkit.Name = "DatabaseTool"
	dbTool.Toolkit.Description = "Toolkit for safe database operations: list tables, describe schema, execute queries,select(find) insert, update  and run transactions."

	// ✅ Register all methods with their specific input types
	dbTool.Toolkit.Register("ListTables", "List all tables in the database", dbTool, dbTool.ListTables, ListTablesInput{})
	dbTool.Toolkit.Register("DescribeTable", "Describe the structure of a table", dbTool, dbTool.DescribeTable, DescribeTableInput{})
	dbTool.Toolkit.Register("ExecuteSelect", "Execute a SELECT query", dbTool, dbTool.ExecuteSelect, ExecuteSelectInput{})
	dbTool.Toolkit.Register("ExecuteQuery", "Execute any SQL query", dbTool, dbTool.ExecuteQuery, ExecuteQueryInput{})
	dbTool.Toolkit.Register("GetTableInfo", "Get comprehensive information about a table", dbTool, dbTool.GetTableInfo, GetTableInfoInput{})
	dbTool.Toolkit.Register("ExecuteTransaction", "Execute multiple queries in a transaction", dbTool, dbTool.ExecuteTransaction, ExecuteTransactionInput{})

	return dbTool
}

// ========================= Input Structs =========================

type ListTablesInput struct {
	Schema string `json:"schema,omitempty" description:"Database schema name (optional, defaults vary by DB)"`
}

type DescribeTableInput struct {
	Table  string `json:"table" description:"Name of the table to describe (shows columns, types, constraints)" required:"true"`
	Schema string `json:"schema,omitempty" description:"Database schema name (optional)"`
}

type ExecuteSelectInput struct {
	Query  string        `json:"query" description:"SELECT query to retrieve data from database" required:"true"`
	Params []interface{} `json:"params,omitempty" description:"Query parameters for prepared statements (optional)"`
	Limit  int           `json:"limit,omitempty" description:"Maximum number of rows to return (optional, default 1000)"`
}

type ExecuteQueryInput struct {
	Query  string        `json:"query" description:"SQL query to execute (INSERT, UPDATE, DELETE, or any SQL statement)" required:"true"`
	Params []interface{} `json:"params,omitempty" description:"Query parameters for prepared statements (optional)"`
}

type GetTableInfoInput struct {
	Table  string `json:"table" description:"Name of the table to get comprehensive information about" required:"true"`
	Schema string `json:"schema,omitempty" description:"Database schema name (optional)"`
}

type ExecuteTransactionInput struct {
	Queries []string `json:"queries" description:"Array of SQL queries to execute atomically in a single transaction" required:"true"`
}

// ========================= Methods =========================

func (t *DatabaseTool) ListTables(params ListTablesInput) (interface{}, error) {
	ctx := context.Background()
	var query string
	var args []interface{}

	switch t.dbType {
	case "postgres":
		schema := "public"
		if params.Schema != "" {
			schema = params.Schema
		}
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name"
		args = []interface{}{schema}
	case "mysql":
		query = "SHOW TABLES"
	case "sqlite3":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	default:
		return nil, fmt.Errorf("unsupported database type: %s", t.dbType)
	}

	rows, err := t.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			continue
		}
		tables = append(tables, table)
	}

	return map[string]interface{}{
		"success": true,
		"tables":  tables,
		"count":   len(tables),
	}, nil
}

func (t *DatabaseTool) DescribeTable(params DescribeTableInput) (interface{}, error) {
	ctx := context.Background()
	var query string
	var args []interface{}

	switch t.dbType {
	case "postgres":
		schema := "public"
		if params.Schema != "" {
			schema = params.Schema
		}
		query = `
			SELECT column_name, data_type, is_nullable, column_default
			FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2
			ORDER BY ordinal_position
		`
		args = []interface{}{schema, params.Table}
	case "mysql":
		query = "DESCRIBE " + t.quoteIdentifier(params.Table)
	case "sqlite3":
		query = "PRAGMA table_info(" + t.quoteIdentifier(params.Table) + ")"
	default:
		return nil, fmt.Errorf("unsupported database type: %s", t.dbType)
	}

	rows, err := t.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var info []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			v := values[i]
			if b, ok := v.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = v
			}
		}
		info = append(info, row)
	}

	return map[string]interface{}{
		"success": true,
		"table":   params.Table,
		"columns": info,
		"count":   len(info),
	}, nil
}

func (t *DatabaseTool) ExecuteSelect(params ExecuteSelectInput) (interface{}, error) {
	if !t.isSelectQuery(params.Query) {
		return nil, fmt.Errorf("only SELECT queries allowed in ExecuteSelect")
	}

	limit := t.maxRows
	if params.Limit > 0 && params.Limit < limit {
		limit = params.Limit
	}

	query := params.Query
	if !strings.Contains(strings.ToUpper(query), "LIMIT") {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	rows, err := t.db.QueryContext(context.Background(), query, params.Params...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]interface{})
		for i, col := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = v
			}
		}
		results = append(results, row)
	}

	return map[string]interface{}{
		"success": true,
		"columns": cols,
		"rows":    results,
		"count":   len(results),
	}, nil
}

func (t *DatabaseTool) ExecuteQuery(params ExecuteQueryInput) (interface{}, error) {
	if t.readOnly && !t.isSelectQuery(params.Query) {
		return nil, fmt.Errorf("only SELECT queries allowed in read-only mode")
	}

	if t.isSelectQuery(params.Query) {
		return t.ExecuteSelect(ExecuteSelectInput{
			Query:  params.Query,
			Params: params.Params,
			Limit:  t.maxRows,
		})
	}

	res, err := t.db.ExecContext(context.Background(), params.Query, params.Params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	lastInsertID, _ := res.LastInsertId()

	return map[string]interface{}{
		"success":        true,
		"rows_affected":  rowsAffected,
		"last_insert_id": lastInsertID,
	}, nil
}

func (t *DatabaseTool) GetTableInfo(params GetTableInfoInput) (interface{}, error) {
	desc, err := t.DescribeTable(DescribeTableInput{
		Table:  params.Table,
		Schema: params.Schema,
	})
	if err != nil {
		return nil, err
	}

	var count int64
	err = t.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+t.quoteIdentifier(params.Table)).Scan(&count)
	if err != nil {
		count = -1
	}

	return map[string]interface{}{
		"success":   true,
		"table":     params.Table,
		"columns":   desc.(map[string]interface{})["columns"],
		"row_count": count,
	}, nil
}

func (t *DatabaseTool) ExecuteTransaction(params ExecuteTransactionInput) (interface{}, error) {
	if t.readOnly {
		return nil, fmt.Errorf("transactions are not allowed in read-only mode")
	}

	if len(params.Queries) == 0 {
		return nil, fmt.Errorf("no queries provided")
	}

	tx, err := t.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	totalRows := int64(0)
	for _, q := range params.Queries {
		res, err := tx.ExecContext(context.Background(), q)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("transaction failed: %w", err)
		}
		if ra, _ := res.RowsAffected(); ra > 0 {
			totalRows += ra
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return map[string]interface{}{
		"success":             true,
		"queries_executed":    len(params.Queries),
		"total_rows_affected": totalRows,
	}, nil
}

// Helpers

func (t *DatabaseTool) isSelectQuery(q string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(q))
	return strings.HasPrefix(trimmed, "SELECT") || strings.HasPrefix(trimmed, "WITH")
}

func (t *DatabaseTool) quoteIdentifier(name string) string {
	switch t.dbType {
	case "mysql":
		return "`" + name + "`"
	case "sqlite3", "postgres":
		return `"` + name + `"`
	default:
		return name
	}
}
