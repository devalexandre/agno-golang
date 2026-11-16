// Heabsy model provider (heabsy.NewHeabsyChat), an OpenAI-compatible endpoint at heabsy.DefaultBaseURL.
//
// The API key is read from HEABSY_API_KEY when models.WithAPIKey is not given;
// the model defaults to heabsy.DefaultModel and can be overridden with
// HEABSY_MODEL. It also resolves through the model factory as "heabsy:<model>".
//
//	export HEABSY_API_KEY="..."
//	go run ./cookbook/models/heabsy
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/heabsy"
)

func main() {
	if os.Getenv(heabsy.EnvAPIKey) == "" {
		log.Fatalf("%s environment variable is required", heabsy.EnvAPIKey)
	}

	modelID := os.Getenv("HEABSY_MODEL")
	if modelID == "" {
		modelID = heabsy.DefaultModel
	}
	model, err := heabsy.NewHeabsyChat(models.WithID(modelID))
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      context.Background(),
		Model:        model,
		Name:         "Heabsy Assistant",
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
