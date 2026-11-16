// Example: using Moonshot (Kimi) with agno-golang.
//
// API key: set MOONSHOT_API_KEY (export MOONSHOT_API_KEY=...).
// Default model: kimi-k3
//
//	go run ./cookbook/models/moonshot
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/moonshot"
)

func main() {
	ctx := context.Background()

	model, err := moonshot.NewMoonShotChat(
		models.WithID(moonshot.DefaultModel),
		// Optional: toggle thinking mode (Kimi K2.x). Omit to use the model default.
		models.WithClientParams(map[string]interface{}{moonshot.ParamUseThinking: true}),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Moonshot (Kimi) Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}
