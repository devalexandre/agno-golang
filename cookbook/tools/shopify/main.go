// Example: analyze Shopify sales with the Shopify toolkit.
//
// Requires SHOPIFY_SHOP_NAME and SHOPIFY_ACCESS_TOKEN (scopes read_orders,
// read_products, read_customers).
//
//	go run ./cookbook/tools/shopify
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/shopify"
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

	shop := shopify.New()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Shopify Analyst",
		Model:         model,
		Instructions:  "Analyze the Shopify store data and give concise, actionable insights.",
		Tools:         []toolkit.Tool{shop},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("What are my top 5 selling products and which products are often bought together?", true, true)
}
