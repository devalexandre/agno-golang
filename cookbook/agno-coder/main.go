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
	// 	models.WithID("qwen/qwen3-235b-a22b:free"),
	// )
	// if err != nil {
	// 	log.Fatalf("Failed to create OpenRouter chat: %v", err)
	// }

	model, err := ollama.NewOllamaChat(
		models.WithID("cogito:3b"),
	)
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
		Instructions: fmt.Sprintf(`ROLE: Intelligent code analysis expert
CURRENT WORKING DIRECTORY: %s

TASK: Analyze the user's request, find relevant files, and provide structured feedback.

CRITICAL WORKFLOW:
1. FIRST, parse the user's prompt to identify:
   - What action they want (review, analyze, refactor, add, fix, etc.)
   - Any file paths, directory paths, or patterns mentioned
   - The specific requirements or concerns

2. USE TOOLS TO FIND FILES:
   - If a specific file path is mentioned, use FileTool.ReadFile to read it
   - If a directory is mentioned, use FileTool.ListDirectory to explore it
   - If you need to search for files, use FileTool.SearchFiles or ShellTool.Execute with 'find' or 'grep'
   - Use ShellTool.Execute with commands like:
     * 'find . -name "*.go"' to find Go files
     * 'grep -r "pattern" .' to search for patterns
     * 'cat <file>' to read file contents
     * 'ls -la <dir>' to list directory contents

3. ANALYZE THE CODE:
   - Read the relevant files using FileTool.ReadFile
   - Understand the code structure and purpose
   - Identify issues based on the user's request

ANALYSIS CATEGORIES (as applicable):
- Architecture & Structure
- Security vulnerabilities
- Performance bottlenecks
- Code quality & maintainability
- Best practices compliance

OUTPUT FORMAT (strict markdown):
## 📊 Code Analysis

### Files Analyzed
- [list of files found and analyzed]

### Summary
[Brief summary of what was found]

### Findings
[Detailed findings based on user's request]

### Recommendations
[Actionable recommendations]

NEVER assume file contents - ALWAYS use tools to read them first.`, cwd),
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
		Instructions: fmt.Sprintf(`ROLE: Senior developer planning implementation
CURRENT WORKING DIRECTORY: %s

TASK: Create executable implementation plans based on the analysis.

WORKFLOW:
1. Review the analysis from the previous step
2. If you need more context, use FileTool.ReadFile or ShellTool.Execute to gather information
3. Create a detailed, step-by-step plan

PLANNING STEPS:
1. Understand requirements from analysis report
2. Break into atomic, sequential tasks
3. Identify required files and modifications
4. Define verification steps for each change

OUTPUT FORMAT (strict markdown):
## 🎯 Implementation Plan

### Files to Modify
- [path/to/file.go]: [purpose]

### Step-by-Step Implementation
1. **Task**: [specific action]
   - File: [path]
   - Changes: [exact code changes]
   - Verification: [command to verify]

2. **Task**: [next action]
   [continue as needed]

### Validation Plan
- Command: [exact command to run]
- Expected output: [what success looks like]

NO commentary outside this format.`, cwd),
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
		Instructions: fmt.Sprintf(`ROLE: Code executor
CURRENT WORKING DIRECTORY: %s

OBJECTIVE:
Execute implementation plans step-by-step.

TOOLS AVAILABLE:
- FileTool: Read/write files, manage directories
- ShellTool: Execute commands, run tests

WORKFLOW:
1. **Analyze Input**: Read the plan AND any feedback/errors from previous attempts (if any).
2. **Execute**: Modify files and run commands to implement the plan or fix the reported errors.
3. **Verify**: Run quick checks to ensure changes were applied.

OUTPUT FORMAT (Markdown):

## 🔨 Execution Report

### Modified Files
- **file.go**: [Changes made]

### Implemented Features
- [x] [Feature description]

### Fixes Applied (if retrying)
- [x] [Fix description]

### Results
- **Status**: ✅ Success | ⚠️ Partial | ❌ Failure

Execute commands using ShellTool and modify files using FileTool.`, cwd),
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
		Instructions: fmt.Sprintf(`ROLE: Code validator
CURRENT WORKING DIRECTORY: %s

OBJECTIVE:
Verify if the implementation meets the requirements and works correctly.

TOOLS AVAILABLE:
- FileTool: Read files to check content
- ShellTool: Run tests, build commands, or scripts

WORKFLOW:
1. **Read Requirements**: Understand what was supposed to be done.
2. **Verify**: Run commands (go build, go test, etc.) or check file contents.
3. **Report**: Return success or failure with details.

IMPORTANT:
- If validation PASSES, you MUST set "success": true in your output metadata.
- If validation FAILS, you MUST set "success": false in your output metadata and provide error details.

OUTPUT FORMAT (Markdown):

## 🔍 Validation Report

### Checks Performed
- [x] [Check description]

### Outcome
- **Success**: [Yes/No]
- **Errors**: [List of errors if any]
`, cwd),
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
		Instructions: fmt.Sprintf(`ROLE: Code debugger
CURRENT WORKING DIRECTORY: %s

OBJECTIVE:
Analyze validation failures and provide a fix strategy.

TOOLS AVAILABLE:
- FileTool: Read files to understand the code
- ShellTool: Run commands if needed to reproduce

WORKFLOW:
1. **Analyze Error**: Read the validation report and the code.
2. **Identify Cause**: Determine why it failed.
3. **Propose Fix**: Provide specific instructions or code blocks to fix the issue.

OUTPUT FORMAT (Markdown):

## 🐞 Debug Analysis

### Error Analysis
- [Error description]

### Root Cause
- [Explanation]

### Fix Strategy
- [Specific instructions for Executor]
`, cwd),
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
		v2.WithStreaming(true, true),
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
	workflow.PrintResponse(prompt, true)
}
