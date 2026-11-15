// Example: structured questions to the user with UserFeedbackTools.
//
// Env vars: none (no API key needed). Requires a local Ollama server with
// llama3.2 at http://localhost:11434.
//
// AskUser stops the tool loop (StopAfterCall); the app reads the questions
// with PendingQuestions, asks the user, and sends the answers back. Native
// HITL "user input" pausing is being added to the agent separately.
//
//	go run ./cookbook/tools/userfeedback
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/userfeedback"
)

func main() {
	ctx := context.Background()
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	uf := userfeedback.NewUserFeedbackTools()
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Trip Planner",
		Model:         model,
		Instructions:  "Help plan trips. Ask the user to choose before planning.\n" + uf.Instructions,
		Tools:         []toolkit.Tool{uf},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Plan a weekend trip for me in Europe.", true, true)

	questions := uf.PendingQuestions()
	if len(questions) == 0 {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	var answers []string
	for _, q := range questions {
		fmt.Printf("\n[%s] %s\n", q.Header, q.Question)
		for i, o := range q.Options {
			fmt.Printf("  %d) %s %s\n", i+1, o.Label, o.Description)
		}
		fmt.Print("> ")
		line, _ := reader.ReadString('\n')
		answers = append(answers, fmt.Sprintf("%s: %s", q.Header, strings.TrimSpace(line)))
	}
	uf.ClearPending()
	ag.PrintResponse("My choices: "+strings.Join(answers, "; "), true, true)
}
