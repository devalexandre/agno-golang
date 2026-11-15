// Example: query an EVM chain and send ETH with EvmTools.
//
// Requires EVM_RPC_URL and EVM_PRIVATE_KEY (use a testnet wallet!).
// SendTransaction moves funds and requires user confirmation.
//
//	go run ./cookbook/tools/evm
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/evm"
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

	tk := evm.NewEvmTools(evm.WithEVMAll())

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Wallet Agent",
		Model:         model,
		Instructions:  "You manage a testnet wallet. Check balances before sending and never send without being asked.",
		Tools:         []toolkit.Tool{tk},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("What is the balance of my wallet?", true, true)
}
