// Example: ask the user for missing information with UserControlFlowTools.
//
// Env vars: none (no API key needed). Requires a local Ollama server with
// llama3.2 at http://localhost:11434.
//
// GetUserInput stops the tool loop (StopAfterCall); the app reads the fields
// with PendingFields, collects the values, and sends them back. Native HITL
// "user input" pausing is being added to the agent separately.
//
//	go run ./cookbook/tools/usercontrolflow
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
	"github.com/devalexandre/agno-golang/agno/tools/usercontrolflow"
)

func main() {
	ctx := context.Background()
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	uc := usercontrolflow.NewUserControlFlowTools()
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Email Drafter",
		Model:         model,
		Instructions:  "You draft emails.\n" + uc.Instructions,
		Tools:         []toolkit.Tool{uc},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Draft an email inviting someone to a meeting.", true, true)

	fields := uc.PendingFields()
	if len(fields) == 0 {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	var values []string
	for _, f := range fields {
		fmt.Printf("%s (%s) - %s: ", f.FieldName, f.FieldType, f.FieldDescription)
		line, _ := reader.ReadString('\n')
		values = append(values, fmt.Sprintf("%s=%s", f.FieldName, strings.TrimSpace(line)))
	}
	uc.ClearPending()
	ag.PrintResponse("Here is the information: "+strings.Join(values, ", "), true, true)
}
