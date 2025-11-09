package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/team"
)

// AgentWrapper wraps an agent to implement the TeamMember interface
type AgentWrapper struct {
	agent *agent.Agent
}

func (aw *AgentWrapper) GetName() string {
	return aw.agent.GetName()
}

func (aw *AgentWrapper) GetRole() string {
	return aw.agent.GetRole()
}

func (aw *AgentWrapper) Run(prompt string) (models.RunResponse, error) {
	return aw.agent.Run(prompt)
}

func (aw *AgentWrapper) RunStream(prompt string, fn func([]byte) error) error {
	return aw.agent.RunStream(prompt, fn)
}

func main() {
	ctx := context.Background()

	fmt.Println("👥 Team Collaboration Example")
	fmt.Println("==============================")
	fmt.Println("")

	// Get API key from environment
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// 1. Create Ollama Cloud model
	fmt.Println("🤖 Setting up Ollama Cloud model...")
	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama Cloud model: %v", err)
	}

	// 2. Create specialized agents for the team
	fmt.Println("👨‍💼 Creating specialized agents...")
	fmt.Println("")

	// Research Agent
	researchAgent, err := agent.NewAgent(agent.AgentConfig{
		Name:        "Research Specialist",
		Model:       ollamaModel,
		Role:        "researcher",
		Description: "Expert in gathering and analyzing information",
		Instructions: `You are a research specialist. Your role is to:
- Gather relevant information on topics
- Analyze data and identify key insights
- Provide well-researched, factual responses
- Cite sources when possible
Keep your responses concise and focused on facts.`,
		Markdown: false,
		Debug:    false,
	})
	if err != nil {
		log.Fatalf("Failed to create research agent: %v", err)
	}
	fmt.Println("✅ Research Specialist created")

	// Writer Agent
	writerAgent, err := agent.NewAgent(agent.AgentConfig{
		Name:        "Content Writer",
		Model:       ollamaModel,
		Role:        "writer",
		Description: "Expert in creating engaging content",
		Instructions: `You are a content writer. Your role is to:
- Transform research into engaging content
- Write clear, compelling narratives
- Adapt tone and style to the audience
- Structure content effectively
Focus on clarity and engagement.`,
		Markdown: false,
		Debug:    false,
	})
	if err != nil {
		log.Fatalf("Failed to create writer agent: %v", err)
	}
	fmt.Println("✅ Content Writer created")

	// Editor Agent
	editorAgent, err := agent.NewAgent(agent.AgentConfig{
		Name:        "Editor",
		Model:       ollamaModel,
		Role:        "editor",
		Description: "Expert in reviewing and improving content",
		Instructions: `You are an editor. Your role is to:
- Review content for clarity and accuracy
- Improve structure and flow
- Ensure consistency in tone and style
- Provide constructive feedback
Be thorough but supportive in your reviews.`,
		Markdown: false,
		Debug:    false,
	})
	if err != nil {
		log.Fatalf("Failed to create editor agent: %v", err)
	}
	fmt.Println("✅ Editor created")

	// 3. Create agent wrappers for team compatibility
	researchMember := &AgentWrapper{agent: researchAgent}
	writerMember := &AgentWrapper{agent: writerAgent}
	editorMember := &AgentWrapper{agent: editorAgent}

	// 4. Create the team
	fmt.Println("\n🎯 Creating collaborative team...")
	contentTeam := team.NewTeam(team.TeamConfig{
		Context:     ctx,
		Name:        "Content Creation Team",
		Description: "A team specialized in creating high-quality content",
		Model:       ollamaModel,
		Members:     []team.TeamMember{researchMember, writerMember, editorMember},
		Mode:        team.CoordinateMode,
		Debug:       false,
		Markdown:    false,
	})

	fmt.Println("✅ Team created with 3 members")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("")

	// 5. Demonstrate team collaboration
	topic := "The benefits of Go programming language for building AI applications"

	fmt.Printf("📋 Task: Create an article about '%s'\n", topic)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("")

	// Step 1: Research phase
	fmt.Println("🔍 Phase 1: Research")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	researchPrompt := fmt.Sprintf(`Research the following topic and provide key points:
Topic: %s

Provide:
1. Main benefits (3-5 points)
2. Key features that support these benefits
3. Real-world use cases`, topic)

	fmt.Printf("\n👨‍💼 Research Specialist working...\n")
	researchResponse, err := researchAgent.Run(researchPrompt)
	if err != nil {
		log.Fatalf("Research failed: %v", err)
	}
	fmt.Printf("\n📊 Research Results:\n%s\n", researchResponse.TextContent)

	// Step 2: Writing phase
	fmt.Println("\n\n✍️ Phase 2: Content Writing")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	writerPrompt := fmt.Sprintf(`Based on this research, write an engaging article:

Research:
%s

Write a 200-word article that:
- Has a compelling introduction
- Presents the benefits clearly
- Includes practical examples
- Ends with a strong conclusion`, researchResponse.TextContent)

	fmt.Printf("\n✍️ Content Writer working...\n")
	writerResponse, err := writerAgent.Run(writerPrompt)
	if err != nil {
		log.Fatalf("Writing failed: %v", err)
	}
	fmt.Printf("\n📝 Draft Article:\n%s\n", writerResponse.TextContent)

	// Step 3: Editing phase
	fmt.Println("\n\n📋 Phase 3: Editorial Review")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	editorPrompt := fmt.Sprintf(`Review and improve this article:

%s

Provide:
1. Improved version of the article
2. Key changes made
3. Overall assessment`, writerResponse.TextContent)

	fmt.Printf("\n📋 Editor reviewing...\n")
	editorResponse, err := editorAgent.Run(editorPrompt)
	if err != nil {
		log.Fatalf("Editing failed: %v", err)
	}
	fmt.Printf("\n✅ Final Article:\n%s\n", editorResponse.TextContent)

	// 6. Demonstrate team-level operation
	fmt.Println("\n\n🎯 Team-Level Operation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	teamTask := "Explain why Go is good for concurrent programming in 100 words"
	fmt.Printf("\n📋 Team Task: %s\n", teamTask)

	teamResponse, err := contentTeam.Run(teamTask)
	if err != nil {
		log.Fatalf("Team execution failed: %v", err)
	}

	fmt.Printf("\n👥 Team Response:\n%s\n", teamResponse.TextContent)

	// 7. Show team statistics
	fmt.Println("\n\n📊 Team Statistics")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("\nTeam Name: %s\n", contentTeam.GetName())
	fmt.Printf("Team Role: %s\n", contentTeam.GetRole())
	fmt.Println("\nTeam Members:")
	fmt.Printf("  1. %s (%s)\n", researchAgent.GetName(), researchAgent.GetRole())
	fmt.Printf("  2. %s (%s)\n", writerAgent.GetName(), writerAgent.GetRole())
	fmt.Printf("  3. %s (%s)\n", editorAgent.GetName(), editorAgent.GetRole())

	fmt.Println("\n\n✨ Team Collaboration example completed!")
	fmt.Println("\n💡 Key Benefits of Team Collaboration:")
	fmt.Println("   • Specialized expertise for each task")
	fmt.Println("   • Better quality through multiple perspectives")
	fmt.Println("   • Efficient task delegation")
	fmt.Println("   • Scalable workflow management")
	fmt.Println("   • Improved output through collaboration")
}
