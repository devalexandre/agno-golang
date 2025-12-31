package main

import (
	"context"
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	likeopenai "github.com/devalexandre/agno-golang/agno/models/openai/like"
)

func main() {
	apiKey := os.Getenv("VLLM_API_KEY")
	if apiKey == "" {
		apiKey = "" // fallback for demo
	}
	baseURL := "https://z2bg1juojbhurv-8000.proxy.runpod.net/v1"
	modelID := "EssentialAI/rnj-1-instruct"

	client, err := likeopenai.NewLikeOpenAIChat(
		func(opts *models.ClientOptions) {
			opts.APIKey = apiKey
			opts.BaseURL = baseURL
			opts.ID = modelID
		},
	)
	if err != nil {
		panic(err)
	}

	messages := []models.Message{
		{Role: models.TypeUserRole, Content: "What is the capital of France?"},
	}

	resp, err := client.Invoke(context.Background(), messages)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Response: %s\n", resp.Content)
}
