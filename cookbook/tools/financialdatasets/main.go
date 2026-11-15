// Example: financial statements and prices with FinancialDatasetsTools.
//
// Requires FINANCIAL_DATASETS_API_KEY.
//
//	go run ./cookbook/tools/financialdatasets
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/financialdatasets"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "Financial Analyst",
		Model:         model,
		Instructions:  "Use the financial datasets tools to answer with real figures.",
		Tools:         []toolkit.Tool{financialdatasets.NewFinancialDatasetsTools()},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("Summarize Apple's last two annual income statements.", true, true)
}
