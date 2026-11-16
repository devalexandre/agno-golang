// Package filters ports agno/filters.py: composable filter expressions for
// knowledge bases and vector databases (EQ, NEQ, GT, GTE, LT, LTE, IN,
// CONTAINS, STARTSWITH, AND, OR, NOT).
//
// Expressions serialize to the same dictionary shape as Python
// ({"op": "EQ", "key": ..., "value": ...}) so they can travel through JSON
// APIs, and FromDict rebuilds them. Match evaluates an expression against a
// metadata map, which backends without native filter support (and the
// in-process post-filtering of search results) can use directly.
package filters

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// MaxFilterDepth bounds nesting when deserializing.
const MaxFilterDepth = 10

// Op names a filter operator.
type Op string

const (
	OpEQ         Op = "EQ"
	OpNEQ        Op = "NEQ"
	OpGT         Op = "GT"
	OpGTE        Op = "GTE"
	OpLT         Op = "LT"
	OpLTE        Op = "LTE"
	OpIN         Op = "IN"
	OpCONTAINS   Op = "CONTAINS"
	OpSTARTSWITH Op = "STARTSWITH"
	OpAND        Op = "AND"
	OpOR         Op = "OR"
	OpNOT        Op = "NOT"
)

// Expr is a filter expression node.
type Expr struct {
	Op         Op
	Key        string
	Value      interface{}
	Values     []interface{} // IN
	Conditions []*Expr       // AND / OR
	Condition  *Expr         // NOT
}

func cmp(op Op, key string, value interface{}) *Expr { return &Expr{Op: op, Key: key, Value: value} }

// EQ matches documents where key equals value.
func EQ(key string, value interface{}) *Expr { return cmp(OpEQ, key, value) }

// NEQ matches documents where key does not equal value.
func NEQ(key string, value interface{}) *Expr { return cmp(OpNEQ, key, value) }

// GT matches documents where key > value.
func GT(key string, value interface{}) *Expr { return cmp(OpGT, key, value) }

// GTE matches documents where key >= value.
func GTE(key string, value interface{}) *Expr { return cmp(OpGTE, key, value) }

// LT matches documents where key < value.
func LT(key string, value interface{}) *Expr { return cmp(OpLT, key, value) }

// LTE matches documents where key <= value.
func LTE(key string, value interface{}) *Expr { return cmp(OpLTE, key, value) }

// CONTAINS matches documents where key contains value (case-insensitive substring).
func CONTAINS(key string, value string) *Expr { return cmp(OpCONTAINS, key, value) }

// STARTSWITH matches documents where key starts with value.
func STARTSWITH(key string, value string) *Expr { return cmp(OpSTARTSWITH, key, value) }

// IN matches documents where key is one of values.
func IN(key string, values ...interface{}) *Expr {
	return &Expr{Op: OpIN, Key: key, Values: values}
}

// AND matches when all expressions match.
func AND(exprs ...*Expr) *Expr { return &Expr{Op: OpAND, Conditions: exprs} }

// OR matches when any expression matches.
func OR(exprs ...*Expr) *Expr { return &Expr{Op: OpOR, Conditions: exprs} }

// NOT negates an expression.
func NOT(expr *Expr) *Expr { return &Expr{Op: OpNOT, Condition: expr} }

// And combines e with other using AND (Python's &).
func (e *Expr) And(other *Expr) *Expr { return AND(e, other) }

// Or combines e with other using OR (Python's |).
func (e *Expr) Or(other *Expr) *Expr { return OR(e, other) }

// Not negates e (Python's ~).
func (e *Expr) Not() *Expr { return NOT(e) }

// ToDict converts the expression to its dictionary representation.
func (e *Expr) ToDict() map[string]interface{} {
	switch e.Op {
	case OpIN:
		vals := e.Values
		if vals == nil {
			vals = []interface{}{}
		}
		return map[string]interface{}{"op": string(e.Op), "key": e.Key, "values": vals}
	case OpAND, OpOR:
		conds := make([]interface{}, 0, len(e.Conditions))
		for _, c := range e.Conditions {
			conds = append(conds, c.ToDict())
		}
		return map[string]interface{}{"op": string(e.Op), "conditions": conds}
	case OpNOT:
		var cond interface{}
		if e.Condition != nil {
			cond = e.Condition.ToDict()
		}
		return map[string]interface{}{"op": string(e.Op), "condition": cond}
	default:
		return map[string]interface{}{"op": string(e.Op), "key": e.Key, "value": e.Value}
	}
}

// MarshalJSON encodes the dictionary representation.
func (e *Expr) MarshalJSON() ([]byte, error) { return json.Marshal(e.ToDict()) }

// UnmarshalJSON decodes the dictionary representation.
func (e *Expr) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	parsed, err := FromDict(m)
	if err != nil {
		return err
	}
	*e = *parsed
	return nil
}

// String renders a readable form, e.g. AND(EQ(status, active), GT(age, 18)).
func (e *Expr) String() string {
	switch e.Op {
	case OpAND, OpOR:
		parts := make([]string, len(e.Conditions))
		for i, c := range e.Conditions {
			parts[i] = c.String()
		}
		return fmt.Sprintf("%s(%s)", e.Op, strings.Join(parts, ", "))
	case OpNOT:
		return fmt.Sprintf("NOT(%v)", e.Condition)
	case OpIN:
		return fmt.Sprintf("IN(%s, %v)", e.Key, e.Values)
	}
	return fmt.Sprintf("%s(%s, %v)", e.Op, e.Key, e.Value)
}

// FromDict reconstructs an expression from its dictionary representation.
func FromDict(d map[string]interface{}) (*Expr, error) { return fromDict(d, 0) }

func fromDict(d map[string]interface{}, depth int) (*Expr, error) {
	if depth > MaxFilterDepth {
		return nil, fmt.Errorf("filter expression exceeds maximum nesting depth of %d", MaxFilterDepth)
	}
	opRaw, ok := d["op"]
	if !ok {
		return nil, fmt.Errorf("invalid filter dictionary: must contain 'op' key. Got: %v", d)
	}
	op := Op(fmt.Sprint(opRaw))
	switch op {
	case OpEQ, OpNEQ, OpGT, OpGTE, OpLT, OpLTE, OpCONTAINS, OpSTARTSWITH:
		key, kok := d["key"]
		val, vok := d["value"]
		if !kok || !vok {
			return nil, fmt.Errorf("%s filter requires 'key' and 'value' fields. Got: %v", op, d)
		}
		return &Expr{Op: op, Key: fmt.Sprint(key), Value: val}, nil
	case OpIN:
		key, kok := d["key"]
		vals, vok := d["values"]
		if !kok || !vok {
			return nil, fmt.Errorf("IN filter requires 'key' and 'values' fields. Got: %v", d)
		}
		list, err := toSlice(vals)
		if err != nil {
			return nil, fmt.Errorf("IN filter 'values' must be a list: %w", err)
		}
		return &Expr{Op: OpIN, Key: fmt.Sprint(key), Values: list}, nil
	case OpAND, OpOR:
		raw, ok := d["conditions"]
		if !ok {
			return nil, fmt.Errorf("%s filter requires 'conditions' field. Got: %v", op, d)
		}
		list, err := toSlice(raw)
		if err != nil {
			return nil, fmt.Errorf("%s 'conditions' must be a list: %w", op, err)
		}
		conds := make([]*Expr, 0, len(list))
		for _, c := range list {
			cm, ok := c.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid filter dictionary: %v", c)
			}
			sub, err := fromDict(cm, depth+1)
			if err != nil {
				return nil, err
			}
			conds = append(conds, sub)
		}
		return &Expr{Op: op, Conditions: conds}, nil
	case OpNOT:
		raw, ok := d["condition"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("NOT filter requires 'condition' field. Got: %v", d)
		}
		sub, err := fromDict(raw, depth+1)
		if err != nil {
			return nil, err
		}
		return &Expr{Op: OpNOT, Condition: sub}, nil
	}
	return nil, fmt.Errorf("unknown filter operator: %s", op)
}

func toSlice(v interface{}) ([]interface{}, error) {
	if s, ok := v.([]interface{}); ok {
		return s, nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, fmt.Errorf("got %T", v)
	}
	out := make([]interface{}, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out, nil
}

// FromMap converts a legacy equality map ({"k": v}) into AND(EQ(k, v)...).
// A slice value becomes IN. Returns nil for an empty map.
func FromMap(m map[string]interface{}) *Expr {
	if len(m) == 0 {
		return nil
	}
	conds := make([]*Expr, 0, len(m))
	for k, v := range m {
		if list, err := toSlice(v); err == nil && v != nil {
			if _, isStr := v.(string); !isStr {
				conds = append(conds, IN(k, list...))
				continue
			}
		}
		conds = append(conds, EQ(k, v))
	}
	if len(conds) == 1 {
		return conds[0]
	}
	return AND(conds...)
}

// Match evaluates the expression against a metadata map. Keys may be dotted
// paths into nested maps ("meta.author"). A missing key never matches a
// comparison (and therefore matches its NOT).
func (e *Expr) Match(metadata map[string]interface{}) bool {
	if e == nil {
		return true
	}
	switch e.Op {
	case OpAND:
		for _, c := range e.Conditions {
			if !c.Match(metadata) {
				return false
			}
		}
		return true
	case OpOR:
		for _, c := range e.Conditions {
			if c.Match(metadata) {
				return true
			}
		}
		return false
	case OpNOT:
		return !e.Condition.Match(metadata)
	}
	actual, ok := lookup(metadata, e.Key)
	if !ok {
		return e.Op == OpNEQ
	}
	switch e.Op {
	case OpEQ:
		return equal(actual, e.Value)
	case OpNEQ:
		return !equal(actual, e.Value)
	case OpIN:
		for _, v := range e.Values {
			if equal(actual, v) {
				return true
			}
		}
		// A list-valued field matches IN when any element is in the list.
		if list, err := toSlice(actual); err == nil {
			if _, isStr := actual.(string); !isStr {
				for _, a := range list {
					for _, v := range e.Values {
						if equal(a, v) {
							return true
						}
					}
				}
			}
		}
		return false
	case OpCONTAINS:
		if list, err := toSlice(actual); err == nil {
			if _, isStr := actual.(string); !isStr {
				for _, a := range list {
					if equal(a, e.Value) {
						return true
					}
				}
				return false
			}
		}
		return strings.Contains(strings.ToLower(fmt.Sprint(actual)), strings.ToLower(fmt.Sprint(e.Value)))
	case OpSTARTSWITH:
		return strings.HasPrefix(fmt.Sprint(actual), fmt.Sprint(e.Value))
	case OpGT, OpGTE, OpLT, OpLTE:
		c, ok := compare(actual, e.Value)
		if !ok {
			return false
		}
		switch e.Op {
		case OpGT:
			return c > 0
		case OpGTE:
			return c >= 0
		case OpLT:
			return c < 0
		default:
			return c <= 0
		}
	}
	return false
}

// MatchAll reports whether metadata matches every expression (nil entries ignored).
func MatchAll(metadata map[string]interface{}, exprs ...*Expr) bool {
	for _, e := range exprs {
		if e != nil && !e.Match(metadata) {
			return false
		}
	}
	return true
}

func lookup(m map[string]interface{}, key string) (interface{}, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	if !strings.Contains(key, ".") {
		return nil, false
	}
	cur := interface{}(m)
	for _, part := range strings.Split(key, ".") {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = mm[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func equal(a, b interface{}) bool {
	if fa, ok := toFloat(a); ok {
		if fb, ok := toFloat(b); ok {
			return fa == fb
		}
	}
	return reflect.DeepEqual(a, b)
}

// compare returns -1/0/1; numbers numerically, times chronologically, else as strings.
func compare(a, b interface{}) (int, bool) {
	if fa, ok := toFloat(a); ok {
		fb, ok := toFloat(b)
		if !ok {
			return 0, false
		}
		switch {
		case fa < fb:
			return -1, true
		case fa > fb:
			return 1, true
		}
		return 0, true
	}
	if ta, ok := a.(time.Time); ok {
		if tb, ok := b.(time.Time); ok {
			return ta.Compare(tb), true
		}
	}
	sa, aok := a.(string)
	sb, bok := b.(string)
	if !aok || !bok {
		return 0, false
	}
	return strings.Compare(sa, sb), true
}
