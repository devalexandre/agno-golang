package os

import (
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/team"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// DomainConfig represents the base configuration for any AgentOS domain
type DomainConfig struct {
	DisplayName *string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
}

// EvalsDomainConfig represents configuration for the Evals domain
type EvalsDomainConfig struct {
	DomainConfig
	AvailableModels []string `json:"available_models,omitempty" yaml:"available_models,omitempty"`
}

// SessionDomainConfig represents configuration for the Session domain
type SessionDomainConfig struct {
	DomainConfig
}

// KnowledgeDomainConfig represents configuration for the Knowledge domain
type KnowledgeDomainConfig struct {
	DomainConfig
}

// MetricsDomainConfig represents configuration for the Metrics domain
type MetricsDomainConfig struct {
	DomainConfig
}

// MemoryDomainConfig represents configuration for the Memory domain
type MemoryDomainConfig struct {
	DomainConfig
}

// DatabaseConfig represents configuration for a domain when used with database
type DatabaseConfig[T any] struct {
	DbID         string `json:"db_id" yaml:"db_id"`
	DomainConfig *T     `json:"domain_config,omitempty" yaml:"domain_config,omitempty"`
}

// EvalsConfig represents the full configuration for the Evals domain
type EvalsConfig struct {
	EvalsDomainConfig
	Dbs []DatabaseConfig[EvalsDomainConfig] `json:"dbs,omitempty" yaml:"dbs,omitempty"`
}

// SessionConfig represents the full configuration for the Session domain
type SessionConfig struct {
	SessionDomainConfig
	Dbs []DatabaseConfig[SessionDomainConfig] `json:"dbs,omitempty" yaml:"dbs,omitempty"`
}

// MemoryConfig represents the full configuration for the Memory domain
type MemoryConfig struct {
	MemoryDomainConfig
	Dbs []DatabaseConfig[MemoryDomainConfig] `json:"dbs,omitempty" yaml:"dbs,omitempty"`
}

// KnowledgeConfig represents the full configuration for the Knowledge domain
type KnowledgeConfig struct {
	KnowledgeDomainConfig
	Dbs []DatabaseConfig[KnowledgeDomainConfig] `json:"dbs,omitempty" yaml:"dbs,omitempty"`
}

// MetricsConfig represents the full configuration for the Metrics domain
type MetricsConfig struct {
	MetricsDomainConfig
	Dbs []DatabaseConfig[MetricsDomainConfig] `json:"dbs,omitempty" yaml:"dbs,omitempty"`
}

// ChatConfig represents configuration for the Chat interface
type ChatConfig struct {
	QuickPrompts map[string][]string `json:"quick_prompts" yaml:"quick_prompts"`
}

// AgentOSConfig represents the general configuration for an AgentOS instance
type AgentOSConfig struct {
	AvailableModels []string         `json:"available_models,omitempty" yaml:"available_models,omitempty"`
	Chat            *ChatConfig      `json:"chat,omitempty" yaml:"chat,omitempty"`
	Evals           *EvalsConfig     `json:"evals,omitempty" yaml:"evals,omitempty"`
	Knowledge       *KnowledgeConfig `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	Memory          *MemoryConfig    `json:"memory,omitempty" yaml:"memory,omitempty"`
	Metrics         *MetricsConfig   `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Session         *SessionConfig   `json:"session,omitempty" yaml:"session,omitempty"`
}

// AgentOSInterface defines the interface for AgentOS integrations
type AgentOSInterface interface {
	GetID() string
	GetName() string
	Initialize() error
	Shutdown() error
}

// AgentOSSettings represents API settings for the AgentOS
type AgentOSSettings struct {
	Port        int           `json:"port" yaml:"port" default:"7777"`
	Host        string        `json:"host" yaml:"host" default:"0.0.0.0"`
	Reload      bool          `json:"reload" yaml:"reload" default:"false"`
	Debug       bool          `json:"debug" yaml:"debug" default:"false"`
	LogLevel    string        `json:"log_level" yaml:"log_level" default:"info"`
	Timeout     time.Duration `json:"timeout" yaml:"timeout" default:"30s"`
	EnableCORS  bool          `json:"enable_cors" yaml:"enable_cors" default:"true"`
	EnableMCP   bool          `json:"enable_mcp" yaml:"enable_mcp" default:"false"`
	Telemetry   bool          `json:"telemetry" yaml:"telemetry" default:"false"`
	SecurityKey string        `json:"security_key,omitempty" yaml:"security_key,omitempty"`
	// TLS Configuration
	EnableTLS bool   `json:"enable_tls" yaml:"enable_tls" default:"false"`
	CertFile  string `json:"cert_file,omitempty" yaml:"cert_file,omitempty"`
	KeyFile   string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
}

// AgentOSOptions represents all options for creating an AgentOS instance
type AgentOSOptions struct {
	OSID         string             `json:"os_id" yaml:"os_id"`
	Name         *string            `json:"name,omitempty" yaml:"name,omitempty"`
	Description  *string            `json:"description,omitempty" yaml:"description,omitempty"`
	Version      *string            `json:"version,omitempty" yaml:"version,omitempty"`
	Agents       []*agent.Agent     `json:"agents,omitempty" yaml:"agents,omitempty"`
	Teams        []*team.Team       `json:"teams,omitempty" yaml:"teams,omitempty"`
	Workflows    []*v2.Workflow     `json:"workflows,omitempty" yaml:"workflows,omitempty"`
	Interfaces   []AgentOSInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
	Config       *AgentOSConfig     `json:"config,omitempty" yaml:"config,omitempty"`
	Settings     *AgentOSSettings   `json:"settings,omitempty" yaml:"settings,omitempty"`
	EnableMCP    bool               `json:"enable_mcp" yaml:"enable_mcp" default:"false"`
	Telemetry    bool               `json:"telemetry" yaml:"telemetry" default:"false"`
	Middleware   []interface{}      `json:"middleware,omitempty" yaml:"middleware,omitempty"`
	CustomRoutes []interface{}      `json:"custom_routes,omitempty" yaml:"custom_routes,omitempty"`
}

// Event represents an event in the AgentOS system
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Session represents a user session in the AgentOS
type Session struct {
	ID        string                 `json:"id"`
	UserID    *string                `json:"user_id,omitempty"`
	AgentID   *string                `json:"agent_id,omitempty"`
	TeamID    *string                `json:"team_id,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	State     map[string]interface{} `json:"state,omitempty"`
	Active    bool                   `json:"active"`
	Runs      []*SessionRun          `json:"runs,omitempty"`
}

// SessionRun represents a run within a session
type SessionRun struct {
	ID        string                 `json:"id"`     // Também conhecido como run_id
	RunID     string                 `json:"run_id"` // Alias para compatibilidade
	AgentID   string                 `json:"agent_id,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Status    string                 `json:"status"`
	Content   string                 `json:"content,omitempty"`   // Resposta completa do agente
	RunInput  string                 `json:"run_input,omitempty"` // Mensagem original do usuário
	Messages  []interface{}          `json:"messages,omitempty"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Message represents a message in a session
type Message struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	Role      string                 `json:"role"` // user, assistant, system
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
