package main

import (
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/anthropic"
	"github.com/devalexandre/agno-golang/agno/models/aws"
	"github.com/devalexandre/agno-golang/agno/models/azure"
	"github.com/devalexandre/agno-golang/agno/models/deepseek"
	"github.com/devalexandre/agno-golang/agno/models/groq"
	"github.com/devalexandre/agno-golang/agno/models/openai/chat"
)

func main() {
	// 1. OpenAI (Using environment variable OPENAI_API_KEY)
	fmt.Println("--- OpenAI Example ---")
	openAIModel, _ := chat.NewOpenAIChat(models.WithID("gpt-4o"))
	openAIAgent, _ := agent.NewAgent(agent.AgentConfig{
		Model:       openAIModel,
		Description: "You are a helpful assistant.",
	})
	resp, _ := openAIAgent.Run("Explain quantum entanglement in one sentence.")
	fmt.Printf("OpenAI: %s\n\n", resp.TextContent)

	// 2. Anthropic (Claude)
	fmt.Println("--- Anthropic Example ---")
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey != "" {
		anthropicModel, _ := anthropic.New(
			models.WithID("claude-3-5-sonnet-20240620"),
			models.WithAPIKey(anthropicKey),
		)
		anthropicAgent, _ := agent.NewAgent(agent.AgentConfig{
			Model: anthropicModel,
		})
		resp, _ = anthropicAgent.Run("What is the capital of France?")
		fmt.Printf("Anthropic: %s\n\n", resp.TextContent)
	} else {
		fmt.Println("Skipping Anthropic (ANTHROPIC_API_KEY not set)\n")
	}

	// 3. DeepSeek
	fmt.Println("--- DeepSeek Example ---")
	deepseekKey := os.Getenv("DEEPSEEK_API_KEY")
	if deepseekKey != "" {
		deepseekModel, _ := deepseek.New(
			models.WithID("deepseek-chat"),
			models.WithAPIKey(deepseekKey),
		)
		deepseekAgent, _ := agent.NewAgent(agent.AgentConfig{
			Model: deepseekModel,
		})
		resp, _ = deepseekAgent.Run("Write a short poem about Go programming.")
		fmt.Printf("DeepSeek: %s\n\n", resp.TextContent)
	} else {
		fmt.Println("Skipping DeepSeek (DEEPSEEK_API_KEY not set)\n")
	}

	// 4. Groq
	fmt.Println("--- Groq Example ---")
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey != "" {
		groqModel, _ := groq.New(
			models.WithID("llama3-70b-8192"),
			models.WithAPIKey(groqKey),
		)
		groqAgent, _ := agent.NewAgent(agent.AgentConfig{
			Model: groqModel,
		})
		resp, _ = groqAgent.Run("Tell me a joke.")
		fmt.Printf("Groq: %s\n\n", resp.TextContent)
	} else {
		fmt.Println("Skipping Groq (GROQ_API_KEY not set)\n")
	}

	// 5. Azure OpenAI
	fmt.Println("--- Azure OpenAI Example ---")
	if os.Getenv("AZURE_OPENAI_API_KEY") != "" {
		azureModel, _ := azure.New(
			models.WithAPIKey(os.Getenv("AZURE_OPENAI_API_KEY")),
			models.WithBaseURL(os.Getenv("AZURE_OPENAI_ENDPOINT")),
			models.WithID(os.Getenv("AZURE_OPENAI_DEPLOYMENT_NAME")),
		)
		azureAgent, _ := agent.NewAgent(agent.AgentConfig{
			Model: azureModel,
		})
		resp, _ = azureAgent.Run("Hello from Azure!")
		fmt.Printf("Azure: %s\n\n", resp.TextContent)
	} else {
		fmt.Println("Skipping Azure (AZURE_OPENAI_API_KEY not set)\n")
	}

	// 6. AWS Bedrock
	fmt.Println("--- AWS Bedrock Example ---")
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		bedrockModel, _ := aws.New(
			models.WithID("anthropic.claude-3-sonnet-20240229-v1:0"),
			// Region and credentials should be in env or default AWS config
		)
		bedrockAgent, _ := agent.NewAgent(agent.AgentConfig{
			Model: bedrockModel,
		})
		resp, _ = bedrockAgent.Run("What can you do through Bedrock?")
		fmt.Printf("Bedrock: %s\n\n", resp.TextContent)
	} else {
		fmt.Println("Skipping AWS Bedrock (AWS credentials not set)\n")
	}
}
