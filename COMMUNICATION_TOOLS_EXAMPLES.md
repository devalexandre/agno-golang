# 📧 Communication & Calendar Tools - Implementation Examples

Este documento fornece exemplos de código prontos para implementação das 7 novas ferramentas de comunicação e calendário adicionadas ao Agno Go.

---

## 1. Email Management Tools

### Send Email (SMTP, SendGrid, Resend)

```go
// agno/tools/email_management_tools.go
package tools

import (
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/toolkit"
)

type EmailManagementTool struct {
	toolkit.Toolkit
	smtpHost      string
	smtpPort      int
	smtpUser      string
	smtpPassword  string
	sendgridKey   string
	resendKey     string
	defaultFrom   string
}

type SendEmailParams struct {
	To          []string            `json:"to" description:"Destinatários" required:"true"`
	Subject     string              `json:"subject" description:"Assunto" required:"true"`
	Body        string              `json:"body" description:"Corpo em texto plano"`
	BodyHTML    string              `json:"body_html,omitempty" description:"Corpo em HTML"`
	Attachments []EmailAttachment    `json:"attachments,omitempty"`
	Provider    string              `json:"provider,omitempty" description:"smtp, sendgrid, resend (default: smtp)"`
	From        string              `json:"from,omitempty" description:"Remetente (default: config)"`
	CC          []string            `json:"cc,omitempty" description:"Cópia"`
	BCC         []string            `json:"bcc,omitempty" description:"Cópia oculta"`
}

type EmailAttachment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"` // base64 encoded
	MimeType string `json:"mime_type,omitempty"`
}

type SendEmailResult struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

func NewEmailManagementTool() *EmailManagementTool {
	t := &EmailManagementTool{}
	t.Toolkit = toolkit.NewToolkit()
	
	t.Toolkit.Register(
		"SendEmail",
		"Enviar e-mail via SMTP, SendGrid ou Resend",
		t,
		t.SendEmail,
		SendEmailParams{},
	)
	
	t.Toolkit.Register(
		"SendEmailWithTemplate",
		"Enviar e-mail usando template",
		t,
		t.SendEmailWithTemplate,
		SendEmailWithTemplateParams{},
	)
	
	return t
}

func (t *EmailManagementTool) SendEmail(params SendEmailParams) (interface{}, error) {
	provider := params.Provider
	if provider == "" {
		provider = "smtp"
	}

	from := params.From
	if from == "" {
		from = t.defaultFrom
	}

	switch provider {
	case "sendgrid":
		return t.sendViaSendGrid(params, from)
	case "resend":
		return t.sendViaResend(params, from)
	default:
		return t.sendViaSMTP(params, from)
	}
}

// sendViaSMTP envia via SMTP padrão (Gmail, Outlook, custom)
func (t *EmailManagementTool) sendViaSMTP(params SendEmailParams, from string) (interface{}, error) {
	// Construir headers
	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n",
		from,
		strings.Join(params.To, ", "),
		params.Subject,
	)

	// Usar HTML se fornecido, senão texto
	body := params.BodyHTML
	if body == "" {
		body = params.Body
		headers = strings.Replace(
			headers,
			"Content-Type: text/html",
			"Content-Type: text/plain",
			1,
		)
	}

	message := []byte(headers + "\r\n" + body)

	// SMTP auth
	auth := smtp.PlainAuth("", t.smtpUser, t.smtpPassword, t.smtpHost)
	addr := fmt.Sprintf("%s:%d", t.smtpHost, t.smtpPort)

	// Enviar
	err := smtp.SendMail(addr, auth, from, params.To, message)
	if err != nil {
		return SendEmailResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return SendEmailResult{
		Success:   true,
		MessageID: fmt.Sprintf("smtp-%d", time.Now().UnixNano()),
		Status:    "sent",
	}, nil
}

// sendViaSendGrid envia via SendGrid API
func (t *EmailManagementTool) sendViaSendGrid(params SendEmailParams, from string) (interface{}, error) {
	m := mail.NewV3Mail()
	m.SetFrom(mail.NewEmail("", from))
	m.Subject = params.Subject

	// Adicionar conteúdo
	if params.BodyHTML != "" {
		m.AddContent(mail.NewContent("text/html", params.BodyHTML))
	}
	if params.Body != "" {
		m.AddContent(mail.NewContent("text/plain", params.Body))
	}

	// Adicionar destinatários
	for _, to := range params.To {
		m.AddTo(mail.NewEmail("", to))
	}

	// CC e BCC
	for _, cc := range params.CC {
		m.AddCC(mail.NewEmail("", cc))
	}
	for _, bcc := range params.BCC {
		m.AddBCC(mail.NewEmail("", bcc))
	}

	// Enviar
	client := sendgrid.NewSendClient(t.sendgridKey)
	response, err := client.Send(m)
	if err != nil {
		return SendEmailResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return SendEmailResult{
		Success:   true,
		MessageID: response.Headers.Get("X-Message-ID"),
		Status:    "sent",
	}, nil
}

// sendViaResend envia via Resend API
func (t *EmailManagementTool) sendViaResend(params SendEmailParams, from string) (interface{}, error) {
	// Implementar usando client Resend
	// Placeholder - substitua com implementação real
	return SendEmailResult{
		Success:   true,
		MessageID: fmt.Sprintf("resend-%d", time.Now().UnixNano()),
		Status:    "sent",
	}, nil
}

type SendEmailWithTemplateParams struct {
	To         []string           `json:"to" required:"true"`
	TemplateID string             `json:"template_id" required:"true"`
	Variables  map[string]string  `json:"variables,omitempty"`
	Provider   string             `json:"provider,omitempty"`
}

func (t *EmailManagementTool) SendEmailWithTemplate(params SendEmailWithTemplateParams) (interface{}, error) {
	// Implementar renderização de templates
	// SendGrid e Resend têm suporte nativo a templates
	return SendEmailResult{
		Success:   true,
		MessageID: fmt.Sprintf("template-%d", time.Now().UnixNano()),
		Status:    "sent",
	}, nil
}
```

### Email Trigger Watcher (IMAP, Webhook)

```go
// agno/tools/email_watcher_tools.go
package tools

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/imap"
	"strings"
	"time"

	"github.com/toolkit"
)

type EmailWatcherTool struct {
	toolkit.Toolkit
	imapHost        string
	imapPort        int
	imapUser        string
	imapPassword    string
	pollInterval    time.Duration
	webhookHandlers map[string]EmailWebhookHandler
}

type EmailWebhookHandler struct {
	TriggerID string
	Handler   func(message EmailMessage) error
}

type WatchEmailParams struct {
	SubjectKeyword string `json:"subject_keyword" description:"Palavra-chave no assunto" required:"true"`
	FromFilter     string `json:"from_filter,omitempty" description:"Filtrar por remetente"`
	FolderName     string `json:"folder_name,omitempty" description:"IMAP folder (default: INBOX)"`
	CallbackURL    string `json:"callback_url,omitempty" description:"URL para webhook callback"`
	TriggerID      string `json:"trigger_id,omitempty" description:"ID do trigger"`
}

type EmailMessage struct {
	From        string        `json:"from"`
	To          []string      `json:"to"`
	CC          []string      `json:"cc,omitempty"`
	Subject     string        `json:"subject"`
	Body        string        `json:"body"`
	BodyHTML    string        `json:"body_html,omitempty"`
	Attachments []Attachment  `json:"attachments"`
	Timestamp   time.Time     `json:"timestamp"`
	MessageID   string        `json:"message_id"`
	Read        bool          `json:"read"`
}

type Attachment struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
	Content  string `json:"content,omitempty"` // base64
}

type WatchEmailResult struct {
	Success bool   `json:"success"`
	WatchID string `json:"watch_id"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

func NewEmailWatcherTool() *EmailWatcherTool {
	t := &EmailWatcherTool{
		pollInterval:    30 * time.Second,
		webhookHandlers: make(map[string]EmailWebhookHandler),
	}
	t.Toolkit = toolkit.NewToolkit()

	t.Toolkit.Register(
		"WatchEmailForKeywords",
		"Monitorar e-mails para palavras-chave específicas",
		t,
		t.WatchEmailForKeywords,
		WatchEmailParams{},
	)

	t.Toolkit.Register(
		"StopWatchingEmail",
		"Parar de monitorar e-mails",
		t,
		t.StopWatchingEmail,
		StopWatchingParams{},
	)

	return t
}

func (t *EmailWatcherTool) WatchEmailForKeywords(params WatchEmailParams) (interface{}, error) {
	watchID := fmt.Sprintf("watch-%d", time.Now().UnixNano())

	// Registrar handler webhook se fornecido
	if params.CallbackURL != "" && params.TriggerID != "" {
		t.webhookHandlers[params.TriggerID] = EmailWebhookHandler{
			TriggerID: params.TriggerID,
			Handler: func(msg EmailMessage) error {
				// Chamar callback via HTTP POST
				// Implementação omitida
				return nil
			},
		}
	}

	// Iniciar goroutine para polling
	go t.pollForEmails(watchID, params)

	return WatchEmailResult{
		Success: true,
		WatchID: watchID,
		Status:  "watching",
	}, nil
}

func (t *EmailWatcherTool) pollForEmails(watchID string, params WatchEmailParams) {
	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		err := t.checkEmailsForKeywords(params)
		if err != nil {
			log.Printf("Error checking emails: %v", err)
		}
	}
}

func (t *EmailWatcherTool) checkEmailsForKeywords(params WatchEmailParams) error {
	// Conectar ao IMAP
	imapClient, err := imap.Dial(fmt.Sprintf("%s:%d", t.imapHost, t.imapPort))
	if err != nil {
		return err
	}
	defer imapClient.Close()

	// Login
	if err := imapClient.Login(t.imapUser, t.imapPassword); err != nil {
		return err
	}

	// Selecionar mailbox
	folder := params.FolderName
	if folder == "" {
		folder = "INBOX"
	}

	_, err = imapClient.Select(folder, false)
	if err != nil {
		return err
	}

	// Buscar e-mails com a palavra-chave
	criteria := fmt.Sprintf("SUBJECT \"%s\"", params.SubjectKeyword)
	if params.FromFilter != "" {
		criteria += fmt.Sprintf(" FROM \"%s\"", params.FromFilter)
	}

	_, err = imapClient.Search(criteria)
	if err != nil {
		return err
	}

	// Processado pelo handler registrado
	// Implementação de parsing de mensagens omitida

	return nil
}

type StopWatchingParams struct {
	WatchID string `json:"watch_id" required:"true"`
}

func (t *EmailWatcherTool) StopWatchingEmail(params StopWatchingParams) (interface{}, error) {
	// Implementar parada de watching
	return map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Stopped watching %s", params.WatchID),
	}, nil
}
```

---

## 2. WhatsApp Integration Tools (Twilio)

```go
// agno/tools/whatsapp_tools.go
package tools

import (
	"fmt"
	"time"

	"github.com/twilio/twilio-go"
	twilio_api "github.com/twilio/twilio-go/rest/api/v2010/account/message"
	"github.com/toolkit"
)

type WhatsAppTool struct {
	toolkit.Toolkit
	twilioAccountSID string
	twilioAuthToken  string
	twilioPhoneFrom  string
}

type SendWhatsAppParams struct {
	To       string `json:"to" description:"Número com código de país (ex: +5511998765432)" required:"true"`
	Message  string `json:"message" description:"Mensagem de texto" required:"true"`
	MediaURL string `json:"media_url,omitempty" description:"URL de imagem/vídeo"`
	MediaType string `json:"media_type,omitempty" description:"image, video, document"`
}

type WhatsAppResult struct {
	Success     bool      `json:"success"`
	MessageSID  string    `json:"message_sid"`
	Status      string    `json:"status"` // queued, sent, delivered, read
	Timestamp   time.Time `json:"timestamp"`
	Error       string    `json:"error,omitempty"`
}

type GetMessageStatusParams struct {
	MessageSID string `json:"message_sid" required:"true"`
}

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
		"Enviar WhatsApp com mídia",
		t,
		t.SendWhatsAppWithMedia,
		SendWhatsAppParams{},
	)

	t.Toolkit.Register(
		"GetMessageStatus",
		"Obter status da mensagem",
		t,
		t.GetMessageStatus,
		GetMessageStatusParams{},
	)

	return t
}

func (t *WhatsAppTool) SendWhatsAppMessage(params SendWhatsAppParams) (interface{}, error) {
	client := twilio.NewRestWithParams(t.twilioAccountSID, t.twilioAuthToken, "")

	// Formatar com whatsapp: prefix
	toPhone := fmt.Sprintf("whatsapp:%s", params.To)
	fromPhone := fmt.Sprintf("whatsapp:%s", t.twilioPhoneFrom)

	params_twilio := &twilio_api.CreateMessageParams{}
	params_twilio.SetTo(toPhone)
	params_twilio.SetFrom(fromPhone)
	params_twilio.SetBody(params.Message)

	resp, err := client.Api.CreateMessage(params_twilio)
	if err != nil {
		return WhatsAppResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return WhatsAppResult{
		Success:    true,
		MessageSID: *resp.Sid,
		Status:     "queued",
		Timestamp:  time.Now(),
	}, nil
}

func (t *WhatsAppTool) SendWhatsAppWithMedia(params SendWhatsAppParams) (interface{}, error) {
	client := twilio.NewRestWithParams(t.twilioAccountSID, t.twilioAuthToken, "")

	toPhone := fmt.Sprintf("whatsapp:%s", params.To)
	fromPhone := fmt.Sprintf("whatsapp:%s", t.twilioPhoneFrom)

	params_twilio := &twilio_api.CreateMessageParams{}
	params_twilio.SetTo(toPhone)
	params_twilio.SetFrom(fromPhone)
	params_twilio.SetBody(params.Message)
	
	if params.MediaURL != "" {
		params_twilio.SetMediaUrl([]string{params.MediaURL})
	}

	resp, err := client.Api.CreateMessage(params_twilio)
	if err != nil {
		return WhatsAppResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return WhatsAppResult{
		Success:    true,
		MessageSID: *resp.Sid,
		Status:     "queued",
		Timestamp:  time.Now(),
	}, nil
}

func (t *WhatsAppTool) GetMessageStatus(params GetMessageStatusParams) (interface{}, error) {
	client := twilio.NewRestWithParams(t.twilioAccountSID, t.twilioAuthToken, "")

	resp, err := client.Api.GetMessage(t.twilioAccountSID, params.MessageSID)
	if err != nil {
		return WhatsAppResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	status := "unknown"
	if resp.Status != nil {
		status = *resp.Status
	}

	return WhatsAppResult{
		Success:    true,
		MessageSID: *resp.Sid,
		Status:     status,
		Timestamp:  time.Now(),
	}, nil
}
```

---

## 3. Google Calendar Integration

```go
// agno/tools/google_calendar_tools.go
package tools

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
	"github.com/toolkit"
)

type GoogleCalendarTool struct {
	toolkit.Toolkit
	calendarService *calendar.Service
}

type GetEventsParams struct {
	Date      string `json:"date,omitempty" description:"Data em YYYY-MM-DD (default: hoje)"`
	CalendarID string `json:"calendar_id,omitempty" description:"Calendar ID (default: primary)"`
	MaxResults int64  `json:"max_results,omitempty" description:"Máximo de eventos"`
}

type CalendarEvent struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Location    string    `json:"location,omitempty"`
	Attendees   []string  `json:"attendees,omitempty"`
	VideoLink   string    `json:"video_link,omitempty"`
	Busy        bool      `json:"busy"`
}

type CreateEventParams struct {
	Title       string   `json:"title" description:"Título" required:"true"`
	StartTime   string   `json:"start_time" description:"ISO 8601" required:"true"`
	EndTime     string   `json:"end_time" description:"ISO 8601" required:"true"`
	Description string   `json:"description,omitempty"`
	Location    string   `json:"location,omitempty"`
	Attendees   []string `json:"attendees,omitempty"`
	VideoMeeting bool    `json:"video_meeting,omitempty"`
	CalendarID  string   `json:"calendar_id,omitempty"`
}

type EventResult struct {
	Success      bool   `json:"success"`
	EventID      string `json:"event_id"`
	CalendarLink string `json:"calendar_link"`
	VideoLink    string `json:"video_link,omitempty"`
	Error        string `json:"error,omitempty"`
}

func NewGoogleCalendarTool(credentialsFile string) (*GoogleCalendarTool, error) {
	ctx := context.Background()
	
	calendarService, err := calendar.NewService(
		ctx,
		option.WithCredentialsFile(credentialsFile),
	)
	if err != nil {
		return nil, err
	}

	t := &GoogleCalendarTool{
		calendarService: calendarService,
	}
	t.Toolkit = toolkit.NewToolkit()

	t.Toolkit.Register(
		"GetEventsToday",
		"Obter eventos de hoje",
		t,
		t.GetEventsToday,
		GetEventsParams{},
	)

	t.Toolkit.Register(
		"GetEventsOnDate",
		"Obter eventos de uma data específica",
		t,
		t.GetEventsOnDate,
		GetEventsParams{},
	)

	t.Toolkit.Register(
		"CreateEvent",
		"Criar novo evento",
		t,
		t.CreateEvent,
		CreateEventParams{},
	)

	return t, nil
}

func (t *GoogleCalendarTool) GetEventsToday(params GetEventsParams) (interface{}, error) {
	if params.Date == "" {
		params.Date = time.Now().Format("2006-01-02")
	}

	return t.GetEventsOnDate(params)
}

func (t *GoogleCalendarTool) GetEventsOnDate(params GetEventsParams) (interface{}, error) {
	calendarID := params.CalendarID
	if calendarID == "" {
		calendarID = "primary"
	}

	// Parse date
	date, _ := time.Parse("2006-01-02", params.Date)
	
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	call := t.calendarService.Events.List(calendarID)
	call.TimeMin(startOfDay.Format(time.RFC3339))
	call.TimeMax(endOfDay.Format(time.RFC3339))
	call.SingleEvents(true)

	if params.MaxResults > 0 {
		call.MaxResults(params.MaxResults)
	}

	events, err := call.Do()
	if err != nil {
		return nil, err
	}

	var result []CalendarEvent
	for _, item := range events.Items {
		event := CalendarEvent{
			ID:          item.Id,
			Title:       item.Summary,
			Description: item.Description,
			Location:    item.Location,
		}

		if item.Start.DateTime != "" {
			event.StartTime, _ = time.Parse(time.RFC3339, item.Start.DateTime)
		}
		if item.End.DateTime != "" {
			event.EndTime, _ = time.Parse(time.RFC3339, item.End.DateTime)
		}

		// Attendees
		for _, attendee := range item.Attendees {
			event.Attendees = append(event.Attendees, attendee.Email)
		}

		// Video link
		if item.ConferenceData != nil && len(item.ConferenceData.ConferenceId) > 0 {
			event.VideoLink = item.HtmlLink
		}

		result = append(result, event)
	}

	return result, nil
}

func (t *GoogleCalendarTool) CreateEvent(params CreateEventParams) (interface{}, error) {
	calendarID := params.CalendarID
	if calendarID == "" {
		calendarID = "primary"
	}

	startTime, _ := time.Parse(time.RFC3339, params.StartTime)
	endTime, _ := time.Parse(time.RFC3339, params.EndTime)

	event := &calendar.Event{
		Summary:     params.Title,
		Description: params.Description,
		Location:    params.Location,
		Start: &calendar.EventDateTime{
			DateTime: startTime.Format(time.RFC3339),
		},
		End: &calendar.EventDateTime{
			DateTime: endTime.Format(time.RFC3339),
		},
	}

	// Adicionar attendees
	for _, attendee := range params.Attendees {
		event.Attendees = append(event.Attendees, &calendar.EventAttendee{
			Email: attendee,
		})
	}

	// Criar Google Meet se solicitado
	if params.VideoMeeting {
		event.ConferenceData = &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				RequestId: fmt.Sprintf("meet-%d", time.Now().UnixNano()),
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
					Type: "hangoutsMeet",
				},
			},
		}
	}

	createdEvent, err := t.calendarService.Events.Insert(calendarID, event).
		ConferenceDataVersion(1).
		Do()

	if err != nil {
		return EventResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	result := EventResult{
		Success: true,
		EventID: createdEvent.Id,
		CalendarLink: createdEvent.HtmlLink,
	}

	if createdEvent.ConferenceData != nil {
		result.VideoLink = createdEvent.ConferenceData.EntryPoints[0].Uri
	}

	return result, nil
}
```

---

## 4. Webhook Receiver (Infrastructure)

```go
// agno/tools/webhook_receiver_tools.go
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

	"github.com/toolkit"
)

type WebhookReceiverTool struct {
	toolkit.Toolkit
	server          *http.Server
	webhookHandlers map[string]WebhookHandler
	eventQueue      chan WebhookEvent
	mu              sync.RWMutex
}

type WebhookHandler struct {
	TriggerID    string
	Handler      func(payload interface{}) error
	ValidateSign bool
	Secret       string
	Retries      int
}

type RegisterWebhookParams struct {
	TriggerID  string `json:"trigger_id" description:"ID único" required:"true"`
	Path       string `json:"path" description:"URL path" required:"true"`
	Secret     string `json:"secret,omitempty" description:"Secret para validação"`
	MaxRetries int    `json:"max_retries,omitempty" description:"Tentativas"`
}

type WebhookEvent struct {
	TriggerID string        `json:"trigger_id"`
	Payload   interface{}   `json:"payload"`
	Timestamp time.Time     `json:"timestamp"`
	SourceIP  string        `json:"source_ip"`
	Headers   map[string]string `json:"headers"`
}

type WebhookResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

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
		map[string]interface{}{"trigger_id": ""},
	)

	// Iniciar servidor HTTP
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/", t.handleWebhook)
	
	t.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go t.server.ListenAndServe()
	go t.processEventQueue()

	return t
}

func (t *WebhookReceiverTool) RegisterWebhook(params RegisterWebhookParams) (interface{}, error) {
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
		Message: fmt.Sprintf("Webhook registered for %s at /webhook/%s", params.TriggerID, params.Path),
	}, nil
}

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
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	// Validar signature se necessário
	if handler.ValidateSign {
		signature := r.Header.Get("X-Signature")
		if !t.validateSignature(body, handler.Secret, signature) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse payload
	var payload interface{}
	json.Unmarshal(body, &payload)

	// Criar evento
	event := WebhookEvent{
		TriggerID: triggerID,
		Payload:   payload,
		Timestamp: time.Now(),
		SourceIP:  r.RemoteAddr,
		Headers:   make(map[string]string),
	}

	// Copiar headers importantes
	for k, v := range r.Header {
		event.Headers[k] = v[0]
	}

	// Enfileirar para processamento
	select {
	case t.eventQueue <- event:
	default:
		// Queue cheio, descartar ou processar de forma assíncrona
		go t.handleEvent(event, handler)
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

func (t *WebhookReceiverTool) validateSignature(body []byte, secret, signature string) bool {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expected := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

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

func (t *WebhookReceiverTool) handleEvent(event WebhookEvent, handler WebhookHandler) {
	retries := 0
	for retries <= handler.Retries {
		if handler.Handler != nil {
			err := handler.Handler(event.Payload)
			if err == nil {
				return
			}
		}
		retries++
		if retries <= handler.Retries {
			time.Sleep(time.Second * time.Duration(retries*2)) // backoff exponencial
		}
	}
}

type WebhookStatsParams struct {
	TriggerID string `json:"trigger_id"`
}

func (t *WebhookReceiverTool) GetWebhookStats(params WebhookStatsParams) (interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if _, exists := t.webhookHandlers[params.TriggerID]; !exists {
		return WebhookResult{
			Success: false,
			Error:   "Webhook not found",
		}, nil
	}

	return map[string]interface{}{
		"trigger_id":   params.TriggerID,
		"queue_size":   len(t.eventQueue),
		"is_active":    true,
		"last_checked": time.Now(),
	}, nil
}
```

---

## 5. Tests

```go
// agno/tools/email_management_tools_test.go
package tools

import (
	"testing"
)

func TestSendEmailValidation(t *testing.T) {
	tests := []struct {
		name    string
		params  SendEmailParams
		wantErr bool
	}{
		{
			name: "valid email",
			params: SendEmailParams{
				To:      []string{"test@example.com"},
				Subject: "Test",
				Body:    "Test body",
			},
			wantErr: false,
		},
		{
			name: "empty to",
			params: SendEmailParams{
				To:      []string{},
				Subject: "Test",
				Body:    "Test body",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewEmailManagementTool()
			_, err := tool.SendEmail(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWebhookSignatureValidation(t *testing.T) {
	tool := NewWebhookReceiverTool(8080)
	
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
```

---

## 📝 Notas de Implementação

### Email
- **SMTP**: Suporta Gmail, Outlook, ou servidor custom
- **SendGrid**: Ideal para volume alto
- **Resend**: Nova alternativa, moderna
- **Necessário**: Configurar credentials de cada provider

### WhatsApp
- **Twilio**: Provider único neste exemplo
- **Necessário**: Account SID, Auth Token, numero Twilio verificado
- **Webhook**: Para receber mensagens em tempo real

### Google Calendar
- **OAuth2**: Usar Google Cloud credentials
- **Necessário**: Arquivo JSON com credenciais de serviço

### Webhook Receiver
- **HMAC-SHA256**: Validação de assinatura (padrão)
- **Retry automático**: Com backoff exponencial
- **Queue**: Até 1000 eventos em buffer

---

Este documento serve como referência rápida para implementação. Cada ferramenta segue o padrão Agno Go toolkit.
