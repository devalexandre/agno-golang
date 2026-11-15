// Package toolcallstub provides an offline, scripted model for cookbook
// examples. It behaves like the real model clients: on a user turn it emits
// the planned tool calls, executes them through the tools given with
// models.WithTools and asks itself again; once it sees tool results it answers
// with a summary of them. Swap it for a real model (e.g. openai/chat) to run
// the examples against an LLM.
package toolcallstub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
)

// Model is the scripted model.
type Model struct {
	// Calls are the tool calls emitted on the first (user) turn.
	Calls []tools.FunctionCall
	// Answer prefixes the final answer.
	Answer string
}

// New creates a model that calls the given tools (name, JSON args pairs).
func New(answer string, nameArgs ...string) *Model {
	m := &Model{Answer: answer}
	for i := 0; i+1 < len(nameArgs); i += 2 {
		m.Calls = append(m.Calls, tools.FunctionCall{Name: nameArgs[i], Arguments: nameArgs[i+1]})
	}
	return m
}

// Invoke implements models.AgnoModelInterface.
func (m *Model) Invoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (*models.MessageResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	co := &models.CallOptions{}
	for _, o := range opts {
		o(co)
	}
	last := msgs[len(msgs)-1]
	if last.Role == models.TypeToolRole || len(m.Calls) == 0 {
		var results []string
		for _, msg := range msgs {
			if msg.Role == models.TypeToolRole {
				results = append(results, msg.Content)
			}
		}
		content := m.Answer
		if len(results) > 0 {
			content += " Tool results: " + strings.Join(results, "; ")
		} else {
			content += " " + last.Content
		}
		return &models.MessageResponse{Role: "assistant", Model: m.GetID(), Content: content}, nil
	}

	assistant := models.Message{Role: models.TypeAssistantRole}
	var toolMsgs []models.Message
	for i, fc := range m.Calls {
		id := fmt.Sprintf("call_%d", i)
		assistant.ToolCalls = append(assistant.ToolCalls, tools.ToolCall{ID: id, Type: "function", Function: fc})
		content := "Tool " + fc.Name + " not found"
		for _, t := range co.ToolCall {
			if _, ok := t.GetMethods()[fc.Name]; ok {
				res, err := t.Execute(fc.Name, json.RawMessage(fc.Arguments))
				if err != nil {
					content = "Error executing tool: " + err.Error()
				} else {
					content = fmt.Sprint(res)
				}
			}
		}
		toolMsgs = append(toolMsgs, models.Message{Role: models.TypeToolRole, Content: content, ToolCallID: &id})
	}
	next := append(append(append([]models.Message(nil), msgs...), assistant), toolMsgs...)
	return m.Invoke(ctx, next, opts...)
}

// AInvoke is not supported by the stub.
func (m *Model) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}

// InvokeStream is not supported by the stub.
func (m *Model) InvokeStream(context.Context, []models.Message, ...models.Option) error { return nil }

// AInvokeStream is not supported by the stub.
func (m *Model) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}

// GetID implements models.AgnoModelInterface.
func (m *Model) GetID() string { return "toolcall-stub" }
