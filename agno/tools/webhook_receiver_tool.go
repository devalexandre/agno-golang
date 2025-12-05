package tools

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// WebhookReceiverTool fornece funcionalidades de recebimento de webhooks
type WebhookReceiverTool struct {
	toolkit.Toolkit
	server          *http.Server
	webhookHandlers map[string]WebhookHandler
	eventQueue      chan WebhookEvent
	mu              sync.RWMutex
	eventCount      int64
	lastEvent       time.Time
}

// WebhookHandler define como tratar um webhook
type WebhookHandler struct {
	TriggerID    string
	Handler      func(payload interface{}) error
	ValidateSign bool
	Secret       string
	Retries      int
}

// RegisterWebhookParams define os parâmetros para registrar um webhook
type RegisterWebhookParams struct {
	TriggerID  string `json:"trigger_id" description:"ID único do trigger" required:"true"`
	Path       string `json:"path" description:"URL path (ex: /webhook/novo-pagamento)" required:"true"`
	Secret     string `json:"secret,omitempty" description:"Secret para validar signature"`
	MaxRetries int    `json:"max_retries,omitempty" description:"Retentativas se falhar"`
}

// GetWebhookStatsParams define os parâmetros para obter estatísticas
type GetWebhookStatsParams struct {
	TriggerID string `json:"trigger_id" description:"ID do webhook" required:"true"`
}

// WebhookEvent representa um evento recebido
type WebhookEvent struct {
	TriggerID string                 `json:"trigger_id"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
	SourceIP  string                 `json:"source_ip"`
	Headers   map[string]string      `json:"headers"`
}

// WebhookResult é o resultado da operação
type WebhookResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// WebhookStats contém estatísticas do webhook
type WebhookStats struct {
	TriggerID  string    `json:"trigger_id"`
	QueueSize  int       `json:"queue_size"`
	IsActive   bool      `json:"is_active"`
	EventCount int64     `json:"event_count"`
	LastEvent  time.Time `json:"last_event"`
}

// NewWebhookReceiverTool cria uma nova instância
func NewWebhookReceiverTool(port int) *WebhookReceiverTool {
	t := &WebhookReceiverTool{
		webhookHandlers: make(map[string]WebhookHandler),
		eventQueue:      make(chan WebhookEvent, 1000),
	}
	t.Toolkit = toolkit.NewToolkit()

	t.Toolkit.Register(
		"RegisterWebhook",
		"Registrar novo webhook endpoint",
		t,
		t.RegisterWebhook,
		RegisterWebhookParams{},
	)

	t.Toolkit.Register(
		"GetWebhookStats",
		"Obter estatísticas de webhook",
		t,
		t.GetWebhookStats,
		GetWebhookStatsParams{},
	)

	// Iniciar servidor HTTP
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/", t.handleWebhook)

	t.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Iniciar servidor em goroutine
	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Webhook server error: %v\n", err)
		}
	}()

	// Processar fila de eventos
	go t.processEventQueue()

	return t
}

// RegisterWebhook registra um novo webhook
func (t *WebhookReceiverTool) RegisterWebhook(params RegisterWebhookParams) (interface{}, error) {
	if params.TriggerID == "" || params.Path == "" {
		return WebhookResult{
			Success: false,
			Error:   "trigger_id e path obrigatórios",
		}, fmt.Errorf("parâmetros obrigatórios faltando")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.webhookHandlers[params.TriggerID] = WebhookHandler{
		TriggerID:    params.TriggerID,
		ValidateSign: params.Secret != "",
		Secret:       params.Secret,
		Retries:      params.MaxRetries,
	}

	return WebhookResult{
		Success: true,
		Message: fmt.Sprintf("Webhook registrado para %s em /webhook/%s", params.TriggerID, params.Path),
	}, nil
}

// handleWebhook processa um webhook recebido
func (t *WebhookReceiverTool) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Extrair trigger ID do path
	triggerID := r.URL.Path[len("/webhook/"):]

	t.mu.RLock()
	handler, exists := t.webhookHandlers[triggerID]
	t.mu.RUnlock()

	if !exists {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	// Ler payload
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		http.Error(w, "Error reading body", http.StatusBadRequest)
		return
	}

	// Validar signature se necessário
	if handler.ValidateSign {
		signature := r.Header.Get("X-Signature")
		if !t.validateSignature(body, handler.Secret, signature) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse payload
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Error parsing JSON", http.StatusBadRequest)
		return
	}

	// Criar evento
	event := WebhookEvent{
		TriggerID: triggerID,
		Payload:   payload,
		Timestamp: time.Now(),
		SourceIP:  r.RemoteAddr,
		Headers:   make(map[string]string),
	}

	// Copiar headers importantes
	for _, k := range []string{"Content-Type", "X-Signature", "X-Event-Type"} {
		if v := r.Header.Get(k); v != "" {
			event.Headers[k] = v
		}
	}

	// Enfileirar para processamento
	select {
	case t.eventQueue <- event:
	default:
		// Queue cheio, processar de forma assíncrona
		go t.handleEvent(event, handler)
	}

	// Atualizar estatísticas
	t.mu.Lock()
	t.eventCount++
	t.lastEvent = time.Now()
	t.mu.Unlock()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

// validateSignature valida a assinatura HMAC-SHA256
func (t *WebhookReceiverTool) validateSignature(body []byte, secret, signature string) bool {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expected := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// processEventQueue processa eventos da fila
func (t *WebhookReceiverTool) processEventQueue() {
	for event := range t.eventQueue {
		t.mu.RLock()
		handler, exists := t.webhookHandlers[event.TriggerID]
		t.mu.RUnlock()

		if exists {
			t.handleEvent(event, handler)
		}
	}
}

// handleEvent processa um evento com retry
func (t *WebhookReceiverTool) handleEvent(event WebhookEvent, handler WebhookHandler) {
	retries := 0
	for retries <= handler.Retries {
		if handler.Handler != nil {
			err := handler.Handler(event.Payload)
			if err == nil {
				return
			}
		} else {
			// Se não há handler customizado, apenas loga
			fmt.Printf("Webhook received: %s at %v\n", event.TriggerID, event.Timestamp)
			return
		}

		retries++
		if retries <= handler.Retries {
			// Exponential backoff: 1s, 2s, 4s, 8s, etc
			backoff := time.Duration(1<<uint(retries-1)) * time.Second
			time.Sleep(backoff)
		}
	}
}

// GetWebhookStats retorna estatísticas do webhook
func (t *WebhookReceiverTool) GetWebhookStats(params GetWebhookStatsParams) (interface{}, error) {
	if params.TriggerID == "" {
		return WebhookStats{}, fmt.Errorf("trigger_id obrigatório")
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if _, exists := t.webhookHandlers[params.TriggerID]; !exists {
		return WebhookStats{}, fmt.Errorf("webhook não encontrado")
	}

	return WebhookStats{
		TriggerID:  params.TriggerID,
		QueueSize:  len(t.eventQueue),
		IsActive:   true,
		EventCount: t.eventCount,
		LastEvent:  t.lastEvent,
	}, nil
}

// Close fecha o servidor webhook
func (t *WebhookReceiverTool) Close() error {
	close(t.eventQueue)
	return t.server.Close()
}
