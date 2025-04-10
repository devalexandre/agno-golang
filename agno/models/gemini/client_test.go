package gemini_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/gemini"
	"github.com/devalexandre/agno-golang/agno/tools"
)

func TestCreateChatCompletion(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. GEMINI_API_KEY is not set.")
	}

	optsClient := []gemini.OptionClient{
		gemini.WithModel("gemini-2.0-flash-lite"),
		gemini.WithAPIKey(apiKey),
	}

	// Create a new Gemini client with a test API key
	client, err := gemini.NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create Gemini client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Hello, Gemini!",
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, models.WithTemperature(0.5))
	if err != nil {
		// Check if the error is due to quota limitations
		if strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "rate limit") {
			t.Skipf("Skipping test due to quota limitations: %v", err)
		} else {
			t.Fatalf("Failed to create chat completion: %v", err)
		}
	}

	// Check the response
	t.Logf("Chat completion response: %+v", chatCompletion.Choices[0].Message.Content)
}

// TestCreateChatCompletion with tool
func TestCreateChatCompletionWithTool(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")

	if apiKey == "" {
		t.Skip("Skipping integration test. GEMINI_API_KEY is not set.")
	}

	optsClient := []gemini.OptionClient{
		gemini.WithModel("gemini-2.0-flash-lite"),
		gemini.WithAPIKey(apiKey),
	}

	// Create a new Gemini client with a test API key
	client, err := gemini.NewClient(optsClient...)

	if err != nil {
		t.Fatalf("Failed to create Gemini client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Qual a temperatura de poços de caldas?",
	}

	callOPtions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]tools.Tool{
			tools.NewWeatherTool(),
		}),
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOPtions...)
	if err != nil {
		// Check if the error is due to quota limitations
		if strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "rate limit") {
			t.Skipf("Skipping test due to quota limitations: %v", err)
		} else {
			t.Fatalf("Failed to create chat completion: %v", err)
		}
	}

	// Check the response
	t.Logf("Chat completion response: %+v", chatCompletion.Choices[0].Message.Content)
}

func TestCreateChatCompletionStream(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. GEMINI_API_KEY is not set.")
	}

	optsClient := []gemini.OptionClient{
		gemini.WithModel("gemini-2.0-flash-lite"),
		gemini.WithAPIKey(apiKey),
	}

	// Create a new Gemini client with a test API key
	client, err := gemini.NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create Gemini client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Brasília",
	}

	var response string
	callOPtions := []models.Option{
		models.WithTemperature(0.5),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			t.Logf("Streaming chunk:: %s", string(chunk))
			response += string(chunk)
			return nil
		}),
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOPtions...)
	if err != nil {
		// Check if the error is due to quota limitations
		if strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "rate limit") {
			t.Skipf("Skipping test due to quota limitations: %v", err)
		} else {
			t.Fatalf("Failed to create chat completion: %v", err)
		}
	}

	// Check the response
	t.Logf("Chat completion response: %+v", chatCompletion.Choices[0].Message.Content)
}

func TestCreateChatCompletionWithTools(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. GEMINI_API_KEY is not set.")
	}

	optsClient := []gemini.OptionClient{
		gemini.WithModel("gemini-2.0-flash-lite"),
		gemini.WithAPIKey(apiKey),
	}

	// Create a new Gemini client with a test API key
	client, err := gemini.NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create Gemini client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Por favor, use a ferramenta WeatherTool para me dizer qual é a temperatura atual em Poços de Caldas - MG. A ferramenta precisa de latitude e longitude, que são -21.7872 e -46.5614 respectivamente.",
	}

	callOPtions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]tools.Tool{
			tools.NewWeatherTool(),
		}),
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOPtions...)
	if err != nil {
		// Check if the error is due to quota limitations
		if strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "rate limit") {
			t.Skipf("Skipping test due to quota limitations: %v", err)
		} else {
			t.Fatalf("Failed to create chat completion: %v", err)
		}
	}

	// Check the response
	t.Logf("Chat completion response: %+v", chatCompletion.Choices[0].Message.Content)
}

func TestCreateChatCompletionStreamWithTools(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. GEMINI_API_KEY is not set.")
	}

	optsClient := []gemini.OptionClient{
		gemini.WithModel("gemini-2.0-flash-lite"),
		gemini.WithAPIKey(apiKey),
	}

	// Create a new Gemini client with a test API key
	client, err := gemini.NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create Gemini client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Por favor, use a ferramenta WeatherTool para me dizer qual é a temperatura atual em Poços de Caldas - MG. A ferramenta precisa de latitude e longitude, que são -21.7872 e -46.5614 respectivamente.",
	}

	var response string
	callOPtions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]tools.Tool{
			tools.NewWeatherTool(),
		}),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			t.Logf("Streaming chunk:: %s", string(chunk))
			response += string(chunk)
			return nil
		}),
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOPtions...)
	if err != nil {
		// Check if the error is due to quota limitations
		if strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "rate limit") {
			t.Skipf("Skipping test due to quota limitations: %v", err)
		} else {
			t.Fatalf("Failed to create chat completion: %v", err)
		}
	}

	// Check the response
	t.Logf("Chat completion response: %s", chatCompletion.Choices[0].Message.Content)
}
