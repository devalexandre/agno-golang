package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/learning"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/qdrant"
)

func main() {
	ctx := context.Background()

	//start time
	timeStart := time.Now()

	fmt.Println("=== Agno Learning Loop Agent Demo ===")

	fmt.Printf("Demo started at: %s\n", timeStart.Format(time.RFC1123))

	fmt.Println("=== Learning Loop Demo ===")
	fmt.Println("Continuous learning on top of the knowledge store (RAG + post-run learning).\n")

	host := "localhost"
	port := 6334

	fmt.Printf("🔗 Qdrant gRPC endpoint: %s (host=%s, port=%d)\n", host, port)

	// 2) Embedder (Ollama)
	fmt.Println("📊 Creating embedder...")
	emb := embedder.NewOllamaEmbedder(
		embedder.WithOllamaModel("nomic-embed-text", 768),
		embedder.WithOllamaHost("http://localhost:11434"),
	)

	// 3) Vector DB
	fmt.Println("🗄️  Setting up vector database...")
	vectorDB, err := qdrant.NewQdrant(qdrant.QdrantConfig{
		Host:       host,
		Port:       port,
		Collection: "learning_loop_demo",
		Embedder:   emb,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	})
	if err != nil {
		log.Fatalf("Failed to create vector database: %v", err)
	}
	if err := vectorDB.Create(ctx); err != nil {
		log.Printf("Warning: Failed to create collection (may already exist): %v", err)
	}

	// 4) Knowledge store
	kb := knowledge.NewBaseKnowledge("learning_loop_demo", vectorDB)

	// 5) Learning manager (continuous learning)
	learningManager := learning.NewManager(kb, learning.DefaultManagerConfig())

	// 6) Chat model (Ollama)
	fmt.Println("🤖 Setting up Ollama chat model...")
	// Get API key
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// Create Ollama Cloud model
	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama Cloud model: %v", err)
	}

	// 7) Agent with Knowledge + LearningManager
	userID := "user123"
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Learning Assistant",
		Model:         ollamaModel,
		UserID:        userID,
		Knowledge:     kb,
		Learning:      learningManager,
		Instructions:  "You are a helpful assistant. When the user asks for how-to guidance, provide concise bullet steps and short code snippets when useful.",
		Markdown:      true,
		ShowToolsCall: false,
		Debug:         false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Turn 1: Ask for a reusable procedure/snippet (this may be saved as a candidate memory).")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	filters := map[string]interface{}{
		"language": "en",
		"domain":   "golang",
	}
	runAndPrint(ag, "How do I add a new HTTP route using net/http in Go? Give short bullet steps and a minimal code snippet.", agent.WithKnowledgeFilters(filters))

	fmt.Println("\n🔎 Learning context that would be injected for the same query:")
	learningCtx, _ := learningManager.RetrieveContextWithFilters(ctx, userID, "Add a new HTTP route using net/http", filters)
	fmt.Println(learningCtx)

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Turn 2: User confirmation (this can promote candidate memories to verified).")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	runAndPrint(ag, "That worked, thanks!", agent.WithKnowledgeFilters(filters))

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Turn 3: Ask again (retrieval should surface verified memories first).")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	runAndPrint(ag, "Remind me quickly: how do I add a new route in net/http?", agent.WithKnowledgeFilters(filters))

	fmt.Println("\n=== Time FInished ===")
	timeEnd := time.Now()
	fmt.Printf("Demo finished at: %s\n", timeEnd.Format(time.RFC1123))
	fmt.Printf("Total demo duration: %s\n", timeEnd.Sub(timeStart).String())
	fmt.Println("\n=== Demo Complete ===")
}

func runAndPrint(ag *agent.Agent, userMessage string, opts ...interface{}) {
	fmt.Printf("\n👤 User: %s\n", userMessage)
	resp, err := ag.Run(userMessage, opts...)
	if err != nil {
		log.Printf("❌ Agent error: %v", err)
		return
	}
	fmt.Printf("🤖 Assistant: %s\n", resp.TextContent)
}
