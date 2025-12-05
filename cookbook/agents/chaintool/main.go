package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/fatih/color"
)

func main() {
	ctx := context.Background()

	fmt.Println("\n🔗 Agent ChainTool Mode - Complete Demo")
	fmt.Println(strings.Repeat("=", 60))

	uppercaseTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			result := strings.ToUpper(input)
			color.Yellow("   [1] UPPERCASE: %q → %q\n", input, result)
			return result, nil
		},
		"Convert to uppercase",
	)

	addUnderscoreTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			result := "_" + input + "_"
			color.Yellow("   [2] ADD_UNDERSCORE: %q → %q\n", input, result)
			return result, nil
		},
		"Add underscores at start and end",
	)

	invertTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			runes := []rune(input)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			result := string(runes)
			color.Yellow("   [3] INVERT: %q → %q\n", input, result)
			return result, nil
		},
		"Invert string (reverse)",
	)

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		fmt.Printf("error: %v", err.Error())
		return
	}

	agnt, err := agent.NewAgent(agent.AgentConfig{
		Context:         ctx,
		Model:           model,
		Name:            "StringTransformer",
		Description:     "Transform strings through a pipeline",
		Instructions:    "Transform strings using the available tools in sequence.",
		Tools:           []toolkit.Tool{uppercaseTool, addUnderscoreTool, invertTool},
		EnableChainTool: true,
	})
	if err != nil {
		fmt.Printf("   ❌ Error creating agent: %v\n", err)
		return
	}

	response, err := agnt.Run("Use the uppercase tool on 'agno'")
	if err != nil {
		fmt.Printf("   ❌ Execution error: %v\n", err)
		return
	}

	fmt.Printf("   🤖 [AGENT] Final response: %q\n", response.TextContent)
}
