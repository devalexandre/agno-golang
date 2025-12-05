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

	fmt.Println("\n🔀 ChainTool with Conditional Execution - Advanced Demo")
	fmt.Println(strings.Repeat("=", 70))

	checkLengthTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			result := fmt.Sprintf("length:%d", len(input))
			color.Yellow("   [1] CHECK_LENGTH: %q → %s\n", input, result)
			return result, nil
		},
		"Check string length and return length:N format",
	)

	uppercaseTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			result := strings.ToUpper(input)
			color.Yellow("   [2] UPPERCASE: %q → %q\n", input, result)
			return result, nil
		},
		"Convert to uppercase",
	)

	addPrefixTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			result := "PREFIX_" + input
			color.Yellow("   [3] ADD_PREFIX: %q → %q\n", input, result)
			return result, nil
		},
		"Add PREFIX_ to string",
	)

	reverseTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			runes := []rune(input)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			result := string(runes)
			color.Yellow("   [4] REVERSE: %q → %q\n", input, result)
			return result, nil
		},
		"Reverse string",
	)

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		fmt.Printf("error: %v\n", err.Error())
		return
	}

	agnt, err := agent.NewAgent(agent.AgentConfig{
		Context:         ctx,
		Model:           model,
		Name:            "ConditionalProcessor",
		Description:     "Process strings with conditional tool execution based on input length",
		Instructions:    "Use CHECK_LENGTH first to determine string length, then conditionally execute UPPERCASE or ADD_PREFIX based on result",
		Tools:           []toolkit.Tool{checkLengthTool, uppercaseTool, addPrefixTool, reverseTool},
		EnableChainTool: true,
	})
	if err != nil {
		fmt.Printf("   ❌ Error creating agent: %v\n", err)
		return
	}

	testCases := []string{
		"hello",
		"go",
		"conditional",
	}

	for _, testCase := range testCases {
		fmt.Printf("\n📝 Test case: %q\n", testCase)
		fmt.Println(strings.Repeat("-", 70))

		prompt := fmt.Sprintf("Check if '%s' is long (>5 chars). If yes, apply UPPERCASE and then REVERSE. If no, apply ADD_PREFIX.", testCase)
		response, err := agnt.Run(prompt)
		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
			continue
		}

		fmt.Printf("   🤖 Final response: %q\n", response.TextContent)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("✅ ChainTool with Conditional Execution Demo Complete!")
}
