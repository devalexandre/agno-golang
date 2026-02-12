package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// This example demonstrates how to use WatchEmails to monitor
// incoming emails and trigger automated actions based on subject or sender.
//
// Usage:
//   GMAIL_EMAIL=you@gmail.com GMAIL_APP_PASSWORD=xxxx go run ./cookbook/tools/email/ --watch
//
// The agent will poll the inbox every 2 minutes looking for new unread
// emails and decide what to do based on content.

func watchExample() {
	email := os.Getenv("GMAIL_EMAIL")
	password := os.Getenv("GMAIL_APP_PASSWORD")

	if email == "" || password == "" {
		log.Fatal("Set GMAIL_EMAIL and GMAIL_APP_PASSWORD environment variables")
	}

	ctx := context.Background()

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	emailTool, err := tools.NewEmailTool(tools.EmailConfig{
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     587,
		SMTPUsername: email,
		SMTPPassword: password,
		IMAPHost:     "imap.gmail.com",
		IMAPPort:     993,
		IMAPUsername: email,
		IMAPPassword: password,
		FromAddress:  email,
	})
	if err != nil {
		log.Fatalf("Failed to create email tool: %v", err)
	}

	// --- Example 1: One-shot watch with filters ---
	fmt.Println("Example 1: One-shot WatchEmails with filters")
	fmt.Println("=============================================")

	watchAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Name:    "Email Watcher",
		Model:   model,
		Instructions: `You are an email monitoring assistant.

When asked to check emails, use WatchEmails with the appropriate filters.
After checking, summarize what you found:
- How many new emails
- List each email: sender, subject, short preview
- Suggest actions for each (reply, archive, forward, etc.)`,
		Tools:         []toolkit.Tool{emailTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Watch for all unread emails in the last 2 hours
	response, err := watchAgent.Run("Check for new unread emails from the last 2 hours and summarize them")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Println(response.TextContent)
	}

	fmt.Println()

	// Watch only for emails from a specific sender
	response, err = watchAgent.Run("Check for new emails from noreply@github.com in the last 24 hours")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Println(response.TextContent)
	}

	fmt.Println()

	// Watch only for emails with a specific subject
	response, err = watchAgent.Run("Check for new emails with 'invoice' or 'payment' in the subject from the last 12 hours")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Println(response.TextContent)
	}

	// --- Example 2: Automated polling loop ---
	fmt.Println()
	fmt.Println("Example 2: Polling loop (press Ctrl+C to stop)")
	fmt.Println("===============================================")

	triggerAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Name:    "Email Trigger",
		Model:   model,
		Instructions: fmt.Sprintf(`You are an email automation agent.

Every time you are asked to check emails, use WatchEmails to look for new unread emails.

Based on what you find, take the following actions:
1. If subject contains "urgent" or "asap" -> Reply with an acknowledgment email using SendEmail
2. If from contains "boss" or "manager" -> Mark as read and summarize the content
3. If subject contains "unsubscribe" -> Just mark as read
4. For anything else -> Just report the email details

Always use SendEmail to %s when replying.
Always use MarkAsRead after processing an email.`, email),
		Tools:         []toolkit.Tool{emailTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Poll every 2 minutes for new emails
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	// Run once immediately
	fmt.Printf("[%s] Checking for new emails...\n", time.Now().Format("15:04:05"))
	response, err = triggerAgent.Run("Check for new unread emails from the last 5 minutes and process them according to the rules")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Println(response.TextContent)
	}

	// Then poll on the ticker
	for range ticker.C {
		fmt.Printf("\n[%s] Checking for new emails...\n", time.Now().Format("15:04:05"))
		response, err = triggerAgent.Run("Check for new unread emails from the last 5 minutes and process them according to the rules")
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}
		fmt.Println(response.TextContent)
	}
}
