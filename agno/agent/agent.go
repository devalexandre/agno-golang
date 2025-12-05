package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/memory"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/reasoning"

	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
	"github.com/google/uuid"
	gpt3encoder "github.com/samber/go-gpt-3-encoder"
)

type AgentConfig struct {
	Context        context.Context
	Model          models.AgnoModelInterface
	Name           string
	Role           string
	Description    string
	Goal           string
	Instructions   string
	ContextData    map[string]interface{}
	ExpectedOutput string
	Tools          []toolkit.Tool
	Stream         bool
	Markdown       bool
	ShowToolsCall  bool
	Debug          bool
	//--- ChainTool Configuration ---
	// Enable ChainTool mode: Agent calls 1 tool, result propagates through all others
	EnableChainTool bool
	// ChainTool error handling configuration
	ChainToolErrorConfig  *ChainToolErrorConfig
	ChainToolErrorHandler ChainToolErrorHandler
	// ChainTool result caching
	ChainToolCache ChainToolCache
	//--- Agent Reasoning ---
	// Enable reasoning by working through the problem step by step.
	Reasoning            bool
	ReasoningModel       models.AgnoModelInterface
	ReasoningAgent       models.AgentInterface
	ReasoningMinSteps    int
	ReasoningMaxSteps    int
	ReasoningPersistence reasoning.ReasoningPersistence

	// Memory and Storage Configuration
	Memory                  memory.MemoryManager
	DB                      storage.DB // Database for storing sessions and runs (Python compatible)
	Storage                 storage.DB // Deprecated: use DB instead
	SessionID               string
	UserID                  string
	AddHistoryToMessages    bool
	NumHistoryRuns          int
	MaxToolCallsFromHistory int
	EnableUserMemories      bool
	EnableAgenticMemory     bool
	EnableSessionSummaries  bool
	ReadChatHistory         bool
	EnableAgenticState      bool // Allow tools to modify session state dynamically

	// Default Tools Configuration
	EnableReadChatHistoryTool     bool // Enable read_chat_history default tool
	EnableUpdateKnowledgeTool     bool // Enable update_knowledge default tool
	EnableReadToolCallHistoryTool bool // Enable read_tool_call_history default tool

	//knowledge
	Knowledge             knowledge.Knowledge
	KnowledgeMaxDocuments int

	//Enable Semantic Compression
	EnableSemanticCompression bool
	SemanticMaxTokens         int
	SemanticModel             models.AgnoModelInterface
	SemanticAgent             models.AgentInterface

	// Input/Output Schema
	// InputSchema provides validation for agent input
	// Pass a struct instance to define the expected input structure
	InputSchema interface{}
	// OutputSchema forces the agent to return structured JSON matching the schema
	// Pass a pointer to a struct to define the expected output structure
	// The struct will be filled automatically with the parsed response
	OutputSchema interface{}
	// Dependencies - available to agent during run (passed to all tools/hooks)
	Dependencies map[string]interface{}
	// AddDependenciesToContext - if true, dependencies are added to the system message
	AddDependenciesToContext bool
	// OutputModel is a separate AI model used specifically for parsing the output JSON
	// This allows using a different model (e.g., faster/cheaper) for JSON generation
	// Similar to how SemanticModel is used for compression
	OutputModel models.AgnoModelInterface
	// OutputModelPrompt allows customizing the prompt used by the OutputModel
	// If not provided, a default prompt will be used
	OutputModelPrompt string
	// ParserModel is a separate AI model used to parse and structure unstructured responses
	// This is useful when the main model returns free-form text that needs to be converted to structured data
	// Different from OutputModel which is used for JSON formatting
	ParserModel models.AgnoModelInterface
	// ParserModelPrompt allows customizing the prompt used by the ParserModel
	// If not provided, a default prompt will be used
	ParserModelPrompt string

	// Culture Manager for cultural knowledge management
	CultureManager          interface{} // *culture.CultureManager
	EnableAgenticCulture    bool
	UpdateCulturalKnowledge bool
	AddCultureToContext     bool

	// ParseResponse controls whether to parse the response into the OutputSchema
	ParseResponse bool

	// --- Hooks ---
	// Functions called before processing starts (for validation, logging, etc.)
	PreHooks []func(ctx context.Context, input interface{}) error
	// Functions called after output is generated but before response is returned
	PostHooks []func(ctx context.Context, output *models.RunResponse) error
	// ToolBeforeHooks are called before a tool is executed
	ToolBeforeHooks []func(ctx context.Context, toolName string, args map[string]interface{}) error
	// ToolAfterHooks are called after a tool is executed
	ToolAfterHooks []func(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error

	// --- Guardrails ---
	// InputGuardrails validate input before processing
	InputGuardrails []Guardrail
	// OutputGuardrails validate output before returning
	OutputGuardrails []Guardrail
	// ToolGuardrails validate tool calls
	ToolGuardrails []Guardrail

	// --- Tool Management ---
	// Maximum number of tool calls allowed per run
	ToolCallLimit int
	// Controls which tool is called: "none", "auto", or specific tool name
	ToolChoice string

	// --- Context Building ---
	// If True, add the agent name to the system message
	AddNameToContext bool
	// If True, add the current datetime to the system message
	AddDatetimeToContext bool
	// If True, add location information to the system message
	AddLocationToContext bool
	// Timezone identifier (e.g., "America/Sao_Paulo", "UTC")
	TimezoneIdentifier string
	// Additional context added to the system message
	AdditionalContext string

	// --- Store Options ---
	// If True, store media in run output
	StoreMedia bool
	// If True, store tool messages in run output
	StoreToolMessages bool
	// If True, store history messages in run output
	StoreHistoryMessages bool
	// If False, media is only available to tools and not sent to the LLM
	SendMediaToModel bool

	// --- System Message ---
	// Custom system message (overrides default building)
	SystemMessage string
	// Role for the system message (default: "system")
	SystemMessageRole string
	// If False, skip context building
	BuildContext bool

	// --- Retry Configuration ---
	// Delay between retries in seconds
	DelayBetweenRetries int
	// If True, use exponential backoff for retries
	ExponentialBackoff bool
}

type Agent struct {
	ctx                    context.Context
	model                  models.AgnoModelInterface
	name                   string
	role                   string
	description            string
	goal                   string
	instructions           string
	additional_information []string
	contextData            map[string]interface{}
	expected_output        string
	tools                  []toolkit.Tool
	stream                 bool
	markdown               bool
	showToolsCall          bool
	debug                  bool
	enableChainTool        bool // If true, Agent calls 1 tool and propagates result
	chainToolErrorConfig   *ChainToolErrorConfig
	chainToolErrorHandler  ChainToolErrorHandler
	chainToolCache         ChainToolCache

	// Memory and Storage
	memory                  memory.MemoryManager
	db                      storage.DB // Database for sessions and runs
	sessionID               string
	userID                  string
	addHistoryToMessages    bool
	numHistoryRuns          int
	maxToolCallsFromHistory int
	enableUserMemories      bool
	enableAgenticMemory     bool
	enableSessionSummaries  bool
	readChatHistory         bool
	enableAgenticState      bool

	// Session state
	messages     []models.Message
	runs         []*storage.AgentRun
	sessionState map[string]interface{} // Dynamic state that tools can modify

	// Active runs tracking for cancellation
	activeRuns map[string]context.CancelFunc
	runMutex   sync.RWMutex

	// Knowledge
	knowledge             knowledge.Knowledge
	knowledgeMaxDocuments int

	// Reasoning
	reasoning            bool
	reasoningModel       models.AgnoModelInterface
	reasoningAgent       models.AgentInterface
	reasoningMinSteps    int
	reasoningMaxSteps    int
	reasoningPersistence reasoning.ReasoningPersistence

	// Semantic Compression
	semanticModel             models.AgnoModelInterface
	semanticAgent             models.AgentInterface
	semanticMaxTokens         int
	enableSemanticCompression bool

	// Input/Output Schema
	inputSchema       interface{}
	outputSchema      interface{}
	outputModel       models.AgnoModelInterface
	outputModelPrompt string
	parserModel       models.AgnoModelInterface
	parserModelPrompt string

	// Dependencies - available to agent during run
	dependencies             map[string]interface{}
	addDependenciesToContext bool

	// Culture Manager
	cultureManager          interface{}
	enableAgenticCulture    bool
	updateCulturalKnowledge bool
	addCultureToContext     bool

	parseResponse bool

	// Hooks
	preHooks        []func(ctx context.Context, input interface{}) error
	postHooks       []func(ctx context.Context, output *models.RunResponse) error
	toolBeforeHooks []func(ctx context.Context, toolName string, args map[string]interface{}) error
	toolAfterHooks  []func(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error

	// Guardrails
	inputGuardrails  []Guardrail
	outputGuardrails []Guardrail
	toolGuardrails   []Guardrail

	// Tool Management
	toolCallLimit int
	toolChoice    string

	// Context Building
	addNameToContext     bool
	addDatetimeToContext bool
	addLocationToContext bool
	timezoneIdentifier   string
	additionalContext    string

	// Store Options
	storeMedia           bool
	storeToolMessages    bool
	storeHistoryMessages bool
	sendMediaToModel     bool

	// System Message
	systemMessage     string
	systemMessageRole string
	buildContext      bool

	// Retry Configuration
	delayBetweenRetries int
	exponentialBackoff  bool

	// Default Tools Configuration
	enableReadChatHistoryTool     bool // Enable read_chat_history default tool
	enableUpdateKnowledgeTool     bool // Enable update_knowledge default tool
	enableReadToolCallHistoryTool bool // Enable read_tool_call_history default tool
}

// Ensure Agent implements models.AgentInterface
var _ models.AgentInterface = (*Agent)(nil)

func NewAgent(config AgentConfig) (*Agent, error) {
	// Ensure context is not nil
	if config.Context == nil {
		config.Context = context.Background()
	}

	config.Context = context.WithValue(config.Context, models.DebugKey, config.Debug)
	config.Context = context.WithValue(config.Context, models.ShowToolsCallKey, config.ShowToolsCall)
	if config.Reasoning {
		config.Context = context.WithValue(config.Context, "reasoning", true)
	}

	// Generate session ID if not provided
	sessionID := config.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	//set mim and max for steps
	if config.ReasoningMinSteps <= 0 {
		config.ReasoningMinSteps = 1
	}
	if config.ReasoningMaxSteps <= 0 {
		config.ReasoningMaxSteps = 3
	}

	if config.KnowledgeMaxDocuments <= 0 {
		config.KnowledgeMaxDocuments = 5
	}

	agent := &Agent{
		ctx:                   config.Context,
		model:                 config.Model,
		name:                  config.Name,
		role:                  config.Role,
		description:           config.Description,
		goal:                  config.Goal,
		instructions:          config.Instructions,
		expected_output:       config.ExpectedOutput,
		contextData:           config.ContextData,
		tools:                 config.Tools,
		stream:                config.Stream,
		markdown:              config.Markdown,
		showToolsCall:         config.ShowToolsCall,
		debug:                 config.Debug,
		enableChainTool:       config.EnableChainTool,
		chainToolErrorConfig:  config.ChainToolErrorConfig,
		chainToolErrorHandler: config.ChainToolErrorHandler,
		chainToolCache: func() ChainToolCache {
			if config.ChainToolCache != nil {
				return config.ChainToolCache
			}
			return &NoCache{}
		}(),

		// Memory and Storage
		memory:                  config.Memory,
		db:                      config.DB,
		sessionID:               sessionID,
		userID:                  config.UserID,
		addHistoryToMessages:    config.AddHistoryToMessages,
		numHistoryRuns:          config.NumHistoryRuns,
		maxToolCallsFromHistory: config.MaxToolCallsFromHistory,
		enableUserMemories:      config.EnableUserMemories,
		enableAgenticMemory:     config.EnableAgenticMemory,
		enableSessionSummaries:  config.EnableSessionSummaries,
		readChatHistory:         config.ReadChatHistory,
		enableAgenticState:      config.EnableAgenticState,

		// Initialize session state
		messages:     []models.Message{},
		runs:         []*storage.AgentRun{},
		sessionState: make(map[string]interface{}),

		//knowledge
		knowledge:             config.Knowledge,
		knowledgeMaxDocuments: config.KnowledgeMaxDocuments,

		// Reasoning
		reasoning:            config.Reasoning,
		reasoningModel:       config.ReasoningModel,
		reasoningAgent:       config.ReasoningAgent,
		reasoningMinSteps:    config.ReasoningMinSteps,
		reasoningMaxSteps:    config.ReasoningMaxSteps,
		reasoningPersistence: config.ReasoningPersistence,

		// Semantic Compression
		semanticModel:             config.SemanticModel,
		semanticAgent:             config.SemanticAgent,
		semanticMaxTokens:         config.SemanticMaxTokens,
		enableSemanticCompression: config.EnableSemanticCompression,

		// Input/Output Schema
		inputSchema:              config.InputSchema,
		outputSchema:             config.OutputSchema,
		dependencies:             config.Dependencies,
		addDependenciesToContext: config.AddDependenciesToContext,
		outputModel:              config.OutputModel,
		outputModelPrompt:        config.OutputModelPrompt,
		parserModel:              config.ParserModel,
		parserModelPrompt:        config.ParserModelPrompt,

		// Culture Manager
		cultureManager:          config.CultureManager,
		enableAgenticCulture:    config.EnableAgenticCulture,
		updateCulturalKnowledge: config.UpdateCulturalKnowledge,
		addCultureToContext:     config.AddCultureToContext,

		parseResponse: config.ParseResponse,

		// Hooks
		preHooks:        config.PreHooks,
		postHooks:       config.PostHooks,
		toolBeforeHooks: config.ToolBeforeHooks,
		toolAfterHooks:  config.ToolAfterHooks,

		// Guardrails
		inputGuardrails:  config.InputGuardrails,
		outputGuardrails: config.OutputGuardrails,
		toolGuardrails:   config.ToolGuardrails,

		// Tool Management
		toolCallLimit: config.ToolCallLimit,
		toolChoice:    config.ToolChoice,

		// Context Building
		addNameToContext:     config.AddNameToContext,
		addDatetimeToContext: config.AddDatetimeToContext,
		addLocationToContext: config.AddLocationToContext,
		timezoneIdentifier:   config.TimezoneIdentifier,
		additionalContext:    config.AdditionalContext,

		// Store Options
		storeMedia:           config.StoreMedia,
		storeToolMessages:    config.StoreToolMessages,
		storeHistoryMessages: config.StoreHistoryMessages,
		sendMediaToModel:     config.SendMediaToModel,

		// System Message
		systemMessage:     config.SystemMessage,
		systemMessageRole: config.SystemMessageRole,
		buildContext:      config.BuildContext,

		// Retry Configuration
		delayBetweenRetries: config.DelayBetweenRetries,
		exponentialBackoff:  config.ExponentialBackoff,

		// Default Tools Configuration
		enableReadChatHistoryTool:     config.EnableReadChatHistoryTool,
		enableUpdateKnowledgeTool:     config.EnableUpdateKnowledgeTool,
		enableReadToolCallHistoryTool: config.EnableReadToolCallHistoryTool,
	}

	// Wrap tools with hooks if configured
	if len(config.ToolBeforeHooks) > 0 || len(config.ToolAfterHooks) > 0 || len(config.ToolGuardrails) > 0 || config.EnableChainTool {
		agent.tools = agent.WrapToolsWithHooks(agent.tools)
	}

	// Add default tools if enabled
	defaultTools := CreateDefaultTools(agent, DefaultToolsConfig{
		EnableReadChatHistory:     config.EnableReadChatHistoryTool,
		EnableUpdateKnowledge:     config.EnableUpdateKnowledgeTool,
		EnableReadToolCallHistory: config.EnableReadToolCallHistoryTool,
	})
	if len(defaultTools) > 0 {
		agent.tools = append(agent.tools, defaultTools...)
	}

	// Set defaults
	if agent.systemMessageRole == "" {
		agent.systemMessageRole = "system"
	}
	if agent.buildContext == false && agent.systemMessage == "" {
		agent.buildContext = true // Default to true if no custom system message
	}
	if agent.delayBetweenRetries <= 0 {
		agent.delayBetweenRetries = 1 // Default 1 second
	}

	// Set default for ParseResponse
	if agent.parseResponse == false && agent.outputSchema != nil {
		agent.parseResponse = true
	}

	if agent.enableSemanticCompression && agent.semanticModel == nil && agent.semanticAgent == nil {
		return nil, fmt.Errorf("semantic compression is enabled but no semantic model or agent provided")
	}

	// Load existing session if storage is provided
	if agent.db != nil {
		agent.loadSession()
	}

	return agent, nil
}

// GetName returns the agent's name (implements TeamMember interface)
func (a *Agent) GetName() string {
	if a.name != "" {
		return a.name
	}
	return "Agent"
}

// GetRole returns the agent's role (implements TeamMember interface)
func (a *Agent) GetRole() string {
	if a.role != "" {
		return a.role
	}
	return "Assistant"
}

// GetModel returns the agent's model
func (a *Agent) GetModel() models.AgnoModelInterface {
	return a.model
}

// GetID returns the agent's ID (sessionID as ID)
func (a *Agent) GetID() string {
	return a.sessionID
}

// GetToolCallLimit returns the agent's tool call limit
func (a *Agent) GetToolCallLimit() int {
	return a.toolCallLimit
}

// GetToolChoice returns the agent's tool choice setting
func (a *Agent) GetToolChoice() string {
	return a.toolChoice
}

// GetStorage returns the agent's storage/database
func (a *Agent) GetStorage() storage.DB {
	return a.db
}

// GetAddHistoryToMessages returns whether history is added to messages
func (a *Agent) GetAddHistoryToMessages() bool {
	return a.addHistoryToMessages
}

// GetEnableSessionSummaries returns whether session summaries are enabled
func (a *Agent) GetEnableSessionSummaries() bool {
	return a.enableSessionSummaries
}

// GetEnableAgenticMemory returns whether agentic memory is enabled
func (a *Agent) GetEnableAgenticMemory() bool {
	return a.enableAgenticMemory
}

// GetEnableReasoning returns whether reasoning is enabled
func (a *Agent) GetEnableReasoning() bool {
	return a.reasoning
}

// GetReadChatHistory returns whether chat history reading is enabled
func (a *Agent) GetReadChatHistory() bool {
	return a.readChatHistory
}

// GetReadToolCallHistory returns whether tool call history reading is enabled
func (a *Agent) GetReadToolCallHistory() bool {
	return a.enableReadToolCallHistoryTool
}

// CancelRun attempts to cancel a running operation by its run ID
// This implementation tracks active runs and uses context cancellation
func (a *Agent) CancelRun(runID string) bool {
	a.runMutex.Lock()
	defer a.runMutex.Unlock()

	// Check if we have a cancellation function for this run ID
	if cancelFunc, exists := a.activeRuns[runID]; exists {
		// Cancel the run
		cancelFunc()
		delete(a.activeRuns, runID)
		return true
	}

	// Run ID not found in active runs
	return false
}

// ContinueRun continues a previous run with updated tools
func (a *Agent) ContinueRun(runID string, updatedTools []map[string]interface{}, sessionID, userID *string) (models.RunResponse, error) {
	// For now, we'll implement a basic continue run mechanism
	// In a more sophisticated implementation, we would load the previous run state
	// and continue from where it left off

	// Track the run for cancellation
	_ = a.trackRun(runID)
	defer a.untrackRun(runID)

	// Create a simple prompt indicating this is a continuation
	prompt := "Continuing from previous run. Please proceed with the task."

	// Add tool information to context if provided
	if len(updatedTools) > 0 {
		toolInfo := "Updated tools available:\n"
		for _, tool := range updatedTools {
			if name, ok := tool["name"].(string); ok {
				toolInfo += fmt.Sprintf("- %s\n", name)
			}
		}
		prompt = toolInfo + "\n" + prompt
	}

	// Execute the agent with the continuation prompt
	// In a real implementation, this would restore the previous state
	options := []interface{}{}
	if sessionID != nil {
		options = append(options, WithSessionID(*sessionID))
	}
	if userID != nil {
		options = append(options, WithUserID(*userID))
	}

	// Add updated tools to the agent's tool set if needed
	// For now, we'll just pass them as context

	response, err := a.Run(prompt, options...)
	if err != nil {
		return models.RunResponse{}, err
	}

	return response, nil
}

// trackRun adds a run to the active runs tracking map
func (a *Agent) trackRun(runID string) context.Context {
	a.runMutex.Lock()
	defer a.runMutex.Unlock()

	// Create a new context with cancellation for this run
	ctx, cancel := context.WithCancel(a.ctx)
	a.activeRuns[runID] = cancel
	return ctx
}

// untrackRun removes a run from the active runs tracking map
func (a *Agent) untrackRun(runID string) {
	a.runMutex.Lock()
	defer a.runMutex.Unlock()

	if cancelFunc, exists := a.activeRuns[runID]; exists {
		// Call cancel to ensure any goroutines are notified
		cancelFunc()
		delete(a.activeRuns, runID)
	}
}

// GetMetadata returns the agent's metadata
func (a *Agent) GetMetadata() map[string]interface{} {
	// Return a copy to prevent external modification
	metadata := make(map[string]interface{})
	// Add any existing metadata fields here
	return metadata
}

// GetKnowledge returns the agent's knowledge base
func (a *Agent) GetKnowledge() knowledge.Knowledge {
	return a.knowledge
}

// GetSessionState returns a copy of the current session state
func (a *Agent) GetSessionState() map[string]interface{} {
	if !a.enableAgenticState {
		return nil
	}

	// Return a copy to prevent external modification
	state := make(map[string]interface{})
	for k, v := range a.sessionState {
		state[k] = v
	}
	return state
}

// SetSessionState sets a value in the session state
func (a *Agent) SetSessionState(key string, value interface{}) error {
	if !a.enableAgenticState {
		return fmt.Errorf("agentic state is not enabled")
	}

	a.sessionState[key] = value

	// Persist to database if configured
	if err := a.saveSessionState(); err != nil {
		return fmt.Errorf("failed to persist session state: %w", err)
	}

	return nil
}

// GetSessionStateValue gets a specific value from the session state
func (a *Agent) GetSessionStateValue(key string) (interface{}, bool) {
	if !a.enableAgenticState {
		return nil, false
	}

	val, ok := a.sessionState[key]
	return val, ok
}

// DeleteSessionState removes a key from the session state
func (a *Agent) DeleteSessionState(key string) error {
	if !a.enableAgenticState {
		return fmt.Errorf("agentic state is not enabled")
	}

	delete(a.sessionState, key)

	// Persist to database if configured
	if err := a.saveSessionState(); err != nil {
		return fmt.Errorf("failed to persist session state: %w", err)
	}

	return nil
}

// ClearSessionState clears all session state
func (a *Agent) ClearSessionState() error {
	if !a.enableAgenticState {
		return fmt.Errorf("agentic state is not enabled")
	}

	a.sessionState = make(map[string]interface{})

	// Persist to database if configured
	if err := a.saveSessionState(); err != nil {
		return fmt.Errorf("failed to persist session state: %w", err)
	}

	return nil
}

// saveSessionState persists the current session state to the database
// This matches Python's behavior of storing session_state in session_data
func (a *Agent) saveSessionState() error {
	if a.db == nil || a.sessionID == "" || !a.enableAgenticState {
		return nil
	}

	// Load current session
	session, err := a.db.ReadSession(a.ctx, a.sessionID)
	if err != nil {
		return fmt.Errorf("failed to read session: %w", err)
	}

	// Ensure SessionData is initialized
	if session.SessionData == nil {
		session.SessionData = make(map[string]interface{})
	}

	// Store session state in session_data (matches Python implementation)
	session.SessionData["session_state"] = a.sessionState
	session.UpdatedAt = time.Now().Unix()

	// Update session in database
	if err := a.db.UpdateSession(a.ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// ExecuteToolBeforeHooks executes all registered tool before hooks
func (a *Agent) ExecuteToolBeforeHooks(ctx context.Context, toolName string, args map[string]interface{}) error {
	for i, hook := range a.toolBeforeHooks {
		if err := hook(ctx, toolName, args); err != nil {
			return fmt.Errorf("tool before hook %d failed for tool '%s': %w", i, toolName, err)
		}
	}
	return nil
}

// ExecuteToolAfterHooks executes all registered tool after hooks
func (a *Agent) ExecuteToolAfterHooks(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
	for i, hook := range a.toolAfterHooks {
		if err := hook(ctx, toolName, args, result); err != nil {
			return fmt.Errorf("tool after hook %d failed for tool '%s': %w", i, toolName, err)
		}
	}
	return nil
}

// ToolWrapper wraps a tool with before/after hooks
type ToolWrapper struct {
	toolkit.Tool
	agent *Agent
}

// Execute wraps the original Execute method with hooks and guardrails
func (tw *ToolWrapper) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	// Parse input to map for hooks
	var inputMap map[string]interface{}
	if err := json.Unmarshal(input, &inputMap); err != nil {
		inputMap = make(map[string]interface{})
	} // Execute tool guardrails
	if len(tw.agent.toolGuardrails) > 0 {
		toolCallData := map[string]interface{}{
			"tool_name":   tw.GetName() + "." + methodName,
			"method_name": methodName,
			"arguments":   inputMap,
		}
		if err := RunGuardrails(tw.agent.ctx, tw.agent.toolGuardrails, toolCallData); err != nil {
			return nil, fmt.Errorf("tool guardrail validation failed: %w", err)
		}
	}

	// Execute before hooks
	if err := tw.agent.ExecuteToolBeforeHooks(tw.agent.ctx, tw.GetName()+"."+methodName, inputMap); err != nil {
		return nil, err
	}

	// Execute original tool
	result, err := tw.Tool.Execute(methodName, input)
	if err != nil {
		return result, err
	}

	// Execute after hooks
	if err := tw.agent.ExecuteToolAfterHooks(tw.agent.ctx, tw.GetName()+"."+methodName, inputMap, result); err != nil {
		return result, err
	}

	return result, nil
}

// WrapToolsWithHooks wraps tools with before/after hooks and guardrails if configured
func (a *Agent) WrapToolsWithHooks(tools []toolkit.Tool) []toolkit.Tool {
	if len(a.toolBeforeHooks) == 0 && len(a.toolAfterHooks) == 0 && len(a.toolGuardrails) == 0 && !a.enableChainTool {
		return tools
	}

	wrappedTools := make([]toolkit.Tool, len(tools))
	for i, tool := range tools {
		wrappedTools[i] = &ToolWrapper{
			Tool:  tool,
			agent: a,
		}
	}

	return wrappedTools
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// validateInput validates and converts input according to input schema (like Python's _validate_input)
// Returns the validated/converted input and an error if validation fails
// If no input schema is set, returns input unchanged
func (a *Agent) validateInput(input interface{}) (interface{}, error) {
	// If no input schema, return input unchanged (matches Python behavior)
	if a.inputSchema == nil {
		return input, nil
	}

	// Handle nil input
	if input == nil {
		return input, nil
	}

	// Handle string input - parse as JSON first
	if strInput, ok := input.(string); ok {
		var parsed interface{}
		err := json.Unmarshal([]byte(strInput), &parsed)
		if err != nil {
			return nil, fmt.Errorf("failed to parse input string as JSON: %w", err)
		}
		input = parsed
	}

	// Get the schema type
	schemaType := reflect.TypeOf(a.inputSchema)
	if schemaType == nil {
		return input, nil
	}

	// Handle map[string]interface{} input - convert to schema struct type
	// This matches Pydantic's dict-to-model conversion
	if mapInput, ok := input.(map[string]interface{}); ok {
		// Create a new instance of the schema type
		newInstance := reflect.New(schemaType.Elem()).Interface()

		// Convert map to JSON and back to struct
		data, err := json.Marshal(mapInput)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal map input: %w", err)
		}

		err = json.Unmarshal(data, newInstance)
		if err != nil {
			return nil, fmt.Errorf("failed to validate dict input to %v: %w", schemaType, err)
		}

		return newInstance, nil
	}

	// Handle case where input is already a struct
	inputType := reflect.TypeOf(input)
	if inputType == schemaType {
		// Already the correct type, return as-is
		return input, nil
	}

	// Handle pointer to struct
	if inputType.Kind() == reflect.Ptr && inputType.Elem() == schemaType.Elem() {
		// Already the correct pointer type
		return input, nil
	}

	// Try to convert to schema type if it's a struct we can unmarshal
	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("cannot validate %T against input_schema: %w", input, err)
	}

	newInstance := reflect.New(schemaType.Elem()).Interface()
	err = json.Unmarshal(data, newInstance)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input into %v: %w", schemaType, err)
	}

	return newInstance, nil
}

// prepareInputWithSchema prepares input according to input schema if configured
func (a *Agent) prepareInputWithSchema(input interface{}) (string, error) {
	// If input is a string, validate and return it
	if str, ok := input.(string); ok {
		// Validate string input against schema
		_, err := a.validateInput(str)
		if err != nil {
			return "", err
		}
		return str, nil
	}

	// Validate and convert input according to schema
	validatedInput, err := a.validateInput(input)
	if err != nil {
		return "", err
	}

	// Marshal non-string input to JSON
	data, err := json.Marshal(validatedInput)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	return string(data), nil
}

// addOutputSchemaToPrompt adds output schema instructions to the system prompt
func (a *Agent) addOutputSchemaToPrompt(systemPrompt string) (string, error) {
	// If using OutputModel, don't add schema instructions to main model
	// The OutputModel will handle JSON formatting
	if a.outputModel != nil {
		return systemPrompt, nil
	}

	// Only add schema instructions if OutputSchema is configured and no OutputModel
	if a.outputSchema == nil {
		return systemPrompt, nil
	}

	schema, err := GenerateJSONSchema(a.outputSchema)
	if err != nil {
		return "", fmt.Errorf("failed to generate output schema: %w", err)
	}

	schemaJSON, err := schema.ToJSONString()
	if err != nil {
		return "", fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	// Check if the output schema is a slice/array
	schemaType := reflect.TypeOf(a.outputSchema)
	if schemaType.Kind() == reflect.Ptr {
		schemaType = schemaType.Elem()
	}
	isArray := schemaType.Kind() == reflect.Slice

	var outputInstructions string
	if isArray {
		// Instructions for array output
		outputInstructions = fmt.Sprintf(`

## Output Format
The block below is the JSON Schema (for reference). DO NOT return the JSON Schema itself.
Instead, RETURN a JSON ARRAY that CONFORMS to this schema.

%s

CRITICAL RULES (read carefully):
- Return ONLY a JSON ARRAY (starts with [ and ends with ]).
- Each element in the array must be an object matching the item schema.
- Do NOT wrap the JSON in backticks or triple backtick markers.
- Do NOT include any text before or after the JSON array.
- Do NOT return separate objects - they must be inside a single array.
- Your entire response must be valid JSON and parseable as an array.

Example of correct format for array:
[{"field1": "value1", "field2": ["item1"]}, {"field1": "value2", "field2": ["item2"]}]

DO NOT use markdown formatting like code blocks.
`, schemaJSON)
	} else {
		// Instructions for object output
		outputInstructions = fmt.Sprintf(`

## Output Format
The block below is the JSON Schema (for reference). DO NOT return the JSON Schema itself.
Instead, RETURN a single JSON object that CONFORMS to this schema.

%s

CRITICAL RULES (read carefully):
- Return ONLY the JSON object instance that matches the schema (no schema, no explanations).
- Do NOT wrap the JSON in backticks or triple backtick markers.
- Do NOT include any text before or after the JSON.
- Include all required fields and use the correct types.
- Your entire response must be valid JSON and parseable.

If you understand, immediately produce an example JSON object that follows the schema (populate fields meaningfully).

Example of correct format:
{"field1": "value1", "field2": ["item1", "item2"]}

DO NOT use markdown formatting like code blocks.
`, schemaJSON)
	}

	return systemPrompt + outputInstructions, nil
}

// parseOutputWithSchema parses the response according to output schema if configured
func (a *Agent) parseOutputWithSchema(response string) (interface{}, error) {
	if a.outputSchema == nil || !a.parseResponse {
		return response, nil
	}

	originalResponse := response // Keep original for debugging

	// Clean the response - remove markdown code blocks if present
	cleaned := strings.TrimSpace(response)

	// Remove markdown code blocks (```json ... ``` or ``` ... ```)
	if strings.Contains(cleaned, "```") {
		// Find the start of JSON (after opening backticks)
		startIdx := strings.Index(cleaned, "```")
		if startIdx != -1 {
			// Skip the opening ``` and optional "json"
			cleaned = cleaned[startIdx+3:]
			cleaned = strings.TrimPrefix(cleaned, "json")
			cleaned = strings.TrimSpace(cleaned) // Find the end (closing backticks)
			endIdx := strings.Index(cleaned, "```")
			if endIdx != -1 {
				cleaned = cleaned[:endIdx]
			}
		}
	}

	cleaned = strings.TrimSpace(cleaned)

	// If debug mode, show what we're trying to parse
	if a.debug {
		fmt.Printf("\n=== DEBUG: Output Parsing ===\n")
		fmt.Printf("Original response length: %d\n", len(originalResponse))
		fmt.Printf("Cleaned response length: %d\n", len(cleaned))
		fmt.Printf("Original response preview (first 200 chars):\n%s\n", truncateString(originalResponse, 200))
		fmt.Printf("Cleaned response preview (first 200 chars):\n%s\n", truncateString(cleaned, 200))
		fmt.Printf("===========================\n\n")
	}

	// Get schema type
	schemaType := reflect.TypeOf(a.outputSchema)
	isPointer := schemaType.Kind() == reflect.Ptr

	if isPointer {
		schemaType = schemaType.Elem()
	}

	// Handle slice types differently
	if schemaType.Kind() == reflect.Slice {
		var result interface{}

		if isPointer {
			// If outputSchema is a pointer, unmarshal directly into it
			if err := json.Unmarshal([]byte(cleaned), a.outputSchema); err != nil {
				preview := truncateString(cleaned, 500)
				return nil, fmt.Errorf("failed to parse response into output schema (slice): %w\nResponse preview: %s", err, preview)
			}
			result = a.outputSchema
		} else {
			// For slices without pointer, create a new slice
			result = reflect.New(schemaType).Interface()
			if err := json.Unmarshal([]byte(cleaned), result); err != nil {
				preview := truncateString(cleaned, 500)
				return nil, fmt.Errorf("failed to parse response into output schema (slice): %w\nResponse preview: %s", err, preview)
			}
		}

		return result, nil
	}

	// For structs
	var result interface{}

	if isPointer {
		// If outputSchema is a pointer, unmarshal directly into it
		if err := json.Unmarshal([]byte(cleaned), a.outputSchema); err != nil {
			preview := truncateString(cleaned, 500)
			return nil, fmt.Errorf("failed to parse response into output schema: %w\nResponse preview: %s", err, preview)
		}
		result = a.outputSchema
	} else {
		// For structs without pointer, create a new instance
		result = reflect.New(schemaType).Interface()
		if err := json.Unmarshal([]byte(cleaned), result); err != nil {
			preview := truncateString(cleaned, 500)
			return nil, fmt.Errorf("failed to parse response into output schema: %w\nResponse preview: %s", err, preview)
		}
	}

	return result, nil
}

// ApplyOutputFormatting applies output formatting using OutputModel if configured
// Similar to ApplySemanticCompression, this method handles the logic of using
// a separate model for JSON formatting or falling back to direct parsing
func (a *Agent) ApplyOutputFormatting(response string) (interface{}, error) {
	if a.outputSchema == nil || !a.parseResponse {
		return response, nil
	}

	// If OutputModel is configured, use it for JSON formatting
	if a.outputModel != nil {
		return a.formatWithOutputModel(response)
	}

	// Otherwise, parse directly from the response
	return a.parseOutputWithSchema(response)
}

// formatWithOutputModel uses the OutputModel to convert response to structured JSON
func (a *Agent) formatWithOutputModel(response string) (interface{}, error) {
	if a.debug {
		fmt.Printf("\n=== DEBUG: Using OutputModel for JSON formatting ===\n")
		fmt.Printf("Original response length: %d\n", len(response))
		fmt.Printf("OutputModel: %T\n", a.outputModel)
		fmt.Printf("===================================================\n\n")
	}

	// Generate schema for the output model
	schema, err := GenerateJSONSchema(a.outputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to generate output schema: %w", err)
	}

	schemaJSON, err := schema.ToJSONString()
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	// Prepare prompt for the output model
	var systemPrompt string
	if a.outputModelPrompt != "" {
		systemPrompt = a.outputModelPrompt
	} else {
		systemPrompt = fmt.Sprintf(`You are a JSON formatting assistant. Your task is to convert the provided text into valid JSON that matches the specified schema.

Schema:
%s

CRITICAL RULES:
- Return ONLY valid JSON matching the schema
- Do NOT wrap in backticks or code blocks
- Do NOT add any explanations
- Extract relevant information from the text and structure it according to the schema
- If information is missing, use reasonable defaults or empty values`, schemaJSON)
	}

	userPrompt := fmt.Sprintf("Convert the following text to JSON:\n\n%s", response)

	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: systemPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: userPrompt,
		},
	}

	// Invoke the output model
	resp, err := a.outputModel.Invoke(a.ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("output model invocation failed: %w", err)
	}

	// Clean the JSON response
	cleaned := strings.TrimSpace(resp.Content)

	// Remove markdown code blocks if present
	if strings.Contains(cleaned, "```") {
		startIdx := strings.Index(cleaned, "```")
		if startIdx != -1 {
			cleaned = cleaned[startIdx+3:]
			cleaned = strings.TrimPrefix(cleaned, "json")
			cleaned = strings.TrimSpace(cleaned)

			endIdx := strings.Index(cleaned, "```")
			if endIdx != -1 {
				cleaned = cleaned[:endIdx]
			}
		}
	}

	cleaned = strings.TrimSpace(cleaned)

	if a.debug {
		fmt.Printf("\n=== DEBUG: OutputModel Response ===\n")
		fmt.Printf("Cleaned JSON length: %d\n", len(cleaned))
		fmt.Printf("JSON preview (first 500 chars):\n%s\n", truncateString(cleaned, 500))
		fmt.Printf("==================================\n\n")
	}

	// Parse the JSON into the output schema
	return a.unmarshalIntoSchema(cleaned)
}

// parseResponseWithParserModel uses the ParserModel to parse and structure unstructured responses
// This is different from OutputModel - ParserModel is used when the main model returns free-form text
// that needs to be converted to structured data, while OutputModel is used for JSON formatting
func (a *Agent) parseResponseWithParserModel(response string) (string, error) {
	if a.debug {
		fmt.Printf("\n=== DEBUG: Using ParserModel for response parsing ===\n")
		fmt.Printf("Original response length: %d\n", len(response))
		fmt.Printf("ParserModel: %T\n", a.parserModel)
		fmt.Printf("=====================================================\n\n")
	}

	// Prepare prompt for the parser model
	var systemPrompt string
	if a.parserModelPrompt != "" {
		systemPrompt = a.parserModelPrompt
	} else {
		systemPrompt = `You are a response parsing assistant. Your task is to parse and structure the provided text into a clear, well-formatted response.

CRITICAL RULES:
- Extract key information and structure it logically
- Maintain the original meaning and intent
- Remove unnecessary verbosity
- Format the response in a clear, readable way
- If the response is already well-structured, return it as-is`
	}

	userPrompt := fmt.Sprintf("Parse and structure the following response:\n\n%s", response)

	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: systemPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: userPrompt,
		},
	}

	// Invoke the parser model
	resp, err := a.parserModel.Invoke(a.ctx, messages)
	if err != nil {
		return "", fmt.Errorf("parser model invocation failed: %w", err)
	}

	parsed := strings.TrimSpace(resp.Content)

	if a.debug {
		fmt.Printf("\n=== DEBUG: ParserModel Response ===\n")
		fmt.Printf("Parsed response length: %d\n", len(parsed))
		fmt.Printf("Response preview (first 500 chars):\n%s\n", truncateString(parsed, 500))
		fmt.Printf("===================================\n\n")
	}

	return parsed, nil
}

// unmarshalIntoSchema unmarshals JSON string into the output schema struct
func (a *Agent) unmarshalIntoSchema(jsonStr string) (interface{}, error) {
	// Get schema type
	schemaType := reflect.TypeOf(a.outputSchema)
	isPointer := schemaType.Kind() == reflect.Ptr

	if isPointer {
		schemaType = schemaType.Elem()
	}

	// Handle slice types
	if schemaType.Kind() == reflect.Slice {
		var result interface{}

		if isPointer {
			if err := json.Unmarshal([]byte(jsonStr), a.outputSchema); err != nil {
				preview := truncateString(jsonStr, 500)
				return nil, fmt.Errorf("failed to parse output model response (slice): %w\nResponse preview: %s", err, preview)
			}
			result = a.outputSchema
		} else {
			result = reflect.New(schemaType).Interface()
			if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
				preview := truncateString(jsonStr, 500)
				return nil, fmt.Errorf("failed to parse output model response (slice): %w\nResponse preview: %s", err, preview)
			}
		}

		return result, nil
	}

	// Handle struct types
	var result interface{}

	if isPointer {
		if err := json.Unmarshal([]byte(jsonStr), a.outputSchema); err != nil {
			preview := truncateString(jsonStr, 500)
			return nil, fmt.Errorf("failed to parse output model response: %w\nResponse preview: %s", err, preview)
		}
		result = a.outputSchema
	} else {
		result = reflect.New(schemaType).Interface()
		if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
			preview := truncateString(jsonStr, 500)
			return nil, fmt.Errorf("failed to parse output model response: %w\nResponse preview: %s", err, preview)
		}
	}

	return result, nil
}

// RunWithOptions is the new method with full options support
func (a *Agent) RunWithOptions(input interface{}, opts ...interface{}) (models.RunResponse, error) {
	// Apply options
	options := &RunOptions{}
	for _, opt := range opts {
		if runOpt, ok := opt.(func(*RunOptions)); ok {
			runOpt(options)
		}
	}

	// Override agent settings with run options if provided
	if options.SessionID != nil {
		a.sessionID = *options.SessionID
	}

	if options.UserID != nil {
		a.userID = *options.UserID
	}

	if options.DebugMode != nil {
		a.debug = *options.DebugMode
	}

	if options.AddHistoryToContext != nil {
		a.addHistoryToMessages = *options.AddHistoryToContext
	}

	// Merge session state if provided
	sessionState := make(map[string]interface{})
	if options.SessionState != nil {
		for k, v := range options.SessionState {
			sessionState[k] = v
		}
	}

	// Merge dependencies if provided
	dependencies := make(map[string]interface{})
	if options.Dependencies != nil {
		for k, v := range options.Dependencies {
			dependencies[k] = v
		}
	}

	// Merge metadata if provided
	metadata := make(map[string]interface{})
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			metadata[k] = v
		}
	}

	// Add media to context if provided
	if len(options.Audio) > 0 {
		metadata["audio"] = options.Audio
	}
	if len(options.Images) > 0 {
		metadata["images"] = options.Images
	}
	if len(options.Videos) > 0 {
		metadata["videos"] = options.Videos
	}
	if len(options.Files) > 0 {
		metadata["files"] = options.Files
	}

	// Apply knowledge filters if provided (for future use)
	if options.KnowledgeFilters != nil {
		// Store for potential future knowledge queries
		metadata["knowledge_filters"] = options.KnowledgeFilters
	}

	// Determine number of retries
	retries := 0
	if options.Retries != nil {
		retries = *options.Retries
	}

	var messages []models.Message

	// Prepare input according to schema if configured
	prompt, err := a.prepareInputWithSchema(input)
	if err != nil {
		return models.RunResponse{}, fmt.Errorf("failed to prepare input: %w", err)
	}

	// Add system message and history normally
	baseMessages := a.prepareMessages(prompt)
	for _, msg := range baseMessages {
		if msg.Role == models.TypeUserRole {
			messages = append(messages, msg)
		} else {
			messages = append([]models.Message{msg}, messages...)
		}
	}

	// Add session state to context if requested
	if options.AddSessionStateToContext != nil && *options.AddSessionStateToContext && len(sessionState) > 0 {
		stateJSON, _ := json.Marshal(sessionState)
		messages = append([]models.Message{{
			Role:    models.TypeSystemRole,
			Content: fmt.Sprintf("Session State: %s", string(stateJSON)),
		}}, messages...)
	}

	// Add dependencies to context if requested
	if options.AddDependenciesToContext != nil && *options.AddDependenciesToContext && len(dependencies) > 0 {
		depsJSON, _ := json.Marshal(dependencies)
		messages = append([]models.Message{{
			Role:    models.TypeSystemRole,
			Content: fmt.Sprintf("Dependencies: %s", string(depsJSON)),
		}}, messages...)
	}

	// Reasoning: if not using agent mode, use simple reasoning
	if a.reasoning && a.reasoningModel != nil {
		// use default reasoning agent
		if a.reasoningAgent == nil {
			reasoningAgent := NewReasoningAgent(a.ctx, a.reasoningModel, a.tools, a.reasoningMinSteps, a.reasoningMaxSteps)
			// Use the reasoning agent directly without assigning to interface
			reasoningSteps, err := reasoningAgent.Reason(prompt)
			if err == nil && len(reasoningSteps) > 0 {
				var allStepsMsg string
				for _, step := range reasoningSteps {
					stepMsg := ""
					if step.Title != "" {
						stepMsg += "**" + step.Title + "**\n"
					}
					if step.Reasoning != "" {
						stepMsg += step.Reasoning + "\n"
					}
					if step.Action != "" {
						stepMsg += "Action: " + step.Action + "\n"
					}
					if step.Result != "" {
						stepMsg += "Result: " + step.Result + "\n"
					}
					allStepsMsg += stepMsg + "\n"
				}
				messages = append(messages, models.Message{
					Role:    "assistant",
					Content: allStepsMsg,
				})
			}
		} else {
			// Use existing reasoning agent
			reasoningInterface, ok := a.reasoningAgent.(interface {
				Reason(prompt string) ([]models.ReasoningStep, error)
			})
			if ok {
				reasoningSteps, err := reasoningInterface.Reason(prompt)
				if err == nil && len(reasoningSteps) > 0 {
					var allStepsMsg string
					for _, step := range reasoningSteps {
						stepMsg := ""
						if step.Title != "" {
							stepMsg += "**" + step.Title + "**\n"
						}
						if step.Reasoning != "" {
							stepMsg += step.Reasoning + "\n"
						}
						if step.Action != "" {
							stepMsg += "Action: " + step.Action + "\n"
						}
						if step.Result != "" {
							stepMsg += "Result: " + step.Result + "\n"
						}
						allStepsMsg += stepMsg + "\n"
					}
					messages = append(messages, models.Message{
						Role:    "assistant",
						Content: allStepsMsg,
					})
				}
			}
		}
	}

	// Retry logic
	var resp *models.MessageResponse
	var lastErr error

	for attempt := 0; attempt <= retries; attempt++ {
		if a.debug && attempt > 0 {
			fmt.Printf("Retry attempt %d/%d\n", attempt, retries)
		}

		resp, lastErr = a.model.Invoke(a.ctx, messages, models.WithTools(a.tools))
		if lastErr == nil {
			break
		}

		if attempt < retries {
			time.Sleep(time.Second * time.Duration(attempt+1))
		}
	}

	if lastErr != nil {
		return models.RunResponse{}, lastErr
	}

	// Save run to storage if enabled
	if a.db != nil {
		if err := a.saveRun(prompt, resp.Content, messages); err != nil && a.debug {
			fmt.Printf("Warning: Failed to save run: %v\n", err)
		}
	}

	// Process memories if enabled
	if a.memory != nil {
		if err := a.processMemories(prompt, resp.Content); err != nil && a.debug {
			fmt.Printf("Warning: Failed to process memories: %v\n", err)
		}
	}

	// Update message history for next interaction
	if a.addHistoryToMessages {
		a.messages = append(a.messages, models.Message{
			Role:    "user",
			Content: prompt,
		})
		a.messages = append(a.messages, models.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Keep only recent messages based on history limit
		if a.numHistoryRuns > 0 {
			maxMessages := a.numHistoryRuns * 2 // user + assistant per run
			if len(a.messages) > maxMessages {
				a.messages = a.messages[len(a.messages)-maxMessages:]
			}
		}
	}

	// Step 1: Parse response with ParserModel if configured
	responseContent := resp.Content
	if a.parserModel != nil {
		parsed, err := a.parseResponseWithParserModel(resp.Content)
		if err != nil {
			log.Printf("Warning: ParserModel failed, using original response: %v", err)
		} else {
			responseContent = parsed
		}
	}

	// Step 2: Parse output using ApplyOutputFormatting method
	parsedContent, err := a.ApplyOutputFormatting(responseContent)
	if err != nil {
		return models.RunResponse{}, err
	}

	var outputContent interface{}
	if parsedContent != resp.Content {
		// Output was parsed/formatted
		outputContent = parsedContent
	}

	return models.RunResponse{
		TextContent:  resp.Content, // Original response from main model
		ContentType:  "text",
		Event:        "RunResponse",
		ParsedOutput: parsedContent, // Deprecated: kept for backwards compatibility
		Output:       outputContent, // Structured output (pointer to filled struct)
		Messages: []models.Message{
			{
				Role:     models.Role(resp.Role),
				Content:  resp.Content,
				Thinking: resp.Thinking,
			},
		},
		Model:     resp.Model,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// Run executes the agent with the given input and options
// This method accepts optional RunOptions using the functional options pattern
func (a *Agent) Run(input interface{}, opts ...interface{}) (models.RunResponse, error) {
	// Apply options
	options := &RunOptions{}
	for _, opt := range opts {
		if runOpt, ok := opt.(func(*RunOptions)); ok {
			runOpt(options)
		}
	}

	// Execute pre-hooks for validation and preprocessing
	if len(a.preHooks) > 0 {
		for i, hook := range a.preHooks {
			if err := hook(a.ctx, input); err != nil {
				return models.RunResponse{}, fmt.Errorf("pre-hook %d failed: %w", i, err)
			}
		}
	}

	// Execute input guardrails
	if len(a.inputGuardrails) > 0 {
		if err := RunGuardrails(a.ctx, a.inputGuardrails, input); err != nil {
			return models.RunResponse{}, fmt.Errorf("input validation failed: %w", err)
		}
	}

	// Override agent settings with run options if provided
	if options.SessionID != nil {
		a.sessionID = *options.SessionID
	}

	if options.UserID != nil {
		a.userID = *options.UserID
	}

	if options.DebugMode != nil {
		a.debug = *options.DebugMode
	}

	if options.AddHistoryToContext != nil {
		a.addHistoryToMessages = *options.AddHistoryToContext
	}

	// Merge session state if provided
	sessionState := make(map[string]interface{})
	if options.SessionState != nil {
		for k, v := range options.SessionState {
			sessionState[k] = v
		}
	}

	// Merge dependencies if provided
	dependencies := make(map[string]interface{})
	if options.Dependencies != nil {
		for k, v := range options.Dependencies {
			dependencies[k] = v
		}
	}

	// Merge metadata if provided
	metadata := make(map[string]interface{})
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			metadata[k] = v
		}
	}

	// Add media to context if provided
	if len(options.Audio) > 0 {
		metadata["audio"] = options.Audio
	}
	if len(options.Images) > 0 {
		metadata["images"] = options.Images
	}
	if len(options.Videos) > 0 {
		metadata["videos"] = options.Videos
	}
	if len(options.Files) > 0 {
		metadata["files"] = options.Files
	}

	// Apply knowledge filters if provided (for future use)
	if options.KnowledgeFilters != nil {
		// Store for potential future knowledge queries
		metadata["knowledge_filters"] = options.KnowledgeFilters
	}

	// Determine number of retries
	retries := 0
	if options.Retries != nil {
		retries = *options.Retries
	}

	var messages []models.Message

	// Prepare input according to schema if configured
	prompt, err := a.prepareInputWithSchema(input)
	if err != nil {
		return models.RunResponse{}, fmt.Errorf("failed to prepare input: %w", err)
	}

	// Add system message and history normally
	baseMessages := a.prepareMessages(prompt)
	for _, msg := range baseMessages {
		if msg.Role == models.TypeUserRole {
			messages = append(messages, msg)
		} else {
			messages = append([]models.Message{msg}, messages...)
		}
	}

	// Add session state to context if requested
	if options.AddSessionStateToContext != nil && *options.AddSessionStateToContext && len(sessionState) > 0 {
		stateJSON, _ := json.Marshal(sessionState)
		messages = append([]models.Message{{
			Role:    models.TypeSystemRole,
			Content: fmt.Sprintf("Session State: %s", string(stateJSON)),
		}}, messages...)
	}

	// Add dependencies to context if requested
	if options.AddDependenciesToContext != nil && *options.AddDependenciesToContext && len(dependencies) > 0 {
		depsJSON, _ := json.Marshal(dependencies)
		messages = append([]models.Message{{
			Role:    models.TypeSystemRole,
			Content: fmt.Sprintf("Dependencies: %s", string(depsJSON)),
		}}, messages...)
	}

	// Reasoning: if not using agent mode, use simple reasoning
	if a.reasoning && a.reasoningModel != nil {
		// use default reasoning agent
		if a.reasoningAgent == nil {
			reasoningAgent := NewReasoningAgent(a.ctx, a.reasoningModel, a.tools, a.reasoningMinSteps, a.reasoningMaxSteps)
			// Use the reasoning agent directly without assigning to interface
			reasoningSteps, err := reasoningAgent.Reason(prompt)
			if err == nil && len(reasoningSteps) > 0 {
				var allStepsMsg string
				for _, step := range reasoningSteps {
					stepMsg := ""
					if step.Title != "" {
						stepMsg += "**" + step.Title + "**\n"
					}
					if step.Reasoning != "" {
						stepMsg += step.Reasoning + "\n"
					}
					if step.Action != "" {
						stepMsg += "Action: " + step.Action + "\n"
					}
					if step.Result != "" {
						stepMsg += "Result: " + step.Result + "\n"
					}
					allStepsMsg += stepMsg + "\n"
				}
				messages = append(messages, models.Message{
					Role:    "assistant",
					Content: allStepsMsg,
				})
			}
		} else {
			// Use existing reasoning agent
			reasoningInterface, ok := a.reasoningAgent.(interface {
				Reason(prompt string) ([]models.ReasoningStep, error)
			})
			if ok {
				reasoningSteps, err := reasoningInterface.Reason(prompt)
				if err == nil && len(reasoningSteps) > 0 {
					var allStepsMsg string
					for _, step := range reasoningSteps {
						stepMsg := ""
						if step.Title != "" {
							stepMsg += "**" + step.Title + "**\n"
						}
						if step.Reasoning != "" {
							stepMsg += step.Reasoning + "\n"
						}
						if step.Action != "" {
							stepMsg += "Action: " + step.Action + "\n"
						}
						if step.Result != "" {
							stepMsg += "Result: " + step.Result + "\n"
						}
						allStepsMsg += stepMsg + "\n"
					}
					messages = append(messages, models.Message{
						Role:    "assistant",
						Content: allStepsMsg,
					})
				}
			}
		}
	}

	// ChainTool mode will be handled during tool execution if enabled

	// Retry logic
	var resp *models.MessageResponse
	var lastErr error

	// Prepare model options - if ChainTool is enabled, only send the first tool
	var toolsToSend []toolkit.Tool
	if a.enableChainTool && len(a.tools) > 1 {
		// ChainTool mode: Send only the first tool to the model
		toolsToSend = []toolkit.Tool{a.tools[0]}
		if a.debug {
			utils.DebugPanel(fmt.Sprintf("ChainTool: Sending only first tool '%s' to model (hiding %d other tools)", a.tools[0].GetName(), len(a.tools)-1))
		}
	} else {
		// Normal mode: Send all tools
		toolsToSend = a.tools
	}

	modelOptions := []models.Option{models.WithTools(toolsToSend)}

	// Check if streaming is enabled
	if options.Stream != nil && *options.Stream {
		resp, lastErr = a.runWithStreaming(prompt, messages)
	} else {
		for attempt := 0; attempt <= retries; attempt++ {
			if a.debug && attempt > 0 {
				fmt.Printf("Retry attempt %d/%d\n", attempt, retries)
			}

			resp, lastErr = a.model.Invoke(a.ctx, messages, modelOptions...)
			if lastErr == nil {
				break
			}

			if attempt < retries {
				// Apply exponential backoff if enabled
				delay := time.Duration(a.delayBetweenRetries) * time.Second
				if a.exponentialBackoff && attempt > 0 {
					delay = delay * time.Duration(1<<uint(attempt)) // 2^attempt
				}
				time.Sleep(delay)
			}
		}
	}

	if lastErr != nil {
		return models.RunResponse{}, lastErr
	}

	// Debug: print response in json
	if a.debug {
		utils.ToolCallPanelWithArgs("resp", resp)
	}

	// Process tool results if present (tools were executed by the model client)
	if len(resp.ToolResults) > 0 && a.enableChainTool && len(a.tools) > 1 {

		// Get the first tool's result
		firstToolResult := resp.ToolResults[0]

		// Get the first tool's result as string (this is what we'll replace in model response)
		var firstToolResultStr string
		if str, ok := firstToolResult.Result.(string); ok {
			firstToolResultStr = str
		} else {
			resultJSON, _ := json.Marshal(firstToolResult.Result)
			firstToolResultStr = string(resultJSON)
		}

		// Start chain from the first tool's result
		currentResult := firstToolResult.Result

		// Execute remaining tools in sequence (tools[1], tools[2], ...)
		for i := 1; i < len(a.tools); i++ {
			tool := a.tools[i]

			// Prepare arguments for the tool
			args := a.prepareToolArgumentsForChain(tool, currentResult)

			// Marshal arguments to JSON
			argsJSON, err := json.Marshal(args)
			if err != nil {
				utils.ErrorPanel(fmt.Errorf("ChainTool: Failed to marshal arguments for %s: %v", tool.GetName(), err))
				continue
			}

			// Get method name
			methods := tool.GetMethods()
			var methodName string
			for name := range methods {
				methodName = name
				break
			}

			if a.debug {
				utils.ToolCallPanelWithArgs(fmt.Sprintf("ChainTool: Executing tool[%d] %s", i, tool.GetName()), args)
			}

			// Execute the tool
			toolResult, err := tool.Execute(methodName, json.RawMessage(argsJSON))
			if err != nil {
				utils.ErrorPanel(fmt.Errorf("ChainTool: Tool execution failed for %s: %v", tool.GetName(), err))
				continue
			}

			if a.debug {
				utils.ToolCallPanelWithArgs(fmt.Sprintf("ChainTool: Result from tool[%d] %s", i, tool.GetName()), toolResult)
			}

			// Update current result for next tool
			currentResult = toolResult
		}

		// Get final result as string
		var finalResult string
		if str, ok := currentResult.(string); ok {
			finalResult = str
		} else {
			resultJSON, _ := json.Marshal(currentResult)
			finalResult = string(resultJSON)
		}

		// Replace the first tool's RESULT with the final chain result in the model's response
		// e.g., replace "AGNO" (first tool result) with "_ONGA_" (final chain result)
		modelResponse := resp.Content
		if firstToolResultStr != "" && firstToolResultStr != finalResult {
			idx := strings.Index(modelResponse, firstToolResultStr)
			if idx != -1 {
				modelResponse = modelResponse[:idx] + finalResult + modelResponse[idx+len(firstToolResultStr):]
				if a.debug {
					utils.InfoPanel(fmt.Sprintf("ChainTool: Substituting '%s' → '%s' in model response", firstToolResultStr, finalResult))
				}
			}
		}

		// Build response with substituted model response
		parsedContent, err := a.ApplyOutputFormatting(modelResponse)
		if err != nil {
			return models.RunResponse{}, err
		}

		var outputContent interface{}
		if parsedContent != modelResponse {
			outputContent = parsedContent
		}

		runResponse := models.RunResponse{
			TextContent:  modelResponse,
			ContentType:  "text",
			Event:        "RunResponse",
			ParsedOutput: parsedContent,
			Output:       outputContent,
			Messages: []models.Message{
				{
					Role:    "assistant",
					Content: modelResponse,
				},
			},
			CreatedAt: time.Now().Unix(),
		}

		// Execute output guardrails
		if len(a.outputGuardrails) > 0 {
			if err := RunGuardrails(a.ctx, a.outputGuardrails, runResponse); err != nil {
				return models.RunResponse{}, fmt.Errorf("output validation failed: %w", err)
			}
		}

		// Execute post-hooks
		if len(a.postHooks) > 0 {
			for i, hook := range a.postHooks {
				if err := hook(a.ctx, &runResponse); err != nil {
					return models.RunResponse{}, fmt.Errorf("post-hook %d failed: %w", i, err)
				}
			}
		}

		return runResponse, nil
	}

	// Process tool calls if present (legacy path for non-ChainTool mode or when ToolResults not available)
	if len(resp.ToolCalls) > 0 {
		utils.InfoPanel(fmt.Sprintf("Processing %d tool calls", len(resp.ToolCalls)))

		// Execute tool calls and get final result
		finalResult, toolMessages, _, _, err := a.processToolCallsFromResponse(resp)
		if err != nil {
			return models.RunResponse{}, fmt.Errorf("tool call processing failed: %w", err)
		}

		// Update response content with tool results
		resp.Content = finalResult

		// Add tool messages to history
		messages = append(messages, toolMessages...)
	}

	// Save run to storage if enabled
	if a.db != nil {
		if err := a.saveRun(prompt, resp.Content, messages); err != nil && a.debug {
			fmt.Printf("Warning: Failed to save run: %v\n", err)
		}
	}

	// Process memories if enabled
	if a.memory != nil {
		if err := a.processMemories(prompt, resp.Content); err != nil && a.debug {
			fmt.Printf("Warning: Failed to process memories: %v\n", err)
		}
	}

	// Update message history for next interaction
	if a.addHistoryToMessages {
		a.messages = append(a.messages, models.Message{
			Role:    "user",
			Content: prompt,
		})
		a.messages = append(a.messages, models.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Keep only recent messages based on history limit
		if a.numHistoryRuns > 0 {
			maxMessages := a.numHistoryRuns * 2 // user + assistant per run
			if len(a.messages) > maxMessages {
				a.messages = a.messages[len(a.messages)-maxMessages:]
			}
		}
	}

	// Step 1: Parse response with ParserModel if configured
	responseContent := resp.Content
	if a.parserModel != nil {
		parsed, err := a.parseResponseWithParserModel(resp.Content)
		if err != nil {
			log.Printf("Warning: ParserModel failed, using original response: %v", err)
		} else {
			responseContent = parsed
		}
	}

	// Step 2: Parse output using ApplyOutputFormatting method
	parsedContent, err := a.ApplyOutputFormatting(responseContent)
	if err != nil {
		return models.RunResponse{}, err
	}

	var outputContent interface{}
	if parsedContent != resp.Content {
		// Output was parsed/formatted
		outputContent = parsedContent
	}

	runResponse := models.RunResponse{
		TextContent:  resp.Content, // Original response from main model
		ContentType:  "text",
		Event:        "RunResponse",
		ParsedOutput: parsedContent, // Deprecated: kept for backwards compatibility
		Output:       outputContent, // Structured output (pointer to filled struct)
		Messages: []models.Message{
			{
				Role:     models.Role(resp.Role),
				Content:  resp.Content,
				Thinking: resp.Thinking,
			},
		},
		Model:     resp.Model,
		CreatedAt: time.Now().Unix(),
	}

	// Execute output guardrails
	if len(a.outputGuardrails) > 0 {
		if err := RunGuardrails(a.ctx, a.outputGuardrails, runResponse); err != nil {
			return models.RunResponse{}, fmt.Errorf("output validation failed: %w", err)
		}
	}

	// Execute post-hooks for validation and post-processing
	if len(a.postHooks) > 0 {
		for i, hook := range a.postHooks {
			if err := hook(a.ctx, &runResponse); err != nil {
				return models.RunResponse{}, fmt.Errorf("post-hook %d failed: %w", i, err)
			}
		}
	}

	return runResponse, nil
}

func (a *Agent) PrintResponse(prompt string, stream bool, markdown bool) {
	fmt.Println("Running agent  stream:", stream, "markdown:", markdown)
	a.stream = stream
	a.markdown = markdown
	utils.SetMarkdownMode(markdown)
	if stream {
		a.print_stream_response(prompt, markdown)
	} else {
		a.print_response(prompt, markdown)
	}
}

func (a *Agent) print_response(prompt string, markdown bool) {
	start := time.Now()
	messages := a.prepareMessages(prompt)

	if a.debug {
		fmt.Printf("DEBUG: Prepared %d messages for model\n", len(messages))
		for i, msg := range messages {
			fmt.Printf("DEBUG: Message %d - Role: %s, Content length: %d\n", i, msg.Role, len(msg.Content))
		}
		fmt.Printf("DEBUG: Using %d tools\n", len(a.tools))
	}

	spinnerResponse := utils.ThinkingPanel(prompt)

	if a.debug {
		fmt.Println("DEBUG: Calling model.Invoke...")
	}

	resp, err := a.model.Invoke(a.ctx, messages, models.WithTools(a.tools))
	if err != nil {
		fmt.Printf("ERROR: Model invoke failed: %v\n", err)
		return
	}

	if a.debug {
		fmt.Printf("DEBUG: Model response received - Content length: %d\n", len(resp.Content))
		fmt.Printf("DEBUG: Response content preview: %.100s...\n", resp.Content)
		fmt.Printf("DEBUG: Response type: %T\n", resp)
		fmt.Printf("DEBUG: Response role: %s\n", resp.Role)
		fmt.Printf("DEBUG: Response model: %s\n", resp.Model)
	}

	utils.ResponsePanel(resp.Content, spinnerResponse, start, markdown)

	if a.debug {
		fmt.Println("DEBUG: ResponsePanel called")
		fmt.Printf("DEBUG: Final response content:\n%s\n", resp.Content)
	}
}

func (a *Agent) print_stream_response(prompt string, markdown bool) {
	start := time.Now()
	messages := a.prepareMessages(prompt)
	// Thinking
	spinnerResponse := utils.ThinkingPanel(prompt)
	contentChan := utils.StartSimplePanel(spinnerResponse, start, markdown)

	// Response
	fullResponse := ""
	var streamBuffer string // Mover para fora do callback
	showResponse := false
	callOptions := []models.Option{
		models.WithTools(a.tools),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			if !showResponse {
				showResponse = true
			}

			// Adicionar chunk ao buffer
			streamBuffer += string(chunk)
			fullResponse += string(chunk)

			// Verificar se devemos fazer flush do buffer
			shouldFlush := false

			// Flush if finding period, exclamation or question mark
			if strings.Contains(streamBuffer, ".") ||
				strings.Contains(streamBuffer, "!") ||
				strings.Contains(streamBuffer, "?") {
				shouldFlush = true
			}

			// Flush se buffer ficar muito grande (mais de 50 caracteres)
			if len(streamBuffer) > 50 {
				shouldFlush = true
			}

			if shouldFlush {
				// Send accumulated content
				contentChan <- utils.ContentUpdateMsg{
					PanelName: "Response",
					Content:   streamBuffer,
				}
				streamBuffer = "" // Limpar buffer
			}

			return nil
		}),
	}

	err := a.model.InvokeStream(a.ctx, messages, callOptions...)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Flush any remaining content in buffer
	if streamBuffer != "" {
		contentChan <- utils.ContentUpdateMsg{
			PanelName: "Response",
			Content:   streamBuffer,
		}
	}

	// Close channel to stop the streaming goroutine
	close(contentChan)
	// We wait a bit to ensure streaming output is finished
	time.Sleep(100 * time.Millisecond)

	// Since StartSimplePanel now handles the panel rendering and clearing,
	// we don't need to do manual clearing or print the final panel here.
	// However, we might want to ensure the final state is consistent.
	// But StartSimplePanel runs in a goroutine that consumes the channel.
	// When we close the channel, the loop finishes.

	// If we want to guarantee the final panel is printed by the goroutine,
	// we just need to ensure all content was sent.
}

// filterToolCallsFromHistory filters the tool calls from message history based on maxToolCallsFromHistory
func (a *Agent) filterToolCallsFromHistory(messages []models.Message) []models.Message {
	if a.maxToolCallsFromHistory <= 0 {
		// If no limit is set, return all messages as is
		return messages
	}

	// Count tool calls from the end of the messages (most recent first)
	var filteredMessages []models.Message
	toolCallCount := 0
	limitReached := false

	// Process messages in reverse order to count from most recent
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		// Check if this message contains tool calls
		if len(msg.ToolCalls) > 0 {
			if limitReached {
				// Skip messages with tool calls after limit is reached
				continue
			}

			// Check if adding these tool calls would exceed the limit
			if toolCallCount+len(msg.ToolCalls) > a.maxToolCallsFromHistory {
				// Calculate how many tool calls we can still include
				remainingSlots := a.maxToolCallsFromHistory - toolCallCount
				if remainingSlots > 0 {
					// Create a copy of the message with limited tool calls
					limitedMsg := msg
					limitedMsg.ToolCalls = msg.ToolCalls[len(msg.ToolCalls)-remainingSlots:]
					filteredMessages = append([]models.Message{limitedMsg}, filteredMessages...)
				}
				// Mark that we've reached the limit
				limitReached = true
			} else {
				// Add all tool calls from this message
				filteredMessages = append([]models.Message{msg}, filteredMessages...)
				toolCallCount += len(msg.ToolCalls)
			}
		} else {
			// Message has no tool calls, always include it
			filteredMessages = append([]models.Message{msg}, filteredMessages...)
		}
	}

	return filteredMessages
}

func (a *Agent) prepareMessages(prompt string) []models.Message {
	// If custom system message is provided and buildContext is false, use it directly
	if a.systemMessage != "" && !a.buildContext {
		messages := []models.Message{
			{
				Role:    models.Role(a.systemMessageRole),
				Content: a.systemMessage,
			},
		}

		// Add history if enabled
		if a.addHistoryToMessages {
			messages = append(messages, a.messages...)
		}

		// Add user prompt
		messages = append(messages, models.Message{
			Role:    models.TypeUserRole,
			Content: prompt,
		})

		return messages
	}

	systemMessage := ""
	originalSystemMessage := ""
	originalPrompt := prompt

	// Add agent name to context if enabled
	if a.addNameToContext && a.name != "" {
		systemMessage += fmt.Sprintf("You are %s.\n\n", a.name)
		originalSystemMessage += fmt.Sprintf("You are %s.\n\n", a.name)
	}

	// Add datetime to context if enabled
	if a.addDatetimeToContext {
		location := time.UTC
		if a.timezoneIdentifier != "" {
			loc, err := time.LoadLocation(a.timezoneIdentifier)
			if err == nil {
				location = loc
			}
		}
		now := time.Now().In(location)
		dateStr := now.Format("Monday, January 2, 2006 at 3:04 PM MST")
		systemMessage += fmt.Sprintf("Current date and time: %s\n\n", dateStr)
		originalSystemMessage += fmt.Sprintf("Current date and time: %s\n\n", dateStr)
	}

	// Add location to context if enabled
	if a.addLocationToContext && a.timezoneIdentifier != "" {
		systemMessage += fmt.Sprintf("Your timezone: %s\n\n", a.timezoneIdentifier)
		originalSystemMessage += fmt.Sprintf("Your timezone: %s\n\n", a.timezoneIdentifier)
	}

	if a.goal != "" {
		systemMessage += fmt.Sprintf("<goal>\n%s\n</goal>\n", a.ApplySemanticCompression(a.goal))
		originalSystemMessage += fmt.Sprintf("<goal>\n%s\n</goal>\n", a.goal)
	}

	if a.description != "" {
		systemMessage += fmt.Sprintf("<description>\n%s\n</description>\n", a.ApplySemanticCompression(a.description))
		originalSystemMessage += fmt.Sprintf("<description>\n%s\n</description>\n", a.description)
	}

	if a.instructions != "" {
		systemMessage += fmt.Sprintf("<instructions>\n%s\n</instructions>\n", a.ApplySemanticCompression(a.instructions))
		originalSystemMessage += fmt.Sprintf("<instructions>\n%s\n</instructions>\n", a.instructions)
	}

	if a.expected_output != "" {
		systemMessage += fmt.Sprintf("<expected_output>\n%s\n</expected_output>\n", a.ApplySemanticCompression(a.expected_output))
		originalSystemMessage += fmt.Sprintf("<expected_output>\n%s\n</expected_output>\n", a.expected_output)
	}

	// Add user memories if enabled and available
	if a.enableUserMemories && a.memory != nil && a.userID != "" {
		userMemories, err := a.memory.GetUserMemories(a.ctx, a.userID)
		if err == nil && len(userMemories) > 0 {
			memoryContent := ""
			// Limit to recent memories (last 10)
			maxMemories := 10
			if len(userMemories) > maxMemories {
				userMemories = userMemories[len(userMemories)-maxMemories:]
			}
			for _, memory := range userMemories {
				memoryContent += fmt.Sprintf("- %s\n", memory.Memory)
			}
			systemMessage += fmt.Sprintf("<user_memories>\nWhat I know about the user:\n%s</user_memories>\n", memoryContent)
			originalSystemMessage += fmt.Sprintf("<user_memories>\nWhat I know about the user:\n%s</user_memories>\n", memoryContent)
		}
	}

	if a.markdown {
		a.additional_information = append(a.additional_information, "Use markdown to format your answers.")
	}

	//if have Knowledge, search for relevant documents
	if a.knowledge != nil {
		relevantDocs, err := a.knowledge.Search(a.ctx, prompt, a.knowledgeMaxDocuments)
		if err == nil && len(relevantDocs) > 0 {
			docContent := ""
			for _, doc := range relevantDocs {
				snippet := doc.Document.Content
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				docContent += fmt.Sprintf("- %s\n", snippet)
			}
			systemMessage += fmt.Sprintf("<knowledge>\nRelevant information I found:\n%s</knowledge>\n", a.ApplySemanticCompression(docContent))
			originalSystemMessage += fmt.Sprintf("<knowledge>\nRelevant information I found:\n%s</knowledge>\n", docContent)
		}
	}

	if len(a.additional_information) > 0 {
		systemMessage += fmt.Sprintf("<additional_information>\n%s\n</additional_information>\n", strings.Join(a.additional_information, "\n"))
	}

	if len(a.contextData) > 0 {
		contextStr := utils.PrettyPrintMap(a.contextData)
		systemMessage += fmt.Sprintf("<context>\n%s\n</context>\n", a.ApplySemanticCompression(contextStr))
		originalSystemMessage += fmt.Sprintf("<context>\n%s\n</context>\n", contextStr)
	}

	// Add output schema or output model instructions if configured
	if a.outputSchema != nil {
		schemaInstructions, err := a.addOutputSchemaToPrompt("")
		if err == nil {
			systemMessage += schemaInstructions
			originalSystemMessage += schemaInstructions
		}
	}

	// Add additional context at the end if provided
	if a.additionalContext != "" {
		systemMessage += fmt.Sprintf("\n<additional_context>\n%s\n</additional_context>\n", a.additionalContext)
		originalSystemMessage += fmt.Sprintf("\n<additional_context>\n%s\n</additional_context>\n", a.additionalContext)
	}

	// Add cultural context if enabled
	if a.addCultureToContext && a.cultureManager != nil {
		// Try to cast cultureManager to the correct type
		if cm, ok := a.cultureManager.(interface {
			AddCultureToContext(ctx context.Context, userID string) (string, error)
		}); ok {
			// Use UserID if available, otherwise use a default
			userID := a.userID
			if userID == "" {
				userID = "default_user"
			}

			culturalContext, err := cm.AddCultureToContext(a.ctx, userID)
			if err != nil {
				log.Printf("Warning: Failed to add cultural context: %v", err)
			} else if culturalContext != "" {
				systemMessage += culturalContext
				originalSystemMessage += culturalContext
			}
		}
	}

	if a.debug {
		utils.DebugPanel(systemMessage)
	}

	messages := []models.Message{}

	if systemMessage != "" {
		messages = append(messages, models.Message{
			Role:    models.TypeSystemRole,
			Content: systemMessage,
		})
	}

	// Add chat history if enabled
	if a.addHistoryToMessages && len(a.messages) > 0 {
		historyMessages := a.filterToolCallsFromHistory(a.messages)
		messages = append(messages, historyMessages...)
	}

	compressedPrompt := a.ApplySemanticCompression(prompt)

	if a.debug && a.enableSemanticCompression {
		encoder, _ := gpt3encoder.NewEncoder()
		// Check token length
		tokensSemantic, err := encoder.Encode(systemMessage)
		if err != nil {
			log.Printf("ERROR: Token encoding tokensSemantic failed: %v\n", err)
		}

		tokensOriginal, err := encoder.Encode(originalSystemMessage)
		if err != nil {
			log.Printf("ERROR: Token encoding tokensOriginal failed: %v\n", err)
		}

		fmt.Println("--------------------------------------System Compression-------------------------------------------------------------")
		fmt.Printf("DEBUG: Original Message System \n\n %s\n\n", originalSystemMessage)
		fmt.Printf("DEBUG: Applying semantic compression original message tokens: %d \n", len(tokensOriginal))
		// Check for token length reduction
		fmt.Printf("DEBUG: Compressed Message \n\n %s \n\n", systemMessage)
		fmt.Printf("DEBUG: Applying semantic compression compressed message tokens: %d\n", len(tokensSemantic))
		fmt.Println("--------------------------------------------------------------------------------------------------------------------------")

		tokensPromptSemantic, _ := encoder.Encode(compressedPrompt)
		tokensPromptOriginal, _ := encoder.Encode(originalPrompt)

		fmt.Println("--------------------------------------Prompt Compression-------------------------------------------------------------")
		fmt.Printf("DEBUG: Original Prompt \n\n %s\n\n", originalPrompt)
		fmt.Printf("DEBUG: Applying semantic compression original prompt tokens: %d \n", len(tokensPromptOriginal))
		// Check for token length reduction
		fmt.Printf("DEBUG: Compressed Prompt \n\n %s \n\n", compressedPrompt)
		fmt.Printf("DEBUG: Applying semantic compression compressed prompt tokens: %d\n", len(tokensPromptSemantic))
		fmt.Println("--------------------------------------------------------------------------------------------------------------------------")

	}

	messages = append(messages, models.Message{
		Role:    models.TypeUserRole,
		Content: compressedPrompt,
	})

	return messages
}

// loadSession loads existing session data from storage
func (a *Agent) loadSession() error {
	if a.db == nil || a.sessionID == "" {
		return nil
	}

	// Load session
	session, err := a.db.ReadSession(a.ctx, a.sessionID)
	if err != nil {
		// Session doesn't exist, create new one
		if err.Error() == "session not found" {
			session = &storage.AgentSession{
				Session: storage.Session{
					SessionID:   a.sessionID,
					UserID:      a.userID,
					Memory:      make(map[string]interface{}),
					SessionData: make(map[string]interface{}),
					ExtraData:   make(map[string]interface{}),
					CreatedAt:   time.Now().Unix(),
					UpdatedAt:   time.Now().Unix(),
				},
				AgentID:   "default-agent",
				AgentData: make(map[string]interface{}),
			}
			if err := a.db.CreateSession(a.ctx, session); err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
		} else {
			return fmt.Errorf("failed to load session: %w", err)
		}
	}

	// Load session state from session_data if enable_agentic_state is enabled
	if a.enableAgenticState && session != nil && session.SessionData != nil {
		if sessionState, ok := session.SessionData["session_state"].(map[string]interface{}); ok {
			// Merge loaded session state with in-memory state
			for k, v := range sessionState {
				a.sessionState[k] = v
			}
		}
	}

	// Load runs if history is enabled
	if a.addHistoryToMessages {
		runs, err := a.db.GetRunsForSession(a.ctx, a.sessionID)
		if err != nil {
			return fmt.Errorf("failed to load session runs: %w", err)
		}

		// Keep only the most recent runs based on numHistoryRuns
		if a.numHistoryRuns > 0 && len(runs) > a.numHistoryRuns {
			runs = runs[len(runs)-a.numHistoryRuns:]
		}

		a.runs = runs

		// Build message history from runs
		a.buildMessageHistoryFromRuns()
	}

	return nil
}

// buildMessageHistoryFromRuns reconstructs message history from stored runs
func (a *Agent) buildMessageHistoryFromRuns() {
	a.messages = []models.Message{}

	for _, run := range a.runs {
		// Add user message
		if run.UserMessage != "" {
			a.messages = append(a.messages, models.Message{
				Role:    "user",
				Content: run.UserMessage,
			})
		}

		// Add assistant response
		if run.AgentMessage != "" {
			a.messages = append(a.messages, models.Message{
				Role:    "assistant",
				Content: run.AgentMessage,
			})
		}
	}
}

// saveRun saves a completed run to storage
func (a *Agent) saveRun(userMessage, agentResponse string, messages []models.Message) error {
	if a.db == nil {
		return nil
	}

	// Convert messages to map format for storage
	messagesMaps := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		messagesMaps[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	run := &storage.AgentRun{
		ID:           uuid.New().String(),
		SessionID:    a.sessionID,
		UserID:       a.userID,
		RunName:      fmt.Sprintf("run_%d", time.Now().Unix()),
		RunData:      make(map[string]interface{}),
		UserMessage:  userMessage,
		AgentMessage: agentResponse,
		Messages:     messagesMaps,
		Metrics:      make(map[string]interface{}),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := a.db.CreateRun(a.ctx, run); err != nil {
		return fmt.Errorf("failed to save run: %w", err)
	}

	// Add to local runs list
	a.runs = append(a.runs, run)

	// Keep only the most recent runs in memory
	if a.numHistoryRuns > 0 && len(a.runs) > a.numHistoryRuns {
		a.runs = a.runs[len(a.runs)-a.numHistoryRuns:]
	}

	return nil
}

// processMemories handles memory extraction and session summarization
func (a *Agent) processMemories(userMessage, agentResponse string) error {
	if a.memory == nil {
		return nil
	}

	// Extract and save user memories if enabled
	if a.enableAgenticMemory && a.userID != "" {
		_, err := a.memory.CreateMemory(a.ctx, a.userID, userMessage, agentResponse)
		if err != nil {
			// Log error but don't fail the whole operation
			if a.debug {
				fmt.Printf("Warning: Failed to create memory: %v\n", err)
			}
		}
	}

	// Generate session summary if enabled
	if a.enableSessionSummaries && a.userID != "" && a.sessionID != "" {
		// Check if we need to create/update session summary
		// This could be done periodically or based on number of interactions
		runCount := len(a.runs)
		if runCount > 0 && runCount%5 == 0 { // Summarize every 5 interactions
			conversation := []map[string]interface{}{}
			for _, run := range a.runs {
				if run.UserMessage != "" {
					conversation = append(conversation, map[string]interface{}{
						"role":    "user",
						"content": run.UserMessage,
					})
				}
				if run.AgentMessage != "" {
					conversation = append(conversation, map[string]interface{}{
						"role":    "assistant",
						"content": run.AgentMessage,
					})
				}
			}

			_, err := a.memory.CreateSessionSummary(a.ctx, a.userID, a.sessionID, conversation)
			if err != nil {
				// Log error but don't fail the whole operation
				if a.debug {
					fmt.Printf("Warning: Failed to create session summary: %v\n", err)
				}
			}
		}
	}

	return nil
}

func (a *Agent) RunStream(prompt string, fn func([]byte) error) error {
	messages := a.prepareMessages(prompt)

	// Collect streaming content for memory processing
	var fullResponse strings.Builder

	opts := []models.Option{
		models.WithTools(a.tools),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			// Collect content for memory processing
			fullResponse.Write(chunk)

			return fn(chunk)
		}),
	}

	err := a.model.InvokeStream(a.ctx, messages, opts...)

	// After streaming is complete, process memory and storage
	if err == nil {
		responseContent := fullResponse.String()

		// Save run to storage if enabled
		if a.db != nil {
			if saveErr := a.saveRun(prompt, responseContent, messages); saveErr != nil && a.debug {
				fmt.Printf("Warning: Failed to save run: %v\n", saveErr)
			}
		}

		// Process memories if enabled
		if a.memory != nil {
			if memErr := a.processMemories(prompt, responseContent); memErr != nil && a.debug {
				fmt.Printf("Warning: Failed to process memories: %v\n", memErr)
			}
		}

		// Update message history for next interaction
		if a.addHistoryToMessages {
			a.messages = append(a.messages, models.Message{
				Role:    "user",
				Content: prompt,
			})
			a.messages = append(a.messages, models.Message{
				Role:    "assistant",
				Content: responseContent,
			})

			// Keep only recent messages based on history limit
			if a.numHistoryRuns > 0 {
				maxMessages := a.numHistoryRuns * 2 // user + assistant per run
				if len(a.messages) > maxMessages {
					a.messages = a.messages[len(a.messages)-maxMessages:]
				}
			}
		}
	}

	return err

}

// Reason executa o reasoning chain usando o modelo configurado.
func (a *Agent) Reason(prompt string) ([]models.ReasoningStep, error) {
	// The model needs to implement the Invoke method.
	invoker := func(ctx context.Context, msgs []string) (string, error) {
		resp, err := a.Run(prompt)
		if err != nil {
			return "", err
		}
		return resp.Messages[0].Thinking, nil
	}

	return reasoning.ReasoningChain(a.ctx, invoker, prompt, a.reasoningMinSteps, a.reasoningMaxSteps)
}

func (a *Agent) ApplySemanticCompression(message string) string {
	if !a.enableSemanticCompression {
		return message
	}

	encoder, _ := gpt3encoder.NewEncoder()
	// Check token length
	tokens, _ := encoder.Encode(message)
	if a.debug {
		fmt.Printf("DEBUG: Applying semantic compression to %d tokens\n", tokens)
	}
	if a.semanticMaxTokens == 0 || len(tokens) < a.semanticMaxTokens {
		// No need to compress
		return message
	}
	var semanticAgent *Agent
	var err error
	var msgcompressed string

	if a.semanticModel != nil && a.semanticAgent == nil {

		semanticAgent, err = NewAgent(AgentConfig{
			Context:      a.ctx,
			Name:         "SemanticCompressor",
			Description:  "Semantic text compression agent.",
			Instructions: "Replace the input text with an ultra-concise version using abbreviations, technical notation, and minimal wording. Preserve all essential facts (dates, versions, IDs, deadlines). Return only the compressed result in the same language as the input. Do not add explanations or comments.",
			Model:        a.semanticModel,
			Markdown:     false,
			Debug:        false,
		})

		if err != nil {
			log.Fatalf("Failed to create assistant agent: %v", err)
		}
	}

	if a.semanticAgent != nil && a.semanticModel == nil {

		newmsg, err := a.semanticAgent.Run(message)
		if err != nil {
			if a.debug {
				fmt.Printf("Warning: Semantic compression failed for message: %v\n", err)
			}

		}
		msgcompressed = newmsg.Messages[0].Content
	}

	if a.semanticModel != nil && a.semanticAgent == nil {

		newmsg, err := semanticAgent.Run(message)
		if err != nil {
			if a.debug {
				fmt.Printf("Warning: Semantic compression failed for message: %v\n", err)
			}

		}
		msgcompressed = newmsg.Messages[0].Content
	}

	return msgcompressed
}

// prepareToolArgumentsForChain prepares arguments for a tool in ChainTool mode
func (a *Agent) prepareToolArgumentsForChain(tool toolkit.Tool, input interface{}) map[string]interface{} {
	args := make(map[string]interface{})

	// Get method names
	methods := tool.GetMethods()
	if len(methods) == 0 {
		return args
	}

	// Use the first method (most tools have one method)
	var methodName string
	for name := range methods {
		methodName = name
		break
	}

	// Get the parameter schema
	schema := tool.GetParameterStruct(methodName)

	// Extract parameter names from schema
	var paramNames []string
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for key := range props {
			paramNames = append(paramNames, key)
		}
	}

	// If no parameters defined, return empty args
	if len(paramNames) == 0 {
		return args
	}

	// Use the first parameter (most tools have one main input parameter)
	firstParam := paramNames[0]

	// Handle different input types
	switch inputValue := input.(type) {
	case string:
		// Try to parse as JSON first
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(inputValue), &parsed); err == nil {
			// Successfully parsed as JSON
			args[firstParam] = parsed
		} else {
			// Use as string
			args[firstParam] = inputValue
		}
	case map[string]interface{}:
		// Pass map directly
		args[firstParam] = inputValue
	default:
		// Convert to appropriate type
		args[firstParam] = inputValue
	}

	return args
}

// processToolCallsFromResponse processes tool calls from model response and returns final result
// Returns: (finalResult, toolMessages, chainToolExecuted, firstToolInput, error)
func (a *Agent) processToolCallsFromResponse(resp *models.MessageResponse) (string, []models.Message, bool, string, error) {
	utils.InfoPanel("Processing tool calls from model response")

	var toolMessages []models.Message
	var finalResult string
	var chainToolWasExecuted bool
	var firstToolInput string

	// Process each tool call
	for callIndex, toolCall := range resp.ToolCalls {
		utils.InfoPanel(fmt.Sprintf("Executing tool call: %s", toolCall.Function.Name))

		// Find the tool - check both wrapped and non-wrapped tools
		var tool toolkit.Tool
		for _, t := range a.tools {
			// Check if it's a ToolWrapper
			if wrapper, ok := t.(*ToolWrapper); ok {
				if wrapper.GetName() == toolCall.Function.Name {
					tool = wrapper
					break
				}
			} else if t.GetName() == toolCall.Function.Name {
				// Direct tool without wrapper
				tool = t
				break
			}
		}

		if tool == nil {
			return "", nil, false, "", fmt.Errorf("tool %s not found", toolCall.Function.Name)
		}

		// Get method name from tool
		methods := tool.GetMethods()
		var methodName string
		for name := range methods {
			methodName = name
			break
		}

		// Capture the first tool's input for later substitution in ChainTool mode
		if callIndex == 0 && a.enableChainTool {
			var inputMap map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &inputMap); err == nil {
				// Try to extract the first parameter value as string
				if len(inputMap) > 0 {
					for _, v := range inputMap {
						if str, ok := v.(string); ok {
							firstToolInput = str
							break
						}
					}
				}
			}
		}

		// Execute the tool with proper method name
		argsJSON := json.RawMessage(toolCall.Function.Arguments)
		result, err := tool.Execute(methodName, argsJSON)
		if err != nil {
			return "", nil, false, "", fmt.Errorf("tool execution failed for %s: %w", toolCall.Function.Name, err)
		}

		utils.InfoPanel(fmt.Sprintf("Tool %s completed, result: %v", toolCall.Function.Name, result))

		// Add tool message to history
		toolMessages = append(toolMessages, models.Message{
			Role:       "tool",
			Content:    fmt.Sprintf("%v", result),
			ToolCallID: &toolCall.ID,
		})

		// DEBUG: Check ChainTool state
		utils.InfoPanel(fmt.Sprintf("DEBUG: enableChainTool=%v, len(tools)=%d, callIndex=%d", a.enableChainTool, len(a.tools), callIndex))

		// In ChainTool mode, check if this was the final result from the chain
		if a.enableChainTool && len(a.tools) > 1 {
			utils.InfoPanel(fmt.Sprintf("ChainTool: Tool execution completed with result: %v", result))
			chainToolWasExecuted = true
		}

		// In ChainTool mode, execute remaining tools in sequence
		if a.enableChainTool && len(a.tools) > 1 && callIndex == 0 {
			// After first tool executes, run the chain for remaining tools
			// Extract the unwrapped tool if it's a wrapper
			var unwrappedTool toolkit.Tool = tool
			if wrapper, ok := tool.(*ToolWrapper); ok {
				unwrappedTool = wrapper.Tool
			}

			chainResult, err := a.executeChainFromTool(unwrappedTool, result)
			if err != nil {
				utils.InfoPanel(fmt.Sprintf("ChainTool: Chain execution warning: %v", err))
			} else if chainResult != nil {
				result = chainResult
			}
		}

		// Convert result to string for final response
		if str, ok := result.(string); ok {
			finalResult = str
		} else {
			resultJSON, err := json.Marshal(result)
			if err != nil {
				finalResult = fmt.Sprintf("%v", result)
			} else {
				finalResult = string(resultJSON)
			}
		}
	}

	return finalResult, toolMessages, chainToolWasExecuted, firstToolInput, nil
}

// executeChainFromTool executes remaining tools in sequence after the given tool was called by the model
// This implements the ChainTool behavior where one tool call triggers the execution of all subsequent tools
func (a *Agent) executeChainFromTool(executedTool toolkit.Tool, result interface{}) (interface{}, error) {
	if a.debug {
		utils.ToolCallPanel("Executing Chain Tools")
	}
	// Find the index of the executed tool
	executedIndex := -1
	for i, tool := range a.tools {
		// Compare both wrapped and unwrapped tools
		var toolToCompare toolkit.Tool
		if wrapper, ok := tool.(*ToolWrapper); ok {
			toolToCompare = wrapper.Tool
		} else {
			toolToCompare = tool
		}

		// Compare by identity and name
		if toolToCompare == executedTool || toolToCompare.GetName() == executedTool.GetName() {
			executedIndex = i
			break
		}
	}

	if executedIndex == -1 {
		if a.debug {
			utils.InfoPanel("ChainTool: Executed tool not found in agent tools list")
		}
		return result, nil // Tool not found, return original result
	}

	// If this was the last tool, no chaining needed
	if executedIndex >= len(a.tools)-1 {
		return result, nil
	}

	var currentInput interface{} = result
	var finalResult interface{} = result

	// Execute remaining tools in sequence
	for i := executedIndex + 1; i < len(a.tools); i++ {
		tool := a.tools[i]

		// Prepare arguments for the tool
		args := a.prepareToolArgumentsForChain(tool, currentInput)

		// Execute before hooks
		if err := a.ExecuteToolBeforeHooks(a.ctx, tool.GetName(), args); err != nil {
			utils.ErrorPanel(fmt.Errorf("ChainTool: Before hook failed for %s: %v", tool.GetName(), err))
			continue // Continue with chain even if hook fails
		}

		// Execute the tool
		argsJSON, err := json.Marshal(args)
		if err != nil {
			utils.ErrorPanel(fmt.Errorf("ChainTool: Failed to marshal arguments for %s: %v", tool.GetName(), err))
			continue
		}

		// Get method name
		methods := tool.GetMethods()
		var methodName string
		for name := range methods {
			methodName = name
			break
		}

		if a.debug {
			utils.ToolCallPanelWithArgs(fmt.Sprintf("Executing Tool Chain: %v", tool.GetName()), args)
		}

		toolResult, err := tool.Execute(methodName, json.RawMessage(argsJSON))
		if err != nil {
			utils.ErrorPanel(fmt.Errorf("ChainTool: Tool execution failed for %s: %v", tool.GetName(), err))
			continue // Continue with chain even if tool fails
		}

		if a.debug {
			utils.ToolCallPanelWithArgs(fmt.Sprintf("Result  Tool Chain: %v", tool.GetName()), toolResult)
		}

		// Execute after hooks
		if err := a.ExecuteToolAfterHooks(a.ctx, tool.GetName(), args, toolResult); err != nil {
			utils.ErrorPanel(fmt.Errorf("chainTool: After hook failed for %v: %v", tool.GetName(), err.Error()))
			continue
		}

		// Update current input for next tool
		currentInput = toolResult
		finalResult = toolResult
	}

	return finalResult, nil
}

// AddTool adds a new tool to the agent dynamically at runtime
func (a *Agent) AddTool(tool toolkit.Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	if tool.GetName() == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	// Check if tool already exists
	if a.GetToolByName(tool.GetName()) != nil {
		return fmt.Errorf("tool with name '%s' already exists", tool.GetName())
	}

	a.tools = append(a.tools, tool)
	return nil
}

// RemoveTool removes a tool from the agent by its name
func (a *Agent) RemoveTool(toolName string) error {
	if toolName == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	for i, tool := range a.tools {
		if tool.GetName() == toolName {
			a.tools = append(a.tools[:i], a.tools[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("tool '%s' not found", toolName)
}

// GetTools returns all tools currently in the agent
func (a *Agent) GetTools() []toolkit.Tool {
	return a.tools
}

// GetToolByName retrieves a specific tool by its name
func (a *Agent) GetToolByName(name string) toolkit.Tool {
	if name == "" {
		return nil
	}

	for _, tool := range a.tools {
		if tool.GetName() == name {
			return tool
		}
	}

	return nil
}
