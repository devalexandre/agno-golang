package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
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
	timeStart := time.Now()

	fmt.Println("=== Agno Learning Loop Agent Demo (Implicit Validation) ===")
	fmt.Println("Shows what usually happens in real usage: users rarely say 'that worked'.")
	fmt.Printf("Demo started at: %s\n\n", timeStart.Format(time.RFC1123))

	// Qdrant (assumes running locally)
	host := "localhost"
	port := 6334
	fmt.Printf("🔗 Qdrant gRPC endpoint: %s (host=%s, port=%d)\n\n", host, host, port)

	// Embedder (Ollama local)
	fmt.Println("📊 Creating embedder...")
	emb := embedder.NewOllamaEmbedder(
		embedder.WithOllamaModel("nomic-embed-text", 768),
		embedder.WithOllamaHost("http://localhost:11434"),
	)

	// Vector DB
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

	// Knowledge + Learning
	kb := knowledge.NewBaseKnowledge("learning_loop_demo", vectorDB)
	learningManager := learning.NewManager(kb, learning.DefaultManagerConfig())

	// Chat model (Ollama Cloud)
	fmt.Println("🤖 Setting up Ollama chat model (cloud)...")
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama Cloud model: %v", err)
	}

	// Agent
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

	filters := map[string]interface{}{
		"language": "en",
		"domain":   "golang",
	}

	// ────────────────────────────────────────────────────────────────
	// Turn 1: initial how-to (likely saved as candidate)
	// ────────────────────────────────────────────────────────────────
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Turn 1: Ask for a reusable procedure/snippet (saved as candidate).")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	runAndPrint(ag,
		"How do I add a new HTTP route using net/http in Go? Give short bullet steps and a minimal code snippet.",
		agent.WithKnowledgeFilters(filters),
	)

	fmt.Println("\n🔎 Learning context that would be injected for the same query (after Turn 1):")
	learningCtx, _ := learningManager.RetrieveContextWithFilters(ctx, userID, "Add a new HTTP route using net/http", filters)
	if strings.TrimSpace(learningCtx) == "" {
		fmt.Println("(no learning memories yet)")
	} else {
		fmt.Println(learningCtx)
	}

	// ────────────────────────────────────────────────────────────────
	// Turn 2: realistic follow-up (implicit positive signal, no explicit confirmation)
	// ────────────────────────────────────────────────────────────────
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Turn 2: Follow-up question (implicit validation: user continues, no explicit 'worked').")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	runAndPrint(ag,
		"Ok. Now how do I read a query parameter like ?name=alex in net/http? Keep it minimal.",
		agent.WithKnowledgeFilters(filters),
	)

	// ────────────────────────────────────────────────────────────────
	// Turn 3: ask again (retrieval should reuse prior memory; evidence increases)
	// ────────────────────────────────────────────────────────────────
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Turn 3: Ask again (retrieval should reuse candidate memory; hits/streak increase).")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	runAndPrint(ag,
		"Remind me quickly: how do I add a new route in net/http?",
		agent.WithKnowledgeFilters(filters),
	)

	fmt.Println("\n🔎 Learning context that would be injected now (after repeated reuse):")
	learningCtx, _ = learningManager.RetrieveContextWithFilters(ctx, userID, "Add a new HTTP route using net/http", filters)
	if strings.TrimSpace(learningCtx) == "" {
		fmt.Println("(no learning memories yet)")
	} else {
		fmt.Println(learningCtx)
	}

	// ────────────────────────────────────────────────────────────────
	// Turn 4: another natural next-step question (more implicit evidence)
	// ────────────────────────────────────────────────────────────────
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Turn 4: Another next-step (implicit validation continues).")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	runAndPrint(ag,
		"Great. What's the simplest way to test a net/http handler in Go using httptest? Short example, please.",
		agent.WithKnowledgeFilters(filters),
	)

	// ────────────────────────────────────────────────────────────────
	// Turn 5: ask again - by now auto-promotion may have happened (depends on thresholds)
	// ────────────────────────────────────────────────────────────────
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Turn 5: Ask the original question again (auto-promotion may have occurred without explicit confirmation).")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	runAndPrint(ag,
		"One more time, super short: add a route with net/http in Go.",
		agent.WithKnowledgeFilters(filters),
	)

	fmt.Println("\n🔎 Learning context that would be injected at the end (check status ordering):")
	learningCtx, _ = learningManager.RetrieveContextWithFilters(ctx, userID, "Add a new HTTP route using net/http", filters)
	if strings.TrimSpace(learningCtx) == "" {
		fmt.Println("(no learning memories yet)")
	} else {
		fmt.Println(learningCtx)
	}

	fmt.Println("\n=== Time Finished ===")
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
