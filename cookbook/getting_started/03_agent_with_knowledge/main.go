package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/utils"
)

func main() {
	ctx := context.Background()

	// Enable markdown
	utils.SetMarkdownMode(true)

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create knowledge base (in-memory, no vector DB required)
	kb := knowledge.NewDocumentKnowledgeBase("agno_docs", nil)

	// Add some knowledge about Agno framework
	documents := []string{
		`Agno is a powerful framework for building AI agents in Go. 
It provides a simple and intuitive API for creating agents with tools, knowledge, and memory.`,

		`Agno agents can use various LLM providers including OpenAI, Anthropic, Google Gemini, and Ollama.
The framework supports streaming responses and structured outputs.`,

		`Knowledge in Agno allows agents to search through documents and retrieve relevant information.
It supports multiple vector databases like Qdrant, PgVector, and ChromaDB.`,

		`Agno tools enable agents to interact with external systems like web search, databases, and APIs.
You can easily create custom tools for your specific use cases using the toolkit.Tool interface.`,

		`Memory in Agno allows agents to remember past conversations and user preferences.
You can use SQLite or other databases to persist memory across sessions.`,
	}

	for i, docContent := range documents {
		doc := document.NewDocument(docContent)
		doc.Name = fmt.Sprintf("agno_doc_%d", i)
		doc.AddMetadata("source", "agno_docs")
		doc.AddMetadata("index", i)

		kb.AddDocument(doc)
	}

	// Note: Without a vector DB, the knowledge base won't do semantic search,
	// but we can still include the documents in the agent's context

	// Create agent with knowledge
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "AgnoExpert",
		Instructions: `You are an expert on the Agno framework for building AI agents in Go.

Here is what you know about Agno:

1. Agno is a powerful framework for building AI agents in Go with a simple and intuitive API
2. It supports multiple LLM providers: OpenAI, Anthropic, Google Gemini, and Ollama
3. It has streaming responses and structured outputs
4. Knowledge system allows searching through documents with vector databases (Qdrant, PgVector, ChromaDB)
5. Tools enable interaction with external systems using the toolkit.Tool interface
6. Memory system remembers conversations and preferences using SQLite or other databases

Use this information to answer questions accurately and provide helpful examples.
Always be specific about Agno's features and capabilities.`,
		Markdown: true,
		Debug:    false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Example usage
	ag.PrintResponse("What is Agno and what can it do?", true, true)

	// More example prompts:
	/*
		ag.PrintResponse("How do I add tools to an Agno agent?", true, true)
		ag.PrintResponse("What vector databases does Agno support?", true, true)
		ag.PrintResponse("Can you explain how memory works in Agno?", true, true)
	*/
}
