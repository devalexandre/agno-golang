package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/memory"
	"github.com/devalexandre/agno-golang/agno/memory/sqlite"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
	"github.com/pterm/pterm"
)

func main() {
	// Big header with Agno colors (reddish orange and white)
	pterm.DefaultBigText.WithLetters(
		pterm.NewLettersFromStringWithStyle("🤖", pterm.NewStyle(pterm.FgRed)),
		pterm.NewLettersFromStringWithStyle(" AGNO ", pterm.NewStyle(pterm.FgRed)),
		pterm.NewLettersFromStringWithStyle("CODER", pterm.NewStyle(pterm.FgWhite)),
	).Render()

	pterm.Println(pterm.FgGray.Sprint("CLI for code analysis, planning and execution"))
	pterm.Println()

	// Get prompt from command line arguments
	args := os.Args[1:]
	if len(args) == 0 {
		pterm.FgRed.Println("✗ No prompt provided")
		pterm.Println()
		pterm.FgBlue.Println("Usage:")
		pterm.Println("  agno-coder \"<your prompt>\"")
		pterm.Println()
		pterm.FgGray.Println("Examples:")
		pterm.Println("  agno-coder \"Review this code for security issues cookbook/agno-coder/main.go\"")
		pterm.Println("  agno-coder \"Add error handling to all functions in ./cmd\"")
		pterm.Println("  agno-coder \"Refactor the authentication module in auth/\"")
		pterm.Println("  agno-coder \"Find all TODO comments in the project and list them\"")
		pterm.Println("  agno-coder \"Create a new REST endpoint for user management\"")
		pterm.Println()
		os.Exit(1)
	}

	// Join all arguments as the prompt
	prompt := strings.Join(args, " ")

	ctx := context.Background()

	// Validate environment
	pterm.FgGray.Print("Checking environment... ")
	if err := ValidateEnvironment(); err != nil {
		pterm.Println()
		os.Exit(1)
	}
	pterm.Println()

	// Silent initialization
	pterm.FgGray.Print("Initializing models... ")

	// Get current working directory for context
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	// Get API key from environment variable
	// apiKey := os.Getenv("OPENROUTER_API_KEY")
	// if apiKey == "" {
	// 	pterm.FgRed.Println("✗ OPENROUTER_API_KEY environment variable not set")
	// 	pterm.Println()
	// 	pterm.FgGray.Println("Please set your OpenRouter API key:")
	// 	pterm.Println("  export OPENROUTER_API_KEY=your-api-key")
	// 	os.Exit(1)
	// }

	// model, err := openrouter.NewOpenRouterChat(
	// 	models.WithID("moonshotai/kimi-k2:free"),
	// )

	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// Create Ollama Cloud model
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create OpenRouter chat: %v", err)
	}

	// model, err := ollama.NewOllamaChat(
	// 	models.WithID("cogito:3b"),
	// )
	if err != nil {
		log.Fatalf("Failed to create Ollama chat: %v", err)
	}
	db, err := sqlite.NewSqliteMemoryDb("user_memories", "agno_coder.db")
	if err != nil {
		pterm.Println()
		pterm.FgRed.Printf("✗ Failed to configure memory: %v\n", err)
		os.Exit(1)
	}
	mem := memory.NewMemory(model, db)

	toolsList := []toolkit.Tool{
		tools.NewFileTool(true), // Enable writing
		tools.NewShellTool(),
	}

	pterm.FgGreen.Println("✓ Ready")
	pterm.Println()

	// CodeAnalyzer Agent - Smart file discovery and analysis
	analyzer, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "CodeAnalyzer",
		Stream:  true,
		Instructions: fmt.Sprintf(`ROLE: Code analysis expert
DIR: %s

1. Find files related to: user request
2. Read and analyze them
3. Report findings

## Output Format
### Files Analyzed: [list]
### Issues: [key findings]
### Recommendations: [fixes]`, cwd),
		Tools:                   toolsList,
		Memory:                  mem,
		MaxToolCallsFromHistory: 5,
		NumHistoryRuns:          4,
	})
	if err != nil {
		pterm.FgRed.Printf("✗ Failed to create analysis agent: %v\n", err)
		os.Exit(1)
	}

	// CodePlanner Agent - Creates implementation plans
	planner, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "CodePlanner",
		Stream:  true,
		Instructions: fmt.Sprintf(`ROLE: Developer planner
DIR: %s

Create step-by-step implementation plan.

## Output Format
### Files to Modify: [list]
### Steps: [numbered actions]
### Verification: [test commands]`, cwd),
		Tools:                   toolsList,
		Memory:                  mem,
		MaxToolCallsFromHistory: 5,
		NumHistoryRuns:          4,
	})
	if err != nil {
		pterm.FgRed.Printf("✗ Failed to create planning agent: %v\n", err)
		os.Exit(1)
	}

	executor, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "CodeExecutor",
		Stream:  true,
		Instructions: fmt.Sprintf(`ROLE: Code executor
DIR: %s

Modify files and run commands to implement the plan.

## Output Format
### Modified Files: [list]
### Status: ✅ Success | ❌ Failure`, cwd),
		Tools:                   toolsList,
		Memory:                  mem,
		MaxToolCallsFromHistory: 5,
		NumHistoryRuns:          4,
	})
	if err != nil {
		pterm.FgRed.Printf("✗ Failed to create execution agent: %v\n", err)
		os.Exit(1)
	}

	validator, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "CodeValidator",
		Stream:  true,
		Instructions: fmt.Sprintf(`ROLE: Code validator
DIR: %s

Run tests and build commands to verify implementation.

## Output Format
### Checks: [list]
### Success: Yes/No
### Errors: [if any]`, cwd),
		Tools:                   toolsList,
		Memory:                  mem,
		MaxToolCallsFromHistory: 5,
		NumHistoryRuns:          4,
	})
	if err != nil {
		pterm.FgRed.Printf("✗ Failed to create validator agent: %v\n", err)
		os.Exit(1)
	}

	debugger, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "CodeDebugger",
		Stream:  true,
		Instructions: fmt.Sprintf(`ROLE: Code debugger
DIR: %s

Analyze failures and propose fixes.

## Output Format
### Error: [error description]
### Root Cause: [explanation]
### Fix: [specific instructions]`, cwd),
		Tools:                   toolsList,
		Memory:                  mem,
		MaxToolCallsFromHistory: 5,
		NumHistoryRuns:          4,
	})
	if err != nil {
		pterm.FgRed.Printf("✗ Failed to create debugger agent: %v\n", err)
		os.Exit(1)
	}

	// Workflow steps
	analyzeStep, err := v2.NewStep(
		v2.WithName("Analysis"),
		v2.WithAgent(analyzer),
		v2.WithStepStreaming(true),
	)
	if err != nil {
		log.Fatal("Error creating analysis step:", err)
	}

	planStep, err := v2.NewStep(
		v2.WithName("Planning"),
		v2.WithAgent(planner),
		v2.WithStepStreaming(true),
	)
	if err != nil {
		log.Fatal("Error creating planning step:", err)
	}

	executeStep, err := v2.NewStep(
		v2.WithName("Execution"),
		v2.WithAgent(executor),
		v2.WithStepStreaming(true),
	)
	if err != nil {
		log.Fatal("Error creating execution step:", err)
	}

	validateStep, err := v2.NewStep(
		v2.WithName("Validation"),
		v2.WithAgent(validator),
		v2.WithStepStreaming(true),
	)
	if err != nil {
		log.Fatal("Error creating validation step:", err)
	}

	debugStep, err := v2.NewStep(
		v2.WithName("Debugging"),
		v2.WithAgent(debugger),
		v2.WithStepStreaming(true),
	)
	if err != nil {
		log.Fatal("Error creating debugging step:", err)
	}

	// Conditional Debugging: Only run if validation failed
	conditionalDebug := v2.NewCondition(
		v2.WithConditionName("ConditionalDebug"),
		v2.WithIf(func(input *v2.StepInput) bool {
			if valOutput, ok := input.PreviousStepOutputs["Validation"]; ok {
				if success, ok := valOutput.Metadata["success"].(bool); ok {
					return !success
				}
			}
			return false
		}),
		v2.WithThen(debugStep),
	)

	// Loop: Execute -> Validate -> (If Fail) Debug -> Repeat
	executionLoop := v2.NewLoop(
		v2.WithLoopName("ImplementationLoop"),
		v2.WithLoopSteps(executeStep, validateStep, conditionalDebug),
		v2.WithMaxIterations(5),
		v2.WithLoopCondition(v2.UntilSuccess()),
	)

	// Workflow
	workflow := v2.NewWorkflow(
		v2.WithWorkflowName("Agno Coder Workflow"),
		v2.WithWorkflowDescription("Workflow for code analysis, planning, execution and validation"),
		v2.WithStreaming(false, false),
		v2.WithWorkflowSteps([]interface{}{
			analyzeStep,
			planStep,
			executionLoop,
		}),
	)

	// Display the prompt
	pterm.FgCyan.Printf("📝 Task: %s\n", prompt)
	pterm.Println()

	// Execute workflow with the user's prompt
	result, err := workflow.Run(ctx, prompt)
	if err != nil {
		pterm.FgRed.Printf("✗ Erro: %v\n", err)
		os.Exit(1)
	}

	// Print final result cleanly
	if result != nil && result.Content != nil {
		pterm.Println()
		fmt.Println(result.Content)
	}
}

// SmartStreamPrinter prints streaming responses with intelligent pauses at natural breakpoints
func SmartStreamPrinter(response string) {
	buffer := ""
	breakPoints := []string{".", "!", "?", "\n"}

	for i, char := range response {
		buffer += string(char)

		// Check if we hit a natural break point
		isBreakPoint := false
		for _, bp := range breakPoints {
			if strings.HasSuffix(buffer, bp) {
				isBreakPoint = true
				break
			}
		}

		// Print at break points or every 50 chars if no punctuation
		if isBreakPoint || (i+1)%50 == 0 {
			fmt.Print(buffer)
			buffer = ""
		}
	}

	// Print remaining buffer
	if buffer != "" {
		fmt.Print(buffer)
	}
}
