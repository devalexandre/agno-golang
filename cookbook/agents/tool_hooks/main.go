package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// CalculatorToolkit is a simple calculator tool
type CalculatorToolkit struct {
	toolkit.Toolkit
}

type AddParams struct {
	A float64 `json:"a" jsonschema:"required,description=First number"`
	B float64 `json:"b" jsonschema:"required,description=Second number"`
}

func (c *CalculatorToolkit) Add(params AddParams) (float64, error) {
	return params.A + params.B, nil
}

type MultiplyParams struct {
	A float64 `json:"a" jsonschema:"required,description=First number"`
	B float64 `json:"b" jsonschema:"required,description=Second number"`
}

func (c *CalculatorToolkit) Multiply(params MultiplyParams) (float64, error) {
	return params.A * params.B, nil
}

func main() {
	ctx := context.Background()

	// Get API key from environment
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// Create Ollama Cloud model
	model, err := ollama.NewOllamaChat(
		models.WithID("deepseek-v3.1:671b-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create calculator toolkit
	calc := &CalculatorToolkit{}
	calc.Name = "calculator"
	calc.Description = "A simple calculator that can add and multiply numbers"
	tk := toolkit.NewToolkit()
	tk.Name = calc.Name
	tk.Description = calc.Description
	tk.Register("add", "Add two numbers", calc, calc.Add, AddParams{})
	tk.Register("multiply", "Multiply two numbers", calc, calc.Multiply, MultiplyParams{})

	// Tool call counter for demonstration
	toolCallCount := 0

	// Create agent with tool hooks
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:     ctx,
		Model:       model,
		Name:        "Calculator Assistant",
		Role:        "Math Helper",
		Description: "I help you with mathematical calculations",
		Instructions: `You are a helpful calculator assistant. 
When asked to perform calculations, use the calculator tool.
Always explain what you're calculating.`,
		Tools:         []toolkit.Tool{&tk},
		ShowToolsCall: true,
		Debug:         false,

		// Configure tool hooks
		ToolBeforeHooks: []func(ctx context.Context, toolName string, args map[string]interface{}) error{
			// Hook 1: Log tool calls
			func(ctx context.Context, toolName string, args map[string]interface{}) error {
				toolCallCount++
				fmt.Printf("\n🔧 [BEFORE HOOK 1] Tool '%s' is about to be called (call #%d)\n", toolName, toolCallCount)
				fmt.Printf("   Arguments: %+v\n", args)
				return nil
			},
			// Hook 2: Validate input ranges (security/guardrails)
			func(ctx context.Context, toolName string, args map[string]interface{}) error {
				fmt.Printf("\n🛡️  [BEFORE HOOK 2] Validating arguments for '%s'\n", toolName)

				// Check if numbers are within acceptable range
				for key, val := range args {
					if num, ok := val.(float64); ok {
						if num > 1000000 || num < -1000000 {
							return fmt.Errorf("value %s=%f is out of acceptable range (-1000000 to 1000000)", key, num)
						}
					}
				}
				fmt.Printf("   ✓ All arguments validated\n")
				return nil
			},
			// Hook 3: Rate limiting simulation
			func(ctx context.Context, toolName string, args map[string]interface{}) error {
				fmt.Printf("\n⏱️  [BEFORE HOOK 3] Checking rate limits for '%s'\n", toolName)
				// Simulate rate limiting check
				if toolCallCount > 10 {
					return fmt.Errorf("rate limit exceeded: maximum 10 tool calls per session")
				}
				fmt.Printf("   ✓ Rate limit OK (%d/10 calls used)\n", toolCallCount)
				return nil
			},
		},

		ToolAfterHooks: []func(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error{
			// Hook 1: Log results
			func(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
				fmt.Printf("\n✅ [AFTER HOOK 1] Tool '%s' completed successfully\n", toolName)
				fmt.Printf("   Result: %v\n", result)
				return nil
			},
			// Hook 2: Audit trail (could save to database)
			func(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
				fmt.Printf("\n📝 [AFTER HOOK 2] Audit: Recording tool execution\n")
				fmt.Printf("   Timestamp: %s\n", time.Now().Format(time.RFC3339))
				fmt.Printf("   Tool: %s\n", toolName)
				fmt.Printf("   Args: %+v\n", args)
				fmt.Printf("   Result: %v\n", result)
				// In production, you would save this to a database
				return nil
			},
			// Hook 3: Result validation
			func(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
				fmt.Printf("\n🔍 [AFTER HOOK 3] Validating result from '%s'\n", toolName)

				// Check if result is reasonable
				if num, ok := result.(float64); ok {
					if num > 10000000 || num < -10000000 {
						fmt.Printf("   ⚠️  Warning: Result %f seems unusually large\n", num)
					} else {
						fmt.Printf("   ✓ Result validated\n")
					}
				}
				return nil
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║          Tool Hooks Example - Calculator Assistant            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("This example demonstrates ToolBeforeHooks and ToolAfterHooks:")
	fmt.Println("- Before hooks: Logging, validation, rate limiting")
	fmt.Println("- After hooks: Result logging, audit trail, result validation")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	// Test 1: Simple calculation
	fmt.Println("\n📊 Test 1: Calculate (5 + 3) * 2")
	fmt.Println("─────────────────────────────────────────────────────────────")

	response, err := ag.Run("Calculate (5 + 3) * 2. First add 5 and 3, then multiply the result by 2.")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("\n🤖 Agent Response:")
	fmt.Println(response.TextContent)

	// Test 2: Invalid input (should trigger validation hook)
	fmt.Println("\n\n📊 Test 2: Try to multiply 2000000 * 3 (should fail validation)")
	fmt.Println("─────────────────────────────────────────────────────────────")

	response, err = ag.Run("Multiply 2000000 by 3")
	if err != nil {
		fmt.Printf("\n❌ Expected error caught: %v\n", err)
	} else {
		fmt.Println("\n🤖 Agent Response:")
		fmt.Println(response.TextContent)
	}

	fmt.Println("\n\n═══════════════════════════════════════════════════════════════")
	fmt.Printf("Total tool calls: %d\n", toolCallCount)
	fmt.Println("═══════════════════════════════════════════════════════════════")
}
