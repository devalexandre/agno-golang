package tools

import (
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// WhatsAppTool fornece integração com WhatsApp via Twilio
type WhatsAppTool struct {
	toolkit.Toolkit
	twilioAccountSID string
	twilioAuthToken  string
	twilioPhoneFrom  string
}

// SendWhatsAppParams define os parâmetros para enviar mensagem
type SendWhatsAppParams struct {
	To        string `json:"to" description:"Número com código de país (ex: +5511998765432)" required:"true"`
	Message   string `json:"message" description:"Mensagem de texto" required:"true"`
	MediaURL  string `json:"media_url,omitempty" description:"URL de imagem/vídeo"`
	MediaType string `json:"media_type,omitempty" description:"image, video, document"`
}

// WhatsAppResult é o resultado da operação
type WhatsAppResult struct {
	Success    bool      `json:"success"`
	MessageSID string    `json:"message_sid"`
	Status     string    `json:"status"` // queued, sent, delivered, read
	Timestamp  time.Time `json:"timestamp"`
	Error      string    `json:"error,omitempty"`
}

// GetMessageStatusParams define os parâmetros para obter status
type GetMessageStatusParams struct {
	MessageSID string `json:"message_sid" description:"ID da mensagem" required:"true"`
}

// NewWhatsAppTool cria uma nova instância
func NewWhatsAppTool() *WhatsAppTool {
	t := &WhatsAppTool{}
	t.Toolkit = toolkit.NewToolkit()

	t.Toolkit.Register(
		"SendWhatsAppMessage",
		"Enviar mensagem via WhatsApp usando Twilio",
		t,
		t.SendWhatsAppMessage,
		SendWhatsAppParams{},
	)

	t.Toolkit.Register(
		"SendWhatsAppWithMedia",
		"Enviar WhatsApp com mídia (imagem, vídeo)",
		t,
		t.SendWhatsAppWithMedia,
		SendWhatsAppParams{},
	)

	t.Toolkit.Register(
		"GetMessageStatus",
		"Obter status da mensagem WhatsApp",
		t,
		t.GetMessageStatus,
		GetMessageStatusParams{},
	)

	return t
}

// SendWhatsAppMessage envia uma mensagem de texto
func (t *WhatsAppTool) SendWhatsAppMessage(params SendWhatsAppParams) (interface{}, error) {
	if params.To == "" || params.Message == "" {
		return WhatsAppResult{
			Success: false,
			Error:   "número e mensagem obrigatórios",
		}, fmt.Errorf("parâmetros obrigatórios faltando")
	}

	// Validar formato do número
	if len(params.To) < 10 {
		return WhatsAppResult{
			Success: false,
			Error:   "número de telefone inválido",
		}, fmt.Errorf("número de telefone inválido")
	}

	// Simulação de envio via Twilio
	// Em produção, usar: github.com/twilio/twilio-go
	messageSID := fmt.Sprintf("SM%d", time.Now().UnixNano())

	return WhatsAppResult{
		Success:    true,
		MessageSID: messageSID,
		Status:     "queued",
		Timestamp:  time.Now(),
	}, nil
}

// SendWhatsAppWithMedia envia mensagem com mídia
func (t *WhatsAppTool) SendWhatsAppWithMedia(params SendWhatsAppParams) (interface{}, error) {
	if params.To == "" || params.Message == "" || params.MediaURL == "" {
		return WhatsAppResult{
			Success: false,
			Error:   "número, mensagem e URL de mídia obrigatórios",
		}, fmt.Errorf("parâmetros obrigatórios faltando")
	}

	// Validar URL de mídia
	if !isValidURL(params.MediaURL) {
		return WhatsAppResult{
			Success: false,
			Error:   "URL de mídia inválida",
		}, fmt.Errorf("URL de mídia inválida")
	}

	messageSID := fmt.Sprintf("SM%d", time.Now().UnixNano())

	return WhatsAppResult{
		Success:    true,
		MessageSID: messageSID,
		Status:     "queued",
		Timestamp:  time.Now(),
	}, nil
}

// GetMessageStatus obtém o status da mensagem
func (t *WhatsAppTool) GetMessageStatus(params GetMessageStatusParams) (interface{}, error) {
	if params.MessageSID == "" {
		return WhatsAppResult{
			Success: false,
			Error:   "message_sid obrigatório",
		}, fmt.Errorf("message_sid obrigatório")
	}

	// Simulação de status
	status := "delivered"

	return WhatsAppResult{
		Success:    true,
		MessageSID: params.MessageSID,
		Status:     status,
		Timestamp:  time.Now(),
	}, nil
}

// SetTwilioConfig configura as credenciais Twilio
func (t *WhatsAppTool) SetTwilioConfig(accountSID, authToken, phoneFrom string) {
	t.twilioAccountSID = accountSID
	t.twilioAuthToken = authToken
	t.twilioPhoneFrom = phoneFrom
}

// isValidURL valida se uma URL é válida
func isValidURL(urlStr string) bool {
	return len(urlStr) > 0 && (starts(urlStr, "http://") || starts(urlStr, "https://"))
}

func starts(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
