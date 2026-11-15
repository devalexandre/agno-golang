// Example: sending email with the Resend toolkit.
//
// Requires RESEND_API_KEY (and a verified sender domain).
//
//	go run ./cookbook/tools/resend
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/resend"
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

	mail := resend.New(resend.WithFromEmail("onboarding@resend.dev"))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Mailer",
		Model:         model,
		Instructions:  "You send short, friendly HTML emails with Resend.",
		Tools:         []toolkit.Tool{mail},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Send an email to delivered@resend.dev with subject 'Hello' saying hi from Agno Go.", true, true)
}
