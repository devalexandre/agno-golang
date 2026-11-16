package internlm

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models/internal/presettest"
)

func TestPreset(t *testing.T) {
	presettest.Run(t, presettest.Case{
		New:            NewInternLMChat,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		DefaultBaseURL: DefaultBaseURL,
	})
}
