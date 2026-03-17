package main

import (
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/chat"
	"github.com/devalexandre/agno-golang/agno/tools/aws"
	"github.com/devalexandre/agno-golang/agno/tools/system"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	// 1. System Tools Example (Native Go performance analysis)
	fmt.Println("--- System Tools Example ---")
	sysTools := system.NewSystemTools()

	// Use OpenAI to analyze system state
	openAIModel, _ := chat.NewOpenAIChat(models.WithID("gpt-4o"))
	sysAgent, _ := agent.NewAgent(agent.AgentConfig{
		Model:       openAIModel,
		Tools:       []toolkit.Tool{sysTools},
		Description: "You are a system administrator assistant. Use tools to check system health.",
	})

	resp, _ := sysAgent.Run("What is the current CPU usage and free memory?")
	fmt.Printf("System Agent Response: %s\n\n", resp.TextContent)

	// 2. AWS Toolkit Example
	fmt.Println("--- AWS Toolkit Example ---")
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		awsRegion = "us-east-1"
	}

	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		awsTools := aws.NewAWSTools(awsRegion)

		awsAgent, _ := agent.NewAgent(agent.AgentConfig{
			Model:       openAIModel,
			Tools:       []toolkit.Tool{awsTools},
			Description: "You are a cloud architect assistant. Use AWS tools to manage resources.",
		})

		resp, _ = awsAgent.Run("List my S3 buckets and check if there are any EC2 instances running.")
		fmt.Printf("AWS Agent Response: %s\n\n", resp.TextContent)
	} else {
		fmt.Println("Skipping AWS Toolkit (AWS credentials not set)\n")
	}

	// Note: GCP and Azure toolkits follow the same pattern:
	// gcpTools := gcp.NewGCPTools(projectID)
	// azureTools := azure.NewAzureTools(subscriptionID)
}
