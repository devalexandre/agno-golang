// Example: query Amazon Redshift with RedshiftTools.
//
// Requires REDSHIFT_HOST, REDSHIFT_DATABASE, REDSHIFT_USER and REDSHIFT_PASSWORD.
//
//	go run ./cookbook/tools/redshift
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/redshift"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	rs := redshift.NewRedshiftTools(redshift.WithTableSchema("public"))
	defer rs.Close()
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "Data Analyst",
		Model:         model,
		Instructions:  "Describe tables before querying them. Only run read-only queries.",
		Tools:         []toolkit.Tool{rs},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("Which tables exist? Summarize the largest one.", true, true)
}
