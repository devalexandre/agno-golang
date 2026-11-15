package deepinfra

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models/internal/presettest"
)

func TestPreset(t *testing.T) {
	presettest.Run(t, presettest.Case{
		New:            NewDeepInfraChat,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		DefaultBaseURL: DefaultBaseURL,
	})
}
