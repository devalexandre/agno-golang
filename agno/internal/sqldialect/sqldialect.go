// Package sqldialect holds tiny helpers shared by the SQL-backed scheduler
// and job queue stores so one query text works on Postgres and SQLite.
package sqldialect

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Dialect selects SQL flavour.
type Dialect string

const (
	Postgres Dialect = "postgres"
	SQLite   Dialect = "sqlite"
)

// Validate returns an error for unknown dialects.
func (d Dialect) Validate() error {
	if d != Postgres && d != SQLite {
		return fmt.Errorf("unsupported SQL dialect %q (want %q or %q)", d, Postgres, SQLite)
	}
	return nil
}

// Rebind rewrites '?' placeholders to '$n' for Postgres.
func (d Dialect) Rebind(q string) string {
	if d != Postgres {
		return q
	}
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// BoolType returns the column type for booleans.
func (d Dialect) BoolType() string {
	if d == Postgres {
		return "BOOLEAN"
	}
	return "INTEGER"
}

var identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// ValidIdent reports whether name is safe to interpolate as a table name.
func ValidIdent(name string) bool { return identRE.MatchString(name) }

// IndexName derives an index name from a (possibly schema-qualified) table.
func IndexName(table, suffix string) string {
	return strings.ReplaceAll(table, ".", "_") + "_" + suffix
}

// IsUniqueViolation reports whether err looks like a unique constraint error.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate key")
}

// JSON encodes v as a nullable JSON string.
func JSON(v any) (sql.NullString, error) {
	if v == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	if string(b) == "null" {
		return sql.NullString{}, nil
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

// UnJSON decodes a nullable JSON string into dst.
func UnJSON(s sql.NullString, dst any) error {
	if !s.Valid || s.String == "" {
		return nil
	}
	return json.Unmarshal([]byte(s.String), dst)
}

// NullStr converts a *string to sql.NullString.
func NullStr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

// NullInt converts a *int64 to sql.NullInt64.
func NullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// StrPtr converts a sql.NullString to *string.
func StrPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

// IntPtr converts a sql.NullInt64 to *int64.
func IntPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}
