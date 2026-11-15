// Example: sending email through Amazon SES with the AWS SES toolkit.
//
// Uses the default AWS credential chain; the sender address must be verified in SES.
//
//	go run ./cookbook/tools/awsses
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/awsses"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	ses := awsses.New(
		awsses.WithSenderEmail("sender@example.com"),
		awsses.WithSenderName("Agno Go"),
		awsses.WithRegion("us-east-1"),
	)

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Email Assistant",
		Model:         model,
		Instructions:  "You write and send concise plain-text emails with Amazon SES.",
		Tools:         []toolkit.Tool{ses},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Email receiver@example.com a short summary of what AWS SES is.", true, true)
}
