// Package hitltest provides a real agent.Agent with an offline stub model and
// a confirmation-gated tool, for end-to-end human-in-the-loop tests of the
// AgentOS protocol interfaces.
package hitltest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// ToolName is the full name of the confirmation-gated method.
const ToolName = "Files_Delete"

// Model calls Files_Delete on the user turn (executing it through the tools
// it was given, like real model clients) and answers "final: <tool result>"
// once it sees the tool result.
type Model struct{}

func (Model) Invoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (*models.MessageResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if last := msgs[len(msgs)-1]; last.Role == models.TypeToolRole {
		return &models.MessageResponse{Role: "assistant", Model: "stub", Content: "final: " + last.Content}, nil
	}
	co := &models.CallOptions{}
	for _, o := range opts {
		o(co)
	}
	fc := tools.FunctionCall{Name: ToolName, Arguments: `{"path":"/tmp/x"}`}
	id := "call_1"
	content := "tool not found"
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
	next := append(append([]models.Message(nil), msgs...),
		models.Message{Role: models.TypeAssistantRole, ToolCalls: []tools.ToolCall{{ID: id, Type: "function", Function: fc}}},
		models.Message{Role: models.TypeToolRole, Content: content, ToolCallID: &id})
	return Model{}.Invoke(ctx, next, opts...)
}

func (Model) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (Model) InvokeStream(context.Context, []models.Message, ...models.Option) error { return nil }
func (Model) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (Model) GetID() string { return "stub" }

// DeleteParams are the parameters of Files_Delete.
type DeleteParams struct {
	Path string `json:"path" description:"Path to delete" required:"true"`
}

// Files is a toolkit whose Delete method requires confirmation.
type Files struct {
	toolkit.Toolkit
	mu      sync.Mutex
	deleted []string
}

// Delete records the deleted path.
func (f *Files) Delete(p DeleteParams) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, p.Path)
	return "deleted " + p.Path, nil
}

// Deleted returns the paths deleted so far.
func (f *Files) Deleted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleted...)
}

// NewAgent builds an agent whose only tool is the confirmation-gated
// Files_Delete, driven by Model.
func NewAgent(name string) (*agent.Agent, *Files, error) {
	f := &Files{Toolkit: toolkit.NewToolkit()}
	f.Name, f.Description = "Files", "File operations"
	f.RegisterWithOptions("Delete", "Delete a file", f, f.Delete, DeleteParams{}, toolkit.WithConfirmation())
	a, err := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Name: name, Model: Model{}, Tools: []toolkit.Tool{f}})
	return a, f, err
}
