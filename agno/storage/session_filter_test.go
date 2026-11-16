package storage

import (
	"strings"
	"testing"
)

func TestNormalizeSessionFilterValidation(t *testing.T) {
	deep := Where("user_id", FilterEq, "u")
	for i := 0; i <= MaxSessionFilterDepth; i++ {
		deep = Not(deep)
	}
	tooMany := make([]*SessionFilter, MaxSessionFilterConditions+1)
	for i := range tooMany {
		tooMany[i] = Where("user_id", FilterEq, "u")
	}
	cases := map[string]*SessionFilter{
		"depth":         deep,
		"conditions":    And(tooMany...),
		"empty AND":     And(),
		"unknown field": Where("foo", FilterEq, "x"),
		"bad operator":  Where("session_type", FilterContains, "a"),
		"bad enum":      Where("session_type", FilterEq, "robot"),
		"non-string":    Where("user_id", FilterEq, 3),
		"bad timestamp": Where("created_at", FilterGt, "yesterday"),
		"empty IN":      WhereIn("user_id"),
		"missing NOT":   Not(nil),
	}
	for name, f := range cases {
		if _, err := NormalizeSessionFilter(f); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	nf, err := NormalizeSessionFilter(Where("created_at", FilterGte, "1970-01-01T00:01:40Z"))
	if err != nil || nf.Value != float64(100) {
		t.Fatalf("datetime normalization: %+v %v", nf, err)
	}
}

func TestParseSessionFilterAndSQL(t *testing.T) {
	f, err := ParseSessionFilter([]byte(`{"op":"OR","conditions":[{"op":"IN","key":"user_id","values":["a","b"]},{"op":"STARTSWITH","key":"agent_id","value":"x_%"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	clause, args, ok := SessionFilterToSQL(f, SessionFilterSQLOptions{
		Columns: map[string]string{"user_id": "user_id", "agent_id": "agent_id"}, BinaryCompare: true, NullIfEmpty: true,
	})
	if !ok || !strings.Contains(clause, "BINARY NULLIF(user_id, '') IN (?, ?)") || !strings.Contains(clause, "LIKE ? ESCAPE '!'") {
		t.Fatalf("clause = %s", clause)
	}
	if args[2] != "x!_!%%" {
		t.Fatalf("like pattern = %v", args[2])
	}
	if _, _, ok := SessionFilterToSQL(Where("session_name", FilterEq, "n"), SessionFilterSQLOptions{Columns: map[string]string{}}); ok {
		t.Fatal("expected untranslatable filter")
	}
}

func TestMatchesSessionThreeValued(t *testing.T) {
	s := NewAgentSession("s", "", "agent")
	if MatchesSession(s, Not(Where("user_id", FilterEq, "x"))) {
		t.Fatal("NOT over NULL must not match")
	}
	if !MatchesSession(s, Or(Where("user_id", FilterEq, "x"), Where("agent_id", FilterEq, "agent"))) {
		t.Fatal("OR with a true branch must match")
	}
}
