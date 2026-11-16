// Example: market data with OpenBBTools.
//
// Requires a running OpenBB Platform API (`pip install openbb openbb-api` then
// `openbb-api`), reachable at OPENBB_API_URL (default http://127.0.0.1:6900/api/v1).
//
//	go run ./cookbook/tools/openbb
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/openbb"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "Market Analyst",
		Model:         model,
		Tools:         []toolkit.Tool{openbb.NewOpenBBTools(openbb.WithAll())},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("Compare the current prices of AAPL and MSFT and show the latest news for AAPL.", true, true)
}
