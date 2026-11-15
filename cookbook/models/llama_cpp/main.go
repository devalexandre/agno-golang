// Example: using llama.cpp with agno-golang.
//
// No API key: start llama-server on http://127.0.0.1:8080.
// Default model: ggml-org/gpt-oss-20b-GGUF
//
//	go run ./cookbook/models/llama_cpp
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	llamacpp "github.com/devalexandre/agno-golang/agno/models/llama_cpp"
)

func main() {
	ctx := context.Background()

	model, err := llamacpp.NewLlamaCppChat(
		models.WithID(llamacpp.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "llama.cpp Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}
