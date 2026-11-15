// Component registry with reserved names (agno/registry).
//
// A Registry catalogs the live objects a stored component config refers to
// by id or name: agents, teams, workflows, models, tools, functions, dbs,
// vector dbs, knowledge, schemas. Framework-owned ids ("superagent",
// "platform-builder", "miniagent" and pre-3.2 manager ids) are reserved: a
// user component claiming one is refused with a message naming the remedy.
//
// Runs fully offline. Requires no env vars or services.
package main

import (
	"fmt"

	"github.com/devalexandre/agno-golang/agno/registry"
	"github.com/devalexandre/agno-golang/agno/tools"
)

type component struct{ id string }

func (c *component) GetID() string { return c.id }

type Report struct {
	Title string `json:"title"`
}

func main() {
	r := registry.New("my-os")

	for _, id := range []string{"support-agent", registry.ManagerTeamID, "os-manager-internal"} {
		if err := r.AddAgent(&component{id}); err != nil {
			fmt.Printf("refused %-22s %v\n", id, err)
		} else {
			fmt.Printf("registered %s\n", id)
		}
	}

	// Tools: a declared tool is buildable; one only discovered on a component is not.
	r.AddTool(tools.NewEchoTool(), registry.ToolDeclared)
	fmt.Println("EchoTool declared:", r.ToolIsDeclared("EchoTool"))

	// Knowledge names must be unambiguous for strict resolution.
	r.AddKnowledge("docs", &component{"kb-1"})
	r.AddKnowledge("docs", &component{"kb-2"})
	if _, err := r.GetKnowledgeStrict("docs"); err != nil {
		fmt.Println("strict lookup:", err)
	}

	// Schemas are resolved by type name, e.g. from a stored output_schema.
	r.AddSchema(Report{})
	fmt.Printf("schema instance: %#v\n", r.GetSchema("Report"))
	fmt.Println("all component ids:", r.AllComponentIDs())
}
