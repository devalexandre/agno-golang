package gemini_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/google/gemini"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// TestInvokeStream
func TestInvokeStream(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. GEMINI_API_KEY is not set.")
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Tell me the current temperature in Pocos de Caldas - MG.",
	}

	var response string
	callOPtions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]toolkit.Tool{
			tools.NewWeatherTool(),
		}),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fmt.Println("Chunk received:", string(chunk))
			response += string(chunk)
			return nil
		}),
	}

	optsClient := []gemini.OptionClient{
		gemini.WithID("gemini-2.5-pro-exp-03-25"),
		gemini.WithAPIKey(apiKey),
	}

	ge, err := gemini.NewGemini(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create Gemini client: %v", err)
	}

	err = ge.InvokeStream(context.Background(), []models.Message{message}, callOPtions...)
	if err != nil {
		t.Fatalf("Failed to create chat completion: %v", err)
	}

	// Check the response
	fmt.Println(response)
}
