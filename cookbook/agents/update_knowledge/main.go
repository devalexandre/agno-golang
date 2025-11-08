package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/qdrant"
	qdrantcontainer "github.com/testcontainers/testcontainers-go/modules/qdrant"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== Update Knowledge Tool Demo ===\n")
	fmt.Println("This example demonstrates the UpdateKnowledge default tool.")
	fmt.Println("The agent can add and search information in the knowledge base.\n")

	// 1. Start Qdrant container
	fmt.Println("🐳 Starting Qdrant container...")
	qdrantContainer, err := qdrantcontainer.Run(ctx, "qdrant/qdrant:latest")
	if err != nil {
		log.Fatalf("Failed to start Qdrant container: %v", err)
	}

	// Ensure cleanup
	defer func() {
		fmt.Println("\n🧹 Cleaning up Qdrant container...")
		if err := qdrantContainer.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate Qdrant container: %v", err)
		}
	}()

	// Get Qdrant gRPC endpoint
	grpcEndpoint, err := qdrantContainer.GRPCEndpoint(ctx)
	if err != nil {
		log.Fatalf("Failed to get Qdrant gRPC endpoint: %v", err)
	}

	// Parse the gRPC endpoint to extract host and port
	host := "localhost"
	port := 6334
	if grpcEndpoint != "" {
		parts := strings.Split(grpcEndpoint, ":")
		if len(parts) == 2 {
			host = parts[0]
			port, _ = strconv.Atoi(parts[1])
		}
	}

	fmt.Printf("🔗 Qdrant running at gRPC: %s (host=%s, port=%d)\n", grpcEndpoint, host, port)

	// 2. Create Ollama embedder (local)
	fmt.Println("📊 Creating embedder...")
	emb := embedder.NewOllamaEmbedder(
		embedder.WithOllamaModel("gemma:2b", 2048),
		embedder.WithOllamaHost("http://localhost:11434"),
	)

	// 3. Create Qdrant vector database
	fmt.Println("🗄️  Setting up vector database...")
	vectorDB, err := qdrant.NewQdrant(qdrant.QdrantConfig{
		Host:       host,
		Port:       port,
		Collection: "knowledge_demo",
		Embedder:   emb,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	})
	if err != nil {
		log.Fatalf("Failed to create vector database: %v", err)
	}

	// Create the collection
	if err := vectorDB.Create(ctx); err != nil {
		log.Printf("Warning: Failed to create collection (may already exist): %v", err)
	}

	// 4. Create knowledge base
	fmt.Println("📚 Creating knowledge base...")
	kb := knowledge.NewBaseKnowledge("demo_knowledge", vectorDB)

	// 5. Create Ollama Cloud model
	fmt.Println("🤖 Setting up cloud LLM...")
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 6. Create agent with knowledge and update_knowledge tool
	fmt.Println("🎯 Creating agent with update_knowledge tool...")
	ag, err := agent.NewAgent(agent.AgentConfig{
		Name:        "Knowledge Assistant",
		Model:       model,
		Description: "AI assistant with knowledge base access",
		Instructions: "You are a helpful assistant with access to a knowledge base. " +
			"You can add information using the update_knowledge tool and search for information when needed. " +
			"Always use the knowledge base to store and retrieve important information.",
		Knowledge:                 kb,
		EnableUpdateKnowledgeTool: true, // Enable default tool
		Markdown:                  true,
		ShowToolsCall:             true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("\n✅ Agent created with update_knowledge tool enabled!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Conversation 1: Ask agent to store information
	fmt.Println("\n--- Conversation 1: Storing Go Best Practices ---")
	runConversation(ag, "Please add this to the knowledge base: 'Go best practice: Always handle errors explicitly. Never ignore error returns.'")

	// Conversation 2: Store more information
	fmt.Println("\n--- Conversation 2: Storing Concurrency Info ---")
	runConversation(ag, "Add to knowledge: 'Go concurrency: Use channels for communication between goroutines. Don't communicate by sharing memory; share memory by communicating.'")

	// Conversation 3: Store framework info
	fmt.Println("\n--- Conversation 3: Storing Framework Info ---")
	runConversation(ag, "Store this: 'agno-golang is an AI agent framework for Go that supports Ollama, OpenAI, and Google models. It includes tools, knowledge bases, and workflow capabilities.'")

	// Conversation 4: Search knowledge base
	fmt.Println("\n--- Conversation 4: Searching Knowledge ---")
	runConversation(ag, "Search the knowledge base for information about Go concurrency. What did we store about it?")

	// Conversation 5: Search for framework
	fmt.Println("\n--- Conversation 5: Framework Search ---")
	runConversation(ag, "Search for information about agno-golang in the knowledge base.")

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\n✨ The agent successfully used the update_knowledge tool to manage the knowledge base!")
	fmt.Println("💡 All information has been persisted and can be searched in the Qdrant vector database.")
}

func runConversation(ag *agent.Agent, userMessage string) {
	fmt.Printf("\n👤 User: %s\n", userMessage)

	// Run agent
	response, err := ag.Run(userMessage)
	if err != nil {
		log.Printf("❌ Agent error: %v", err)
		return
	}

	fmt.Printf("🤖 Assistant: %s\n", response.TextContent)
}
