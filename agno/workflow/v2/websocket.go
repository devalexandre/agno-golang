package v2

import (
	"encoding/json"
	"fmt"
	"sync"
)

// WebSocketHandler defines the interface for WebSocket event streaming
type WebSocketHandler interface {
	// SendEvent sends a workflow event over WebSocket
	SendEvent(event *WorkflowRunResponseEvent) error
	// Close closes the WebSocket connection
	Close() error
}

// DefaultWebSocketHandler is a basic implementation of WebSocketHandler
// This can be extended to use actual WebSocket connections (e.g., gorilla/websocket)
type DefaultWebSocketHandler struct {
	mu       sync.Mutex
	events   []*WorkflowRunResponseEvent
	callback func(*WorkflowRunResponseEvent) error
}

// NewDefaultWebSocketHandler creates a new default WebSocket handler
func NewDefaultWebSocketHandler(callback func(*WorkflowRunResponseEvent) error) *DefaultWebSocketHandler {
	return &DefaultWebSocketHandler{
		events:   make([]*WorkflowRunResponseEvent, 0),
		callback: callback,
	}
}

// SendEvent sends an event through the callback function
func (h *DefaultWebSocketHandler) SendEvent(event *WorkflowRunResponseEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Store event
	h.events = append(h.events, event)

	// Call callback if provided
	if h.callback != nil {
		return h.callback(event)
	}

	return nil
}

// Close closes the handler (cleanup)
func (h *DefaultWebSocketHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.events = nil
	h.callback = nil
	return nil
}

// GetEvents returns all events sent through this handler
func (h *DefaultWebSocketHandler) GetEvents() []*WorkflowRunResponseEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.events
}

// JSONWebSocketHandler sends events as JSON strings
type JSONWebSocketHandler struct {
	mu     sync.Mutex
	writer func(string) error
	closed bool
}

// NewJSONWebSocketHandler creates a handler that writes JSON to a writer function
func NewJSONWebSocketHandler(writer func(string) error) *JSONWebSocketHandler {
	return &JSONWebSocketHandler{
		writer: writer,
		closed: false,
	}
}

// SendEvent serializes the event to JSON and sends it
func (h *JSONWebSocketHandler) SendEvent(event *WorkflowRunResponseEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("websocket handler is closed")
	}

	// Convert event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Send through writer
	if h.writer != nil {
		return h.writer(string(eventJSON))
	}

	return nil
}

// Close closes the JSON handler
func (h *JSONWebSocketHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.closed = true
	h.writer = nil
	return nil
}
