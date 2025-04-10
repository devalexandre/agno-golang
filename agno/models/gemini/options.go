package gemini

import (
	"net/http"
)

// ClientOptions representa as opções para o cliente da API do Gemini
type ClientOptions struct {
	APIKey         string                 `json:"-"`                       // Chave da API
	Organization   string                 `json:"-"`                       // Organização associada
	BaseURL        string                 `json:"-"`                       // URL base da API
	Timeout        int                    `json:"-"`                       // Tempo limite da solicitação
	MaxRetries     int                    `json:"-"`                       // Número máximo de tentativas
	DefaultHeaders http.Header            `json:"-"`                       // Cabeçalhos padrão
	DefaultQuery   map[string]string      `json:"-"`                       // Parâmetros de consulta padrão
	HTTPClient     *http.Client           `json:"-"`                       // Cliente HTTP personalizado
	ClientParams   map[string]interface{} `json:"client_params,omitempty"` // Parâmetros adicionais do cliente
	// Campos adicionais para solicitações de chat
	Model            string   // Modelo a ser usado
	Temperature      *float32 // Temperatura da resposta
	MaxTokens        *int     // Número máximo de tokens
	TopP             *float32 // Parâmetro Top-P
	FrequencyPenalty *float32 // Penalidade de frequência
	PresencePenalty  *float32 // Penalidade de presença
}

// DefaultOptions retorna as opções padrão para o cliente da API do Gemini
func DefaultOptions() *ClientOptions {
	return &ClientOptions{
		Model:            "gemini-2.0-flash-lite",
		Temperature:      floatPtr(0.3),
		MaxTokens:        intPtr(1024),
		TopP:             floatPtr(1.0),
		FrequencyPenalty: floatPtr(0.0),
		PresencePenalty:  floatPtr(0.0),
	}
}

// WithModel define o modelo a ser usado
func WithModel(model string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.Model = model
	}
}

// WithAPIKey define a chave da API para o cliente
func WithAPIKey(key string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.APIKey = key
	}
}

// Funções auxiliares para criar ponteiros para campos opcionais
func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float32) *float32 { return &f }
func intPtr(i int) *int           { return &i }
func strPtr(s string) *string     { return &s }
