// Example: read/write files confined to a directory with LocalFileSystemTools.
//
//	go run ./cookbook/tools/localfs
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/localfs"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	fs, err := localfs.NewLocalFileSystemTools(localfs.WithTargetDirectory("agent_files"), localfs.WithDefaultExtension("md"))
	if err != nil {
		log.Fatal(err)
	}
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "Note Taker",
		Model:         model,
		Tools:         []toolkit.Tool{fs},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("Write a short note called 'ideas' with three project ideas, then read it back to me.", true, true)
}
