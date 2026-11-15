// Y-API model provider (yapi.NewYAPIChat), an OpenAI-compatible gateway at yapi.DefaultBaseURL.
//
// The API key is read from YAPI_API_KEY when models.WithAPIKey is not given;
// the model defaults to yapi.DefaultModel and can be overridden with
// YAPI_MODEL. It also resolves through the model factory as "yapi:<model>".
//
//	export YAPI_API_KEY="..."
//	go run ./cookbook/models/yapi
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/yapi"
)

func main() {
	if os.Getenv(yapi.EnvAPIKey) == "" {
		log.Fatalf("%s environment variable is required", yapi.EnvAPIKey)
	}

	modelID := os.Getenv("YAPI_MODEL")
	if modelID == "" {
		modelID = yapi.DefaultModel
	}
	model, err := yapi.NewYAPIChat(models.WithID(modelID))
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      context.Background(),
		Model:        model,
		Name:         "Y-API Assistant",
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
