package xiaomi

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"

	"github.com/devalexandre/agno-golang/agno/models/internal/presettest"
)

func TestPreset(t *testing.T) {
	presettest.Run(t, presettest.Case{
		New:            NewMiMoChat,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		DefaultBaseURL: DefaultBaseURL,
	})
}

func TestUseThinking(t *testing.T) {
	rec := presettest.Install(t)
	m, err := NewMiMoChat(models.WithAPIKey("k"), models.WithClientParams(map[string]interface{}{ParamUseThinking: false}))
	if err != nil {
		t.Fatal(err)
	}
	req := presettest.Invoke(t, rec, m)
	thinking, _ := req.Body["thinking"].(map[string]interface{})
	if thinking["type"] != "disabled" {
		t.Fatalf("thinking = %v, want disabled", req.Body["thinking"])
	}
}
