package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/document"
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

	fmt.Println("=== Knowledge Filters Example ===\n")
	fmt.Println("This example demonstrates using WithKnowledgeFilters to query specific metadata.")
	fmt.Println("Filters allow precise targeting of knowledge base entries by category, language, etc.\n")

	// 1. Start Qdrant container
	fmt.Println("🐳 Starting Qdrant container...")
	qdrantContainer, err := qdrantcontainer.Run(ctx, "qdrant/qdrant:latest")
	if err != nil {
		log.Fatalf("Failed to start Qdrant container: %v", err)
	}

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

	host := "localhost"
	port := 6334
	if grpcEndpoint != "" {
		parts := strings.Split(grpcEndpoint, ":")
		if len(parts) == 2 {
			host = parts[0]
			port, _ = strconv.Atoi(parts[1])
		}
	}

	fmt.Printf("🔗 Qdrant running at: %s\n", grpcEndpoint)

	// 2. Create embedder
	fmt.Println("📊 Creating embedder...")
	emb := embedder.NewOllamaEmbedder(
		embedder.WithOllamaModel("gemma:2b", 2048),
		embedder.WithOllamaHost("http://localhost:11434"),
	)

	// 3. Create vector database
	fmt.Println("🗄️  Setting up vector database...")
	vectorDB, err := qdrant.NewQdrant(qdrant.QdrantConfig{
		Host:       host,
		Port:       port,
		Collection: "filtered_knowledge",
		Embedder:   emb,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	})
	if err != nil {
		log.Fatalf("Failed to create vector database: %v", err)
	}

	if err := vectorDB.Create(ctx); err != nil {
		log.Printf("Warning: Failed to create collection: %v", err)
	}

	// 4. Create knowledge base and populate with categorized data
	fmt.Println("📚 Creating and populating knowledge base...")
	kb := knowledge.NewBaseKnowledge("filtered_kb", vectorDB)

	// Add documents with different categories and languages
	documents := []document.Document{
		// Go Programming - English
		{
			ID:      "go_001",
			Content: "Go channels are typed conduits for synchronizing goroutines. They can be buffered or unbuffered.",
			Metadata: map[string]interface{}{
				"category":  "programming",
				"language":  "go",
				"topic":     "concurrency",
				"level":     "intermediate",
				"lang_code": "en",
			},
		},
		{
			ID:      "go_002",
			Content: "Go error handling: Always check errors explicitly. Use multiple return values and the error interface.",
			Metadata: map[string]interface{}{
				"category":  "programming",
				"language":  "go",
				"topic":     "error_handling",
				"level":     "beginner",
				"lang_code": "en",
			},
		},
		// Python Programming - English
		{
			ID:      "py_001",
			Content: "Python decorators are functions that modify the behavior of other functions. Use @decorator syntax.",
			Metadata: map[string]interface{}{
				"category":  "programming",
				"language":  "python",
				"topic":     "decorators",
				"level":     "intermediate",
				"lang_code": "en",
			},
		},
		// DevOps - English
		{
			ID:      "devops_001",
			Content: "Docker containers package applications with dependencies. Use docker-compose for multi-container setups.",
			Metadata: map[string]interface{}{
				"category":  "devops",
				"language":  "docker",
				"topic":     "containers",
				"level":     "beginner",
				"lang_code": "en",
			},
		},
		// Go Programming - Portuguese
		{
			ID:      "go_003_pt",
			Content: "Goroutines são threads leves gerenciadas pelo runtime do Go. Use a palavra-chave 'go' para iniciar.",
			Metadata: map[string]interface{}{
				"category":  "programming",
				"language":  "go",
				"topic":     "concurrency",
				"level":     "beginner",
				"lang_code": "pt",
			},
		},
		// Database - English
		{
			ID:      "db_001",
			Content: "PostgreSQL supports JSONB for efficient JSON storage with indexing. Use it for semi-structured data.",
			Metadata: map[string]interface{}{
				"category":  "database",
				"language":  "sql",
				"topic":     "postgresql",
				"level":     "advanced",
				"lang_code": "en",
			},
		},
	}

	if err := kb.LoadDocuments(ctx, documents, false); err != nil {
		log.Fatalf("Failed to load documents: %v", err)
	}

	fmt.Printf("✅ Loaded %d documents with metadata\n", len(documents))

	// 5. Create cloud model
	fmt.Println("🤖 Setting up cloud LLM...")
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 6. Create agent
	fmt.Println("🎯 Creating agent...")
	ag, err := agent.NewAgent(agent.AgentConfig{
		Name:          "Filtered Knowledge Assistant",
		Model:         model,
		Description:   "AI assistant with filtered knowledge access",
		Instructions:  "You are a helpful assistant. Use the knowledge base to answer questions based on the provided filters.",
		Knowledge:     kb,
		Markdown:      true,
		ShowToolsCall: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("\n✅ Agent created with filtered knowledge!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Query 1: Filter by category = "programming" AND language = "go"
	fmt.Println("--- Query 1: Go Programming Only ---")
	filters1 := map[string]interface{}{
		"category": "programming",
		"language": "go",
	}

	response1, err := ag.Run(
		"What do you know about Go?",
		agent.WithKnowledgeFilters(filters1),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: What do you know about Go?\n")
		fmt.Printf("🔍 Filters: category=programming, language=go\n")
		fmt.Printf("🤖 Assistant: %s\n", response1.TextContent)
	}

	// Query 2: Filter by category = "devops"
	fmt.Println("\n--- Query 2: DevOps Only ---")
	filters2 := map[string]interface{}{
		"category": "devops",
	}

	response2, err := ag.Run(
		"Tell me about containerization",
		agent.WithKnowledgeFilters(filters2),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: Tell me about containerization\n")
		fmt.Printf("🔍 Filters: category=devops\n")
		fmt.Printf("🤖 Assistant: %s\n", response2.TextContent)
	}

	// Query 3: Filter by level = "beginner"
	fmt.Println("\n--- Query 3: Beginner Level Only ---")
	filters3 := map[string]interface{}{
		"level": "beginner",
	}

	response3, err := ag.Run(
		"Give me beginner-friendly information",
		agent.WithKnowledgeFilters(filters3),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: Give me beginner-friendly information\n")
		fmt.Printf("🔍 Filters: level=beginner\n")
		fmt.Printf("🤖 Assistant: %s\n", response3.TextContent)
	}

	// Query 4: Filter by lang_code = "pt" (Portuguese)
	fmt.Println("\n--- Query 4: Portuguese Content Only ---")
	filters4 := map[string]interface{}{
		"lang_code": "pt",
	}

	response4, err := ag.Run(
		"O que você sabe sobre Go?",
		agent.WithKnowledgeFilters(filters4),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: O que você sabe sobre Go?\n")
		fmt.Printf("🔍 Filters: lang_code=pt\n")
		fmt.Printf("🤖 Assistant: %s\n", response4.TextContent)
	}

	// Query 5: Multiple filters (category + level)
	fmt.Println("\n--- Query 5: Programming + Advanced ---")
	filters5 := map[string]interface{}{
		"category": "programming",
		"level":    "advanced",
	}

	response5, err := ag.Run(
		"Show me advanced programming concepts",
		agent.WithKnowledgeFilters(filters5),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: Show me advanced programming concepts\n")
		fmt.Printf("🔍 Filters: category=programming, level=advanced\n")
		fmt.Printf("🤖 Assistant: %s\n", response5.TextContent)
	}

	// Query 6: No filters (search all)
	fmt.Println("\n--- Query 6: No Filters (All Documents) ---")

	response6, err := ag.Run("What topics do you have in your knowledge base?")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: What topics do you have in your knowledge base?\n")
		fmt.Printf("🔍 Filters: none (search all)\n")
		fmt.Printf("🤖 Assistant: %s\n", response6.TextContent)
	}

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\n✨ Key Features Demonstrated:")
	fmt.Println("   • WithKnowledgeFilters - Target specific metadata fields")
	fmt.Println("   • Single field filters (category, language, level)")
	fmt.Println("   • Multiple field filters (category + level)")
	fmt.Println("   • Language-specific content filtering (en/pt)")
	fmt.Println("   • No filters (search entire knowledge base)")
	fmt.Println("\n💡 Use Cases:")
	fmt.Println("   • Multi-language documentation")
	fmt.Println("   • Skill-level targeting (beginner/intermediate/advanced)")
	fmt.Println("   • Category-based knowledge separation")
	fmt.Println("   • Topic-specific searches")
}
