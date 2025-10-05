package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPTool implements the Tool interface for MCP filesystem operations
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

// GenericParams represents parameters for any MCP tool call
type GenericParams struct {
	Args map[string]interface{} `json:"args" description:"Arguments for the MCP tool"`
}

// NewMCPTool creates a new MCP tool instance
func NewMCPTool(command string, timeoutSeconds int) (*MCPTool, error) {
	// Validate command
	if command == "" {
		return nil, errors.New("command cannot be empty")
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Initialize toolkit
	tk := toolkit.NewToolkit()
	tk.Name = "MCP"
	tk.Description = "MCP (Model Context Protocol) tools - dynamically loaded from MCP server"

	tool := &MCPTool{
		Toolkit:        tk,
		command:        command,
		timeoutSeconds: timeoutSeconds,
		logger:         logger,
	}

	// Note: Tools will be registered dynamically after connection via registerDynamicTools()
	return tool, nil
}

// Connect establishes connection to MCP server and registers dynamic tools
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

	// Register dynamic tools from the MCP server
	if err := m.registerDynamicTools(ctx); err != nil {
		m.logger.Error("Failed to register dynamic tools", "error", err)
		// Don't fail the connection, but log the error
	}

	return nil
}

// registerDynamicTools queries the MCP server for available tools and registers them dynamically
func (m *MCPTool) registerDynamicTools(ctx context.Context) error {
	if !m.connected || m.session == nil {
		return errors.New("not connected to MCP server")
	}

	// List available tools from MCP server
	toolsResponse, err := m.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("failed to list tools from MCP server: %w", err)
	}

	// Register each tool dynamically
	for _, tool := range toolsResponse.Tools {
		// Create a wrapper function for this specific MCP tool
		toolName := tool.Name
		toolFunc := m.createToolWrapper(toolName)

		// Generate parameter struct based on tool schema
		paramStruct := m.generateParamStruct(tool)

		// Register the tool with the toolkit
		m.Toolkit.Register(tool.Name, m, toolFunc, paramStruct)
	}

	return nil
}

// createToolWrapper creates a wrapper function for a specific MCP tool
func (m *MCPTool) createToolWrapper(toolName string) interface{} {
	// Return different wrapper functions based on tool type
	switch toolName {
	case "list_directory":
		return func(params struct {
			Path string `json:"path" description:"Directory path to list files and subdirectories. Use empty string or '.' for current directory"`
		}) (interface{}, error) {
			args := map[string]interface{}{
				"path": params.Path,
			}
			if params.Path == "" {
				args["path"] = "."
			}
			return m.callMCPTool(toolName, args)
		}
	case "read_file", "read_text_file":
		return func(params struct {
			Path string `json:"path" description:"File path to read"`
		}) (interface{}, error) {
			args := map[string]interface{}{
				"path": params.Path,
			}
			return m.callMCPTool(toolName, args)
		}
	case "write_file":
		return func(params struct {
			Path    string `json:"path" description:"File path to write"`
			Content string `json:"content" description:"Content to write to file"`
		}) (interface{}, error) {
			args := map[string]interface{}{
				"path":    params.Path,
				"content": params.Content,
			}
			return m.callMCPTool(toolName, args)
		}
	default:
		// Generic wrapper for unknown tools
		return func(params GenericParams) (interface{}, error) {
			args := params.Args
			if args == nil {
				args = make(map[string]interface{})
			}
			m.logger.Debug("Wrapper calling MCP tool", "tool", toolName, "args", args)
			return m.callMCPTool(toolName, args)
		}
	}
}

// generateParamStruct creates a parameter structure based on MCP tool schema
func (m *MCPTool) generateParamStruct(tool *mcp.Tool) interface{} {
	// For common filesystem tools, create specific param structs
	switch tool.Name {
	case "list_directory":
		return struct {
			Path string `json:"path" description:"Directory path to list files and subdirectories. Use empty string or '.' for current directory"`
		}{}
	case "read_file", "read_text_file":
		return struct {
			Path string `json:"path" description:"File path to read"`
		}{}
	case "write_file":
		return struct {
			Path    string `json:"path" description:"File path to write"`
			Content string `json:"content" description:"Content to write to file"`
		}{}
	case "create_directory":
		return struct {
			Path string `json:"path" description:"Directory path to create"`
		}{}
	case "search_files":
		return struct {
			Pattern string `json:"pattern" description:"Pattern to search for"`
			Path    string `json:"path,omitempty" description:"Directory to search in (optional)"`
		}{}
	case "get_file_info":
		return struct {
			Path string `json:"path" description:"File or directory path to get info about"`
		}{}
	default:
		// For unknown tools, return generic params
		return GenericParams{}
	}
}

// Note: Individual tool methods are no longer needed as they are registered dynamically
// from the MCP server's tool list via registerDynamicTools()

// callMCPTool executes an MCP tool with the given arguments
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

	// Call the tool
	result, err := m.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		m.logger.Error("MCP tool call failed", "tool", toolName, "error", err)
		return nil, fmt.Errorf("failed to call MCP tool %s: %w", toolName, err)
	}

	// Process result
	if len(result.Content) == 0 {
		return "", nil
	}

	// Return first content item (most common case)
	firstContent := result.Content[0]
	switch content := firstContent.(type) {
	case *mcp.TextContent:
		return content.Text, nil
	case *mcp.ImageContent:
		return map[string]interface{}{
			"type":     "image",
			"data":     content.Data,
			"mimeType": content.MIMEType,
		}, nil
	default:
		// Try to marshal as JSON for other types
		if data, err := json.Marshal(content); err == nil {
			return string(data), nil
		}
		fallback := fmt.Sprintf("%+v", content)
		return fallback, nil
	}
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
