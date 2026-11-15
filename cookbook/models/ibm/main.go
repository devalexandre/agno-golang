// Example: using IBM watsonx.ai with agno-golang.
//
// API key: set IBM_WATSONX_API_KEY and IBM_WATSONX_PROJECT_ID
// (optional IBM_WATSONX_URL, default https://eu-de.ml.cloud.ibm.com).
// Default model: ibm/granite-20b-code-instruct
//
//	go run ./cookbook/models/ibm
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ibm"
)

func main() {
	ctx := context.Background()

	model, err := ibm.NewWatsonXChat(
		models.WithID(ibm.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "WatsonX Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}
