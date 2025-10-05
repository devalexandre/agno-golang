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
	mu          sync.RWMutex
	connected   bool
	initialized bool
	cancelFunc  context.CancelFunc
}

// Parameters for different MCP operations
type ListDirectoryParams struct {
	Path string `json:"path" description:"Directory path to list files and subdirectories"`
}

type ReadFileParams struct {
	Path string `json:"path" description:"File path to read"`
	Head int    `json:"head,omitempty" description:"Number of lines from beginning (optional)"`
	Tail int    `json:"tail,omitempty" description:"Number of lines from end (optional)"`
}

type WriteFileParams struct {
	Path    string `json:"path" description:"File path to write"`
	Content string `json:"content" description:"Content to write to file"`
}

type DirectoryTreeParams struct {
	Path string `json:"path" description:"Directory path to show tree structure"`
}

type GetFileInfoParams struct {
	Path string `json:"path" description:"File path to get information about"`
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
	tk.Name = "MCPFilesystem"
	tk.Description = "MCP filesystem tools for file and directory operations using Model Context Protocol"

	tool := &MCPTool{
		Toolkit:        tk,
		command:        command,
		timeoutSeconds: timeoutSeconds,
		logger:         logger,
	}

	// Register MCP methods following the same pattern as WeatherTool
	tool.Toolkit.Register("ListDirectory", tool, tool.ListDirectory, ListDirectoryParams{})
	tool.Toolkit.Register("ReadFile", tool, tool.ReadFile, ReadFileParams{})
	tool.Toolkit.Register("WriteFile", tool, tool.WriteFile, WriteFileParams{})
	tool.Toolkit.Register("DirectoryTree", tool, tool.DirectoryTree, DirectoryTreeParams{})
	tool.Toolkit.Register("GetFileInfo", tool, tool.GetFileInfo, GetFileInfoParams{})

	return tool, nil
}

// Connect establishes connection to MCP server
func (m *MCPTool) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connected {
		return nil
	}

	m.logger.Info("Connecting to MCP server", "command", m.command)

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

	m.logger.Info("Successfully connected to MCP server")
	return nil
}

// ListDirectory lists files in a directory
func (m *MCPTool) ListDirectory(params ListDirectoryParams) (interface{}, error) {
	return m.callMCPTool("list_directory", map[string]interface{}{
		"path": params.Path,
	})
}

// ReadFile reads content from a file
func (m *MCPTool) ReadFile(params ReadFileParams) (interface{}, error) {
	args := map[string]interface{}{
		"path": params.Path,
	}
	if params.Head > 0 {
		args["head"] = params.Head
	}
	if params.Tail > 0 {
		args["tail"] = params.Tail
	}
	return m.callMCPTool("read_file", args)
}

// WriteFile writes content to a file
func (m *MCPTool) WriteFile(params WriteFileParams) (interface{}, error) {
	return m.callMCPTool("write_file", map[string]interface{}{
		"path":    params.Path,
		"content": params.Content,
	})
}

// DirectoryTree shows directory tree structure
func (m *MCPTool) DirectoryTree(params DirectoryTreeParams) (interface{}, error) {
	return m.callMCPTool("directory_tree", map[string]interface{}{
		"path": params.Path,
	})
}

// GetFileInfo gets information about a file
func (m *MCPTool) GetFileInfo(params GetFileInfoParams) (interface{}, error) {
	return m.callMCPTool("get_file_info", map[string]interface{}{
		"path": params.Path,
	})
}

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

	m.logger.Debug("Calling MCP tool", "tool", toolName, "args", args)

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
		m.logger.Debug("MCP tool returned empty content", "tool", toolName)
		return "", nil
	}

	// Return first content item (most common case)
	firstContent := result.Content[0]
	switch content := firstContent.(type) {
	case *mcp.TextContent:
		m.logger.Debug("MCP tool success", "tool", toolName, "content_length", len(content.Text))
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
	m.logger.Info("MCP connection closed")

	return nil
}