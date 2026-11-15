// Example: explore a Neo4j graph with Neo4jTools (HTTP Query API v2).
//
// Requires NEO4J_URI (e.g. http://localhost:7474 or bolt://localhost:7687),
// NEO4J_USERNAME and NEO4J_PASSWORD.
//
//	go run ./cookbook/tools/neo4j
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/neo4j"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "Graph Analyst",
		Model:         model,
		Instructions:  "Inspect the schema before writing Cypher queries.",
		Tools:         []toolkit.Tool{neo4j.NewNeo4jTools()},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("What node labels exist and how many nodes of each label are there?", true, true)
}
