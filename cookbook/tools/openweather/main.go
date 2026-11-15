// Example: weather questions with OpenWeatherTools.
//
// Requires OPENWEATHER_API_KEY.
//
//	go run ./cookbook/tools/openweather
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/openweather"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "Weather Assistant",
		Model:         model,
		Tools:         []toolkit.Tool{openweather.NewOpenWeatherTools(openweather.WithUnits("metric"))},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("What's the weather and air quality in London right now? And the forecast for 3 days?", true, true)
}
