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

	callCounts := make(map[string]int)

	normalizeTool := tools.NewToolFromFunction(
		func(ctx context.Context, text string) (string, error) {
			callCounts["NORMALIZE"]++
			time.Sleep(100 * time.Millisecond)
			return strings.ToLower(strings.TrimSpace(text)), nil
		},
		"Normalizes input text",
	)

	tokenizeTool := tools.NewToolFromFunction(
		func(ctx context.Context, text string) (string, error) {
			callCounts["TOKENIZE"]++
			time.Sleep(150 * time.Millisecond)
			words := strings.Fields(text)
			return fmt.Sprintf("[%s]", strings.Join(words, "|")), nil
		},
		"Tokenizes text into words",
	)

	countTokensTool := tools.NewToolFromFunction(
		func(ctx context.Context, tokens string) (string, error) {
			callCounts["COUNT_TOKENS"]++
			time.Sleep(50 * time.Millisecond)
			count := strings.Count(tokens, "|") + 1
			return fmt.Sprintf("TOKEN_COUNT:%d", count), nil
		},
		"Counts the number of tokens",
	)

	analyzeTool := tools.NewToolFromFunction(
		func(ctx context.Context, result string) (string, error) {
			callCounts["ANALYZE_RESULT"]++
			time.Sleep(75 * time.Millisecond)
			return fmt.Sprintf("ANALYZED(%s)", result), nil
		},
		"Analyzes final result",
	)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ChainTool with Result Caching")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	cache := agent.NewMemoryCache(time.Minute*5, 100)

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "First execution - Will be cached",
			input: "Hello World",
		},
		{
			name:  "Second execution - Same input, should hit cache",
			input: "Hello World",
		},
		{
			name:  "Different input - Will create new cache entries",
			input: "Go Programming Language",
		},
		{
			name:  "Repeat first input - Should hit all caches",
			input: "Hello World",
		},
	}

	for idx, tc := range testCases {
		fmt.Printf("\n%s\n", tc.name)
		fmt.Println(strings.Repeat("-", 80))

		ag, err := agent.NewAgent(agent.AgentConfig{
			Context:         ctx,
			Model:           model,
			Name:            "TextProcessor",
			Description:     "Text processing pipeline with caching",
			Tools:           []toolkit.Tool{normalizeTool, tokenizeTool, countTokensTool, analyzeTool},
			EnableChainTool: true,
			ChainToolCache:  cache,
			Debug:           false,
		})
		if err != nil {
			utils.ErrorPanel(err)
			continue
		}

		input := fmt.Sprintf("Process: %s", tc.input)
		start := time.Now()
		response, err := ag.Run(input)
		elapsed := time.Since(start)

		if err != nil {
			utils.ErrorPanel(err)
			continue
		}

		fmt.Printf("✓ Response: %s\n", response.TextContent)
		fmt.Printf("✓ Execution time: %v\n", elapsed)

		if idx > 0 {
			fmt.Printf("\n✓ Tool Call Counts (cache effectiveness):\n")
			for tool, count := range callCounts {
				fmt.Printf("  %s: %d calls\n", tool, count)
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Cache Statistics")
	fmt.Println(strings.Repeat("=", 80))

	stats := cache.Stats()
	fmt.Printf(`
Cache Performance Metrics:
- Total Cache Hits:    %d
- Total Cache Misses:  %d
- Cache Hit Rate:      %.2f%%
- Items in Cache:      %d

Caching Benefits:
- Reduced latency: Cached results returned instantly
- Reduced resource usage: Fewer tool executions
- Better performance: Repeated queries served from memory
- Configurable TTL: Automatic expiration of stale entries

Use Cases:
- High-frequency queries with repeated inputs
- API rate limit protection
- Cost optimization for expensive operations
- Real-time processing with repeated patterns
`, stats.TotalHits, stats.TotalMisses, stats.HitRate*100, stats.ItemCount)

	fmt.Println(strings.Repeat("=", 80) + "\n")
}
