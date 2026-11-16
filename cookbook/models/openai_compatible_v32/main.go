// Example: the OpenAI-compatible providers added for Agno v3.2 parity:
// Y-API, FlexAI, Heabsy and AI/ML API.
//
// Each reads its API key from the environment when WithAPIKey is not given:
//
//	YAPI_API_KEY, FLEXAI_API_KEY, HEABSY_API_KEY, AIMLAPI_API_KEY
//
//	export AIMLAPI_API_KEY="..."
//	go run ./cookbook/models/openai_compatible_v32 aimlapi
//
// They can also be resolved by string through the model factory, e.g.
// factory.GetModel("yapi:deepseek/deepseek-v4-flash").
package main

import (
	"context"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/aimlapi"
	"github.com/devalexandre/agno-golang/agno/models/factory"
	"github.com/devalexandre/agno-golang/agno/models/flexai"
	"github.com/devalexandre/agno-golang/agno/models/heabsy"
	"github.com/devalexandre/agno-golang/agno/models/yapi"
)

func main() {
	provider := "aimlapi"
	if len(os.Args) > 1 {
		provider = os.Args[1]
	}

	var (
		model models.AgnoModelInterface
		err   error
	)
	switch provider {
	case "yapi":
		model, err = yapi.NewYAPIChat() // default model: yapi.DefaultModel
	case "flexai":
		model, err = flexai.NewFlexAIChat()
	case "heabsy":
		model, err = heabsy.NewHeabsyChat()
	case "aimlapi":
		// AI/ML API attribution headers are added automatically.
		model, err = aimlapi.NewAIMLAPIChat()
	default:
		// Any "provider:model" spec known to the factory.
		model, err = factory.GetModel(provider)
	}
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      context.Background(),
		Model:        model,
		Name:         "Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("Give me one fun fact about octopuses.", true, true)
}
