package filters

import (
	"encoding/json"
	"testing"
)

func TestMatch(t *testing.T) {
	doc := map[string]interface{}{
		"status": "published", "views": 1500, "category": "tech",
		"tags": []interface{}{"go", "ai"}, "meta": map[string]interface{}{"author": "Ana"},
	}
	cases := []struct {
		e    *Expr
		want bool
	}{
		{EQ("status", "published"), true},
		{NEQ("status", "archived"), true},
		{GT("views", 1000), true},
		{LTE("views", 1000.0), false},
		{IN("category", "tech", "science"), true},
		{IN("tags", "ai"), true},
		{CONTAINS("status", "PUB"), true},
		{CONTAINS("tags", "go"), true},
		{STARTSWITH("category", "te"), true},
		{EQ("meta.author", "Ana"), true},
		{EQ("missing", 1), false},
		{NOT(EQ("status", "archived")), true},
		{AND(EQ("status", "published"), GT("views", 1000), IN("category", "tech")), true},
		{OR(EQ("status", "draft"), EQ("category", "tech")), true},
		{EQ("status", "draft").Or(EQ("status", "x")), false},
		{GTE("created", "2025-01-01"), false},
	}
	for i, c := range cases {
		if got := c.e.Match(doc); got != c.want {
			t.Errorf("case %d %s: got %v want %v", i, c.e, got, c.want)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	e := OR(AND(EQ("type", "article"), GT("word_count", 500)), AND(EQ("type", "tutorial"), NOT(EQ("difficulty", "beginner"))), IN("x", 1, 2))
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var back Expr
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	data2, _ := json.Marshal(&back)
	if string(data) != string(data2) {
		t.Fatalf("round trip mismatch\n%s\n%s", data, data2)
	}
	doc := map[string]interface{}{"type": "tutorial", "difficulty": "advanced"}
	if !back.Match(doc) {
		t.Fatal("decoded expr should match")
	}
}

func TestFromDictErrors(t *testing.T) {
	if _, err := FromDict(map[string]interface{}{"op": "BOGUS"}); err == nil {
		t.Fatal("expected unknown op error")
	}
	if _, err := FromDict(map[string]interface{}{"op": "EQ", "key": "a"}); err == nil {
		t.Fatal("expected missing value error")
	}
	deep := map[string]interface{}{"op": "EQ", "key": "a", "value": 1}
	for i := 0; i < MaxFilterDepth+2; i++ {
		deep = map[string]interface{}{"op": "NOT", "condition": deep}
	}
	if _, err := FromDict(deep); err == nil {
		t.Fatal("expected depth error")
	}
}

func TestFromMap(t *testing.T) {
	e := FromMap(map[string]interface{}{"a": 1, "b": []string{"x", "y"}})
	if !e.Match(map[string]interface{}{"a": 1, "b": "y"}) {
		t.Fatal("FromMap should match")
	}
	if FromMap(nil) != nil {
		t.Fatal("nil map")
	}
}
