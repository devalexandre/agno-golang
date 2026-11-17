package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

// requestBody runs one completion against a fake Messages API and returns
// the JSON body the client sent.
func requestBody(t *testing.T, options ...models.Option) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"m",` +
			`"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	model, err := NewAnthropic(models.WithID("claude-opus-5"), models.WithAPIKey("k"), models.WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "hi"}}, options...); err != nil {
		t.Fatal(err)
	}
	return body
}

// Claude models from the 5 family reject temperature and top_p, so nothing
// the caller did not set may be sent.
func TestNoSamplingParametersUnlessSet(t *testing.T) {
	body := requestBody(t)
	for _, field := range []string{"temperature", "top_p", "top_k"} {
		if _, ok := body[field]; ok {
			t.Errorf("request sent %s = %v without the caller setting it", field, body[field])
		}
	}
}

func TestSamplingParametersTheCallerSetAreSent(t *testing.T) {
	body := requestBody(t, models.WithTemperature(0.2))
	// The option is a float32, so 0.2 arrives as 0.20000000298...
	if temp, ok := body["temperature"].(float64); !ok || float32(temp) != 0.2 {
		t.Fatalf("temperature = %v, want 0.2", body["temperature"])
	}
	if _, ok := body["top_p"]; ok {
		t.Fatalf("top_p sent without being set: %v", body["top_p"])
	}
}
