package agent

// AgentOption applies configuration to AgentConfig before creating an Agent.
type AgentOption func(*AgentConfig)

// NewAgentWithOptions creates an Agent after applying AgentOption functions.
func NewAgentWithOptions(config AgentConfig, opts ...AgentOption) (*Agent, error) {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&config)
	}
	return NewAgent(config)
}

// WithLearningLoop sets a Learning Loop manager on AgentConfig.
func WithLearningLoop(manager interface{}) AgentOption {
	return func(cfg *AgentConfig) {
		cfg.Learning = manager
	}
}

