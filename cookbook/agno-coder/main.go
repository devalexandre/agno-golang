package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

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

	var task string
	var analyze string
	var implement string
	var prompt string
	var path string

	flag.StringVar(&task, "task", "", "General task for the coder")
	flag.StringVar(&analyze, "analyze", "", "Code or file to analyze")
	flag.StringVar(&implement, "implement", "", "Implementation to be done")
	flag.StringVar(&prompt, "prompt", "", "Custom prompt/instruction for the task")
	flag.StringVar(&path, "path", "", "Path to file or folder to analyze")
	flag.Parse()

	if task == "" && analyze == "" && implement == "" && prompt == "" {
		pterm.FgRed.Println("✗ Required parameters not provided")
		pterm.Println()
		pterm.FgBlue.Println("Usage:")
		pterm.Println("  agno-coder --task <task>")
		pterm.Println("  agno-coder --analyze <code>")
		pterm.Println("  agno-coder --implement <implementation>")
		pterm.Println("  agno-coder --prompt <custom_prompt> --path <file_or_folder>")
		pterm.Println()
		pterm.FgGray.Println("Examples:")
		pterm.Println("  agno-coder --analyze main.go")
		pterm.Println("  agno-coder --prompt 'Add error handling' --path ./cmd")
		pterm.Println("  agno-coder --task 'Refactor authentication module'")
		pterm.Println()
		os.Exit(1)
	}

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

	model, err := ollama.NewOllamaChat(
		models.WithID("qwen3-coder:480b-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(os.Getenv("OLLAMA_API_KEY")),
	)
	if err != nil {
		pterm.Println()
		pterm.FgRed.Printf("✗ Failed to initialize model: %v\n", err)
		os.Exit(1)
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

	// Criar agentes silenciosamente
	analyzer, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "CodeAnalyzer",
		Instructions: `You are a Go code analyst specialized in using Qwen Coder.

OBJECTIVE:
Analyze Go code files and provide structured feedback on architecture, security, performance, and maintainability.

WORKFLOW:
1. **Read the file** - If given a file path, use FileTool.ReadFile to read its contents
2. **Analyze the code** - Review structure, patterns, issues
3. **Provide feedback** - Format as markdown below

TOOLS AVAILABLE:
- FileTool: Read files, list directories, search code
- ShellTool: Execute commands, get system info

OUTPUT FORMAT (Markdown):

## 📊 Code Analysis

### General Structure
- Architecture and design patterns
- Dependencies and imports

### Critical Issues
- **Security**: Vulnerabilities, validations
- **Performance**: Bottlenecks, complexity
- **Maintainability**: Code duplication, long functions

### Quality Issues
- Go conventions
- Naming and documentation

### Improvements
- Refactoring suggestions
- Best practices to adopt

### Metrics
- Complexity score
- Maintainability (1-10)

Be objective and actionable.`,
		Tools:  toolsList,
		Memory: mem,
	})
	if err != nil {
		pterm.FgRed.Printf("✗ Failed to create analysis agent: %v\n", err)
		os.Exit(1)
	}

	planner, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "CodePlanner",
		Instructions: `You are a code planner using Qwen Coder.

OBJECTIVE:
Create clear, executable implementation plans.

TOOLS AVAILABLE:
- FileTool: Read/write files, list directories, search
- ShellTool: Execute commands, check environment

OUTPUT FORMAT (Markdown):

## 🎯 Implementation Plan

### Context
- Objective: [What needs to be done]
- Scope: [What's included]
- Constraints: [Limitations]

### Steps
1. **Preparation**
   - Check dependencies
   - Setup environment
   - Create backups if needed

2. **Implementation**
   - [Specific steps]

3. **Testing**
   - Unit tests
   - Integration tests
   - Validation

### Resources Needed
- Dependencies
- Tools
- Infrastructure

### Risks
- [Risk]: [Mitigation]

### Success Criteria
- [Measurable outcomes]

Make it executable by a senior developer.`,
		Tools:  toolsList,
		Memory: mem,
	})
	if err != nil {
		pterm.FgRed.Printf("✗ Failed to create planning agent: %v\n", err)
		os.Exit(1)
	}

	executor, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "CodeExecutor",
		Instructions: `You are a code executor using Qwen Coder.

OBJECTIVE:
Execute implementation plans step-by-step.

TOOLS AVAILABLE:
- FileTool: Read/write files, manage directories
- ShellTool: Execute commands, run tests

WORKFLOW:
1. Validate the plan
2. Execute each step carefully
3. Test changes
4. Document results

OUTPUT FORMAT (Markdown):

## 🔨 Execution Report

### Modified Files
- **file.go**: [Changes made]

### Implemented Features
- [x] [Feature description]

### Tests Performed
- [x] [Test description]

### Results
- **Status**: ✅ Success | ⚠️ Partial | ❌ Failure
- **Tests**: [Pass/Fail count]
- **Notes**: [Important observations]

Execute commands using ShellTool and modify files using FileTool.`,
		Tools:  toolsList,
		Memory: mem,
	})
	if err != nil {
		pterm.FgRed.Printf("✗ Failed to create execution agent: %v\n", err)
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

	// Workflow
	workflow := v2.NewWorkflow(
		v2.WithWorkflowName("Agno Coder Workflow"),
		v2.WithWorkflowDescription("Workflow for code analysis, planning and execution"),
		v2.WithWorkflowSteps([]*v2.Step{
			analyzeStep,
			planStep,
			executeStep,
		}),
	)

	// Determine input based on flags
	var input string

	// Priority 1: Custom prompt with path
	if prompt != "" {
		if path != "" {
			// Check if path is a directory or file
			fileInfo, err := os.Stat(path)
			if err != nil {
				pterm.Error.Printf("Path not found: %s\n", path)
				os.Exit(1)
			}

			if fileInfo.IsDir() {
				input = fmt.Sprintf("%s\n\nAnalyze all files in the directory: %s", prompt, path)
			} else {
				input = fmt.Sprintf("%s\n\nRead and analyze the file at path: %s", prompt, path)
			}
		} else {
			input = prompt
		}
	} else if analyze != "" {
		// Priority 2: Legacy analyze flag
		input = fmt.Sprintf("Read and analyze the file at path: %s\nProvide a detailed analysis with improvement suggestions.", analyze)
	} else if implement != "" {
		// Priority 3: Implement flag
		input = fmt.Sprintf("Implement: %s", implement)
	} else {
		// Priority 4: General task
		input = task
	}

	// Execute workflow
	pterm.FgGray.Println("Running workflow...")
	pterm.Println()

	workflow.PrintResponse(input, true)
}
