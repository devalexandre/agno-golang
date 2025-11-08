package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

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

	fmt.Println("🚀 Knowledge-based RAG Agent with PDF Example")
	fmt.Println("==============================================")
	fmt.Println("")

	// 1. Start Qdrant container
	fmt.Println("🐳 Starting Qdrant container...")
	qdrantContainer, err := qdrantcontainer.Run(ctx, "qdrant/qdrant:v1.7.4")
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

	// Get Qdrant gRPC endpoint (required for Go client)
	grpcEndpoint, err := qdrantContainer.GRPCEndpoint(ctx)
	if err != nil {
		log.Fatalf("Failed to get Qdrant gRPC endpoint: %v", err)
	}

	// Parse the gRPC endpoint to extract host and port
	// GRPCEndpoint returns "localhost:port" format
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

	// 2. Initialize Ollama embedder (local model for cost efficiency)
	fmt.Println("📊 Initializing Ollama embedder...")
	ollamaEmbedder := embedder.NewOllamaEmbedder(
		embedder.WithOllamaModel("gemma:2b", 2048),
		embedder.WithOllamaHost("http://localhost:11434"),
	)

	// 3. Initialize Qdrant vector database
	fmt.Println("🗄️  Setting up Qdrant vector database...")
	vectorDB, err := qdrant.NewQdrant(qdrant.QdrantConfig{
		Host:       host,
		Port:       port,
		Collection: "mistral_knowledge",
		Embedder:   ollamaEmbedder,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	})
	if err != nil {
		log.Fatalf("Failed to create Qdrant vector DB: %v", err)
	}

	// 4. Create PDF knowledge base
	fmt.Println("📚 Creating PDF knowledge base...")
	kb := knowledge.NewPDFKnowledgeBase("mistral_docs", vectorDB)

	// 5. Load PDF document
	pdfPath := "/home/devalexandre/Downloads/Learn Mistral Elevating Mistral systems through embeddings, agents, RAG, AWS Bedrock, and Vertex AI (Pavlo Cherkashin).pdf"

	fmt.Printf("📄 Loading PDF: %s\n", pdfPath)
	fmt.Println("⏳ This may take a few minutes for large PDFs...")
	fmt.Println("💡 For datasets > 500 docs, automatic parallel processing will be used")

	startTime := time.Now()

	// Check if file exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		log.Fatalf("PDF file not found: %s", pdfPath)
	}

	// Load PDF from path (will automatically parse, chunk, embed, and store)
	if err := kb.LoadDocumentFromPath(ctx, pdfPath, nil); err != nil {
		log.Fatalf("Failed to load PDF: %v", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("✅ PDF loaded successfully in %v\n", duration)

	// Get document count
	count, err := kb.GetCount(ctx)
	if err != nil {
		log.Printf("Warning: Could not get document count: %v", err)
	} else {
		fmt.Printf("📊 Total chunks in knowledge base: %d\n", count)
	}

	// 6. Create cloud-based LLM agent (using Ollama)
	fmt.Println("\n🤖 Creating agent with cloud LLM...")
	cloudModel, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create agent with RAG capabilities
	agt, err := agent.NewAgent(agent.AgentConfig{
		Name:        "Mistral Expert",
		Model:       cloudModel,
		Description: "An AI assistant that can answer questions about Mistral AI using RAG",
		Instructions: "You are an expert on Mistral AI and its applications. " +
			"Use the knowledge base to provide accurate, detailed answers. " +
			"Always cite specific information from the documentation when available. " +
			"If you're unsure about something, say so clearly.",
		Knowledge:             kb,
		KnowledgeMaxDocuments: 5,
		Markdown:              true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 7. Test the RAG system with queries
	fmt.Println("\n🔍 Testing RAG system with sample queries...")
	fmt.Println("============================================================")

	queries := []string{
		"What are the key features of Mistral AI?",
		"How do embeddings work in Mistral?",
		"What is the difference between RAG and fine-tuning?",
	}

	for i, query := range queries {
		fmt.Printf("\n📝 Query %d: %s\n", i+1, query)
		fmt.Println("------------------------------------------------------------")

		response, err := agt.Run(query)
		if err != nil {
			log.Printf("Error running query: %v", err)
			continue
		}

		fmt.Printf("💬 Response:\n%s\n", response.TextContent)
	}

	fmt.Println("\n✨ Example completed successfully!")
}
