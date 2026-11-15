package cloudflare

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/internal/presettest"
)

func TestDefaults(t *testing.T) {
	rec := presettest.Install(t)
	t.Setenv(EnvAPIKey, "cf-token")
	t.Setenv(EnvAccountID, "acct")
	t.Setenv(EnvGatewayID, "")

	m, err := NewCloudflareChat()
	if err != nil {
		t.Fatal(err)
	}
	if m.GetID() != "workers-ai/"+DefaultModel {
		t.Fatalf("GetID = %q", m.GetID())
	}
	req := presettest.Invoke(t, rec, m)
	if req.URL != "https://gateway.ai.cloudflare.com/v1/acct/default/compat/chat/completions" {
		t.Fatalf("URL = %q", req.URL)
	}
	if req.Header.Get("Authorization") != "Bearer cf-token" {
		t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
	}
	if req.Body["model"] != "workers-ai/"+DefaultModel {
		t.Fatalf("model = %v", req.Body["model"])
	}
}

func TestGatewayParamAndModel(t *testing.T) {
	rec := presettest.Install(t)
	m, err := NewCloudflareChat(
		models.WithAPIKey("k"),
		models.WithID("openai/gpt-5.4-mini"),
		models.WithClientParams(map[string]interface{}{ParamAccountID: "a1", ParamGatewayID: "gw"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := presettest.Invoke(t, rec, m)
	if req.URL != "https://gateway.ai.cloudflare.com/v1/a1/gw/compat/chat/completions" {
		t.Fatalf("URL = %q", req.URL)
	}
	if req.Body["model"] != "openai/gpt-5.4-mini" {
		t.Fatalf("model = %v", req.Body["model"])
	}
}

func TestMissingAccount(t *testing.T) {
	t.Setenv(EnvAccountID, "")
	if _, err := NewCloudflareChat(models.WithAPIKey("k")); err == nil {
		t.Fatal("expected error without account id")
	}
}
