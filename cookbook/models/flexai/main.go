// FlexAI model provider (flexai.NewFlexAIChat), an OpenAI-compatible endpoint at flexai.DefaultBaseURL.
//
// The API key is read from FLEXAI_API_KEY when models.WithAPIKey is not given;
// the model defaults to flexai.DefaultModel and can be overridden with
// FLEXAI_MODEL. It also resolves through the model factory as "flexai:<model>".
//
//	export FLEXAI_API_KEY="..."
//	go run ./cookbook/models/flexai
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/flexai"
)

func main() {
	if os.Getenv(flexai.EnvAPIKey) == "" {
		log.Fatalf("%s environment variable is required", flexai.EnvAPIKey)
	}

	modelID := os.Getenv("FLEXAI_MODEL")
	if modelID == "" {
		modelID = flexai.DefaultModel
	}
	model, err := flexai.NewFlexAIChat(models.WithID(modelID))
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      context.Background(),
		Model:        model,
		Name:         "FlexAI Assistant",
		Instructions: "You are a concise, helpful assistant.",
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := ag.Run("Give me one fun fact about octopuses.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[%s] %s\n", model.GetID(), resp.TextContent)
}
