package openai

import (
	"context"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/models"
)

// ToolCall representa uma chamada de ferramenta externa.
type ToolCall struct {
	Type      string `json:"type"`      // Tipo da ferramenta.
	Function  string `json:"function"`  // Nome da função da ferramenta.
	Arguments string `json:"arguments"` // Argumentos da função.
}

type OpenAIRequest struct {
	Model               string                 `json:"model"`                           // Modelo a ser usado.
	Messages            []models.Message       `json:"messages"`                        // Histórico da conversa.
	Tools               []ToolCall             `json:"tool_calls,omitempty"`            // Chamadas de ferramentas externas.
	Store               *bool                  `json:"store,omitempty"`                 // Armazenamento da saída.
	ReasoningEffort     *string                `json:"reasoning_effort,omitempty"`      // Esforço de raciocínio.
	Metadata            map[string]interface{} `json:"metadata,omitempty"`              // Metadados adicionais.
	FrequencyPenalty    *float32               `json:"frequency_penalty,omitempty"`     // Penalidade de frequência.
	LogitBias           map[string]float32     `json:"logit_bias,omitempty"`            // Viés nos logits dos tokens.
	Logprobs            *int                   `json:"logprobs,omitempty"`              // Número máximo de logprobs por token.
	TopLogprobs         *int                   `json:"top_logprobs,omitempty"`          // Número máximo de logprobs por token.
	MaxTokens           *int                   `json:"max_tokens,omitempty"`            // Número máximo de tokens na resposta.
	MaxCompletionTokens *int                   `json:"max_completion_tokens,omitempty"` // Número máximo de tokens na conclusão.
	Modalities          []string               `json:"modalities,omitempty"`            // Modalidades suportadas.
	Audio               map[string]interface{} `json:"audio,omitempty"`                 // Dados de áudio.
	PresencePenalty     *float32               `json:"presence_penalty,omitempty"`      // Penalidade de presença.
	ResponseFormat      interface{}            `json:"response_format,omitempty"`       // Formato da resposta.
	Seed                *int                   `json:"seed,omitempty"`                  // Semente para reproduzibilidade.
	Stop                interface{}            `json:"stop,omitempty"`                  // Sequências de parada.
	Temperature         *float32               `json:"temperature,omitempty"`           // Temperatura da resposta.
	TopP                *float32               `json:"top_p,omitempty"`                 // Parâmetro Top-P.
	ExtraHeaders        http.Header            `json:"-"`                               // Cabeçalhos adicionais.
	ExtraQuery          map[string]string      `json:"-"`                               // Parâmetros de consulta adicionais.
	RequestParams       map[string]interface{} `json:"request_params,omitempty"`        // Parâmetros adicionais da solicitação.
	Stream              *bool                  `json:"stream,omitempty"`                // Se a solicitação é de streaming.
}

// New type definitions for chat completion.
type Choices struct {
	Index        int            `json:"index"`
	Message      models.Message `json:"message"`
	Logprobs     interface{}    `json:"logprobs"`
	FinishReason string         `json:"finish_reason"`
	Delta        models.Message `json:"delta"`
}
type CompletionChunk struct {
	ID                string    `json:"id"`
	Object            string    `json:"object"`
	Created           int64     `json:"created"`
	Model             string    `json:"model"`
	SystemFingerprint string    `json:"system_fingerprint"`
	Choices           []Choices `json:"choices"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type CompletionResponse struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choices `json:"choices"`
	Usage   Usage     `json:"usage"`
}

// ClientInterface define a interface para a comunicação com a API da OpenAI.
type ClientInterface interface {
	CreateChatCompletion(ctx context.Context, messages []models.Message, options ...Option) (*CompletionResponse, error)
}

type ChatCompletionMessage = models.Message
type ChatCompletionResponse = CompletionResponse
type ChatCompletionChunk = CompletionChunk
type ChatCompletionRequest = OpenAIRequest
type ChatCompletionChoice = Choices
