// AI/ML API model provider (aimlapi.NewAIMLAPIChat), an OpenAI-compatible gateway at aimlapi.DefaultBaseURL. Agno attribution
// headers (aimlapi.AttributionHeaders) are added to every call; headers passed
// with models.WithDefaultHeaders win on a clash.
//
// The API key is read from AIMLAPI_API_KEY when models.WithAPIKey is not given;
// the model defaults to aimlapi.DefaultModel and can be overridden with
// AIMLAPI_MODEL. It also resolves through the model factory as "aimlapi:<model>".
//
//	export AIMLAPI_API_KEY="..."
//	go run ./cookbook/models/aimlapi
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/aimlapi"
)

func main() {
	if os.Getenv(aimlapi.EnvAPIKey) == "" {
		log.Fatalf("%s environment variable is required", aimlapi.EnvAPIKey)
	}

	modelID := os.Getenv("AIMLAPI_MODEL")
	if modelID == "" {
		modelID = aimlapi.DefaultModel
	}
	model, err := aimlapi.NewAIMLAPIChat(models.WithID(modelID))
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      context.Background(),
		Model:        model,
		Name:         "AI/ML API Assistant",
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
