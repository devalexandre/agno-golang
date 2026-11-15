// Example: query a SQL database with SQLTools (SQLite in-memory demo).
//
//	go run ./cookbook/tools/sqltools
package main

import (
	"context"
	"log"

	_ "modernc.org/sqlite"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/sqltools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	sqlTools, err := sqltools.NewSQLToolsFromDSN("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	db := sqlTools.DB()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE orders (id INTEGER PRIMARY KEY, customer TEXT, total REAL);
		INSERT INTO orders(customer,total) VALUES ('ann',120.5),('bob',80),('ann',42);`); err != nil {
		log.Fatal(err)
	}

	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "SQL Analyst",
		Model:         model,
		Instructions:  "List and describe tables before querying.",
		Tools:         []toolkit.Tool{sqlTools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("Which customer spent the most?", true, true)
}
