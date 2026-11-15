package aimlapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestDefaultsApplied(t *testing.T) {
	t.Setenv(EnvAPIKey, "env-key")
	opts := &models.ClientOptions{}
	for _, o := range buildOptions() {
		o(opts)
	}
	if opts.BaseURL != DefaultBaseURL || opts.ID != DefaultModel || opts.APIKey != "env-key" {
		t.Fatalf("defaults not applied: base=%q id=%q key=%q", opts.BaseURL, opts.ID, opts.APIKey)
	}
}

func TestRequestUsesEnvKeyAndDefaultModel(t *testing.T) {
	t.Setenv(EnvAPIKey, "env-key")
	var gotAuth, gotModel, gotPath, gotTitle, gotSource string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotTitle = r.Header.Get("X-Title")
		gotSource = r.Header.Get("X-AIMLAPI-Source")
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel, _ = body["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	m, err := NewAIMLAPIChat(models.WithBaseURL(srv.URL), models.WithDefaultHeaders(http.Header{"x-title": {"MyApp"}}))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hi" {
		t.Fatalf("unexpected content %q", resp.Content)
	}
	if gotAuth != "Bearer env-key" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotModel != DefaultModel {
		t.Errorf("model = %q, want %q", gotModel, DefaultModel)
	}
	if gotTitle != "MyApp" {
		t.Errorf("caller X-Title should win, got %q", gotTitle)
	}
	if gotSource != "agent/agno" {
		t.Errorf("attribution header missing, got %q", gotSource)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q", gotPath)
	}
}
