package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/fatih/color"
)

func main() {
	ctx := context.Background()

	fmt.Println("\n🔀 ChainTool with Advanced Conditions")
	fmt.Println(strings.Repeat("=", 70))

	validateEmailTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			isValid := strings.Contains(input, "@") && strings.Contains(input, ".")
			result := fmt.Sprintf("valid:%v", isValid)
			status := "✓"
			if !isValid {
				status = "✗"
			}
			color.Yellow("   [1] VALIDATE_EMAIL %s: %q → %s\n", status, input, result)
			return result, nil
		},
		"Validate email format and return valid:true or valid:false",
	)

	normalizeEmailTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			result := strings.ToLower(strings.TrimSpace(input))
			color.Yellow("   [2] NORMALIZE_EMAIL: %q → %q\n", input, result)
			return result, nil
		},
		"Normalize email to lowercase and trimmed",
	)

	extractDomainTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			parts := strings.Split(input, "@")
			domain := "unknown"
			if len(parts) == 2 {
				domain = parts[1]
			}
			color.Yellow("   [3] EXTRACT_DOMAIN: %q → %q\n", input, domain)
			return domain, nil
		},
		"Extract domain from email",
	)

	verifyDomainTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			isCommon := strings.Contains(input, "gmail") || strings.Contains(input, "outlook") || strings.Contains(input, "yahoo")
			result := fmt.Sprintf("domain_type:%s", map[bool]string{true: "common", false: "custom"}[isCommon])
			color.Yellow("   [4] VERIFY_DOMAIN: %q → %s\n", input, result)
			return result, nil
		},
		"Verify if domain is common provider or custom",
	)

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		fmt.Printf("error: %v\n", err.Error())
		return
	}

	agnt, err := agent.NewAgent(agent.AgentConfig{
		Context:         ctx,
		Model:           model,
		Name:            "EmailProcessor",
		Description:     "Process and validate email addresses with conditional steps",
		Instructions:    "1. First validate the email format. 2. If valid, normalize it. 3. If normalized, extract domain. 4. Then verify the domain type.",
		Tools:           []toolkit.Tool{validateEmailTool, normalizeEmailTool, extractDomainTool, verifyDomainTool},
		EnableChainTool: true,
	})
	if err != nil {
		fmt.Printf("   ❌ Error creating agent: %v\n", err)
		return
	}

	testEmails := []string{
		"user@gmail.com",
		"  ADMIN@EXAMPLE.COM  ",
		"invalid.email",
		"test@company.co.uk",
	}

	for _, email := range testEmails {
		fmt.Printf("\n📧 Processing: %q\n", email)
		fmt.Println(strings.Repeat("-", 70))

		prompt := fmt.Sprintf("Process this email address: %q. Validate it, normalize it if valid, extract the domain, and determine if it's a common provider.", email)
		response, err := agnt.Run(prompt)
		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
			continue
		}

		fmt.Printf("   🤖 Result: %q\n", response.TextContent)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("✅ Email Processing with Conditional Execution Complete!")
}
