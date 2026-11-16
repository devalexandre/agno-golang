// Example: think / search / analyze over a knowledge base with KnowledgeTools.
//
// Env vars: none (no API key needed). Requires a local Ollama server with
// llama3.2 at http://localhost:11434.
//
// The demo uses a tiny in-memory knowledge base so it runs without a vector DB.
// In a real app pass your knowledge.Knowledge (the same one given to the agent,
// e.g. agent.GetKnowledge()) to knowledgetools.NewKnowledgeTools.
//
//	go run ./cookbook/tools/knowledgetools
package main

import (
	"context"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/knowledgetools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// memoryKB is a keyword-matching stand-in for a vector knowledge base.
type memoryKB []string

func (kb memoryKB) Search(_ context.Context, query string, n int) ([]*knowledge.SearchResult, error) {
	var out []*knowledge.SearchResult
	for _, text := range kb {
		for _, w := range strings.Fields(strings.ToLower(query)) {
			if len(w) > 3 && strings.Contains(strings.ToLower(text), w) {
				out = append(out, &knowledge.SearchResult{Document: &document.Document{Content: text}, Score: 1})
				break
			}
		}
		if len(out) >= n {
			break
		}
	}
	return out, nil
}

func main() {
	ctx := context.Background()
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	kb := memoryKB{
		"Pad Thai is a stir-fried rice noodle dish with tamarind, fish sauce, peanuts and egg.",
		"Tom Yum is a hot and sour Thai soup flavoured with lemongrass, galangal and lime leaves.",
		"Green curry uses green chillies, coconut milk and Thai basil.",
	}
	kt, err := knowledgetools.NewKnowledgeTools(kb, knowledgetools.WithNumDocuments(3))
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Thai Recipes",
		Model:         model,
		Instructions:  kt.Instructions,
		Tools:         []toolkit.Tool{kt},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Which Thai dish uses lemongrass, and what else goes into it?", true, true)
}
