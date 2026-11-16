// Example: multimodal input — sending an image to a vision-capable model.
//
// Requires the OPENAI_API_KEY environment variable:
//
//	export OPENAI_API_KEY="sk-..."
//	go run ./cookbook/multimodal
//
// The same run options work across providers: each client converts the media into
// the wire format its API expects (OpenAI content parts, Anthropic image blocks,
// Gemini inline data) and drops what it cannot represent. Use
// models.CapabilitiesFor("anthropic") to check what a provider accepts, and
// models.UnsupportedMedia to find out what would be dropped.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/chat"
)

func main() {
	ctx := context.Background()

	model, err := chat.NewOpenAIChat(models.WithID("gpt-4o"))
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "VisionAssistant",
		Instructions: "You describe images accurately and concisely.",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Images can be referenced by URL, which is passed to the provider as-is.
	resp, err := ag.Run(
		"What is in this image? Answer in one sentence.",
		agent.WithImages(agent.Image{
			URL: "https://upload.wikimedia.org/wikipedia/commons/b/bd/Golden_Gate_Bridge_from_Battery_Spencer.jpg",
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("From URL:", resp.TextContent)

	// Local files are sent inline, encoded as base64 by the provider client.
	data, err := os.ReadFile("photo.png")
	if err != nil {
		fmt.Println("(skipping local image: photo.png not found)")
		return
	}

	resp, err = ag.Run(
		"Describe this image.",
		agent.WithImages(agent.Image{Data: data, MimeType: "image/png"}),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("From bytes:", resp.TextContent)
}
