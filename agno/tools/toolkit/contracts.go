package toolkit

import (
	"encoding/json"
	"reflect"
	"sync"
	"time"
)

// HookFunc is a function that can be executed before or after a tool method call.
// It receives the method name and the raw JSON input.
// Returning an error from a PreHook aborts the execution.
type HookFunc func(methodName string, input json.RawMessage) error

// PostHookFunc is a function executed after a tool method call.
// It receives the method name, the raw JSON input, the result and any error from execution.
type PostHookFunc func(methodName string, input json.RawMessage, result interface{}, err error)

// CacheConfig controls per-toolkit result caching behavior.
type CacheConfig struct {
	Enabled bool
	TTL     time.Duration
}

type cacheEntry struct {
	result    interface{}
	err       error
	expiresAt time.Time
}

// Toolkit stores the tool information and its registered methods.
type Toolkit struct {
	Name        string
	Description string
	// Timeout bounds the primary I/O operation of the toolkit (e.g. HTTP
	// requests via HTTPClient). Zero means DefaultHTTPTimeout.
	Timeout time.Duration
	methods map[string]Method
	// Hook system
	preHooks  []HookFunc
	postHooks []PostHookFunc
	// Caching
	Cache CacheConfig
	cache *sync.Map
	// Filtering
	includedTools map[string]bool
	excludedTools map[string]bool
}

// Method stores the execution function and its parameter schema.
type Method struct {
	Receiver    interface{}
	Description string
	Function    interface{}
	Schema      map[string]interface{}
	ParamType   reflect.Type
	// RequiresConfirmation indicates that the agent should ask for user confirmation
	// before executing this method (e.g., destructive operations like delete, payment).
	RequiresConfirmation bool
	// StopAfterCall tells the agent to stop the tool-calling loop after this method executes.
	StopAfterCall bool
	// ShowResult makes the agent include the method result in the run content.
	ShowResult bool
	// RequiresUserInput pauses the run until the user provides the
	// UserInputFields (all parameters when UserInputFields is empty).
	RequiresUserInput bool
	// UserInputFields lists the parameters provided by the user. They are
	// hidden from the model-facing schema.
	UserInputFields []string
	// UserInputSchema describes the user-provided parameters (derived at registration).
	UserInputSchema []UserInputFieldSpec
	// UserInputBuilder builds the user-input fields from the call arguments
	// (WithUserInputFromArgs).
	UserInputBuilder UserInputBuilder
	// UserFeedbackBuilder builds feedback questions from the call arguments
	// (WithUserFeedback).
	UserFeedbackBuilder UserFeedbackBuilder
	// ExternalExecution pauses the run: the call is executed outside the agent
	// and its result handed back through ContinueRun.
	ExternalExecution bool
	// ExternalExecutionSilent suppresses the verbose paused message for external execution.
	ExternalExecutionSilent bool
	// ApprovalType is ApprovalRequired (blocking) or ApprovalAudit (audit trail), or empty.
	ApprovalType string
	// Per-method hooks (in addition to toolkit-level hooks)
	PreHooks  []HookFunc
	PostHooks []PostHookFunc
}

// Tool is the interface that defines the basic operations for any tool.
type Tool interface {
	GetName() string                                                       // Returns the tool name
	GetDescription() string                                                // Returns the tool description
	GetParameterStruct(methodName string) map[string]interface{}           // Returns the JSON schema based on the registered method
	GetMethods() map[string]Method                                         // Returns the registered methods
	GetFunction(methodName string) interface{}                             // Returns the execution function
	GetDescriptionOfMethod(methodName string) string                       // Returns the description of a specific method
	Execute(methodName string, input json.RawMessage) (interface{}, error) // Executes the function
}

// ConnectableTool is an optional interface for tools that require connection lifecycle management.
// Tools that connect to external services (databases, gRPC, APIs with sessions) should implement this.
type ConnectableTool interface {
	Tool
	Connect() error
	Close() error
}
