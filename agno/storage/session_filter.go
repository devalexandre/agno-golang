package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Session filter schema — a port of Python agno/db/session_filters.py.
//
// A filter is a tree: AND/OR nodes hold 1..100 Conditions, NOT holds a single
// Condition, and leaves compare one session field (Key) against Value (or
// Values for IN). Filters are validated with NormalizeSessionFilter before
// being evaluated, and comparisons follow SQL NULL semantics: a missing field
// never matches a comparison nor its negation.

const (
	// MaxSessionFilterDepth is the maximum nesting depth of a filter tree
	// (Python filter_converter.MAX_FILTER_DEPTH).
	MaxSessionFilterDepth = 10
	// MaxSessionFilterConditions is the maximum number of conditions of an AND/OR node.
	MaxSessionFilterConditions = 100
	// MaxSessionFilterValues is the maximum number of values of an IN leaf.
	MaxSessionFilterValues = 100
)

// Session filter operators.
const (
	FilterAnd        = "AND"
	FilterOr         = "OR"
	FilterNot        = "NOT"
	FilterEq         = "EQ"
	FilterNeq        = "NEQ"
	FilterIn         = "IN"
	FilterContains   = "CONTAINS"
	FilterStartsWith = "STARTSWITH"
	FilterGt         = "GT"
	FilterGte        = "GTE"
	FilterLt         = "LT"
	FilterLte        = "LTE"
)

// SessionFilterField describes one filterable session field.
type SessionFilterField struct {
	Label     string   `json:"label"`
	Type      string   `json:"type"` // "string", "enum" or "datetime"
	Operators []string `json:"operators"`
	Values    []string `json:"values,omitempty"`
}

var stringFilterOperators = []string{FilterEq, FilterNeq, FilterIn, FilterContains, FilterStartsWith}

// SessionFilterFields is the schema of filterable fields (Python SESSION_FILTER_FIELDS).
var SessionFilterFields = map[string]SessionFilterField{
	"session_id":   {Label: "Session ID", Type: "string", Operators: stringFilterOperators},
	"session_name": {Label: "Saved Session Name", Type: "string", Operators: stringFilterOperators},
	"user_id":      {Label: "User ID", Type: "string", Operators: stringFilterOperators},
	"agent_id":     {Label: "Agent ID", Type: "string", Operators: stringFilterOperators},
	"team_id":      {Label: "Team ID", Type: "string", Operators: stringFilterOperators},
	"workflow_id":  {Label: "Workflow ID", Type: "string", Operators: stringFilterOperators},
	"session_type": {Label: "Session Type", Type: "enum", Operators: []string{FilterEq, FilterNeq, FilterIn}, Values: []string{"agent", "team", "workflow"}},
	"created_at":   {Label: "Created At", Type: "datetime", Operators: []string{FilterEq, FilterNeq, FilterGt, FilterGte, FilterLt, FilterLte}},
	"updated_at":   {Label: "Updated At", Type: "datetime", Operators: []string{FilterEq, FilterNeq, FilterGt, FilterGte, FilterLt, FilterLte}},
}

// SessionFilter is one node of a session filter tree.
type SessionFilter struct {
	Op         string           `json:"op"`
	Key        string           `json:"key,omitempty"`
	Value      interface{}      `json:"value,omitempty"`
	Values     []interface{}    `json:"values,omitempty"`
	Conditions []*SessionFilter `json:"conditions,omitempty"`
	Condition  *SessionFilter   `json:"condition,omitempty"`
}

// Helpers to build filters fluently.

// And builds an AND node.
func And(conds ...*SessionFilter) *SessionFilter {
	return &SessionFilter{Op: FilterAnd, Conditions: conds}
}

// Or builds an OR node.
func Or(conds ...*SessionFilter) *SessionFilter {
	return &SessionFilter{Op: FilterOr, Conditions: conds}
}

// Not builds a NOT node.
func Not(cond *SessionFilter) *SessionFilter { return &SessionFilter{Op: FilterNot, Condition: cond} }

// Where builds a leaf comparing key with value using op.
func Where(key, op string, value interface{}) *SessionFilter {
	return &SessionFilter{Op: op, Key: key, Value: value}
}

// WhereIn builds an IN leaf.
func WhereIn(key string, values ...interface{}) *SessionFilter {
	return &SessionFilter{Op: FilterIn, Key: key, Values: values}
}

// ParseSessionFilter decodes and validates a JSON filter expression.
func ParseSessionFilter(raw []byte) (*SessionFilter, error) {
	if len(strings.TrimSpace(string(raw))) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var f SessionFilter
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("invalid session filter: %w", err)
	}
	return NormalizeSessionFilter(&f)
}

// NormalizeSessionFilter validates every node and returns a normalized deep
// copy where datetime values are converted to float64 epoch seconds. A nil
// filter is valid and returns nil.
func NormalizeSessionFilter(f *SessionFilter) (*SessionFilter, error) {
	if f == nil {
		return nil, nil
	}
	return normalizeSessionFilter(f, 0)
}

func normalizeSessionFilter(f *SessionFilter, depth int) (*SessionFilter, error) {
	if depth > MaxSessionFilterDepth {
		return nil, fmt.Errorf("session filter exceeds maximum nesting depth")
	}
	if f == nil {
		return nil, fmt.Errorf("each filter must be an object")
	}
	switch f.Op {
	case FilterAnd, FilterOr:
		if len(f.Conditions) == 0 || len(f.Conditions) > MaxSessionFilterConditions {
			return nil, fmt.Errorf("AND/OR require between 1 and %d conditions", MaxSessionFilterConditions)
		}
		out := &SessionFilter{Op: f.Op, Conditions: make([]*SessionFilter, 0, len(f.Conditions))}
		for _, c := range f.Conditions {
			nc, err := normalizeSessionFilter(c, depth+1)
			if err != nil {
				return nil, err
			}
			out.Conditions = append(out.Conditions, nc)
		}
		return out, nil
	case FilterNot:
		nc, err := normalizeSessionFilter(f.Condition, depth+1)
		if err != nil {
			return nil, err
		}
		return &SessionFilter{Op: FilterNot, Condition: nc}, nil
	}

	field, ok := SessionFilterFields[f.Key]
	if !ok {
		return nil, fmt.Errorf("unknown session filter field: %s", f.Key)
	}
	if !containsString(field.Operators, f.Op) {
		return nil, fmt.Errorf("unsupported operator for %s: %s", f.Key, f.Op)
	}
	values := []interface{}{f.Value}
	if f.Op == FilterIn {
		values = f.Values
	}
	if len(values) == 0 || len(values) > MaxSessionFilterValues {
		return nil, fmt.Errorf("IN requires between 1 and %d values", MaxSessionFilterValues)
	}
	normalized := make([]interface{}, 0, len(values))
	for _, v := range values {
		if field.Type == "datetime" {
			ts, err := filterTimestamp(v)
			if err != nil {
				return nil, fmt.Errorf("invalid timestamp for %s", f.Key)
			}
			normalized = append(normalized, ts)
			continue
		}
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s requires string values", f.Key)
		}
		if field.Type == "enum" && !containsString(field.Values, s) {
			return nil, fmt.Errorf("invalid value for %s: %s", f.Key, s)
		}
		normalized = append(normalized, s)
	}
	out := &SessionFilter{Op: f.Op, Key: f.Key}
	if f.Op == FilterIn {
		out.Values = normalized
	} else {
		out.Value = normalized[0]
	}
	return out, nil
}

func filterTimestamp(v interface{}) (float64, error) {
	var ts float64
	switch x := v.(type) {
	case int:
		ts = float64(x)
	case int32:
		ts = float64(x)
	case int64:
		ts = float64(x)
	case float32:
		ts = float64(x)
	case float64:
		ts = x
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return 0, err
		}
		ts = f
	case time.Time:
		ts = float64(x.UnixNano()) / 1e9
	case string:
		t, err := parseFilterTime(x)
		if err != nil {
			return 0, err
		}
		ts = float64(t.UnixNano()) / 1e9
	default:
		return 0, fmt.Errorf("unsupported timestamp type %T", v)
	}
	if math.IsNaN(ts) || math.IsInf(ts, 0) {
		return 0, fmt.Errorf("timestamp must be finite")
	}
	return ts, nil
}

func parseFilterTime(s string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid datetime %q", s)
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Fields returns the set of keys referenced by the filter.
func (f *SessionFilter) Fields() map[string]bool {
	out := map[string]bool{}
	var walk func(*SessionFilter)
	walk = func(n *SessionFilter) {
		if n == nil {
			return
		}
		switch n.Op {
		case FilterAnd, FilterOr:
			for _, c := range n.Conditions {
				walk(c)
			}
		case FilterNot:
			walk(n.Condition)
		default:
			out[n.Key] = true
		}
	}
	walk(f)
	return out
}

// SessionFilterRecord flattens an AgentSession into the field map evaluated by
// session filters. Empty strings are treated as missing (SQL NULL). Fields the
// Go session model does not keep as columns (session_name, team_id,
// workflow_id, session_type) are read from SessionData, then ExtraData.
func SessionFilterRecord(s *AgentSession) map[string]interface{} {
	rec := map[string]interface{}{}
	if s == nil {
		return rec
	}
	set := func(k, v string) {
		if v != "" {
			rec[k] = v
		}
	}
	lookup := func(k string) string {
		for _, m := range []map[string]interface{}{s.SessionData, s.ExtraData, s.AgentData} {
			if v, ok := m[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	set("session_id", s.SessionID)
	set("user_id", s.UserID)
	set("agent_id", s.AgentID)
	set("session_name", lookup("session_name"))
	teamID, workflowID := lookup("team_id"), lookup("workflow_id")
	set("team_id", teamID)
	set("workflow_id", workflowID)
	st := lookup("session_type")
	if st == "" {
		switch {
		case workflowID != "" && s.AgentID == "":
			st = "workflow"
		case teamID != "" && s.AgentID == "":
			st = "team"
		default:
			st = "agent"
		}
	}
	rec["session_type"] = st
	rec["created_at"] = float64(s.CreatedAt)
	rec["updated_at"] = float64(s.UpdatedAt)
	return rec
}

// MatchesSessionFilter evaluates a normalized filter against a session record
// (as built by SessionFilterRecord). A nil filter matches everything.
func MatchesSessionFilter(record map[string]interface{}, f *SessionFilter) bool {
	return evalSessionFilter(record, f) == triTrue
}

// MatchesSession is MatchesSessionFilter applied to an AgentSession.
func MatchesSession(s *AgentSession, f *SessionFilter) bool {
	return MatchesSessionFilter(SessionFilterRecord(s), f)
}

type tri int8

const (
	triUnknown tri = iota
	triFalse
	triTrue
)

func boolTri(b bool) tri {
	if b {
		return triTrue
	}
	return triFalse
}

func evalSessionFilter(rec map[string]interface{}, f *SessionFilter) tri {
	if f == nil {
		return triTrue
	}
	switch f.Op {
	case FilterAnd:
		res := triTrue
		for _, c := range f.Conditions {
			switch evalSessionFilter(rec, c) {
			case triFalse:
				return triFalse
			case triUnknown:
				res = triUnknown
			}
		}
		return res
	case FilterOr:
		res := triFalse
		for _, c := range f.Conditions {
			switch evalSessionFilter(rec, c) {
			case triTrue:
				return triTrue
			case triUnknown:
				res = triUnknown
			}
		}
		return res
	case FilterNot:
		switch evalSessionFilter(rec, f.Condition) {
		case triTrue:
			return triFalse
		case triFalse:
			return triTrue
		}
		return triUnknown
	}
	actual, ok := rec[f.Key]
	if !ok || actual == nil {
		return triUnknown
	}
	if f.Op == FilterIn {
		for _, v := range f.Values {
			if compareFilterValues(actual, v) == 0 {
				return triTrue
			}
		}
		return triFalse
	}
	switch f.Op {
	case FilterEq:
		return boolTri(compareFilterValues(actual, f.Value) == 0)
	case FilterNeq:
		return boolTri(compareFilterValues(actual, f.Value) != 0)
	case FilterGt:
		return boolTri(compareFilterValues(actual, f.Value) > 0)
	case FilterGte:
		return boolTri(compareFilterValues(actual, f.Value) >= 0)
	case FilterLt:
		return boolTri(compareFilterValues(actual, f.Value) < 0)
	case FilterLte:
		return boolTri(compareFilterValues(actual, f.Value) <= 0)
	case FilterContains:
		a, _ := actual.(string)
		v, _ := f.Value.(string)
		return boolTri(strings.Contains(strings.ToLower(a), strings.ToLower(v)))
	case FilterStartsWith:
		a, _ := actual.(string)
		v, _ := f.Value.(string)
		return boolTri(strings.HasPrefix(strings.ToLower(a), strings.ToLower(v)))
	}
	return triUnknown
}

// compareFilterValues compares two strings or two numbers; -2 means incomparable.
func compareFilterValues(a, b interface{}) int {
	if as, ok := a.(string); ok {
		bs, ok := b.(string)
		if !ok {
			return -2
		}
		return strings.Compare(as, bs)
	}
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if !aok || !bok {
		return -2
	}
	switch {
	case af < bf:
		return -1
	case af > bf:
		return 1
	}
	return 0
}

func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	}
	return 0, false
}

// SessionFilterSQLOptions configures SessionFilterToSQL.
type SessionFilterSQLOptions struct {
	// Columns maps filter keys to SQL expressions. A filter that references a
	// key missing from Columns cannot be translated (ok=false).
	Columns map[string]string
	// BinaryCompare forces case-sensitive string equality (MySQL/SingleStore,
	// whose default collations are case-insensitive).
	BinaryCompare bool
	// NullIfEmpty wraps string columns in NULLIF(col, '') so empty strings
	// behave as SQL NULL, matching the Go evaluator.
	NullIfEmpty bool
}

// SessionFilterToSQL translates a normalized filter into a WHERE clause using
// "?" placeholders. ok is false when a referenced field has no column.
func SessionFilterToSQL(f *SessionFilter, opts SessionFilterSQLOptions) (clause string, args []interface{}, ok bool) {
	if f == nil {
		return "1=1", nil, true
	}
	for k := range f.Fields() {
		if _, has := opts.Columns[k]; !has {
			return "", nil, false
		}
	}
	var b strings.Builder
	writeSQLFilter(&b, &args, f, opts)
	return b.String(), args, true
}

func writeSQLFilter(b *strings.Builder, args *[]interface{}, f *SessionFilter, opts SessionFilterSQLOptions) {
	switch f.Op {
	case FilterAnd, FilterOr:
		b.WriteString("(")
		for i, c := range f.Conditions {
			if i > 0 {
				b.WriteString(" " + f.Op + " ")
			}
			writeSQLFilter(b, args, c, opts)
		}
		b.WriteString(")")
		return
	case FilterNot:
		b.WriteString("(NOT ")
		writeSQLFilter(b, args, f.Condition, opts)
		b.WriteString(")")
		return
	}
	col := opts.Columns[f.Key]
	isString := SessionFilterFields[f.Key].Type != "datetime"
	if isString && opts.NullIfEmpty {
		col = "NULLIF(" + col + ", '')"
	}
	cmpCol := col
	if isString && opts.BinaryCompare {
		cmpCol = "BINARY " + col
	}
	switch f.Op {
	case FilterIn:
		b.WriteString(cmpCol + " IN (")
		for i, v := range f.Values {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("?")
			*args = append(*args, sqlFilterArg(v))
		}
		b.WriteString(")")
	case FilterContains, FilterStartsWith:
		v, _ := f.Value.(string)
		escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(v))
		pattern := escaped + "%"
		if f.Op == FilterContains {
			pattern = "%" + pattern
		}
		b.WriteString("LOWER(" + col + ") LIKE ? ESCAPE '!'")
		*args = append(*args, pattern)
	default:
		ops := map[string]string{FilterEq: "=", FilterNeq: "<>", FilterGt: ">", FilterGte: ">=", FilterLt: "<", FilterLte: "<="}
		b.WriteString(cmpCol + " " + ops[f.Op] + " ?")
		*args = append(*args, sqlFilterArg(f.Value))
	}
}

func sqlFilterArg(v interface{}) interface{} {
	if f, ok := v.(float64); ok && f == math.Trunc(f) {
		return int64(f)
	}
	return v
}

// SessionSearchOptions configures SearchSessions (Python BaseDb.search_sessions).
type SessionSearchOptions struct {
	// UserID restricts results to one user when non-nil.
	UserID *string
	// Limit is the page size (default 20).
	Limit int
	// Page is 1-based (default 1).
	Page int
	// SortBy is "created_at" (default) or "updated_at".
	SortBy string
	// SortOrder is "desc" (default) or "asc".
	SortOrder string
}

// Normalize validates and fills defaults.
func (o SessionSearchOptions) Normalize() (SessionSearchOptions, error) {
	if o.Limit == 0 {
		o.Limit = 20
	}
	if o.Page == 0 {
		o.Page = 1
	}
	if o.SortBy == "" {
		o.SortBy = "created_at"
	}
	if o.SortOrder == "" {
		o.SortOrder = "desc"
	}
	if o.Page < 1 || o.Limit < 1 {
		return o, fmt.Errorf("page and limit must be positive")
	}
	if (o.SortBy != "created_at" && o.SortBy != "updated_at") || (o.SortOrder != "asc" && o.SortOrder != "desc") {
		return o, fmt.Errorf("invalid session sort")
	}
	return o, nil
}

// SessionSearcher is implemented by backends that support advanced session
// search with a filter tree. It returns the requested page and the total
// number of matching sessions.
type SessionSearcher interface {
	SearchSessions(ctx context.Context, filter *SessionFilter, opts SessionSearchOptions) ([]*AgentSession, int, error)
}

// FilterSortPaginate is the portable search fallback: it filters sessions in
// memory with a normalized filter, sorts and paginates them. Backends without
// native filtering use it.
func FilterSortPaginate(sessions []*AgentSession, filter *SessionFilter, opts SessionSearchOptions) ([]*AgentSession, int, error) {
	opts, err := opts.Normalize()
	if err != nil {
		return nil, 0, err
	}
	nf, err := NormalizeSessionFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	var matches []*AgentSession
	for _, s := range sessions {
		if opts.UserID != nil && s.UserID != *opts.UserID {
			continue
		}
		if MatchesSession(s, nf) {
			matches = append(matches, s)
		}
	}
	SortSessions(matches, opts.SortBy, opts.SortOrder)
	total := len(matches)
	start := (opts.Page - 1) * opts.Limit
	if start >= total {
		return []*AgentSession{}, total, nil
	}
	end := start + opts.Limit
	if end > total {
		end = total
	}
	return matches[start:end], total, nil
}

// SortSessions sorts sessions by created_at or updated_at, breaking ties by session_id.
func SortSessions(sessions []*AgentSession, sortBy, sortOrder string) {
	key := func(s *AgentSession) int64 {
		if sortBy == "updated_at" {
			return s.UpdatedAt
		}
		return s.CreatedAt
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		a, b := key(sessions[i]), key(sessions[j])
		if a == b {
			if sortOrder == "asc" {
				return sessions[i].SessionID < sessions[j].SessionID
			}
			return sessions[i].SessionID > sessions[j].SessionID
		}
		if sortOrder == "asc" {
			return a < b
		}
		return a > b
	})
}
