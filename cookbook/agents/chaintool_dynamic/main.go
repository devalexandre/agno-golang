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

	// Initial tools
	validateTool := tools.NewToolFromFunction(
		func(ctx context.Context, data string) (string, error) {
			if strings.TrimSpace(data) == "" {
				return "", fmt.Errorf("validation failed: empty data")
			}
			if len(data) < 3 {
				return "", fmt.Errorf("validation failed: data too short")
			}
			return fmt.Sprintf("VALIDATED_%s", strings.ToUpper(data)), nil
		},
		"Validates input data format and length",
	)

	transformTool := tools.NewToolFromFunction(
		func(ctx context.Context, data string) (string, error) {
			return fmt.Sprintf("TRANSFORMED[%s]", data), nil
		},
		"Transforms validated data to required format",
	)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ChainTool with Dynamic Tools (Add/Remove at Runtime)")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	// Create agent with initial 2 tools
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:         ctx,
		Model:           model,
		Name:            "DynamicProcessor",
		Description:     "Agent with dynamic tool management",
		Tools:           []toolkit.Tool{validateTool, transformTool},
		EnableChainTool: true,
		ChainToolErrorConfig: &agent.ChainToolErrorConfig{
			Strategy:   agent.RollbackToPrevious,
			MaxRetries: 1,
		},
		Debug: false,
	})
	if err != nil {
		utils.ErrorPanel(err)
		return
	}

	// === PHASE 1: Run with initial 2 tools ===
	fmt.Println("📋 PHASE 1: Initial Pipeline (2 tools)")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println("\nAvailable Tools:")
	for i, tool := range ag.GetTools() {
		fmt.Printf("  %d. %s\n", i+1, tool.GetName())
	}

	input1 := "mydata"
	fmt.Printf("\n🚀 Running: %s\n", input1)
	response1, err := ag.Run(input1)
	if err != nil {
		utils.ErrorPanel(err)
	} else {
		fmt.Printf("✓ Response: %s\n", response1.TextContent)
	}

	// === PHASE 2: Add enrichment tool ===
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("📋 PHASE 2: Adding New Tool (Enrichment)")
	fmt.Println(strings.Repeat("-", 80))

	enrichTool := tools.NewToolFromFunction(
		func(ctx context.Context, data string) (string, error) {
			if !strings.Contains(data, "TRANSFORMED") {
				return "", fmt.Errorf("enrichment failed: invalid input")
			}
			return fmt.Sprintf("ENRICHED{%s,timestamp=%d}", data, time.Now().Unix()), nil
		},
		"Enriches transformed data",
	)

	err = ag.AddTool(enrichTool)
	if err != nil {
		utils.ErrorPanel(err)
		return
	}

	fmt.Println("")
	fmt.Println("✓ Successfully added: Enriches transformed data")
	fmt.Println("")
	fmt.Println("Available Tools:")
	for i, tool := range ag.GetTools() {
		fmt.Printf("  %d. %s\n", i+1, tool.GetName())
	}

	// === PHASE 3: Run with 3 tools ===
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("📋 PHASE 3: Running with Complete Pipeline (3 tools)")
	fmt.Println(strings.Repeat("-", 80))

	input2 := "testdata"
	fmt.Printf("\n🚀 Running: %s\n", input2)
	response2, err := ag.Run(input2)
	if err != nil {
		utils.ErrorPanel(err)
	} else {
		fmt.Printf("✓ Response: %s\n", response2.TextContent)
	}

	// === PHASE 4: Add storage tool ===
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("📋 PHASE 4: Adding Another Tool (Storage)")
	fmt.Println(strings.Repeat("-", 80))

	storeTool := tools.NewToolFromFunction(
		func(ctx context.Context, data string) (string, error) {
			if !strings.Contains(data, "ENRICHED") {
				return "", fmt.Errorf("storage failed: data not enriched")
			}
			return fmt.Sprintf("STORED{%s}", data), nil
		},
		"Stores enriched data",
	)

	err = ag.AddTool(storeTool)
	if err != nil {
		utils.ErrorPanel(err)
		return
	}

	fmt.Println("")
	fmt.Println("✓ Successfully added: Stores enriched data")
	fmt.Println("")
	fmt.Println("Available Tools:")
	for i, tool := range ag.GetTools() {
		fmt.Printf("  %d. %s\n", i+1, tool.GetName())
	}

	// === PHASE 5: Run with 4 tools ===
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("📋 PHASE 5: Running with Full Pipeline (4 tools)")
	fmt.Println(strings.Repeat("-", 80))

	input3 := "finaldata123"
	fmt.Printf("\n🚀 Running: %s\n", input3)
	response3, err := ag.Run(input3)
	if err != nil {
		utils.ErrorPanel(err)
	} else {
		fmt.Printf("✓ Response: %s\n", response3.TextContent)
	}

	// === PHASE 6: Remove enrichment tool ===
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("📋 PHASE 6: Removing a Tool (Enrichment)")
	fmt.Println(strings.Repeat("-", 80))

	err = ag.RemoveTool("enrichesTransformedData")
	if err != nil {
		utils.ErrorPanel(err)
	} else {
		fmt.Println("\n✓ Successfully removed: Enriches transformed data")
		fmt.Println()
	}

	fmt.Println("Available Tools:")
	for i, tool := range ag.GetTools() {
		fmt.Printf("  %d. %s\n", i+1, tool.GetName())
	}

	// === PHASE 7: Run with 3 tools again ===
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("📋 PHASE 7: Running with Modified Pipeline (3 tools)")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println("⚠️  Note: storesEnrichedData expects 'ENRICHED' prefix but enrichesTransformedData was removed")
	fmt.Println("    This demonstrates error handling with RollbackToPrevious strategy")
	fmt.Println()

	input4 := "modifieddata"
	fmt.Printf("🚀 Running: %s\n", input4)
	response4, err := ag.Run(input4)
	if err != nil {
		utils.ErrorPanel(err)
	} else {
		fmt.Printf("✓ Response: %s\n", response4.TextContent)
	}

	// === PHASE 8: Get specific tool ===
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("📋 PHASE 8: Retrieving Specific Tool")
	fmt.Println(strings.Repeat("-", 80))

	tool := ag.GetToolByName("transformsValidatedDataToRequiredFormat")
	if tool != nil {
		fmt.Printf("\n✓ Found tool: %s\n", tool.GetName())
	} else {
		fmt.Println("\n✗ Tool not found")
	}

	// === Summary ===
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Dynamic Tool Management API Summary")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf(`
Available Methods:

✓ AddTool(tool) 
  - Add new tool dynamically
  - Returns error if tool already exists or is nil

✓ RemoveTool(name)
  - Remove tool by name
  - Returns error if tool not found

✓ GetTools()
  - Get all available tools
  - Returns []toolkit.Tool slice

✓ GetToolByName(name)
  - Get specific tool by name
  - Returns nil if not found

Features:
✓ Compatible with ChainTool
✓ Compatible with Error Handling
✓ Dynamic modification at runtime
✓ Progressive tool enablement

Use Cases:
- Feature flags (enable/disable tools)
- A/B testing (swap implementations)
- Progressive enhancement (add tools dynamically)
- Tool swapping (replace tools)
- Conditional tool addition (based on data type)
`)
	fmt.Println(strings.Repeat("=", 80) + "\n")
}
