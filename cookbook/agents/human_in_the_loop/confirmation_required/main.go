package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// HackerNewsTool is a custom tool for fetching HN stories
type HackerNewsTool struct {
	toolkit.Toolkit
}

type GetTopStoriesParams struct {
	Count int `json:"count" jsonschema:"description=Number of stories to fetch,default=3"`
}

func (h *HackerNewsTool) GetTopStories(params GetTopStoriesParams) (string, error) {
	// Simulated response for the example
	// In a real app, you would call the HN API here
	return fmt.Sprintf(`Here are the top %d stories:
1. Agno Framework v2 Released (150 points)
2. Go 1.24 Features Preview (120 points)
3. The Future of AI Agents (95 points)`, params.Count), nil
}

func main() {
	ctx := context.Background()

	// 1. Initialize the model (Ollama)
	apiKey := os.Getenv("OLLAMA_API_KEY") // Optional for local Ollama
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"), // Use a reliable local model
		models.WithBaseURL("http://localhost:11434"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 2. Initialize the tool (HackerNews)
	// We create a custom tool wrapper that implements toolkit.Tool
	hn := &HackerNewsTool{}
	hn.Name = "HackerNews"
	hn.Description = "Get stories from HackerNews"

	tk := toolkit.NewToolkit()
	tk.Name = hn.Name
	tk.Description = hn.Description
	tk.Register("GetTopStories", "Get top stories from HackerNews", hn, hn.GetTopStories, GetTopStoriesParams{})

	hn.Toolkit = tk

	// 3. Create the Agent with Tool Hooks
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "News Assistant",
		Model:         model,
		Instructions:  "You are a helpful assistant that fetches news. Always ask before fetching news.",
		Tools:         []toolkit.Tool{hn},
		ShowToolsCall: true, // Show tool calls in output

		// ToolBeforeHooks allow us to intercept tool execution
		ToolBeforeHooks: []func(ctx context.Context, toolName string, args map[string]interface{}) error{
			func(ctx context.Context, toolName string, args map[string]interface{}) error {
				// We only want to confirm specific tools/methods
				// In this case, we confirm everything for demonstration
				fmt.Printf("\n🛑 CONFIRMATION REQUIRED\n")
				fmt.Printf("Agent wants to call tool: %s\n", toolName)
				fmt.Printf("Arguments: %v\n", args)
				fmt.Print("Do you want to proceed? (y/n): ")

				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(strings.ToLower(input))

				if input != "y" && input != "yes" {
					fmt.Println("❌ Action cancelled by user.")
					return fmt.Errorf("user cancelled execution of tool %s", toolName)
				}

				fmt.Println("✅ Action confirmed.")
				return nil
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Human-in-the-Loop Example ===")
	fmt.Println("The agent will try to fetch Hacker News stories.")
	fmt.Println("You will be asked to confirm the action.")
	fmt.Println("=================================")

	// We ask the agent to fetch stories. This should trigger the tool and our hook.
	response, err := ag.Run("Fetch the top 3 stories from Hacker News")
	if err != nil {
		// If the user cancels, the agent might return an error or handle it gracefully
		// depending on how the model reacts to the tool error.
		log.Printf("Run finished with error (expected if cancelled): %v", err)
	} else {
		fmt.Println("\n🤖 Agent Response:")
		fmt.Println(response.TextContent)
	}
}
