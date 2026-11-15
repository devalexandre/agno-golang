package langdb

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/internal/presettest"
)

func TestDefaults(t *testing.T) {
	rec := presettest.Install(t)
	t.Setenv(EnvAPIKey, "env-secret")
	t.Setenv(EnvProjectID, "proj")
	t.Setenv(EnvBaseHostURL, "")

	m, err := NewLangDBChat(models.WithClientParams(map[string]interface{}{ParamLabel: "agno"}))
	if err != nil {
		t.Fatal(err)
	}
	req := presettest.Invoke(t, rec, m)
	if req.URL != DefaultBaseHostURL+"/proj/v1/chat/completions" {
		t.Fatalf("URL = %q", req.URL)
	}
	if req.Header.Get("Authorization") != "Bearer env-secret" {
		t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
	}
	if req.Header.Get("X-Label") != "agno" {
		t.Fatalf("x-label = %q", req.Header.Get("X-Label"))
	}
	if req.Body["model"] != DefaultModel {
		t.Fatalf("model = %v", req.Body["model"])
	}
}

func TestMissingProject(t *testing.T) {
	t.Setenv(EnvProjectID, "")
	if _, err := NewLangDBChat(models.WithAPIKey("k")); err == nil {
		t.Fatal("expected error without project id")
	}
}
