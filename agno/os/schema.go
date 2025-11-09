package os

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/team"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status         string `json:"status"`
	InstantiatedAt string `json:"instantiated_at,omitempty"`
}

// Model represents a simple model reference for the models endpoint
type Model struct {
	ID       *string `json:"id,omitempty"`
	Provider *string `json:"provider,omitempty"`
}

// ModelResponse represents model information
type ModelResponse struct {
	Name     *string `json:"name,omitempty"`
	Model    *string `json:"model,omitempty"`
	Provider *string `json:"provider,omitempty"`
}

// AgentSummaryResponse represents a simplified agent for config endpoint
type AgentSummaryResponse struct {
	ID          *string `json:"id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	DBID        *string `json:"db_id,omitempty"`
}

// AgentResponse represents a complete agent with all configurations
type AgentResponse struct {
	ID                 *string                 `json:"id,omitempty"`
	Name               *string                 `json:"name,omitempty"`
	Description        *string                 `json:"description,omitempty"`
	DBID               *string                 `json:"db_id,omitempty"`
	Model              *ModelResponse          `json:"model,omitempty"`
	Tools              *map[string]interface{} `json:"tools,omitempty"`
	Sessions           *map[string]interface{} `json:"sessions,omitempty"`
	Knowledge          *map[string]interface{} `json:"knowledge,omitempty"`
	Memory             *map[string]interface{} `json:"memory,omitempty"`
	Reasoning          *map[string]interface{} `json:"reasoning,omitempty"`
	DefaultTools       *map[string]interface{} `json:"default_tools,omitempty"`
	SystemMessage      *map[string]interface{} `json:"system_message,omitempty"`
	ExtraMessages      *map[string]interface{} `json:"extra_messages,omitempty"`
	ResponseSettings   *map[string]interface{} `json:"response_settings,omitempty"`
	Streaming          *map[string]interface{} `json:"streaming,omitempty"`
	Metadata           *map[string]interface{} `json:"metadata,omitempty"`
	Guardrails         *map[string]interface{} `json:"guardrails,omitempty"`
	MaxToolCalls       *int                    `json:"max_tool_calls,omitempty"`
	MaxRetries         *int                    `json:"max_retries,omitempty"`
	EnableAgenticState *bool                   `json:"enable_agentic_state,omitempty"`
}

// TeamSummaryResponse represents a simplified team for config endpoint
type TeamSummaryResponse struct {
	ID          *string `json:"id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	DBID        *string `json:"db_id,omitempty"`
}

// WorkflowSummaryResponse represents a simplified workflow for config endpoint
type WorkflowSummaryResponse struct {
	ID          *string `json:"id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	DBID        *string `json:"db_id,omitempty"`
}

// InterfaceResponse represents an interface configuration
type InterfaceResponse struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	Route   string `json:"route"`
}

// ResponseDatabaseConfig represents database configuration for responses
type ResponseDatabaseConfig struct {
	DBID         string                 `json:"db_id"`
	DomainConfig map[string]interface{} `json:"domain_config"`
}

// ConfigResponse represents the complete OS configuration
type ConfigResponse struct {
	OSID            string                    `json:"os_id"`
	Name            *string                   `json:"name,omitempty"`
	Description     *string                   `json:"description,omitempty"`
	AvailableModels *[]string                 `json:"available_models,omitempty"`
	Databases       []string                  `json:"databases"`
	Chat            *map[string]interface{}   `json:"chat,omitempty"`
	Session         *map[string]interface{}   `json:"session,omitempty"`
	Metrics         *map[string]interface{}   `json:"metrics,omitempty"`
	Memory          *map[string]interface{}   `json:"memory,omitempty"`
	Knowledge       *map[string]interface{}   `json:"knowledge,omitempty"`
	Evals           *map[string]interface{}   `json:"evals,omitempty"`
	Agents          []AgentSummaryResponse    `json:"agents"`
	Teams           []TeamSummaryResponse     `json:"teams"`
	Workflows       []WorkflowSummaryResponse `json:"workflows"`
	Interfaces      []InterfaceResponse       `json:"interfaces"`
}

// ErrorResponse represents error responses
type ErrorResponse struct {
	Detail    string  `json:"detail"`
	ErrorCode *string `json:"error_code,omitempty"`
}

// SessionSchema represents session information
type SessionSchema struct {
	SessionID    string                  `json:"session_id"`
	SessionName  string                  `json:"session_name"`
	SessionState *map[string]interface{} `json:"session_state,omitempty"`
	CreatedAt    *time.Time              `json:"created_at,omitempty"`
	UpdatedAt    *time.Time              `json:"updated_at,omitempty"`
}

// CreateSessionRequest represents a request to create a session
type CreateSessionRequest struct {
	SessionID    *string                 `json:"session_id,omitempty"`
	SessionName  *string                 `json:"session_name,omitempty"`
	SessionState *map[string]interface{} `json:"session_state,omitempty"`
	Metadata     *map[string]interface{} `json:"metadata,omitempty"`
	UserID       *string                 `json:"user_id,omitempty"`
	AgentID      *string                 `json:"agent_id,omitempty"`
	TeamID       *string                 `json:"team_id,omitempty"`
	WorkflowID   *string                 `json:"workflow_id,omitempty"`
}

// UpdateSessionRequest represents a request to update a session
type UpdateSessionRequest struct {
	SessionName  *string                 `json:"session_name,omitempty"`
	SessionState *map[string]interface{} `json:"session_state,omitempty"`
	Metadata     *map[string]interface{} `json:"metadata,omitempty"`
	Summary      *map[string]interface{} `json:"summary,omitempty"`
}

// DeleteSessionRequest represents a request to delete sessions
type DeleteSessionRequest struct {
	SessionIDs   []string `json:"session_ids"`
	SessionTypes []string `json:"session_types"`
}

// AgentSessionDetailSchema represents detailed agent session information
type AgentSessionDetailSchema struct {
	UserID         *string                   `json:"user_id,omitempty"`
	AgentSessionID string                    `json:"agent_session_id"`
	SessionID      string                    `json:"session_id"`
	SessionName    string                    `json:"session_name"`
	SessionSummary *map[string]interface{}   `json:"session_summary,omitempty"`
	SessionState   *map[string]interface{}   `json:"session_state,omitempty"`
	AgentID        *string                   `json:"agent_id,omitempty"`
	TotalTokens    *int                      `json:"total_tokens,omitempty"`
	AgentData      *map[string]interface{}   `json:"agent_data,omitempty"`
	Metrics        *map[string]interface{}   `json:"metrics,omitempty"`
	Metadata       *map[string]interface{}   `json:"metadata,omitempty"`
	ChatHistory    *[]map[string]interface{} `json:"chat_history,omitempty"`
	CreatedAt      *time.Time                `json:"created_at,omitempty"`
	UpdatedAt      *time.Time                `json:"updated_at,omitempty"`
}

// TeamSessionDetailSchema represents detailed team session information
type TeamSessionDetailSchema struct {
	SessionID      string                    `json:"session_id"`
	SessionName    string                    `json:"session_name"`
	UserID         *string                   `json:"user_id,omitempty"`
	TeamID         *string                   `json:"team_id,omitempty"`
	SessionSummary *map[string]interface{}   `json:"session_summary,omitempty"`
	SessionState   *map[string]interface{}   `json:"session_state,omitempty"`
	Metrics        *map[string]interface{}   `json:"metrics,omitempty"`
	TeamData       *map[string]interface{}   `json:"team_data,omitempty"`
	Metadata       *map[string]interface{}   `json:"metadata,omitempty"`
	ChatHistory    *[]map[string]interface{} `json:"chat_history,omitempty"`
	CreatedAt      *time.Time                `json:"created_at,omitempty"`
	UpdatedAt      *time.Time                `json:"updated_at,omitempty"`
	TotalTokens    *int                      `json:"total_tokens,omitempty"`
}

// WorkflowSessionDetailSchema represents detailed workflow session information
type WorkflowSessionDetailSchema struct {
	UserID       *string                 `json:"user_id,omitempty"`
	WorkflowID   *string                 `json:"workflow_id,omitempty"`
	WorkflowName *string                 `json:"workflow_name,omitempty"`
	SessionID    string                  `json:"session_id"`
	SessionName  string                  `json:"session_name"`
	SessionData  *map[string]interface{} `json:"session_data,omitempty"`
	SessionState *map[string]interface{} `json:"session_state,omitempty"`
	WorkflowData *map[string]interface{} `json:"workflow_data,omitempty"`
	Metadata     *map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    *int64                  `json:"created_at,omitempty"`
	UpdatedAt    *int64                  `json:"updated_at,omitempty"`
}

// RunSchema represents a run/execution record
type RunSchema struct {
	RunID             string                    `json:"run_id"`
	ParentRunID       *string                   `json:"parent_run_id,omitempty"`
	AgentID           *string                   `json:"agent_id,omitempty"`
	UserID            *string                   `json:"user_id,omitempty"`
	RunInput          *string                   `json:"run_input,omitempty"`
	Content           *interface{}              `json:"content,omitempty"`
	RunResponseFormat *string                   `json:"run_response_format,omitempty"`
	ReasoningContent  *string                   `json:"reasoning_content,omitempty"`
	ReasoningSteps    *[]map[string]interface{} `json:"reasoning_steps,omitempty"`
	Metrics           *map[string]interface{}   `json:"metrics,omitempty"`
	Messages          *[]map[string]interface{} `json:"messages,omitempty"`
	Tools             *[]map[string]interface{} `json:"tools,omitempty"`
	Events            *[]map[string]interface{} `json:"events,omitempty"`
	CreatedAt         *time.Time                `json:"created_at,omitempty"`
	References        *[]map[string]interface{} `json:"references,omitempty"`
	ReasoningMessages *[]map[string]interface{} `json:"reasoning_messages,omitempty"`
	Images            *[]map[string]interface{} `json:"images,omitempty"`
	Videos            *[]map[string]interface{} `json:"videos,omitempty"`
	Audio             *[]map[string]interface{} `json:"audio,omitempty"`
	Files             *[]map[string]interface{} `json:"files,omitempty"`
	ResponseAudio     *map[string]interface{}   `json:"response_audio,omitempty"`
	InputMedia        *map[string]interface{}   `json:"input_media,omitempty"`
}

// TeamRunSchema represents a team run/execution record
type TeamRunSchema struct {
	RunID             string                    `json:"run_id"`
	ParentRunID       *string                   `json:"parent_run_id,omitempty"`
	TeamID            *string                   `json:"team_id,omitempty"`
	Content           *interface{}              `json:"content,omitempty"`
	ReasoningContent  *string                   `json:"reasoning_content,omitempty"`
	ReasoningSteps    *[]map[string]interface{} `json:"reasoning_steps,omitempty"`
	RunInput          *string                   `json:"run_input,omitempty"`
	RunResponseFormat *string                   `json:"run_response_format,omitempty"`
	Metrics           *map[string]interface{}   `json:"metrics,omitempty"`
	Tools             *[]map[string]interface{} `json:"tools,omitempty"`
	Messages          *[]map[string]interface{} `json:"messages,omitempty"`
	Events            *[]map[string]interface{} `json:"events,omitempty"`
	CreatedAt         *time.Time                `json:"created_at,omitempty"`
	References        *[]map[string]interface{} `json:"references,omitempty"`
	ReasoningMessages *[]map[string]interface{} `json:"reasoning_messages,omitempty"`
	InputMedia        *map[string]interface{}   `json:"input_media,omitempty"`
	Images            *[]map[string]interface{} `json:"images,omitempty"`
	Videos            *[]map[string]interface{} `json:"videos,omitempty"`
	Audio             *[]map[string]interface{} `json:"audio,omitempty"`
	Files             *[]map[string]interface{} `json:"files,omitempty"`
	ResponseAudio     *map[string]interface{}   `json:"response_audio,omitempty"`
}

// WorkflowRunSchema represents a workflow run/execution record
type WorkflowRunSchema struct {
	RunID             string                    `json:"run_id"`
	RunInput          *string                   `json:"run_input,omitempty"`
	Events            *[]map[string]interface{} `json:"events,omitempty"`
	WorkflowID        *string                   `json:"workflow_id,omitempty"`
	UserID            *string                   `json:"user_id,omitempty"`
	Content           *interface{}              `json:"content,omitempty"`
	ContentType       *string                   `json:"content_type,omitempty"`
	Status            *string                   `json:"status,omitempty"`
	StepResults       *[]map[string]interface{} `json:"step_results,omitempty"`
	StepExecutorRuns  *[]map[string]interface{} `json:"step_executor_runs,omitempty"`
	Metrics           *map[string]interface{}   `json:"metrics,omitempty"`
	CreatedAt         *int64                    `json:"created_at,omitempty"`
	ReasoningContent  *string                   `json:"reasoning_content,omitempty"`
	ReasoningSteps    *[]map[string]interface{} `json:"reasoning_steps,omitempty"`
	References        *[]map[string]interface{} `json:"references,omitempty"`
	ReasoningMessages *[]map[string]interface{} `json:"reasoning_messages,omitempty"`
	Images            *[]map[string]interface{} `json:"images,omitempty"`
	Videos            *[]map[string]interface{} `json:"videos,omitempty"`
	Audio             *[]map[string]interface{} `json:"audio,omitempty"`
	Files             *[]map[string]interface{} `json:"files,omitempty"`
	ResponseAudio     *map[string]interface{}   `json:"response_audio,omitempty"`
}

// SortOrder represents sort order for pagination
type SortOrder string

const (
	SortOrderASC  SortOrder = "asc"
	SortOrderDESC SortOrder = "desc"
)

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Page         int     `json:"page"`
	Limit        int     `json:"limit"`
	TotalPages   int     `json:"total_pages"`
	TotalCount   int     `json:"total_count"`
	SearchTimeMs float64 `json:"search_time_ms"`
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data []interface{}  `json:"data"`
	Meta PaginationInfo `json:"meta"`
}

// filterMeaningfulConfig filters out fields that match their default values
func filterMeaningfulConfig(config map[string]interface{}, defaults map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})

	for key, value := range config {
		if value == nil {
			continue
		}

		// Skip if value matches the default exactly
		if defaultValue, exists := defaults[key]; exists && reflect.DeepEqual(value, defaultValue) {
			continue
		}

		// Keep non-default values
		filtered[key] = value
	}

	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

// AgentResponseFromAgent creates an AgentResponse from an agent.Agent
func AgentResponseFromAgent(agentInstance *agent.Agent) (*AgentResponse, error) {
	if agentInstance == nil {
		return nil, fmt.Errorf("agent cannot be nil")
	}

	// Define default values for filtering
	agentDefaults := map[string]interface{}{
		// Sessions defaults
		"add_history_to_context":   false,
		"num_history_runs":         3,
		"enable_session_summaries": false,
		"search_session_history":   false,
		"cache_session":            false,
		// Knowledge defaults
		"add_references":                   false,
		"references_format":                "json",
		"enable_agentic_knowledge_filters": false,
		// Memory defaults
		"enable_agentic_memory": false,
		"enable_user_memories":  false,
		// Reasoning defaults
		"reasoning":           false,
		"reasoning_min_steps": 1,
		"reasoning_max_steps": 10,
		// Default tools defaults
		"read_chat_history":      false,
		"search_knowledge":       true,
		"update_knowledge":       false,
		"read_tool_call_history": false,
		// System message defaults
		"system_message_role":     "system",
		"build_context":           true,
		"markdown":                false,
		"add_name_to_context":     false,
		"add_datetime_to_context": false,
		"add_location_to_context": false,
		"resolve_in_context":      true,
		// Extra messages defaults
		"user_message_role":  "user",
		"build_user_context": true,
		// Response settings defaults
		"retries":               0,
		"delay_between_retries": 1,
		"exponential_backoff":   false,
		"parse_response":        true,
		"use_json_mode":         false,
		// Streaming defaults
		"stream_events":             false,
		"stream_intermediate_steps": false,
	}

	// Build tools info
	toolsInfo := map[string]interface{}{
		"tool_call_limit": agentInstance.GetToolCallLimit(),
		"tool_choice":     agentInstance.GetToolChoice(),
	}

	// Get session table name
	sessionTable := ""
	if agentInstance.GetStorage() != nil {
		sessionTable = "agno_sessions" // Default session table name
	}

	// Build sessions info
	sessionsInfo := map[string]interface{}{
		"session_table":            sessionTable,
		"add_history_to_context":   agentInstance.GetAddHistoryToMessages(),
		"enable_session_summaries": agentInstance.GetEnableSessionSummaries(),
		"num_history_runs":         3, // Default
		"search_session_history":   false,
		"cache_session":            false,
	}

	// Build knowledge info
	knowledgeInfo := map[string]interface{}{
		"enable_agentic_knowledge_filters": false,
		"references_format":                "json",
	}

	// Build memory info
	memoryInfo := map[string]interface{}{
		"enable_agentic_memory": agentInstance.GetEnableAgenticMemory(),
		"enable_user_memories":  false,
	}

	// Build reasoning info
	reasoningInfo := map[string]interface{}{
		"reasoning":           agentInstance.GetEnableReasoning(),
		"reasoning_min_steps": 1,
		"reasoning_max_steps": 10,
	}

	// Build default tools info
	defaultToolsInfo := map[string]interface{}{
		"read_chat_history":      agentInstance.GetReadChatHistory(),
		"search_knowledge":       true,
		"update_knowledge":       false,
		"read_tool_call_history": agentInstance.GetReadToolCallHistory(),
	}

	// Build system message info
	systemMessageInfo := map[string]interface{}{
		"system_message_role":     "system",
		"build_context":           true,
		"markdown":                false,
		"add_name_to_context":     false,
		"add_datetime_to_context": false,
		"add_location_to_context": false,
		"resolve_in_context":      true,
	}

	// Build extra messages info
	extraMessagesInfo := map[string]interface{}{
		"user_message_role":  "user",
		"build_user_context": true,
	}

	// Build response settings info
	responseSettingsInfo := map[string]interface{}{
		"retries":               0,
		"delay_between_retries": 1,
		"exponential_backoff":   false,
		"parse_response":        true,
		"use_json_mode":         false,
	}

	// Build streaming info
	streamingInfo := map[string]interface{}{
		"stream_events":             false,
		"stream_intermediate_steps": false,
	}

	// Build model response if model exists
	var modelResponse *ModelResponse
	if agentInstance.GetModel() != nil {
		modelID := agentInstance.GetModel().GetID()
		modelProvider := "unknown"
		modelName := modelID // Using ID as name since GetName() doesn't exist

		modelResponse = &ModelResponse{
			Name:     &modelName,
			Model:    &modelID,
			Provider: &modelProvider,
		}
	}

	// Get agent ID and name
	agentID := agentInstance.GetID()
	agentName := agentInstance.GetName()
	agentDescription := agentInstance.GetRole()

	response := &AgentResponse{
		ID:          &agentID,
		Name:        &agentName,
		Description: &agentDescription,
		Model:       modelResponse,
		Metadata:    &map[string]interface{}{}, // agentInstance.GetMetadata() returns map[string]interface{}
	}

	// Only add non-default configurations
	if tools := filterMeaningfulConfig(toolsInfo, map[string]interface{}{}); tools != nil {
		response.Tools = &tools
	}
	if sessions := filterMeaningfulConfig(sessionsInfo, agentDefaults); sessions != nil {
		response.Sessions = &sessions
	}
	if knowledge := filterMeaningfulConfig(knowledgeInfo, agentDefaults); knowledge != nil {
		response.Knowledge = &knowledge
	}
	if memory := filterMeaningfulConfig(memoryInfo, agentDefaults); memory != nil {
		response.Memory = &memory
	}
	if reasoning := filterMeaningfulConfig(reasoningInfo, agentDefaults); reasoning != nil {
		response.Reasoning = &reasoning
	}
	if defaultTools := filterMeaningfulConfig(defaultToolsInfo, agentDefaults); defaultTools != nil {
		response.DefaultTools = &defaultTools
	}
	if systemMessage := filterMeaningfulConfig(systemMessageInfo, agentDefaults); systemMessage != nil {
		response.SystemMessage = &systemMessage
	}
	if extraMessages := filterMeaningfulConfig(extraMessagesInfo, agentDefaults); extraMessages != nil {
		response.ExtraMessages = &extraMessages
	}
	if responseSettings := filterMeaningfulConfig(responseSettingsInfo, agentDefaults); responseSettings != nil {
		response.ResponseSettings = &responseSettings
	}
	if streaming := filterMeaningfulConfig(streamingInfo, agentDefaults); streaming != nil {
		response.Streaming = &streaming
	}

	return response, nil
}

// TeamResponseFromTeam creates a TeamResponse from a team.Team
func TeamResponseFromTeam(teamInstance *team.Team) (*TeamResponse, error) {
	if teamInstance == nil {
		return nil, fmt.Errorf("team cannot be nil")
	}

	// Define default values for filtering
	teamDefaults := map[string]interface{}{
		// Sessions defaults
		"add_history_to_context":   false,
		"num_history_runs":         3,
		"enable_session_summaries": false,
		"cache_session":            false,
		// Knowledge defaults
		"add_references":                   false,
		"references_format":                "json",
		"enable_agentic_knowledge_filters": false,
		// Memory defaults
		"enable_agentic_memory": false,
		"enable_user_memories":  false,
		// Reasoning defaults
		"reasoning":           false,
		"reasoning_min_steps": 1,
		"reasoning_max_steps": 10,
		// Default tools defaults
		"search_knowledge":            true,
		"read_team_history":           false,
		"get_member_information_tool": false,
		// System message defaults
		"system_message_role":     "system",
		"markdown":                false,
		"add_datetime_to_context": false,
		"add_location_to_context": false,
		"resolve_in_context":      true,
		// Response settings defaults
		"parse_response": true,
		"use_json_mode":  false,
		// Streaming defaults
		"stream_events":             false,
		"stream_intermediate_steps": false,
		"stream_member_events":      false,
	}

	// Build tools info
	toolsInfo := map[string]interface{}{
		"tool_call_limit": 0,  // teamInstance doesn't have this field directly
		"tool_choice":     "", // teamInstance doesn't have this field directly
	}

	// Get session table name
	sessionTable := ""
	// Note: We can't directly access teamInstance.storage as it's unexported
	// For now, we'll leave sessionTable as empty string
	// In a real implementation, we might need to add a getter method to the Team struct

	// Build sessions info
	// Note: We can't directly access unexported fields, so we'll use default values
	// In a real implementation, we would need getter methods for these fields
	sessionsInfo := map[string]interface{}{
		"session_table":            sessionTable,
		"add_history_to_context":   false, // Default value
		"enable_session_summaries": false, // Default value
		"num_history_runs":         3,     // Default value
		"cache_session":            false,
	}

	// Build knowledge info
	knowledgeInfo := map[string]interface{}{
		"enable_agentic_knowledge_filters": false,
		"references_format":                "json",
	}

	// Build memory info
	// Note: We can't directly access unexported fields, so we'll use default values
	memoryInfo := map[string]interface{}{
		"enable_agentic_memory": false, // Default value
		"enable_user_memories":  false, // Default value
	}

	// Build reasoning info
	reasoningInfo := map[string]interface{}{
		"reasoning":           false,
		"reasoning_min_steps": 1,
		"reasoning_max_steps": 10,
	}

	// Build default tools info
	// Note: We can't directly access unexported fields, so we'll use default values
	defaultToolsInfo := map[string]interface{}{
		"search_knowledge":            true,
		"read_team_history":           false, // Default value
		"get_member_information_tool": false,
	}

	// Build system message info
	systemMessageInfo := map[string]interface{}{
		"system_message_role":     "system",
		"markdown":                false,
		"add_datetime_to_context": false,
		"add_location_to_context": false,
		"resolve_in_context":      true,
	}

	// Build response settings info
	responseSettingsInfo := map[string]interface{}{
		"parse_response": true,
		"use_json_mode":  false,
	}

	// Build streaming info
	streamingInfo := map[string]interface{}{
		"stream_events":             false,
		"stream_intermediate_steps": false,
		"stream_member_events":      false,
	}

	// Build model response if model exists
	// Note: We can't directly access teamInstance.model as it's unexported
	// For now, we'll leave modelResponse as nil
	// In a real implementation, we would need a getter method for the model
	var modelResponse *ModelResponse

	// Get team ID and name
	teamID := teamInstance.GetName()
	teamName := teamInstance.GetName()
	teamDescription := teamInstance.GetRole()

	response := &TeamResponse{
		ID:          &teamID,
		Name:        &teamName,
		Description: &teamDescription,
		Model:       modelResponse,
		Metadata:    &map[string]interface{}{}, // Using empty map as there's no Metadata field
	}

	// Only add non-default configurations
	if tools := filterMeaningfulConfig(toolsInfo, map[string]interface{}{}); tools != nil {
		response.Tools = &tools
	}
	if sessions := filterMeaningfulConfig(sessionsInfo, teamDefaults); sessions != nil {
		response.Sessions = &sessions
	}
	if knowledge := filterMeaningfulConfig(knowledgeInfo, teamDefaults); knowledge != nil {
		response.Knowledge = &knowledge
	}
	if memory := filterMeaningfulConfig(memoryInfo, teamDefaults); memory != nil {
		response.Memory = &memory
	}
	if reasoning := filterMeaningfulConfig(reasoningInfo, teamDefaults); reasoning != nil {
		response.Reasoning = &reasoning
	}
	if defaultTools := filterMeaningfulConfig(defaultToolsInfo, teamDefaults); defaultTools != nil {
		response.DefaultTools = &defaultTools
	}
	if systemMessage := filterMeaningfulConfig(systemMessageInfo, teamDefaults); systemMessage != nil {
		response.SystemMessage = &systemMessage
	}
	if responseSettings := filterMeaningfulConfig(responseSettingsInfo, teamDefaults); responseSettings != nil {
		response.ResponseSettings = &responseSettings
	}
	if streaming := filterMeaningfulConfig(streamingInfo, teamDefaults); streaming != nil {
		response.Streaming = &streaming
	}

	return response, nil
}

// WorkflowResponseFromWorkflow creates a WorkflowResponse from a workflow
func WorkflowResponseFromWorkflow(workflow *v2.Workflow) (*WorkflowResponse, error) {
	if workflow == nil {
		return nil, fmt.Errorf("workflow cannot be nil")
	}

	// Get workflow ID and name
	workflowID := workflow.WorkflowID
	workflowName := workflow.Name

	response := &WorkflowResponse{
		ID:          &workflowID,
		Name:        &workflowName,
		Description: &workflow.Description,
		// Metadata:    workflow.Metadata, // Workflow doesn't have Metadata field
	}

	// Process steps if they exist
	// Note: WorkflowSteps is an interface, so we can't range over it directly
	// We'll leave steps as nil for now since we can't easily access them
	/*
		if workflow.Steps != nil {
			steps := make([]map[string]interface{}, 0)
			for _, step := range workflow.Steps {
				stepMap := map[string]interface{}{
					"name":        step.Name,
					"description": step.Description,
				}
				steps = append(steps, stepMap)
			}
			response.Steps = &steps
		}
	*/

	return response, nil
}

// TeamResponse represents a complete team with all configurations
type TeamResponse struct {
	ID               *string                 `json:"id,omitempty"`
	Name             *string                 `json:"name,omitempty"`
	DBID             *string                 `json:"db_id,omitempty"`
	Description      *string                 `json:"description,omitempty"`
	Model            *ModelResponse          `json:"model,omitempty"`
	Tools            *map[string]interface{} `json:"tools,omitempty"`
	Sessions         *map[string]interface{} `json:"sessions,omitempty"`
	Knowledge        *map[string]interface{} `json:"knowledge,omitempty"`
	Memory           *map[string]interface{} `json:"memory,omitempty"`
	Reasoning        *map[string]interface{} `json:"reasoning,omitempty"`
	DefaultTools     *map[string]interface{} `json:"default_tools,omitempty"`
	SystemMessage    *map[string]interface{} `json:"system_message,omitempty"`
	ResponseSettings *map[string]interface{} `json:"response_settings,omitempty"`
	Streaming        *map[string]interface{} `json:"streaming,omitempty"`
	Members          *[]interface{}          `json:"members,omitempty"`
	Metadata         *map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowResponse represents a complete workflow with all configurations
type WorkflowResponse struct {
	ID          *string                   `json:"id,omitempty"`
	Name        *string                   `json:"name,omitempty"`
	DBID        *string                   `json:"db_id,omitempty"`
	Description *string                   `json:"description,omitempty"`
	InputSchema *map[string]interface{}   `json:"input_schema,omitempty"`
	Steps       *[]map[string]interface{} `json:"steps,omitempty"`
	Agent       *AgentResponse            `json:"agent,omitempty"`
	Team        *TeamResponse             `json:"team,omitempty"`
	Metadata    *map[string]interface{}   `json:"metadata,omitempty"`
}

// GetAgentModelInfo extracts model information from an agent
func GetAgentModelInfo(agent *agent.Agent) (string, string) {
	modelName := "gpt-4"
	modelProvider := "openai"

	if agent.GetModel() != nil {
		modelID := agent.GetModel().GetID()
		if modelID != "" {
			modelName = modelID
			// Detect provider from model name
			modelLower := strings.ToLower(modelID)
			if strings.Contains(modelLower, "llama") ||
				strings.Contains(modelLower, "mistral") ||
				strings.Contains(modelLower, "qwen") ||
				strings.Contains(modelLower, "phi") {
				modelProvider = "ollama"
			} else if strings.Contains(modelLower, "gemini") {
				modelProvider = "google"
			} else if strings.Contains(modelLower, "gpt") ||
				strings.Contains(modelLower, "o1") ||
				strings.Contains(modelLower, "claude") {
				modelProvider = "openai"
			}
		}
	}

	return modelName, modelProvider
}

// Helper function to convert interface{} to map[string]interface{}
func toMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}

	if m, ok := v.(map[string]interface{}); ok {
		return m
	}

	// Try to convert from JSON
	if data, err := json.Marshal(v); err == nil {
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err == nil {
			return result
		}
	}

	return nil
}

// Helper function to convert interface{} to []map[string]interface{}
func toMapSlice(v interface{}) []map[string]interface{} {
	if v == nil {
		return nil
	}

	if slice, ok := v.([]interface{}); ok {
		result := make([]map[string]interface{}, 0, len(slice))
		for _, item := range slice {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result
	}

	if slice, ok := v.([]map[string]interface{}); ok {
		return slice
	}

	return nil
}

// ContextKey is a type for context keys
type ContextKey string

// Context keys for request state
const (
	ContextKeyUserID       ContextKey = "user_id"
	ContextKeySessionID    ContextKey = "session_id"
	ContextKeySessionState ContextKey = "session_state"
	ContextKeyDependencies ContextKey = "dependencies"
	ContextKeyMetadata     ContextKey = "metadata"
)
