package tools

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// SleepTool provides a delay/pause capability useful for rate limiting and timed waits.
type SleepTool struct {
	toolkit.Toolkit
}

// SleepParams defines the parameters for the Sleep method.
type SleepParams struct {
	Seconds float64 `json:"seconds" description:"Number of seconds to sleep (supports decimals, e.g. 0.5 for 500ms)." required:"true"`
}

// NewSleepTool creates a new Sleep tool.
func NewSleepTool() *SleepTool {
	t := &SleepTool{}

	tk := toolkit.NewToolkit()
	tk.Name = "SleepTool"
	tk.Description = "Pause execution for a specified number of seconds."

	t.Toolkit = tk
	t.Toolkit.Register("Sleep", "Pause execution for the specified number of seconds.", t, t.Sleep, SleepParams{})

	return t
}

// Sleep pauses execution for the specified number of seconds.
func (t *SleepTool) Sleep(params SleepParams) (interface{}, error) {
	if params.Seconds <= 0 {
		return nil, fmt.Errorf("seconds must be greater than 0")
	}
	if params.Seconds > 300 {
		return nil, fmt.Errorf("sleep duration cannot exceed 300 seconds")
	}

	duration := time.Duration(params.Seconds * float64(time.Second))
	time.Sleep(duration)

	return map[string]interface{}{
		"slept_seconds": params.Seconds,
		"message":       fmt.Sprintf("Slept for %.2f seconds.", params.Seconds),
	}, nil
}

// Execute implements the toolkit.Tool interface.
func (t *SleepTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
