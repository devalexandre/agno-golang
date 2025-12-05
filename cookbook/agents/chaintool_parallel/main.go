package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
)

func main() {
	ctx := context.Background()

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		utils.ErrorPanel(err)
		return
	}

	extractTitleTool := tools.NewToolFromFunction(
		func(ctx context.Context, text string) (string, error) {
			time.Sleep(200 * time.Millisecond)
			return fmt.Sprintf("TITLE[%s]", strings.Split(text, " ")[0]), nil
		},
		"Extracts title from text",
	)

	extractKeywordsTool := tools.NewToolFromFunction(
		func(ctx context.Context, text string) (string, error) {
			time.Sleep(300 * time.Millisecond)
			words := strings.Fields(text)
			if len(words) > 3 {
				words = words[:3]
			}
			return fmt.Sprintf("KEYWORDS[%s]", strings.Join(words, ",")), nil
		},
		"Extracts keywords from text",
	)

	calculateStatsTool := tools.NewToolFromFunction(
		func(ctx context.Context, text string) (string, error) {
			time.Sleep(250 * time.Millisecond)
			charCount := len(text)
			wordCount := len(strings.Fields(text))
			return fmt.Sprintf("STATS[chars:%d,words:%d]", charCount, wordCount), nil
		},
		"Calculates text statistics",
	)

	detectLanguageTool := tools.NewToolFromFunction(
		func(ctx context.Context, text string) (string, error) {
			time.Sleep(150 * time.Millisecond)
			return "LANGUAGE[ENGLISH]", nil
		},
		"Detects text language",
	)

	aggregateResultsTool := tools.NewToolFromFunction(
		func(ctx context.Context, results string) (string, error) {
			time.Sleep(100 * time.Millisecond)
			return fmt.Sprintf("AGGREGATED{%s}", results), nil
		},
		"Aggregates all analysis results",
	)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ChainTool with Parallel Execution")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	testText := "Machine Learning and Artificial Intelligence are transforming modern technology landscape"

	fmt.Printf("\nText Analysis: %s\n", testText)
	fmt.Println(strings.Repeat("-", 80))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:         ctx,
		Model:           model,
		Name:            "TextAnalyzer",
		Description:     "Parallel text analysis pipeline",
		Tools:           []toolkit.Tool{extractTitleTool, extractKeywordsTool, calculateStatsTool, detectLanguageTool, aggregateResultsTool},
		EnableChainTool: true,
		Debug:           false,
	})
	if err != nil {
		utils.ErrorPanel(err)
		return
	}

	start := time.Now()
	response, err := ag.Run(fmt.Sprintf("Analyze: %s", testText))
	elapsed := time.Since(start)

	if err != nil {
		utils.ErrorPanel(err)
		return
	}

	fmt.Printf("✓ Response: %s\n", response.TextContent)
	fmt.Printf("✓ Execution time: %v\n", elapsed)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Parallel Execution Strategies")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf(`
Execution Patterns:

1. Sequential Strategy
   - Executes tools one after another
   - Each tool waits for previous to complete
   - Predictable behavior, easier debugging
   - Performance: ~1000ms (4x250ms avg)

2. Parallel Strategy
   - Executes all tools concurrently
   - Respects max concurrency limit
   - Maximum resource utilization
   - Performance: ~300ms (concurrent execution)

3. Pipelined Strategy
   - Organizes tools in stages
   - Each stage runs in parallel
   - Results pass to next stage
   - Performance: Optimal based on stages

Benefits of Parallelization:
✓ Reduced total execution time
✓ Better resource utilization
✓ Improved throughput
✓ Concurrent independent operations
✓ Scalable architecture
`)

	fmt.Println(strings.Repeat("=", 80) + "\n")
}
