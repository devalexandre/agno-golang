package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
)

// ErrRunPaused is matched (errors.Is) by the error RunStream returns when the
// run paused on a human-in-the-loop tool. Use errors.As with *RunPausedError
// to get the paused response and resolve its requirements.
var ErrRunPaused = errors.New("run paused waiting for human-in-the-loop resolution")

// RunPausedError carries the PAUSED response of a streamed run.
type RunPausedError struct {
	Response models.RunResponse
}

func (e *RunPausedError) Error() string {
	return fmt.Sprintf("run %s paused: %d requirement(s) need resolution", e.Response.RunID, len(e.Response.ActiveRequirements()))
}

// Is makes errors.Is(err, ErrRunPaused) true.
func (e *RunPausedError) Is(target error) bool { return target == ErrRunPaused }

// RunStream streams the model response to fn.
//
// When a tool needing confirmation, user input or external execution is
// called, the stream stops and RunStream returns a *RunPausedError (matching
// ErrRunPaused); resolve its requirements and call ContinueRun. Use
// RunStreamWithResponse to pass run options or get the full response.
func (a *Agent) RunStream(prompt string, fn func([]byte) error) error {
	resp, err := a.RunStreamWithResponse(prompt, fn)
	if err != nil {
		return err
	}
	if resp.IsPaused() {
		return &RunPausedError{Response: resp}
	}
	return nil
}

// RunStreamWithResponse streams the model response to fn and returns the run
// response. It accepts the same run options as Run (WithSessionID,
// WithUserID, WithRunID, ...). A run that hits a human-in-the-loop tool
// returns a response with Status PAUSED and its Requirements; continue it with
// ContinueRun. The paused state is kept in memory, and in the session DB when
// the agent has a DB and a session ID.
func (a *Agent) RunStreamWithResponse(prompt string, fn func([]byte) error, opts ...interface{}) (models.RunResponse, error) {
	options := applyRunOptions(opts)
	return a.executeRun(options, func() (models.RunResponse, error) {
		return a.runStream(prompt, fn, options)
	})
}

func (a *Agent) runStream(prompt string, fn func([]byte) error, options *RunOptions) (models.RunResponse, error) {
	if options.SessionID != nil {
		a.sessionID = *options.SessionID
	}
	if options.UserID != nil {
		a.userID = *options.UserID
	}
	if a.currentRun != nil {
		a.currentRun.prompt = prompt
	}

	messages := a.prepareMessages(prompt, nil)

	var fullResponse strings.Builder
	callOpts := []models.Option{
		models.WithTools(a.tools),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fullResponse.Write(chunk)
			if fn == nil {
				return nil
			}
			return fn(chunk)
		}),
	}
	if len(a.modelOptions) > 0 {
		callOpts = append(callOpts, a.modelOptions...)
	}

	// Record tool calls so the run can pause on HITL tools, as Run does.
	hitlRec := a.beginHITL()
	defer a.endHITL(hitlRec)

	err := a.model.InvokeStream(hitlRec.ctx, messages, callOpts...)
	if hitlRec.interrupted() {
		return a.handleInterrupt(hitlRec, prompt, messages)
	}
	if err != nil {
		return models.RunResponse{}, err
	}

	responseContent := fullResponse.String()
	if shown := hitlRec.shownText(); shown != "" && responseContent == "" {
		responseContent = shown
	}

	if a.db != nil {
		if saveErr := a.saveRun(prompt, responseContent, messages); saveErr != nil && a.debug {
			fmt.Printf("Warning: Failed to save run: %v\n", saveErr)
		}
	}
	if a.memory != nil {
		if memErr := a.processMemories(prompt, responseContent); memErr != nil && a.debug {
			fmt.Printf("Warning: Failed to process memories: %v\n", memErr)
		}
	}
	if a.addHistoryToMessages {
		a.messages = append(a.messages,
			models.Message{Role: "user", Content: prompt},
			models.Message{Role: "assistant", Content: responseContent},
		)
		a.trimHistory()
	}

	hitlRec.mu.Lock()
	executed := append([]*models.ToolExecution(nil), hitlRec.executed...)
	hitlRec.mu.Unlock()

	return models.RunResponse{
		TextContent:    responseContent,
		ContentType:    "text",
		Event:          "RunResponse",
		Messages:       []models.Message{{Role: models.TypeAssistantRole, Content: responseContent}},
		RunID:          a.runID(),
		SessionID:      a.sessionID,
		Model:          a.model.GetID(),
		ToolExecutions: executed,
		CreatedAt:      time.Now().Unix(),
	}, nil
}
