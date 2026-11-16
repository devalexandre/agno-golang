// Example: analyse a CSV file with CsvTools.
//
// Env vars: none (no API key needed). Requires a local Ollama server with
// llama3.2 at http://localhost:11434.
//
//	go run ./cookbook/tools/csvtools
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/csvtools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	path := filepath.Join(os.TempDir(), "movies.csv")
	if err := os.WriteFile(path, []byte("Title,Year,Rating\nAlien,1979,8.5\nHeat,1995,8.3\nUp,2009,8.2\n"), 0o644); err != nil {
		log.Fatal(err)
	}
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "CSV Analyst",
		Model:         model,
		Instructions:  "First get the columns, then answer with QueryCsvFile (SQLite SQL).",
		Tools:         []toolkit.Tool{csvtools.NewCsvTools([]string{path})},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("What is the average rating of movies released after 1990?", true, true)
}
