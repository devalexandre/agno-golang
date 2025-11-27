package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
)

// SearchTool is a simple search tool for demonstration
type SearchTool struct {
	toolkit.Toolkit
}

// SearchParams defines the parameters for search
type SearchParams struct {
	Query string `json:"query" description:"The search query"`
}

// Search performs a simulated web search
func (s *SearchTool) Search(params SearchParams) (string, error) {
	// This is a simplified demo - in production you'd call a real search API
	results := fmt.Sprintf(`Search results for "%s":

1. Latest developments in %s - TechNews
   Comprehensive overview of recent developments and trends in %s
   
2. %s: A Complete Guide - TechGuide  
   Everything you need to know about %s, updated for 2024
   
3. How %s is Transforming Industries - IndustryWeek
   Analysis of how %s is revolutionizing various industries`,
		params.Query, params.Query, params.Query, params.Query, params.Query, params.Query, params.Query)

	return results, nil
}

func main() {
	ctx := context.Background()

	// Enable markdown for beautiful output
	utils.SetMarkdownMode(true)

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create search tool
	searchTool := tools.NewDuckDuckGoTool()

	// Create a News Reporter Agent with search capabilities
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "NewsReporter",
		Instructions: `You are an enthusiastic news reporter with a flair for storytelling! 🗽
Think of yourself as a mix between a witty comedian and a sharp journalist.

Follow these guidelines for every report:
1. Start with an attention-grabbing headline using relevant emoji
2. Use the search tool to find information about topics
3. Present news with authentic NYC enthusiasm and local flavor
4. Structure your reports in clear sections:
    - Catchy headline
    - Brief summary
    - Key details
    - Local impact or context
5. Keep responses concise but informative (2-3 paragraphs max)
6. Include NYC-style commentary and local references
7. End with a signature sign-off phrase

Sign-off examples:
- 'Back to you in the studio, folks!'
- 'Reporting live from the city that never sleeps!'
- 'This is NewsReporter, live from the heart of Manhattan!'

Remember: Use the search tool to find information and maintain that authentic NYC energy!`,
		Tools:    []toolkit.Tool{searchTool},
		Debug:    false,
		Markdown: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Example usage
	ag.PrintResponse("Tell me about the latest developments in AI technology.", true, true)

	// More example prompts to try:
	/*
		ag.PrintResponse("What's happening in the tech world today?", true, true)
		ag.PrintResponse("Tell me about recent innovations in renewable energy", true, true)
		ag.PrintResponse("What are the latest trends in software development?", true, true)
	*/
}
