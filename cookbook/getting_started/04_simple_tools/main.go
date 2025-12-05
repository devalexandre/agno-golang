package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Define simple tools as regular Go functions - Python style!

func add(a int, b int) (int, error) {
	return a + b, nil
}

func multiply(a int, b int) (int, error) {
	return a * b, nil
}

func greet(name string) (string, error) {
	return fmt.Sprintf("Hello, %s! Welcome to Agno.", name), nil
}

func main() {
	ctx := context.Background()

	// Create an Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create tools from simple functions - just like Python!
	addTool := tools.NewToolFromFunction(add, "Add two numbers together")
	multiplyTool := tools.NewToolFromFunction(multiply, "Multiply two numbers together")
	greetTool := tools.NewToolFromFunction(greet, "Generate a greeting message for someone")

	// Convert to toolkit.Tool interface for compatibility
	toolsList := []toolkit.Tool{
		addTool,
		multiplyTool,
		greetTool,
	}

	// Create agent with tools
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "Math Assistant",
		Instructions: `You are a helpful math assistant. Use the available tools to help solve math problems.
When asked a question, use the appropriate tools and provide clear answers.`,
		Tools: toolsList,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Use the agent
	fmt.Println("=== Simple Tools Example (Python-like API) ===\n")

	// Example 1: Math operations
	fmt.Println("Example 1: Math question")
	ag.PrintResponse("What is 5 plus 3? Then multiply the result by 2", true, true)

	// Example 2: Greeting
	fmt.Println("Example 2: Greeting")
	ag.PrintResponse("Please greet someone named Alice", true, true)

	// Example 3: Combined
	fmt.Println("Example 3: Combined question")
	ag.PrintResponse("Calculate 10 times 5, then greet Bob", true, true)

	fmt.Println("\n✅ Simple tools example completed!")
}
