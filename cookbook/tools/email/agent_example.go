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
	// Run watch example if --watch flag is passed
	if len(os.Args) > 1 && os.Args[1] == "--watch" {
		watchExample()
		return
	}

	fmt.Println("Email Tool Agent Examples")
	fmt.Println("=========================")

	// Get credentials from environment
	gmailEmail := os.Getenv("GMAIL_EMAIL")
	gmailPassword := os.Getenv("GMAIL_APP_PASSWORD")

	if gmailEmail == "" || gmailPassword == "" {
		log.Fatal("Set GMAIL_EMAIL and GMAIL_APP_PASSWORD environment variables")
	}

	fmt.Printf("Using account: %s\n\n", gmailEmail)

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
	emailTool, err := tools.NewEmailTool(tools.EmailConfig{
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     587,
		SMTPUsername: gmailEmail,
		SMTPPassword: gmailPassword,
		IMAPHost:     "imap.gmail.com",
		IMAPPort:     993,
		IMAPUsername: gmailEmail,
		IMAPPassword: gmailPassword,
		FromAddress:  gmailEmail,
	})
	if err != nil {
		log.Fatalf("Failed to create email tool: %v", err)
	}

	// Create Email Management Agent
	emailAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Name:    "Email Manager",
		Model:   model,
		Instructions: `You are an email management expert. Use the EmailTool methods to:
- SendEmail: Send plain text emails with To, Subject, Body fields
- SendHTMLEmail: Send HTML emails with To, Subject, HTML fields
- ListEmails: List emails from a mailbox with Mailbox, Limit, Unread fields
- SearchEmails: Search emails with Mailbox, Query, Limit fields
- ListMailboxes: List all available mailboxes
- MarkAsRead: Mark emails as read with Mailbox, UID fields
- MarkAsUnread: Mark emails as unread with Mailbox, UID fields
- WatchEmails: Monitor for new unread emails with optional SubjectFilter, SenderFilter, SinceMinutes
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
		"List my inbox emails with limit 5",
		"Search for emails with 'Test' in the subject",
		"Watch for new unread emails from the last 30 minutes",
		"List all available mailboxes",
	}

	for i, task := range tasks {
		fmt.Printf("\nTask %d: %s\n", i+1, task)
		fmt.Println("--------------------")

		response, err := emailAgent.Run(task)
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}

		fmt.Println("Response:")
		fmt.Println(response.TextContent)
		fmt.Println()
	}

	fmt.Println("\nEmail Tool Examples Completed!")
}
