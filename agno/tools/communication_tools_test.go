package tools

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

// TestSendWhatsAppValidation testa validação de WhatsApp
func TestSendWhatsAppValidation(t *testing.T) {
	tool := NewWhatsAppTool()
	tool.SetTwilioConfig("ACCOUNT_SID", "AUTH_TOKEN", "+5511988776655")

	tests := []struct {
		name    string
		params  SendWhatsAppParams
		wantErr bool
	}{
		{
			name: "valid message",
			params: SendWhatsAppParams{
				To:      "+5511998765432",
				Message: "Hello World",
			},
			wantErr: false,
		},
		{
			name: "missing to",
			params: SendWhatsAppParams{
				To:      "",
				Message: "Hello World",
			},
			wantErr: true,
		},
		{
			name: "invalid phone",
			params: SendWhatsAppParams{
				To:      "123",
				Message: "Hello World",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.SendWhatsAppMessage(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendWhatsAppMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				res, ok := result.(WhatsAppResult)
				if !ok {
					t.Errorf("Expected WhatsAppResult, got %T", result)
				}
				if !res.Success || res.MessageSID == "" {
					t.Errorf("Expected successful send with MessageSID")
				}
			}
		})
	}
}

// TestSendWhatsAppWithMedia testa envio com mídia
func TestSendWhatsAppWithMedia(t *testing.T) {
	tool := NewWhatsAppTool()
	tool.SetTwilioConfig("ACCOUNT_SID", "AUTH_TOKEN", "+5511988776655")

	params := SendWhatsAppParams{
		To:       "+5511998765432",
		Message:  "Check this image",
		MediaURL: "https://example.com/image.jpg",
	}

	result, err := tool.SendWhatsAppWithMedia(params)
	if err != nil {
		t.Errorf("SendWhatsAppWithMedia() error = %v", err)
	}

	res, ok := result.(WhatsAppResult)
	if !ok {
		t.Errorf("Expected WhatsAppResult, got %T", result)
	}

	if !res.Success || res.Status != "queued" {
		t.Errorf("Expected queued status")
	}
}

// TestGetMessageStatus testa obtenção de status
func TestGetMessageStatus(t *testing.T) {
	tool := NewWhatsAppTool()
	tool.SetTwilioConfig("ACCOUNT_SID", "AUTH_TOKEN", "+5511988776655")

	params := GetMessageStatusParams{
		MessageSID: "SM123456789",
	}

	result, err := tool.GetMessageStatus(params)
	if err != nil {
		t.Errorf("GetMessageStatus() error = %v", err)
	}

	res, ok := result.(WhatsAppResult)
	if !ok {
		t.Errorf("Expected WhatsAppResult, got %T", result)
	}

	if !res.Success || res.MessageSID != "SM123456789" {
		t.Errorf("Expected successful status check")
	}
}

// TestCreateEventValidation testa criação de eventos
func TestCreateEventValidation(t *testing.T) {
	tool := NewGoogleCalendarTool("API_KEY")

	tests := []struct {
		name    string
		params  CreateEventParams
		wantErr bool
	}{
		{
			name: "valid event",
			params: CreateEventParams{
				Title:     "Meeting",
				StartTime: time.Now().Format(time.RFC3339),
				EndTime:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			},
			wantErr: false,
		},
		{
			name: "missing title",
			params: CreateEventParams{
				Title:     "",
				StartTime: time.Now().Format(time.RFC3339),
				EndTime:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			},
			wantErr: true,
		},
		{
			name: "invalid time",
			params: CreateEventParams{
				Title:     "Meeting",
				StartTime: "invalid",
				EndTime:   time.Now().Format(time.RFC3339),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.CreateEvent(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				res, ok := result.(EventResult)
				if !ok {
					t.Errorf("Expected EventResult, got %T", result)
				}
				if !res.Success || res.EventID == "" {
					t.Errorf("Expected successful creation with EventID")
				}
			}
		})
	}
}

// TestGetEventsToday testa obtenção de eventos
func TestGetEventsToday(t *testing.T) {
	tool := NewGoogleCalendarTool("API_KEY")

	result, err := tool.GetEventsToday(GetEventsParams{})
	if err != nil {
		t.Errorf("GetEventsToday() error = %v", err)
	}

	events, ok := result.([]CalendarEvent)
	if !ok {
		t.Errorf("Expected []CalendarEvent, got %T", result)
	}

	if len(events) == 0 {
		t.Errorf("Expected at least one event")
	}
}

// TestRegisterWebhook testa registro de webhook
func TestRegisterWebhook(t *testing.T) {
	tool := NewWebhookReceiverTool(8080)
	defer tool.Close()

	params := RegisterWebhookParams{
		TriggerID:  "test-trigger",
		Path:       "/webhook/test",
		Secret:     "test-secret",
		MaxRetries: 3,
	}

	result, err := tool.RegisterWebhook(params)
	if err != nil {
		t.Errorf("RegisterWebhook() error = %v", err)
	}

	res, ok := result.(WebhookResult)
	if !ok {
		t.Errorf("Expected WebhookResult, got %T", result)
	}

	if !res.Success {
		t.Errorf("Expected successful registration: %s", res.Error)
	}
}

// TestWebhookSignatureValidation testa validação de assinatura
func TestWebhookSignatureValidation(t *testing.T) {
	tool := NewWebhookReceiverTool(8081)
	defer tool.Close()

	body := []byte(`{"test": "data"}`)
	secret := "my-secret"

	// Gerar assinatura válida
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	validSig := hex.EncodeToString(h.Sum(nil))

	if !tool.validateSignature(body, secret, validSig) {
		t.Error("Valid signature should pass validation")
	}

	if tool.validateSignature(body, secret, "invalid-sig") {
		t.Error("Invalid signature should fail validation")
	}
}

// TestGetWebhookStats testa obtenção de estatísticas
func TestGetWebhookStats(t *testing.T) {
	tool := NewWebhookReceiverTool(8082)
	defer tool.Close()

	// Registrar webhook primeiro
	registerParams := RegisterWebhookParams{
		TriggerID: "test-stats",
		Path:      "/webhook/stats",
	}
	tool.RegisterWebhook(registerParams)

	// Obter estatísticas
	params := GetWebhookStatsParams{TriggerID: "test-stats"}
	result, err := tool.GetWebhookStats(params)
	if err != nil {
		t.Errorf("GetWebhookStats() error = %v", err)
	}

	stats, ok := result.(WebhookStats)
	if !ok {
		t.Errorf("Expected WebhookStats, got %T", result)
	}

	if !stats.IsActive {
		t.Errorf("Expected active webhook")
	}
}
