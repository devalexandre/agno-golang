package tools

import (
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// MultiAgentHandoff gerencia transferência de contexto entre agentes
type MultiAgentHandoff struct {
	toolkit.Toolkit
	activeConversations map[string]Conversation
	handoffHistory      []HandoffRecord
	agentRegistry       map[string]AgentInfo
	contextBridge       map[string]ContextState
}

// Conversation representa uma conversa entre agentes
type Conversation struct {
	ConversationID string                 `json:"conversation_id"`
	Participants   []string               `json:"participants"` // agent IDs
	Context        map[string]interface{} `json:"context"`
	State          string                 `json:"state"` // active, paused, closed
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Messages       []ConversationMessage  `json:"messages"`
}

// ConversationMessage mensagem na conversa
type ConversationMessage struct {
	Timestamp time.Time              `json:"timestamp"`
	From      string                 `json:"from"` // agent ID
	To        string                 `json:"to"`   // agent ID
	Type      string                 `json:"type"` // request, response, context, status
	Content   map[string]interface{} `json:"content"`
	Priority  int                    `json:"priority"`
}

// HandoffRecord registra um handoff
type HandoffRecord struct {
	HandoffID      string    `json:"handoff_id"`
	FromAgent      string    `json:"from_agent"`
	ToAgent        string    `json:"to_agent"`
	ConversationID string    `json:"conversation_id"`
	Reason         string    `json:"reason"`
	ContextSize    int       `json:"context_size_bytes"`
	Status         string    `json:"status"` // success, failed, pending
	Timestamp      time.Time `json:"timestamp"`
	ExecutionTime  int64     `json:"execution_time_ms"`
}

// AgentInfo informações sobre um agente
type AgentInfo struct {
	AgentID         string   `json:"agent_id"`
	Name            string   `json:"name"`
	Capabilities    []string `json:"capabilities"`
	Status          string   `json:"status"` // active, idle, busy, unavailable
	MaxContextSize  int      `json:"max_context_size"`
	SupportedTopics []string `json:"supported_topics"`
	Rating          float64  `json:"rating"` // 0-5
}

// ContextState estado do contexto
type ContextState struct {
	StateID      string                 `json:"state_id"`
	AgentID      string                 `json:"agent_id"`
	Data         map[string]interface{} `json:"data"`
	Timestamp    time.Time              `json:"timestamp"`
	ExpiresIn    int                    `json:"expires_in_seconds"`
	IsCompressed bool                   `json:"is_compressed"`
}

// InitiateConversationParams parâmetros para iniciar conversa
type InitiateConversationParams struct {
	InitiatorID   string                 `json:"initiator_id" description:"ID do agente iniciador"`
	ParticipantID string                 `json:"participant_id" description:"ID do agente participante"`
	Topic         string                 `json:"topic" description:"Tópico da conversa"`
	Context       map[string]interface{} `json:"context" description:"Contexto inicial"`
	Priority      int                    `json:"priority" description:"Prioridade 1-5"`
}

// TransferContextParams parâmetros para transferir contexto
type TransferContextParams struct {
	FromAgent      string                 `json:"from_agent" description:"ID do agente origem"`
	ToAgent        string                 `json:"to_agent" description:"ID do agente destino"`
	ConversationID string                 `json:"conversation_id" description:"ID da conversa"`
	Context        map[string]interface{} `json:"context" description:"Dados a transferir"`
	Reason         string                 `json:"reason" description:"Motivo do handoff"`
	PreserveState  bool                   `json:"preserve_state" description:"Manter histórico"`
}

// HandoffResult resultado de operação de handoff
type HandoffResult struct {
	Success         bool                   `json:"success"`
	HandoffID       string                 `json:"handoff_id,omitempty"`
	ConversationID  string                 `json:"conversation_id"`
	Message         string                 `json:"message"`
	FromAgent       string                 `json:"from_agent"`
	ToAgent         string                 `json:"to_agent"`
	ContextReceived map[string]interface{} `json:"context_received,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// NewMultiAgentHandoff cria novo gerenciador
func NewMultiAgentHandoff() *MultiAgentHandoff {
	m := &MultiAgentHandoff{
		activeConversations: make(map[string]Conversation),
		handoffHistory:      make([]HandoffRecord, 0),
		agentRegistry:       make(map[string]AgentInfo),
		contextBridge:       make(map[string]ContextState),
	}
	m.Toolkit = toolkit.NewToolkit()

	m.Toolkit.Register(
		"RegisterAgent",
		"Registrar um novo agente no sistema",
		m,
		m.RegisterAgent,
		AgentInfo{},
	)

	m.Toolkit.Register(
		"InitiateConversation",
		"Iniciar conversa entre agentes",
		m,
		m.InitiateConversation,
		InitiateConversationParams{},
	)

	m.Toolkit.Register(
		"TransferContext",
		"Transferir contexto entre agentes",
		m,
		m.TransferContext,
		TransferContextParams{},
	)

	m.Toolkit.Register(
		"GetConversationState",
		"Obter estado de uma conversa",
		m,
		m.GetConversationState,
		GetConversationParams{},
	)

	m.Toolkit.Register(
		"SendMessage",
		"Enviar mensagem entre agentes",
		m,
		m.SendMessage,
		HandoffSendMessageParams{},
	)

	m.Toolkit.Register(
		"CompleteHandoff",
		"Completar handoff e fechar conversa",
		m,
		m.CompleteHandoff,
		CompleteHandoffParams{},
	)

	return m
}

// RegisterAgent registra novo agente
func (m *MultiAgentHandoff) RegisterAgent(params AgentInfo) (interface{}, error) {
	if params.AgentID == "" {
		return HandoffResult{Success: false}, fmt.Errorf("agent_id obrigatório")
	}

	if params.MaxContextSize == 0 {
		params.MaxContextSize = 10 * 1024 * 1024 // 10MB default
	}

	params.Status = "idle"
	params.Rating = 5.0

	m.agentRegistry[params.AgentID] = params

	return map[string]interface{}{
		"success":   true,
		"agent_id":  params.AgentID,
		"name":      params.Name,
		"status":    params.Status,
		"timestamp": time.Now(),
	}, nil
}

// InitiateConversation inicia conversa entre agentes
func (m *MultiAgentHandoff) InitiateConversation(params InitiateConversationParams) (interface{}, error) {
	if params.InitiatorID == "" || params.ParticipantID == "" {
		return HandoffResult{Success: false}, fmt.Errorf("IDs de agentes obrigatórios")
	}

	// Verificar se agentes existem
	if _, exists := m.agentRegistry[params.InitiatorID]; !exists {
		return HandoffResult{Success: false}, fmt.Errorf("agente iniciador não registrado")
	}

	if _, exists := m.agentRegistry[params.ParticipantID]; !exists {
		return HandoffResult{Success: false}, fmt.Errorf("agente participante não registrado")
	}

	conversationID := fmt.Sprintf("conv_%d", time.Now().UnixNano())

	conversation := Conversation{
		ConversationID: conversationID,
		Participants:   []string{params.InitiatorID, params.ParticipantID},
		Context:        params.Context,
		State:          "active",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Messages:       make([]ConversationMessage, 0),
	}

	m.activeConversations[conversationID] = conversation

	return HandoffResult{
		Success:        true,
		ConversationID: conversationID,
		FromAgent:      params.InitiatorID,
		ToAgent:        params.ParticipantID,
		Message:        fmt.Sprintf("Conversa iniciada entre %s e %s sobre %s", params.InitiatorID, params.ParticipantID, params.Topic),
		Timestamp:      time.Now(),
	}, nil
}

// TransferContext transfere contexto entre agentes
func (m *MultiAgentHandoff) TransferContext(params TransferContextParams) (interface{}, error) {
	if params.FromAgent == "" || params.ToAgent == "" {
		return HandoffResult{Success: false}, fmt.Errorf("agentes origem e destino obrigatórios")
	}

	startTime := time.Now()

	// Verificar conversa
	conv, exists := m.activeConversations[params.ConversationID]
	if !exists {
		return HandoffResult{Success: false}, fmt.Errorf("conversa não encontrada")
	}

	// Registrar handoff
	handoffID := fmt.Sprintf("handoff_%d", time.Now().UnixNano())

	contextSize := len(fmt.Sprint(params.Context))
	handoff := HandoffRecord{
		HandoffID:      handoffID,
		FromAgent:      params.FromAgent,
		ToAgent:        params.ToAgent,
		ConversationID: params.ConversationID,
		Reason:         params.Reason,
		ContextSize:    contextSize,
		Status:         "success",
		Timestamp:      time.Now(),
		ExecutionTime:  time.Since(startTime).Milliseconds(),
	}

	m.handoffHistory = append(m.handoffHistory, handoff)

	// Armazenar contexto na bridge
	stateID := fmt.Sprintf("state_%d", time.Now().UnixNano())
	contextState := ContextState{
		StateID:      stateID,
		AgentID:      params.ToAgent,
		Data:         params.Context,
		Timestamp:    time.Now(),
		ExpiresIn:    3600, // 1 hora
		IsCompressed: false,
	}

	m.contextBridge[stateID] = contextState

	// Atualizar estado da conversa
	conv.UpdatedAt = time.Now()
	conv.Context = params.Context
	m.activeConversations[params.ConversationID] = conv

	// Adicionar mensagem de handoff
	msg := ConversationMessage{
		Timestamp: time.Now(),
		From:      params.FromAgent,
		To:        params.ToAgent,
		Type:      "context",
		Content: map[string]interface{}{
			"state_id": stateID,
			"reason":   params.Reason,
		},
		Priority: 5,
	}

	conv.Messages = append(conv.Messages, msg)
	m.activeConversations[params.ConversationID] = conv

	return HandoffResult{
		Success:         true,
		HandoffID:       handoffID,
		ConversationID:  params.ConversationID,
		FromAgent:       params.FromAgent,
		ToAgent:         params.ToAgent,
		ContextReceived: params.Context,
		Message:         fmt.Sprintf("Contexto transferido com sucesso. Handoff ID: %s", handoffID),
		Timestamp:       time.Now(),
	}, nil
}

// GetConversationState obtém estado da conversa
func (m *MultiAgentHandoff) GetConversationState(params GetConversationParams) (interface{}, error) {
	conv, exists := m.activeConversations[params.ConversationID]
	if !exists {
		return nil, fmt.Errorf("conversa não encontrada")
	}

	return map[string]interface{}{
		"conversation_id": conv.ConversationID,
		"participants":    conv.Participants,
		"state":           conv.State,
		"context":         conv.Context,
		"message_count":   len(conv.Messages),
		"created_at":      conv.CreatedAt,
		"updated_at":      conv.UpdatedAt,
		"timestamp":       time.Now(),
	}, nil
}

// SendMessage envia mensagem entre agentes
func (m *MultiAgentHandoff) SendMessage(params HandoffSendMessageParams) (interface{}, error) {
	if params.ConversationID == "" {
		return nil, fmt.Errorf("conversation_id obrigatório")
	}

	conv, exists := m.activeConversations[params.ConversationID]
	if !exists {
		return nil, fmt.Errorf("conversa não encontrada")
	}

	msg := ConversationMessage{
		Timestamp: time.Now(),
		From:      params.FromAgent,
		To:        params.ToAgent,
		Type:      params.Type,
		Content:   params.Content,
		Priority:  params.Priority,
	}

	conv.Messages = append(conv.Messages, msg)
	conv.UpdatedAt = time.Now()
	m.activeConversations[params.ConversationID] = conv

	return map[string]interface{}{
		"success":    true,
		"message_id": fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		"timestamp":  time.Now(),
	}, nil
}

// CompleteHandoff completa handoff
func (m *MultiAgentHandoff) CompleteHandoff(params CompleteHandoffParams) (interface{}, error) {
	conv, exists := m.activeConversations[params.ConversationID]
	if !exists {
		return nil, fmt.Errorf("conversa não encontrada")
	}

	conv.State = "closed"
	conv.UpdatedAt = time.Now()
	m.activeConversations[params.ConversationID] = conv

	return map[string]interface{}{
		"success":         true,
		"conversation_id": params.ConversationID,
		"status":          "closed",
		"timestamp":       time.Now(),
	}, nil
}

// GetConversationParams parâmetros para obter conversa
type GetConversationParams struct {
	ConversationID string `json:"conversation_id" description:"ID da conversa"`
}

// HandoffSendMessageParams parâmetros para enviar mensagem em handoff
type HandoffSendMessageParams struct {
	ConversationID string                 `json:"conversation_id" description:"ID da conversa"`
	FromAgent      string                 `json:"from_agent" description:"ID do remetente"`
	ToAgent        string                 `json:"to_agent" description:"ID do destinatário"`
	Type           string                 `json:"type" description:"Tipo (request, response, context, status)"`
	Content        map[string]interface{} `json:"content" description:"Conteúdo da mensagem"`
	Priority       int                    `json:"priority" description:"Prioridade 1-5"`
}

// CompleteHandoffParams parâmetros para completar handoff
type CompleteHandoffParams struct {
	ConversationID string `json:"conversation_id" description:"ID da conversa"`
	Status         string `json:"status" description:"Status final (success, failed)"`
}
