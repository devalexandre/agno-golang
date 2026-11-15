// Example: using Cloudflare AI Gateway with agno-golang.
//
// API key: set CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID (optional CLOUDFLARE_AI_GATEWAY_ID).
// Default model: @cf/meta/llama-3.3-70b-instruct-fp8-fast
//
//	go run ./cookbook/models/cloudflare
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/cloudflare"
)

func main() {
	ctx := context.Background()

	model, err := cloudflare.NewCloudflareChat(
		models.WithID(cloudflare.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Cloudflare AI Gateway Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}
