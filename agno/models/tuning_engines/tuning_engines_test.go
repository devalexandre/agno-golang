package tuningengines

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models/internal/presettest"
)

func TestPreset(t *testing.T) {
	presettest.Run(t, presettest.Case{
		New:            NewTuningEnginesChat,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		DefaultBaseURL: DefaultBaseURL,
	})
}

func TestEnvBaseURL(t *testing.T) {
	rec := presettest.Install(t)
	t.Setenv(EnvBaseURL, "http://tuning.invalid/v2")
	m, err := NewTuningEnginesChat()
	if err != nil {
		t.Fatal(err)
	}
	if got := presettest.Invoke(t, rec, m).URL; got != "http://tuning.invalid/v2/chat/completions" {
		t.Fatalf("URL = %q", got)
	}
}
