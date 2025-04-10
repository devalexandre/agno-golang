package models

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/tools"
)

// CallOptions define as opções comuns que podem ser aplicadas para ambos os modelos (OpenAI e Gemini).
type CallOptions struct {
	Store               *bool                               `json:"store,omitempty"`
	ReasoningEffort     *string                             `json:"reasoning_effort,omitempty"`
	Metadata            map[string]interface{}              `json:"metadata,omitempty"`
	FrequencyPenalty    *float32                            `json:"frequency_penalty,omitempty"`
	LogitBias           map[string]float32                  `json:"logit_bias,omitempty"`
	Logprobs            *int                                `json:"logprobs,omitempty"`
	TopLogprobs         *int                                `json:"top_logprobs,omitempty"`
	MaxTokens           *int                                `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                                `json:"max_completion_tokens,omitempty"`
	Modalities          []string                            `json:"modalities,omitempty"`
	Audio               map[string]interface{}              `json:"audio,omitempty"`
	PresencePenalty     *float32                            `json:"presence_penalty,omitempty"`
	ResponseFormat      interface{}                         `json:"response_format,omitempty"`
	Seed                *int                                `json:"seed,omitempty"`
	Stop                interface{}                         `json:"stop,omitempty"`
	Stream              *bool                               `json:"stream,omitempty"`
	Temperature         *float32                            `json:"temperature,omitempty"`
	TopP                *float32                            `json:"top_p,omitempty"`
	ExtraHeaders        map[string]string                   `json:"-"`
	ExtraQuery          map[string]string                   `json:"-"`
	RequestParams       map[string]interface{}              `json:"request_params,omitempty"`
	StreamingFunc       func(context.Context, []byte) error `json:"-"` // Função de callback para streaming
	ToolCall            []tools.Tool                        `json:"-"` // Ferramentas para chamadas de função
	Tools               []tools.Tools                       `json:"tools,omitempty"`
}

// Option é uma função que modifica CallOptions.
type Option func(*CallOptions)

// DefaultCallOptions retorna as opções padrão para a solicitação.
func DefaultCallOptions() *CallOptions {
	return &CallOptions{
		Temperature:      floatPtr(0.7),
		MaxTokens:        intPtr(100),
		TopP:             floatPtr(1.0),
		FrequencyPenalty: floatPtr(0.0),
		PresencePenalty:  floatPtr(0.0),
	}
}

// WithStore especifica se a saída deve ser armazenada.
func WithStore(store bool) Option {
	return func(o *CallOptions) {
		o.Store = boolPtr(store)
	}
}

// WithStreamingFunc adiciona uma função de callback para processar fragmentos de streaming.
func WithStreamingFunc(f func(context.Context, []byte) error) Option {
	return func(o *CallOptions) {
		o.StreamingFunc = f
		o.Stream = boolPtr(true)
	}
}

// WithTools adiciona ferramentas à solicitação
func WithTools(tool []tools.Tool) Option {
	return func(o *CallOptions) {
		o.ToolCall = tool
	}
}

// Helper functions to create pointers for optional fields.
func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float32) *float32 { return &f }
func intPtr(i int) *int           { return &i }
func strPtr(s string) *string     { return &s }

// WithReasoningEffort define o esforço de raciocínio
func WithReasoningEffort(reasoningEffort string) Option {
	return func(o *CallOptions) {
		o.ReasoningEffort = strPtr(reasoningEffort)
	}
}

// WithMetadata define os metadados adicionais
func WithMetadata(metadata map[string]interface{}) Option {
	return func(o *CallOptions) {
		o.Metadata = metadata
	}
}

// WithFrequencyPenalty define a penalidade de frequência
func WithFrequencyPenalty(penalty float32) Option {
	return func(o *CallOptions) {
		o.FrequencyPenalty = floatPtr(penalty)
	}
}

// WithLogitBias define o viés nos logits dos tokens
func WithLogitBias(logitBias map[string]float32) Option {
	return func(o *CallOptions) {
		o.LogitBias = logitBias
	}
}

// WithLogprobs define o número máximo de logprobs por token
func WithLogprobs(logprobs int) Option {
	return func(o *CallOptions) {
		o.Logprobs = intPtr(logprobs)
	}
}

// WithTopLogprobs define o número máximo de top logprobs por token
func WithTopLogprobs(topLogprobs int) Option {
	return func(o *CallOptions) {
		o.TopLogprobs = intPtr(topLogprobs)
	}
}

// WithMaxTokens define o número máximo de tokens na resposta
func WithMaxTokens(tokens int) Option {
	return func(o *CallOptions) {
		o.MaxTokens = intPtr(tokens)
	}
}

// WithMaxCompletionTokens define o número máximo de tokens na conclusão
func WithMaxCompletionTokens(tokens int) Option {
	return func(o *CallOptions) {
		o.MaxCompletionTokens = intPtr(tokens)
	}
}

// WithModalities define as modalidades suportadas
func WithModalities(modalities []string) Option {
	return func(o *CallOptions) {
		o.Modalities = modalities
	}
}

// WithAudio define os dados de áudio
func WithAudio(audio map[string]interface{}) Option {
	return func(o *CallOptions) {
		o.Audio = audio
	}
}

// WithPresencePenalty define a penalidade de presença
func WithPresencePenalty(penalty float32) Option {
	return func(o *CallOptions) {
		o.PresencePenalty = floatPtr(penalty)
	}
}

// WithResponseFormat define o formato da resposta
func WithResponseFormat(format interface{}) Option {
	return func(o *CallOptions) {
		o.ResponseFormat = format
	}
}

// WithSeed define a semente para reproduzibilidade
func WithSeed(seed int) Option {
	return func(o *CallOptions) {
		o.Seed = intPtr(seed)
	}
}

// WithStop define as sequências de parada
func WithStop(stop interface{}) Option {
	return func(o *CallOptions) {
		o.Stop = stop
	}
}

// WithTemperature define a temperatura da resposta
func WithTemperature(temp float32) Option {
	return func(o *CallOptions) {
		o.Temperature = floatPtr(temp)
	}
}

// WithTopP define o parâmetro Top-P
func WithTopP(topP float32) Option {
	return func(o *CallOptions) {
		o.TopP = floatPtr(topP)
	}
}

// WithExtraHeaders define cabeçalhos adicionais
func WithExtraHeaders(headers http.Header) Option {
	return func(o *CallOptions) {
		o.ExtraHeaders = make(map[string]string)
		for k, v := range headers {
			if len(v) > 0 {
				o.ExtraHeaders[k] = v[0]
			}
		}
	}
}

// WithExtraQuery define parâmetros de consulta adicionais
func WithExtraQuery(query map[string]string) Option {
	return func(o *CallOptions) {
		o.ExtraQuery = query
	}
}

// WithRequestParams define parâmetros adicionais da solicitação
func WithRequestParams(params map[string]interface{}) Option {
	return func(o *CallOptions) {
		o.RequestParams = params
	}
}

func (o *CallOptions) MarshalJSON() ([]byte, error) {
	type Alias CallOptions
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(o),
	})
}
