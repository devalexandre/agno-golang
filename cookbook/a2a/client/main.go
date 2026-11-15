// A2A client: call a remote A2A agent, stream its answer, and hand it to a
// local agent as a tool.
//
// What it shows
//   - a2a.Client: agent card discovery (protocol version negotiated from the
//     card), SendMessage, StreamMessage, multi-turn via the context id
//   - a2a.RemoteAgent.AsTool: a local agent delegates to the remote agent
//   - an input-required task is continued by sending the resolved
//     requirements back with the same task id (see SendOptions.TaskID/Data)
//
// Try it
//
//	go run ./cookbook/a2a/server     # in another terminal
//	A2A_AGENT_URL=http://localhost:7778/a2a/agents/<id> go run ./cookbook/a2a/client
//
// Requires: A2A_AGENT_URL (any A2A v1.0 or v0.3 agent), and a local Ollama
// (OLLAMA_HOST / OLLAMA_MODEL) for the delegating agent.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/a2a"
	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	ctx := context.Background()
	url := os.Getenv("A2A_AGENT_URL")
	if url == "" {
		log.Fatal("set A2A_AGENT_URL, e.g. http://localhost:7778/a2a/agents/<id>")
	}
	client := a2a.NewClient(url)
	card, err := client.GetAgentCard(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Remote agent: %s - %s\n", card.Name, card.Description)

	// 1. Blocking call.
	res, err := client.SendMessage(ctx, "Tell me a joke about Go.", a2a.SendOptions{UserID: "demo-user"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[%s] %s\n", res.Status, res.Content)
	if res.IsInputRequired() {
		// A paused run: resolve its requirements and send them back.
		data, _ := res.Data.(map[string]any)
		fmt.Printf("task waits on: %v (answer with SendOptions{TaskID: %q, Data: ...})\n", data, res.TaskID)
	}

	// 2. Streaming, continuing the same conversation (context id).
	fmt.Print("stream: ")
	_, err = client.StreamMessage(ctx, "Another one, shorter.", a2a.SendOptions{ContextID: res.ContextID}, func(ev a2a.StreamEvent) error {
		if ev.EventType == "content" && (ev.Raw == nil || ev.Raw.ArtifactUpdate == nil || !ev.Raw.ArtifactUpdate.LastChunk) {
			fmt.Print(ev.Content)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println()

	// 3. The remote agent as a tool of a local agent.
	remote := a2a.NewRemoteAgent(client, "")
	_ = remote.Refresh(ctx)
	model, err := ollama.NewOllamaChat(models.WithID(env("OLLAMA_MODEL", "llama3.2:latest")), models.WithBaseURL(env("OLLAMA_HOST", "http://localhost:11434")))
	if err != nil {
		log.Fatal(err)
	}
	host, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Host",
		Model:        model,
		Instructions: "When asked for jokes, delegate to the remote joke agent tool.",
		Tools:        []toolkit.Tool{remote.AsTool("", "")},
	})
	if err != nil {
		log.Fatal(err)
	}
	out, err := host.Run("Get me a joke from the joke agent and explain why it is funny.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("host:", out.TextContent)
}
