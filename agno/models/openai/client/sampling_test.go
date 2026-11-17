package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func chatRequestBody(t *testing.T, options ...models.Option) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","object":"chat.completion","created":0,"model":"m",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	// A router or gateway serving a Claude 5 model in the OpenAI format.
	c, err := NewClient(models.WithID("anthropic/claude-opus-5"), models.WithAPIKey("k"), models.WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateChatCompletion(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "hi"}}, options...); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestNoSamplingParametersUnlessSet(t *testing.T) {
	body := chatRequestBody(t)
	for _, field := range []string{"temperature", "top_p", "frequency_penalty", "presence_penalty"} {
		if _, ok := body[field]; ok {
			t.Errorf("request sent %s = %v without the caller setting it", field, body[field])
		}
	}
}

func TestSamplingParametersTheCallerSetAreSent(t *testing.T) {
	body := chatRequestBody(t, models.WithTemperature(0.5), models.WithTopP(0.9))
	if temp, ok := body["temperature"].(float64); !ok || float32(temp) != 0.5 {
		t.Fatalf("temperature = %v, want 0.5", body["temperature"])
	}
	if topP, ok := body["top_p"].(float64); !ok || float32(topP) != 0.9 {
		t.Fatalf("top_p = %v, want 0.9", body["top_p"])
	}
}
