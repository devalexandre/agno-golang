package azure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestAzureResponses(t *testing.T) {
	var path, query, apiKey, auth string
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		apiKey, auth = r.Header.Get("api-key"), r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = io.WriteString(w, `{"id":"resp_1","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`)
	}))
	defer srv.Close()

	t.Setenv(EnvAPIKey, "az-key")
	t.Setenv(EnvEndpoint, srv.URL+"/")
	t.Setenv(EnvAPIVersion, "")

	m, err := NewAzureResponses(ResponsesConfig{Deployment: "my-deploy"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q", resp.Content)
	}
	if path != "/openai/responses" || query != "api-version="+DefaultResponsesAPIVersion {
		t.Fatalf("path=%q query=%q", path, query)
	}
	if apiKey != "az-key" || auth != "" {
		t.Fatalf("api-key=%q Authorization=%q", apiKey, auth)
	}
	if body["model"] != "my-deploy" {
		t.Fatalf("model = %v", body["model"])
	}
}

func TestAzureResponsesADToken(t *testing.T) {
	var auth, apiKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, apiKey = r.Header.Get("Authorization"), r.Header.Get("api-key")
		_, _ = io.WriteString(w, `{"id":"r","status":"completed","output":[]}`)
	}))
	defer srv.Close()
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvADToken, "ad-token")

	m, err := NewAzureResponsesFromOptions(models.WithID("dep"), models.WithBaseURL(srv.URL),
		models.WithClientParams(map[string]interface{}{"api_version": "2025-03-01-preview"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "hi"}}); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer ad-token" || apiKey != "" {
		t.Fatalf("Authorization=%q api-key=%q", auth, apiKey)
	}
}

func TestAzureResponsesRequiresDeployment(t *testing.T) {
	if _, err := NewAzureResponses(ResponsesConfig{APIKey: "k", Endpoint: "http://x"}); err == nil {
		t.Fatal("expected error")
	}
}
