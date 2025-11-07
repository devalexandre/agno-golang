package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
	// Check if prompt is provided as argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go \"Your question here\"")
		fmt.Println("Example: go run main.go \"Explique como o agno-go funciona\"")
		os.Exit(1)
	}

	prompt := os.Args[1]
	modelName := "llama3.2:latest"
	baseURL := "http://localhost:11434"
	debug := false

	fmt.Println("=== Workflow Prompt Example ===")
	fmt.Printf("Model: %s\n", modelName)
	fmt.Printf("Prompt: %s\n", prompt)
	fmt.Println()

	ctx := context.Background()

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID(modelName),
		models.WithBaseURL(baseURL),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create agents for the workflow
	analyzerAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:     ctx,
		Name:        "Analyzer",
		Role:        "Question Analyzer",
		Description: "Analyzes user questions to understand intent and context",
		Goal:        "To understand and categorize user questions",
		Instructions: `Analyze the user's question and provide:
1. The main topic or subject
2. The type of question (explanation, how-to, factual, creative, etc.)
3. Key concepts involved
4. Suggested approach for answering

Format your response as JSON with these fields: topic, question_type, key_concepts, approach`,
		Model: model,
		Debug: debug,
	})
	if err != nil {
		log.Fatalf("Failed to create analyzer agent: %v", err)
	}

	processorAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:     ctx,
		Name:        "Processor",
		Role:        "Content Processor",
		Description: "Processes questions and provides detailed responses",
		Goal:        "To provide comprehensive and helpful answers",
		Instructions: `Based on the analysis provided, give a comprehensive answer to the user's question. 
Make your response:
- Clear and well-structured
- Informative and accurate
- Engaging and helpful
- Include examples when appropriate`,
		Model: model,
		Debug: debug,
	})
	if err != nil {
		log.Fatalf("Failed to create processor agent: %v", err)
	}

	reviewerAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:     ctx,
		Name:        "Reviewer",
		Role:        "Quality Reviewer",
		Description: "Reviews and refines responses for quality and completeness",
		Goal:        "To ensure responses are of high quality and completeness",
		Instructions: `Review the provided response and:
1. Check for accuracy and completeness
2. Improve clarity and structure if needed
3. Add any missing important information
4. Ensure the tone is appropriate and helpful

Provide the final, polished response.`,
		Model: model,
		Debug: debug,
	})
	if err != nil {
		log.Fatalf("Failed to create reviewer agent: %v", err)
	}

	// Create the workflow
	workflow := v2.NewWorkflow(
		v2.WithWorkflowID("prompt-processing-workflow"),
		v2.WithDebugMode(debug),
		v2.WithStreaming(true, true),
	)

	// Create wrapper functions for agents to match the expected interface
	analyzeExecutor := func(input *v2.StepInput) (*v2.StepOutput, error) {
		message := input.GetMessageAsString()
		response, err := analyzerAgent.Run(message)
		if err != nil {
			return nil, err
		}
		return &v2.StepOutput{
			Content:      response.TextContent,
			StepName:     "analyze",
			ExecutorType: "agent",
			ExecutorName: analyzerAgent.GetName(),
		}, nil
	}

	processExecutor := func(input *v2.StepInput) (*v2.StepOutput, error) {
		// Combine original message with analysis
		message := input.GetMessageAsString()
		if analysis := input.GetStepContent("analyze"); analysis != nil {
			message = fmt.Sprintf("Original Question: %s\n\nAnalysis: %v", message, analysis)
		}

		response, err := processorAgent.Run(message)
		if err != nil {
			return nil, err
		}
		return &v2.StepOutput{
			Content:      response.TextContent,
			StepName:     "process",
			ExecutorType: "agent",
			ExecutorName: processorAgent.GetName(),
		}, nil
	}

	reviewExecutor := func(input *v2.StepInput) (*v2.StepOutput, error) {
		// Get the processed response for review
		processedContent := input.GetStepContent("process")
		if processedContent == nil {
			return nil, fmt.Errorf("no processed content to review")
		}

		reviewMessage := fmt.Sprintf("Please review and improve this response:\n\n%v", processedContent)
		response, err := reviewerAgent.Run(reviewMessage)
		if err != nil {
			return nil, err
		}
		return &v2.StepOutput{
			Content:      response.TextContent,
			StepName:     "review",
			ExecutorType: "agent",
			ExecutorName: reviewerAgent.GetName(),
		}, nil
	}

	// Define workflow steps using ExecutorFunc
	analyzeStep, err := v2.NewStep(
		v2.WithName("analyze"),
		v2.WithDescription("Analyze the user's question"),
		v2.WithExecutor(analyzeExecutor),
		v2.WithTimeout(30),
	)
	if err != nil {
		log.Fatalf("Failed to create analyze step: %v", err)
	}

	processStep, err := v2.NewStep(
		v2.WithName("process"),
		v2.WithDescription("Process and answer the question"),
		v2.WithExecutor(processExecutor),
		v2.WithTimeout(60),
	)
	if err != nil {
		log.Fatalf("Failed to create process step: %v", err)
	}

	reviewStep, err := v2.NewStep(
		v2.WithName("review"),
		v2.WithDescription("Review and refine the response"),
		v2.WithExecutor(reviewExecutor),
		v2.WithTimeout(45),
	)
	if err != nil {
		log.Fatalf("Failed to create review step: %v", err)
	}

	// Add event handlers for real-time feedback
	workflow.OnEvent(v2.StepStartedEvent, func(event *v2.WorkflowRunResponseEvent) {
		if stepName, ok := event.Metadata["step_name"].(string); ok {
			fmt.Printf("🔄 Starting step: %s\n", stepName)
		}
	})

	workflow.OnEvent(v2.StepCompletedEvent, func(event *v2.WorkflowRunResponseEvent) {
		if stepName, ok := event.Metadata["step_name"].(string); ok {
			fmt.Printf("✅ Completed step: %s\n", stepName)
			if debug && event.Data != nil {
				if stepOutput, ok := event.Data.(*v2.StepOutput); ok {
					fmt.Printf("   Output preview: %s\n", truncateString(fmt.Sprintf("%v", stepOutput.Content), 100))
				}
			}
		}
	})

	workflow.OnEvent(v2.WorkflowCompletedEvent, func(event *v2.WorkflowRunResponseEvent) {
		fmt.Printf("🎉 Workflow completed successfully!\n")
	})

	// Set the workflow steps directly as a slice of steps
	workflow.Steps = []*v2.Step{analyzeStep, processStep, reviewStep}

	// Prepare workflow input
	input := &v2.WorkflowExecutionInput{
		Message: prompt,
		AdditionalData: map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"model":     modelName,
		},
	}

	// Execute the workflow
	fmt.Println("🚀 Starting workflow execution...")
	fmt.Println(strings.Repeat("-", 60))

	response, err := workflow.Run(ctx, input)
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	// Display results
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("📋 WORKFLOW RESULTS:")
	fmt.Println(strings.Repeat("-", 60))

	// Get step outputs for detailed view using the step getter method
	if debug {
		if analyzeOutput := workflow.GetStepOutput("analyze"); analyzeOutput != nil {
			fmt.Println("🔍 ANALYSIS:")
			fmt.Printf("%v\n", analyzeOutput.Content)
			fmt.Println()
		}

		if processOutput := workflow.GetStepOutput("process"); processOutput != nil {
			fmt.Println("⚙️ PROCESSING:")
			fmt.Printf("%v\n", processOutput.Content)
			fmt.Println()
		}
	}

	if reviewOutput := workflow.GetStepOutput("review"); reviewOutput != nil {
		fmt.Println("📝 FINAL RESPONSE:")
		fmt.Printf("%v\n", reviewOutput.Content)
		fmt.Println()
	}

	// Final response from workflow
	fmt.Println("🏁 WORKFLOW OUTPUT:")
	fmt.Printf("%v\n", response.Content)

	// Show execution metrics if debug enabled
	if debug && workflow.GetMetrics() != nil {
		metrics := workflow.GetMetrics()
		fmt.Println(strings.Repeat("-", 60))
		fmt.Println("📊 EXECUTION METRICS:")
		fmt.Printf("Total Duration: %v\n", time.Duration(metrics.DurationMs)*time.Millisecond)
		if metrics.StepsExecuted > 0 {
			successRate := float64(metrics.StepsSucceeded) / float64(metrics.StepsExecuted) * 100
			fmt.Printf("Success Rate: %.2f%%\n", successRate)
		}

		for stepName, stepMetrics := range metrics.StepMetrics {
			fmt.Printf("Step '%s': %v (retries: %d)\n",
				stepName, time.Duration(stepMetrics.DurationMs)*time.Millisecond, stepMetrics.RetryCount)
		}
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("✨ Example completed successfully!")
}

// truncateString truncates a string to a maximum length
func truncateString(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen] + "..."
}
