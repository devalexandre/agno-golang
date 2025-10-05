package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPTool implements the Tool interface for MCP operations
// Follows the same pattern as Python agno-agi/agno implementation
type MCPTool struct {
	toolkit.Toolkit

	// Core MCP components
	client  *mcp.Client
	session *mcp.ClientSession

	// Configuration
	command        string
	timeoutSeconds int
	logger         *slog.Logger

	// State management
	mu         sync.RWMutex
	connected  bool
	cancelFunc context.CancelFunc
}

// NewMCPTool creates a new MCP tool instance
// Follows the same initialization pattern as Python MCPTools class
func NewMCPTool(command string, timeoutSeconds int) (*MCPTool, error) {
	// Validate command
	if command == "" {
		return nil, errors.New("command cannot be empty")
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Initialize toolkit (no tools are registered at creation - they come from MCP server)
	tk := toolkit.NewToolkit()
	tk.Name = "MCP"
	tk.Description = "MCP (Model Context Protocol) tools - dynamically loaded from MCP server"

	tool := &MCPTool{
		Toolkit:        tk,
		command:        command,
		timeoutSeconds: timeoutSeconds,
		logger:         logger,
	}

	return tool, nil
}

// Connect establishes connection to MCP server and registers dynamic tools
// Follows the same pattern as Python MCPTools.initialize() method
func (m *MCPTool) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connected {
		return nil
	}

	// Create context with cancel for cleanup
	ctx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel

	// Create MCP client
	m.client = mcp.NewClient(&mcp.Implementation{
		Name:    "agno-mcp-client",
		Version: "1.0.0",
	}, nil)

	// Prepare command
	parts := strings.Fields(m.command)
	cmd := exec.Command(parts[0], parts[1:]...)

	// Create MCP transport using CommandTransport
	transport := &mcp.CommandTransport{Command: cmd}

	// Start MCP session
	session, err := m.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	m.session = session
	m.connected = true

	// Initialize the session (same as Python initialize)
	if err := m.initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize MCP session: %w", err)
	}

	return nil
}

// initialize registers tools from MCP server dynamically
// This is the Go equivalent of Python's MCPTools.initialize() method
func (m *MCPTool) initialize(ctx context.Context) error {
	if !m.connected || m.session == nil {
		return errors.New("session not connected")
	}

	m.logger.Info("Initializing MCP session...")

	// List available tools from MCP server (same as Python list_tools())
	toolsResponse, err := m.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		m.logger.Error("Failed to list tools from MCP server", "error", err)
		return fmt.Errorf("failed to list tools from MCP server: %w", err)
	}

	m.logger.Info("Found MCP tools", "count", len(toolsResponse.Tools))

	// Register each tool dynamically (same as Python loop through filtered_tools)
	for _, tool := range toolsResponse.Tools {
		m.logger.Info("Registering MCP tool", "name", tool.Name, "description", tool.Description)

		if err := m.registerToolFunction(tool); err != nil {
			m.logger.Error("Failed to register tool", "tool", tool.Name, "error", err)
			continue // Don't fail entire initialization for one tool
		}

		m.logger.Info("Successfully registered tool", "name", tool.Name)
	}

	m.logger.Info("MCP toolkit initialized", "registered_tools", len(m.Toolkit.GetMethods()))
	return nil
}

// registerToolFunction registers a single MCP tool as a Function
// This is the Go equivalent of the Python tool registration loop
func (m *MCPTool) registerToolFunction(tool *mcp.Tool) error {
	// Create entrypoint for this tool (same as get_entrypoint_for_tool in Python)
	entrypoint := m.createEntrypointForTool(tool)

	// Log the schema we received
	schemaBytes, _ := json.Marshal(tool.InputSchema)
	m.logger.Info("Tool schema", "tool", tool.Name, "schema", string(schemaBytes))

	// Create a generic parameter struct that can hold any fields
	// This follows the same pattern as WeatherTool using WeatherParams{}
	paramStruct := m.createParamStruct(tool)

	// Register with toolkit using the exact same pattern as WeatherTool
	m.Toolkit.Register(tool.Name, m, entrypoint, paramStruct)

	return nil
}

// createEntrypointForTool creates an entrypoint function for an MCP tool
// This is the Go equivalent of get_entrypoint_for_tool function in Python
func (m *MCPTool) createEntrypointForTool(tool *mcp.Tool) interface{} {
	toolName := tool.Name

	// Return a function that matches the expected signature
	// The toolkit will call this function with parameters based on the inputSchema
	return func(params interface{}) (interface{}, error) {
		m.logger.Info("MCP tool called", "tool", toolName, "params_type", fmt.Sprintf("%T", params), "params_value", fmt.Sprintf("%+v", params))

		// Convert params to map[string]interface{} for MCP call
		args, err := m.paramsToMap(params)
		if err != nil {
			m.logger.Error("Failed to convert parameters", "tool", toolName, "error", err)
			return nil, fmt.Errorf("failed to convert parameters: %w", err)
		}

		m.logger.Info("Converted parameters", "tool", toolName, "args", args)

		// Call the actual MCP tool (same as Python session.call_tool)
		return m.callMCPTool(toolName, args)
	}
}

// paramsToMap converts any parameter type to map[string]interface{}
// This handles the parameter conversion needed for MCP calls
func (m *MCPTool) paramsToMap(params interface{}) (map[string]interface{}, error) {
	if params == nil {
		return map[string]interface{}{}, nil
	}

	// Use reflection to handle any parameter type
	v := reflect.ValueOf(params)

	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return map[string]interface{}{}, nil
		}
		v = v.Elem()
	}

	// Convert via JSON for maximum compatibility
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	var args map[string]interface{}
	if err := json.Unmarshal(paramBytes, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parameters: %w", err)
	}

	return args, nil
}

// createParamStruct creates a parameter struct based on the MCP tool schema
// This creates simple structs that the toolkit can use for parameter examples
func (m *MCPTool) createParamStruct(tool *mcp.Tool) interface{} {
	// For now, create a generic struct that can handle common MCP parameters
	// In the future, this could parse the JSON schema and create dynamic structs

	// Generic parameter struct that works for most MCP tools
	type GenericParams struct {
		Path        string `json:"path,omitempty" description:"File or directory path"`
		Content     string `json:"content,omitempty" description:"Content for file operations"`
		Pattern     string `json:"pattern,omitempty" description:"Search pattern"`
		Source      string `json:"source,omitempty" description:"Source path"`
		Destination string `json:"destination,omitempty" description:"Destination path"`
		Head        int    `json:"head,omitempty" description:"Number of lines from head"`
		Tail        int    `json:"tail,omitempty" description:"Number of lines from tail"`
	}

	return GenericParams{}
}

// callMCPTool executes an MCP tool with the given arguments
// This is the Go equivalent of the Python call_tool function
func (m *MCPTool) callMCPTool(toolName string, args map[string]interface{}) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.connected || m.session == nil {
		return nil, errors.New("not connected to MCP server")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(m.timeoutSeconds)*time.Second,
	)
	defer cancel()

	m.logger.Info("Calling MCP tool", "tool", toolName, "args", args)

	// Call the tool (same as Python session.call_tool(tool_name, kwargs))
	callParams := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	result, err := m.session.CallTool(ctx, callParams)
	if err != nil {
		m.logger.Error("MCP tool call failed", "tool", toolName, "error", err)
		return nil, fmt.Errorf("failed to call MCP tool %s: %w", toolName, err)
	}

	// Process result (same as Python result processing)
	if len(result.Content) == 0 {
		return "", nil
	}

	// Handle different content types (same as Python content processing)
	var responseStr strings.Builder
	for i, content := range result.Content {
		switch c := content.(type) {
		case *mcp.TextContent:
			responseStr.WriteString(c.Text)
			if i < len(result.Content)-1 {
				responseStr.WriteString("\n")
			}
		case *mcp.ImageContent:
			// Return structured image data (same as Python)
			return map[string]interface{}{
				"type":     "image",
				"data":     c.Data,
				"mimeType": c.MIMEType,
			}, nil
		default:
			// Handle other content types
			if data, err := json.Marshal(content); err == nil {
				responseStr.WriteString(string(data))
			} else {
				responseStr.WriteString(fmt.Sprintf("%+v", content))
			}
		}
	}

	m.logger.Info("MCP tool completed", "tool", toolName, "response_length", responseStr.Len())
	return responseStr.String(), nil
}

// Close closes the MCP connection and cleans up resources
func (m *MCPTool) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancelFunc != nil {
		m.cancelFunc()
	}

	if m.session != nil {
		if err := m.session.Close(); err != nil {
			m.logger.Error("Failed to close MCP session", "error", err)
		}
		m.session = nil
	}

	m.connected = false
	return nil
}
