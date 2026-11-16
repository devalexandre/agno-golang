package registry

import (
	"errors"
	"strings"
	"testing"
)

type comp struct{ id string }

func (c *comp) GetID() string { return c.id }

type tool struct{ name string }

func (t *tool) GetName() string { return t.name }

type modelA struct{ id string }

func (m *modelA) GetID() string { return m.id }

type Out struct{ A int }

func TestReservedIDs(t *testing.T) {
	r := New("test")
	var re *ReservedIDError
	if err := r.AddAgent(&comp{ManagerTeamID}); !errors.As(err, &re) {
		t.Fatalf("expected reserved error, got %v", err)
	}
	err := r.AddTeam(&comp{"os-manager-internal"})
	if err == nil || !strings.Contains(err.Error(), "replaced by 'superagent'") {
		t.Fatalf("former id message: %v", err)
	}
	r.ReserveComponentIDs("mine")
	if r.AddWorkflow(&comp{"mine"}) == nil {
		t.Fatal("custom reserved id must be refused")
	}
	if err := r.AddAgent(&comp{"a1"}); err != nil {
		t.Fatal(err)
	}
	if r.AddAgent(&comp{"a1"}) == nil {
		t.Fatal("duplicate id must fail")
	}
	if len(r.AllComponentIDs()) != 1 || r.GetAgent("a1") == nil {
		t.Fatal("lookup")
	}
}

func TestCatalog(t *testing.T) {
	r := New("")
	r.AddModel(&modelA{"gpt"})
	r.AddModel(&modelA{"gpt"})
	if len(r.Models()) != 1 || r.GetModel("gpt", "modela") == nil {
		t.Fatal("model dedupe")
	}
	r.AddTool(&tool{"search"}, ToolDiscovered)
	if r.ToolIsDeclared("search") {
		t.Fatal("discovered tool is not declared")
	}
	r.AddTool(&tool{"search"}, ToolDeclared)
	if !r.ToolIsDeclared("search") {
		t.Fatal("declaration wins")
	}
	r.AddKnowledge("docs", &comp{"k1"})
	r.AddKnowledge("docs", &comp{"k2"})
	if !r.KnowledgeNameIsAmbiguous("docs") || r.GetKnowledge("docs").(*comp).id != "k1" {
		t.Fatal("ambiguity")
	}
	if _, err := r.GetKnowledgeStrict("docs"); err == nil {
		t.Fatal("strict must refuse")
	}
	r.AddSchema(Out{})
	if _, ok := r.GetSchema("Out").(*Out); !ok {
		t.Fatal("schema")
	}
}
