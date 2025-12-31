package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	fmt.Println("🚀 Email Tool Agent Examples")
	fmt.Println("=============================")

	// Get credentials from environment
	gmailEmail := os.Getenv("GMAIL_EMAIL")
	gmailPassword := os.Getenv("GMAIL_APP_PASSWORD")

	if gmailEmail == "" || gmailPassword == "" {
		log.Fatal("❌ Set GMAIL_EMAIL and GMAIL_APP_PASSWORD environment variables")
	}

	fmt.Printf("📧 Using account: %s\n\n", gmailEmail)

	ctx := context.Background()

	// Initialize Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Configure and initialize Email Tool
	emailConfig := tools.EmailConfig{
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     587,
		SMTPUsername: gmailEmail,
		SMTPPassword: gmailPassword,
		IMAPHost:     "imap.gmail.com",
		IMAPPort:     993,
		IMAPUsername: gmailEmail,
		IMAPPassword: gmailPassword,
		FromAddress:  gmailEmail,
	}

	emailTool := tools.NewEmailTool(emailConfig)

	// Create Email Management Agent
	emailAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Name:    "Email Manager",
		Model:   model,
		Instructions: `You are an email management expert. Use the EmailTool methods to:
- send_email: Send plain text emails with To, Subject, Body fields
- send_html_email: Send HTML emails with To, Subject, HTML fields
- list_emails: List emails from a mailbox with Mailbox, Limit, Unread fields
- search_emails: Search emails with Mailbox, Query, Limit fields
- list_mailboxes: List all available mailboxes
- mark_as_read: Mark emails as read with Mailbox, UID fields
- mark_as_unread: Mark emails as unread with Mailbox, UID fields
Always use the correct field names as specified.`,
		Tools:         []toolkit.Tool{emailTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Example tasks
	tasks := []string{
		fmt.Sprintf("Send a plain text email to %s with subject 'Test Email from Agno Agent' and body 'Hello! This is sent from Agno Email Agent'", gmailEmail),
		fmt.Sprintf("Send an HTML email to %s with subject 'HTML Email from Agent' and HTML content with a nice formatted message", gmailEmail),
		"List my inbox emails with limit 5",
		"Search for emails with 'Test' in the subject",
		"List all available mailboxes",
	}

	for i, task := range tasks {
		fmt.Printf("\n📧 Task %d: %s\n", i+1, task)
		fmt.Println(string([]byte{45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45}))

		response, err := emailAgent.Run(task)
		if err != nil {
			log.Printf("❌ Error: %v\n", err)
			continue
		}

		fmt.Println("✅ Response:")
		fmt.Println(response.TextContent)
		fmt.Println()
	}

	fmt.Println("\n✅ Email Tool Examples Completed!")
}
