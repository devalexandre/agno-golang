package agent

import (
	"encoding/json"
)

// RunOption is a function type for configuring agent runs
type RunOption func(*RunOptions)

// RunOptions contains optional parameters for the Run method
type RunOptions struct {
	// Stream enables streaming response
	Stream *bool
	// StreamEvents enables streaming of intermediate events
	StreamEvents *bool
	// StreamIntermediateSteps is deprecated, use StreamEvents instead
	StreamIntermediateSteps *bool
	// UserID specifies the user making the request
	UserID *string
	// SessionID specifies the session for this run
	SessionID *string
	// SessionState contains state to persist across runs
	SessionState map[string]interface{}
	// Audio inputs
	Audio []Audio
	// Images inputs
	Images []Image
	// Videos inputs
	Videos []Video
	// Files inputs
	Files []File
	// Retries number of retry attempts
	Retries *int
	// KnowledgeFilters for filtering knowledge base queries
	KnowledgeFilters map[string]interface{}
	// AddHistoryToContext includes conversation history in context
	AddHistoryToContext *bool
	// AddDependenciesToContext includes dependencies in context
	AddDependenciesToContext *bool
	// AddSessionStateToContext includes session state in context
	AddSessionStateToContext *bool
	// Dependencies available for tools and prompt functions
	Dependencies map[string]interface{}
	// Metadata for this run
	Metadata map[string]interface{}
	// DebugMode enables detailed debug logging
	DebugMode *bool
	// SmartMemoryManager configuration for this run
	SmartMemoryManager *SmartMemoryManagerOptions
}

// WithStream enables streaming response
func WithStream(stream bool) RunOption {
	return func(o *RunOptions) {
		o.Stream = &stream
	}
}

// WithStreamEvents enables streaming of intermediate events
func WithStreamEvents(streamEvents bool) RunOption {
	return func(o *RunOptions) {
		o.StreamEvents = &streamEvents
	}
}

// WithUserID specifies the user making the request
func WithUserID(userID string) RunOption {
	return func(o *RunOptions) {
		o.UserID = &userID
	}
}

// WithSessionID specifies the session for this run
func WithSessionID(sessionID string) RunOption {
	return func(o *RunOptions) {
		o.SessionID = &sessionID
	}
}

// WithSessionState sets session state for this run
func WithSessionState(sessionState map[string]interface{}) RunOption {
	return func(o *RunOptions) {
		o.SessionState = sessionState
	}
}

// WithImages adds image inputs to the run
func WithImages(images ...Image) RunOption {
	return func(o *RunOptions) {
		o.Images = images
	}
}

// WithAudio adds audio inputs to the run
func WithAudio(audio ...Audio) RunOption {
	return func(o *RunOptions) {
		o.Audio = audio
	}
}

// WithVideos adds video inputs to the run
func WithVideos(videos ...Video) RunOption {
	return func(o *RunOptions) {
		o.Videos = videos
	}
}

// WithFiles adds file inputs to the run
func WithFiles(files ...File) RunOption {
	return func(o *RunOptions) {
		o.Files = files
	}
}

// WithRetries sets number of retry attempts
func WithRetries(retries int) RunOption {
	return func(o *RunOptions) {
		o.Retries = &retries
	}
}

// WithKnowledgeFilters sets knowledge filters for this run
func WithKnowledgeFilters(knowledgeFilters map[string]interface{}) RunOption {
	return func(o *RunOptions) {
		o.KnowledgeFilters = knowledgeFilters
	}
}

// WithAddHistoryToContext includes conversation history in context
func WithAddHistoryToContext(addHistoryToContext bool) RunOption {
	return func(o *RunOptions) {
		o.AddHistoryToContext = &addHistoryToContext
	}
}

// WithAddDependenciesToContext includes dependencies in context
func WithAddDependenciesToContext(addDependenciesToContext bool) RunOption {
	return func(o *RunOptions) {
		o.AddDependenciesToContext = &addDependenciesToContext
	}
}

// WithAddSessionStateToContext includes session state in context
func WithAddSessionStateToContext(addSessionStateToContext bool) RunOption {
	return func(o *RunOptions) {
		o.AddSessionStateToContext = &addSessionStateToContext
	}
}

// WithDependencies sets dependencies available for tools and prompt functions
func WithDependencies(dependencies map[string]interface{}) RunOption {
	return func(o *RunOptions) {
		o.Dependencies = dependencies
	}
}

// WithDebugMode enables detailed debug logging
func WithDebugMode(debugMode bool) RunOption {
	return func(o *RunOptions) {
		o.DebugMode = &debugMode
	}
}

// WithMetadata sets metadata for this run
func WithMetadata(metadata map[string]interface{}) RunOption {
	return func(o *RunOptions) {
		o.Metadata = metadata
	}
}

// WithSmartMemoryManager configures smart memory manager for this run
func WithSmartMemoryManager(opts SmartMemoryManagerOptions) RunOption {
	return func(o *RunOptions) {
		o.SmartMemoryManager = &opts
	}
}

// Media types for agent inputs

// Audio represents an audio input
type Audio struct {
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
	Data     []byte `json:"data,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// Image represents an image input
type Image struct {
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
	Data     []byte `json:"data,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// Video represents a video input
type Video struct {
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
	Data     []byte `json:"data,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// File represents a file input
type File struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	Data     []byte `json:"data,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// SmartMemoryManagerOptions contains configuration options for smart memory management
// Used with WithSmartMemoryManager() option function
type SmartMemoryManagerOptions struct {
	// Enable smart memory manager
	Enabled bool
	// Model to use for memory management (if different from main agent model)
	Model interface{} // models.AgnoModelInterface
	// Custom prompt for memory extraction
	Prompt string
	// Maximum tokens for memory content
	MaxTokens int
	// Cache size for processed memories
	CacheSize int
}

// MarshalJSON implements custom serialization for RunOptions.
func (o *RunOptions) MarshalJSON() ([]byte, error) {
	type Alias RunOptions
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(o),
	})
}
