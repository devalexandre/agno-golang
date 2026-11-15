package ramp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/internal/presettest"
)

const reply = `{"id":"resp_1","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"pong"}]}]}`

// install answers every request with a Responses payload via the default
// transport, so the real default base URL is asserted without network access.
func install(t *testing.T) *presettest.Recorder {
	rec := presettest.Install(t)
	rec.Reply = reply
	return rec
}

func TestDefaults(t *testing.T) {
	rec := install(t)
	t.Setenv(EnvAPIKey, "ramp-key")
	m, err := NewRampRouter(models.WithClientParams(map[string]interface{}{"allow_flex_tier": true}))
	if err != nil {
		t.Fatal(err)
	}
	if m.GetID() != DefaultModel {
		t.Fatalf("GetID = %q", m.GetID())
	}
	req := presettest.Invoke(t, rec, m)
	if req.URL != DefaultBaseURL+"/responses" {
		t.Fatalf("URL = %q", req.URL)
	}
	if req.Header.Get("Authorization") != "Bearer ramp-key" {
		t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
	}
	if req.Body["model"] != DefaultModel || req.Body["allow_flex_tier"] != true {
		t.Fatalf("body = %v", req.Body)
	}
	if _, ok := req.Body["store"]; ok {
		t.Fatalf("store must be left unset")
	}
}

func TestModelsCandidatesDropModel(t *testing.T) {
	rec := install(t)
	m, err := NewRampRouter(models.WithAPIKey("k"), models.WithClientParams(map[string]interface{}{
		"models": []string{"openai:gpt-5-nano", "anthropic:claude-sonnet-4-5"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Invoke(context.Background(), []models.Message{{Role: models.TypeUserRole, Content: "x"}}); err != nil {
		t.Fatal(err)
	}
	body := rec.Last(t).Body
	if _, ok := body["model"]; ok {
		raw, _ := json.Marshal(body)
		t.Fatalf("model must be omitted when models is set: %s", raw)
	}
	if body["models"] == nil {
		t.Fatalf("models missing")
	}
}

func TestMissingKey(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	if _, err := NewRampRouter(); err == nil {
		t.Fatal("expected error")
	}
}
