package tools

import (
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// GoogleCalendarTool fornece integração com Google Calendar
type GoogleCalendarTool struct {
	toolkit.Toolkit
	apiKey string
}

// GetEventsParams define os parâmetros para obter eventos
type GetEventsParams struct {
	Date       string `json:"date,omitempty" description:"Data em YYYY-MM-DD (default: hoje)"`
	CalendarID string `json:"calendar_id,omitempty" description:"Calendar ID (default: primary)"`
	MaxResults int64  `json:"max_results,omitempty" description:"Máximo de eventos"`
}

// CalendarEvent representa um evento
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

// CreateEventParams define os parâmetros para criar evento
type CreateEventParams struct {
	Title        string   `json:"title" description:"Título" required:"true"`
	StartTime    string   `json:"start_time" description:"ISO 8601 format" required:"true"`
	EndTime      string   `json:"end_time" description:"ISO 8601 format" required:"true"`
	Description  string   `json:"description,omitempty"`
	Location     string   `json:"location,omitempty"`
	Attendees    []string `json:"attendees,omitempty"`
	VideoMeeting bool     `json:"video_meeting,omitempty" description:"Criar Google Meet"`
	CalendarID   string   `json:"calendar_id,omitempty" description:"Default: primary"`
}

// EventResult é o resultado da operação
type EventResult struct {
	Success      bool   `json:"success"`
	EventID      string `json:"event_id"`
	CalendarLink string `json:"calendar_link"`
	VideoLink    string `json:"video_link,omitempty"`
	Error        string `json:"error,omitempty"`
}

// NewGoogleCalendarTool cria uma nova instância
func NewGoogleCalendarTool(apiKey string) *GoogleCalendarTool {
	t := &GoogleCalendarTool{
		apiKey: apiKey,
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
		"Criar novo evento no calendário",
		t,
		t.CreateEvent,
		CreateEventParams{},
	)

	return t
}

// GetEventsToday obtém eventos de hoje
func (t *GoogleCalendarTool) GetEventsToday(params GetEventsParams) (interface{}, error) {
	if params.Date == "" {
		params.Date = time.Now().Format("2006-01-02")
	}

	return t.GetEventsOnDate(params)
}

// GetEventsOnDate obtém eventos de uma data específica
func (t *GoogleCalendarTool) GetEventsOnDate(params GetEventsParams) (interface{}, error) {
	if params.Date == "" {
		return nil, fmt.Errorf("data obrigatória")
	}

	// Parse date
	date, err := time.Parse("2006-01-02", params.Date)
	if err != nil {
		return nil, fmt.Errorf("formato de data inválido: %v", err)
	}

	calendarID := params.CalendarID
	if calendarID == "" {
		calendarID = "primary"
	}

	// Simulação de eventos
	events := []CalendarEvent{}

	// Exemplo de evento
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(1 * time.Hour)

	events = append(events, CalendarEvent{
		ID:        "event-001",
		Title:     "Daily Standup",
		StartTime: startOfDay,
		EndTime:   endOfDay,
		Location:  "Conference Room A",
		Attendees: []string{"team@example.com"},
		Busy:      true,
	})

	return events, nil
}

// CreateEvent cria um novo evento
func (t *GoogleCalendarTool) CreateEvent(params CreateEventParams) (interface{}, error) {
	if params.Title == "" || params.StartTime == "" || params.EndTime == "" {
		return EventResult{
			Success: false,
			Error:   "título, start_time e end_time obrigatórios",
		}, fmt.Errorf("parâmetros obrigatórios faltando")
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, params.StartTime)
	if err != nil {
		return EventResult{
			Success: false,
			Error:   fmt.Sprintf("formato de start_time inválido: %v", err),
		}, err
	}

	endTime, err := time.Parse(time.RFC3339, params.EndTime)
	if err != nil {
		return EventResult{
			Success: false,
			Error:   fmt.Sprintf("formato de end_time inválido: %v", err),
		}, err
	}

	// Validar que end > start
	if endTime.Before(startTime) {
		return EventResult{
			Success: false,
			Error:   "end_time deve ser após start_time",
		}, fmt.Errorf("tempo inválido")
	}

	calendarID := params.CalendarID
	if calendarID == "" {
		calendarID = "primary"
	}

	eventID := fmt.Sprintf("event-%d", time.Now().UnixNano())
	calendarLink := fmt.Sprintf("https://calendar.google.com/calendar/u/0/r/eventedit/%s", eventID)

	result := EventResult{
		Success:      true,
		EventID:      eventID,
		CalendarLink: calendarLink,
	}

	// Adicionar Google Meet se solicitado
	if params.VideoMeeting {
		result.VideoLink = fmt.Sprintf("https://meet.google.com/%s", generateMeetCode())
	}

	return result, nil
}

// generateMeetCode gera um código de reunião
func generateMeetCode() string {
	return fmt.Sprintf("meet-%d", time.Now().Unix())
}

// SetAPIKey configura a chave de API
func (t *GoogleCalendarTool) SetAPIKey(apiKey string) {
	t.apiKey = apiKey
}
