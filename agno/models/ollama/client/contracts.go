package client

import (
	"encoding/json"

	"github.com/devalexandre/agno-golang/agno/tools"
)

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  struct {
		Type       string   `json:"type"`
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type        string   `json:"type"`
			Description string   `json:"description"`
			Enum        []string `json:"enum,omitempty"`
		} `json:"properties"`
	} `json:"parameters"`
}

// CompletionChunk represents a streaming response chunk.
type CompletionChunk struct {
	Model     string      `json:"model"`
	CreatedAt string      `json:"created_at"`
	Message   ChatMessage `json:"message"`
	Done      bool        `json:"done"`
}

type CompletionResponse struct {
	Model        string      `json:"model"`
	CreatedAt    string      `json:"created_at"`
	Message      ChatMessage `json:"message"`
	Done         bool        `json:"done"`
	Context      []int       `json:"context"`
	EvalCount    int         `json:"eval_count"`
	EvalTime     int64       `json:"eval_duration"`
	PromptTokens int         `json:"prompt_eval_count"`
	PromptTime   int64       `json:"prompt_eval_duration"`
	TotalTime    int64       `json:"total_duration"`
}

// OllamaFunctionCall é uma versão personalizada de FunctionCall para o Ollama
type OllamaFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// OllamaToolCall is a customized version of ToolCall for Ollama
type OllamaToolCall struct {
	ID       string             `json:"id,omitempty"`
	Type     tools.ToolType     `json:"type"`
	Function OllamaFunctionCall `json:"function,omitempty"`
}

type ChatMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Thinking  string           `json:"thinking,omitempty"`
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
}
