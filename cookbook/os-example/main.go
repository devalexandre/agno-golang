package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	agentOS "github.com/devalexandre/agno-golang/agno/os"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/qdrant"
)

func main() {

	ctx := context.Background()
	embedderModel := embedder.NewOllamaEmbedder(
		embedder.WithOllamaModel("embeddinggemma:latest", 768),
		embedder.WithOllamaHost("http://localhost:11434"),
	)

	if embedderModel == nil {
		log.Fatalf("Failed to create Ollama embedder model")
	}
	qdrantConfig := qdrant.QdrantConfig{
		Host:       "localhost",
		Port:       6334, // gRPC port
		Collection: "pdf-knowledge-base",
		Embedder:   embedderModel,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	}
	vectorDB, err := qdrant.NewQdrant(qdrantConfig)
	if err != nil {
		log.Fatalf("Failed to create Qdrant vector DB: %v", err)
	}

	// Create knowledge base - ContentsDB is automatically created by default
	// This enables knowledge content management endpoints out of the box
	knowledgeBase := knowledge.NewPDFKnowledgeBase("pdf-knowledge", vectorDB)

	// Configure PDFs
	knowledgeBase.URLs = []string{
		"https://arxiv.org/pdf/2305.13245.pdf",
	}

	// err = knowledgeBase.Load(ctx, true)
	// if err != nil {
	// 	log.Fatalf("Failed to load knowledge base: %v", err)
	// }

	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	assistant, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Assistant",
		Description:  "A helpful AI assistant",
		Instructions: "You are a helpful AI assistant.",
		Model:        ollamaModel,
		Knowledge:    knowledgeBase,
		Markdown:     true,
		Debug:        true,
	})
	if err != nil {
		log.Fatalf("Failed to create assistant agent: %v", err)
	}

	osInstance, err := agentOS.NewAgentOS(agentOS.AgentOSOptions{
		OSID:        "my-first-os",
		Description: agentOS.StringPtr("My first AgentOS"),
		Agents:      []*agent.Agent{assistant},
		Settings: &agentOS.AgentOSSettings{
			Port:       8080,      // User can change if needed
			Host:       "0.0.0.0", // Accept all connections
			Debug:      false,
			EnableCORS: true,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create AgentOS: %v", err)
	}

	if err := osInstance.Serve(); err != nil {
		log.Fatalf("Failed to start AgentOS: %v", err)
	}
}
