// Example: Airflow DAG Writer with the airflow toolkit.
//
// Writes DAG files under ./dags (no credentials needed).
//
//	go run ./cookbook/tools/airflow
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/airflow"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	tools := airflow.New(airflow.WithDagsDir("./dags"))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Airflow DAG Writer",
		Model:         model,
		Instructions:  "Write Apache Airflow DAGs in Python and save them with SaveDagFile.",
		Tools:         []toolkit.Tool{tools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Write a DAG named hello_world.py that prints 'hello' every day at 9am, save it and read it back.", true, true)
}
