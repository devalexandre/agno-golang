package os

// HealthResponse represents the health check response
type HealthResponse struct {
	Status string `json:"status"`
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
	ID               *string                `json:"id,omitempty"`
	Name             *string                `json:"name,omitempty"`
	DBID             *string                `json:"db_id,omitempty"`
	Model            *ModelResponse         `json:"model,omitempty"`
	Tools            *map[string]interface{} `json:"tools,omitempty"`
	Sessions         *map[string]interface{} `json:"sessions,omitempty"`
	Knowledge        *map[string]interface{} `json:"knowledge,omitempty"`
	Memory           *map[string]interface{} `json:"memory,omitempty"`
	Reasoning        *map[string]interface{} `json:"reasoning,omitempty"`
	DefaultTools     *map[string]interface{} `json:"default_tools,omitempty"`
	SystemMessage    *map[string]interface{} `json:"system_message,omitempty"`
	ExtraMessages    *map[string]interface{} `json:"extra_messages,omitempty"`
	ResponseSettings *map[string]interface{} `json:"response_settings,omitempty"`
	Streaming        *map[string]interface{} `json:"streaming,omitempty"`
	Metadata         *map[string]interface{} `json:"metadata,omitempty"`
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
	OSID             string                     `json:"os_id"`
	Name             *string                    `json:"name,omitempty"`
	Description      *string                    `json:"description,omitempty"`
	AvailableModels  *[]string                  `json:"available_models,omitempty"`
	Databases        []string                   `json:"databases"`
	Chat             *map[string]interface{}    `json:"chat,omitempty"`
	Session          *map[string]interface{}    `json:"session,omitempty"`
	Metrics          *map[string]interface{}    `json:"metrics,omitempty"`
	Memory           *map[string]interface{}    `json:"memory,omitempty"`
	Knowledge        *map[string]interface{}    `json:"knowledge,omitempty"`
	Evals            *map[string]interface{}    `json:"evals,omitempty"`
	Agents           []AgentSummaryResponse     `json:"agents"`
	Teams            []TeamSummaryResponse      `json:"teams"`
	Workflows        []WorkflowSummaryResponse  `json:"workflows"`
	Interfaces       []InterfaceResponse        `json:"interfaces"`
}