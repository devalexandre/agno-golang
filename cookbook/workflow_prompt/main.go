package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
	// Check if blog topic is provided as argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go \"Blog post topic\"")
		fmt.Println("Example: go run main.go \"Best practices for building AI agents in Go\"")
		os.Exit(1)
	}

	topic := os.Args[1]
	debug := false

	fmt.Println("=== Blog Post Generator Workflow ===")
	fmt.Printf("Topic: %s\n", topic)
	fmt.Println()

	ctx := context.Background()

	// Create Ollama Cloud model
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(os.Getenv("OLLAMA_API_KEY")),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create output directory for blog posts
	outputDir := "blog_posts"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Create agents for the blog post generation workflow
	researcherAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:     ctx,
		Name:        "Researcher",
		Role:        "Content Researcher",
		Description: "Researches topics and creates comprehensive outlines for blog posts",
		Goal:        "To create detailed, well-structured blog post outlines",
		Instructions: `Based on the topic provided, create a comprehensive blog post outline including:
1. A catchy title
2. Meta description (150-160 characters)
3. Main sections with subsections
4. Key points to cover in each section
5. Suggested examples or case studies
6. Target audience
7. Estimated reading time

Format your response as a structured outline in Markdown.`,
		Model: model,
		Debug: debug,
	})
	if err != nil {
		log.Fatalf("Failed to create researcher agent: %v", err)
	}

	writerAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:     ctx,
		Name:        "Writer",
		Role:        "Content Writer",
		Description: "Writes engaging blog post content based on outlines",
		Goal:        "To create high-quality, engaging blog post content",
		Instructions: `Based on the outline provided, write a complete blog post with:
- Engaging introduction that hooks the reader
- Well-structured sections with clear headings
- Code examples when relevant (use proper markdown code blocks)
- Real-world examples and use cases
- Clear explanations suitable for the target audience
- Strong conclusion with key takeaways

Write in a professional yet conversational tone. Use Markdown formatting.`,
		Model: model,
		Debug: debug,
	})
	if err != nil {
		log.Fatalf("Failed to create writer agent: %v", err)
	}

	editorAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:     ctx,
		Name:        "Editor",
		Role:        "Content Editor",
		Description: "Reviews and polishes blog posts for publication",
		Goal:        "To ensure blog posts are publication-ready",
		Instructions: `Review the blog post and improve it by:
1. Checking grammar, spelling, and punctuation
2. Improving clarity and flow
3. Ensuring consistent tone and style
4. Verifying code examples are correct and well-formatted
5. Adding relevant internal/external link suggestions
6. Optimizing for SEO (keywords, headings, meta description)
7. Ensuring the post is engaging and valuable

Provide the final, polished blog post in Markdown format with proper frontmatter including:
- title
- date
- author
- tags
- description`,
		Model: model,
		Debug: debug,
	})
	if err != nil {
		log.Fatalf("Failed to create editor agent: %v", err)
	}

	// Create the workflow
	workflow := v2.NewWorkflow(
		v2.WithWorkflowID("prompt-processing-workflow"),
		v2.WithDebugMode(debug),
		v2.WithStreaming(true, true),
	)

	// Create wrapper functions for agents to match the expected interface
	researchExecutor := func(input *v2.StepInput) (*v2.StepOutput, error) {
		message := input.GetMessageAsString()
		response, err := researcherAgent.Run(message)
		if err != nil {
			return nil, err
		}
		return &v2.StepOutput{
			Content:      response.TextContent,
			StepName:     "research",
			ExecutorType: "agent",
			ExecutorName: researcherAgent.GetName(),
		}, nil
	}

	writeExecutor := func(input *v2.StepInput) (*v2.StepOutput, error) {
		// Get the outline from research step
		outline := input.GetStepContent("research")
		if outline == nil {
			return nil, fmt.Errorf("no outline from research step")
		}

		message := fmt.Sprintf("Write a complete blog post based on this outline:\n\n%v", outline)
		response, err := writerAgent.Run(message)
		if err != nil {
			return nil, err
		}
		return &v2.StepOutput{
			Content:      response.TextContent,
			StepName:     "write",
			ExecutorType: "agent",
			ExecutorName: writerAgent.GetName(),
		}, nil
	}

	editExecutor := func(input *v2.StepInput) (*v2.StepOutput, error) {
		// Get the draft from write step
		draft := input.GetStepContent("write")
		if draft == nil {
			return nil, fmt.Errorf("no draft from write step")
		}

		message := fmt.Sprintf("Edit and finalize this blog post:\n\n%v", draft)
		response, err := editorAgent.Run(message)
		if err != nil {
			return nil, err
		}
		return &v2.StepOutput{
			Content:      response.TextContent,
			StepName:     "edit",
			ExecutorType: "agent",
			ExecutorName: editorAgent.GetName(),
		}, nil
	}

	// Define workflow steps using ExecutorFunc
	researchStep, err := v2.NewStep(
		v2.WithName("research"),
		v2.WithDescription("Research topic and create blog post outline"),
		v2.WithExecutor(researchExecutor),
		v2.WithTimeout(60),
	)
	if err != nil {
		log.Fatalf("Failed to create research step: %v", err)
	}

	writeStep, err := v2.NewStep(
		v2.WithName("write"),
		v2.WithDescription("Write blog post content based on outline"),
		v2.WithExecutor(writeExecutor),
		v2.WithTimeout(120),
	)
	if err != nil {
		log.Fatalf("Failed to create write step: %v", err)
	}

	editStep, err := v2.NewStep(
		v2.WithName("edit"),
		v2.WithDescription("Edit and finalize blog post for publication"),
		v2.WithExecutor(editExecutor),
		v2.WithTimeout(60),
	)
	if err != nil {
		log.Fatalf("Failed to create edit step: %v", err)
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
		fmt.Printf("🎉 Blog post generation completed!\n")
	})

	// Set the workflow steps directly as a slice of steps
	workflow.Steps = []*v2.Step{researchStep, writeStep, editStep}

	// Prepare workflow input
	input := &v2.WorkflowExecutionInput{
		Message: topic,
		AdditionalData: map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"model":     model.GetID(),
		},
	}

	// Execute the workflow
	fmt.Println("🚀 Starting blog post generation workflow...")
	fmt.Println(strings.Repeat("-", 60))

	response, err := workflow.Run(ctx, input)
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	// Display results
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("📋 BLOG POST GENERATION RESULTS:")
	fmt.Println(strings.Repeat("-", 60))

	// Get step outputs for detailed view using the step getter method
	if debug {
		if researchOutput := workflow.GetStepOutput("research"); researchOutput != nil {
			fmt.Println("🔍 RESEARCH & OUTLINE:")
			fmt.Printf("%v\n", researchOutput.Content)
			fmt.Println()
		}

		if writeOutput := workflow.GetStepOutput("write"); writeOutput != nil {
			fmt.Println("✍️ DRAFT:")
			fmt.Printf("%v\n", writeOutput.Content)
			fmt.Println()
		}
	}

	// Get the final edited blog post
	var finalBlogPost string
	if editOutput := workflow.GetStepOutput("edit"); editOutput != nil {
		fmt.Println("📝 FINAL BLOG POST:")
		finalBlogPost = fmt.Sprintf("%v", editOutput.Content)
		fmt.Printf("%v\n", finalBlogPost)
		fmt.Println()
	} else {
		finalBlogPost = fmt.Sprintf("%v", response.Content)
	}

	// Generate filename from topic (sanitize for filesystem)
	timestamp := time.Now().Format("2006-01-02")
	filename := generateFilename(topic, timestamp)
	filePath := filepath.Join(outputDir, filename)

	// Save blog post to file
	if err := os.WriteFile(filePath, []byte(finalBlogPost), 0644); err != nil {
		log.Fatalf("Failed to save blog post: %v", err)
	}

	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("💾 Blog post saved to: %s\n", filePath)
	fmt.Println(strings.Repeat("-", 60))

	// Show execution metrics if debug enabled
	if debug && workflow.GetMetrics() != nil {
		metrics := workflow.GetMetrics()
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
		fmt.Println(strings.Repeat("-", 60))
	}

	fmt.Println("✨ Blog post generation completed successfully!")
	fmt.Printf("📄 Open your blog post: %s\n", filePath)
}

// generateFilename creates a filesystem-safe filename from a topic and date
func generateFilename(topic, date string) string {
	// Convert to lowercase and replace spaces with hyphens
	safe := strings.ToLower(topic)
	safe = strings.ReplaceAll(safe, " ", "-")

	// Remove special characters, keep only alphanumeric and hyphens
	var result strings.Builder
	for _, r := range safe {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	// Clean up multiple consecutive hyphens
	cleaned := strings.ReplaceAll(result.String(), "--", "-")
	cleaned = strings.Trim(cleaned, "-")

	// Limit length
	if len(cleaned) > 50 {
		cleaned = cleaned[:50]
	}

	return fmt.Sprintf("%s-%s.md", date, cleaned)
}

// truncateString truncates a string to a maximum length
func truncateString(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen] + "..."
}
