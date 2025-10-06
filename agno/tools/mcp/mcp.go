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

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPTool implementa a interface Tool usando apenas ClientSession
// Descobre ferramentas dinamicamente e as expõe através da interface Tool
type MCPTool struct {
	name    string
	command string
	session *mcp.ClientSession
	client  *mcp.Client
	logger  *slog.Logger
	mu      sync.RWMutex

	// Cache das ferramentas MCP descobertas dinamicamente
	tools   map[string]*mcp.Tool
	methods map[string]toolkit.Method
}

// NewMCPTool cria uma nova instância MCPTool
func NewMCPTool(name, command string) (*MCPTool, error) {
	if command == "" {
		return nil, errors.New("command cannot be empty")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &MCPTool{
		name:    name,
		command: command,
		logger:  logger,
		tools:   make(map[string]*mcp.Tool),
		methods: make(map[string]toolkit.Method),
	}, nil
}

// Connect conecta ao servidor MCP e descobre as ferramentas
func (m *MCPTool) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Criar cliente MCP
	m.client = mcp.NewClient(&mcp.Implementation{
		Name:    "agno-mcp-client",
		Version: "1.0.0",
	}, nil)

	// Preparar comando
	parts := strings.Fields(m.command)
	cmd := exec.Command(parts[0], parts[1:]...)

	// Conectar usando CommandTransport
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := m.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP: %w", err)
	}

	m.session = session

	// Descobrir ferramentas disponíveis usando ClientSession.ListTools
	toolsResult, err := m.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	// Armazenar ferramentas e criar métodos dinamicamente
	for _, tool := range toolsResult.Tools {
		m.tools[tool.Name] = tool

		// Criar método para esta ferramenta
		m.methods[tool.Name] = toolkit.Method{
			Receiver:  m,
			Function:  m.createToolFunction(tool),
			ParamType: m.getParamType(tool),
		}
	}

	return nil
}

// createToolFunction cria uma função que chama a ferramenta MCP
func (m *MCPTool) createToolFunction(tool *mcp.Tool) interface{} {
	toolName := tool.Name

	// Retornar função que aceita qualquer struct e chama ClientSession.CallTool
	return func(params interface{}) (interface{}, error) {
		// Converter params para map[string]interface{}
		args, err := m.paramsToMap(params)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parameters: %w", err)
		}

		// Chamar ferramenta usando ClientSession.CallTool
		result, err := m.session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		})
		if err != nil {
			return nil, fmt.Errorf("MCP tool call failed: %w", err)
		}

		// Processar resultado
		return m.processResult(result)
	}
}

// paramsToMap converte qualquer struct em map[string]interface{}
func (m *MCPTool) paramsToMap(params interface{}) (map[string]interface{}, error) {
	if params == nil {
		return map[string]interface{}{}, nil
	}

	// Converter via JSON para máxima compatibilidade
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

// processResult processa o resultado da chamada MCP
func (m *MCPTool) processResult(result *mcp.CallToolResult) (interface{}, error) {
	if result.IsError {
		// Processar os erros contidos no Content
		errorMessages := []string{}
		for _, content := range result.Content {
			if textContent, ok := content.(*mcp.TextContent); ok {
				errorMessages = append(errorMessages, textContent.Text)
			}
		}
		errorMsg := strings.Join(errorMessages, "; ")
		m.logger.Error("MCP tool returned error", "message", errorMsg)
		return nil, fmt.Errorf("MCP tool error: %s", errorMsg)
	}

	if len(result.Content) == 0 {
		return "", nil
	}

	var response strings.Builder
	for i, content := range result.Content {
		switch c := content.(type) {
		case *mcp.TextContent:
			response.WriteString(c.Text)
			if i < len(result.Content)-1 {
				response.WriteString("\n")
			}
		case *mcp.ImageContent:
			response.WriteString("Image content received\n")
		default:
			// Try to extract text from unknown content types
			if data, err := json.Marshal(content); err == nil {
				response.WriteString(string(data))
			} else {
				response.WriteString(fmt.Sprintf("Unknown content type: %T", content))
			}
		}
	}

	return response.String(), nil
}

// getParamType retorna o tipo de parâmetro baseado no schema da ferramenta
func (m *MCPTool) getParamType(tool *mcp.Tool) reflect.Type {
	// Retornar tipo genérico que pode aceitar qualquer campo JSON
	return reflect.TypeOf(map[string]interface{}{})
}

// Close fecha a conexão MCP
func (m *MCPTool) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session != nil {
		return m.session.Close()
	}
	return nil
}

// Implementação da interface Tool

func (m *MCPTool) GetName() string {
	return m.name
}

func (m *MCPTool) GetDescription() string {
	return fmt.Sprintf("MCP integration for %s", m.name)
}

func (m *MCPTool) GetParameterStruct(methodName string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Retornar schema baseado na ferramenta MCP
	if tool, exists := m.tools[methodName]; exists && tool.InputSchema != nil {
		if schema, ok := tool.InputSchema.(map[string]interface{}); ok {
			return schema
		}
	}

	// Fallback para schema genérico
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (m *MCPTool) GetMethods() map[string]toolkit.Method {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Retornar cópia dos métodos descobertos
	result := make(map[string]toolkit.Method)
	for name, method := range m.methods {
		result[name] = method
	}
	return result
}

func (m *MCPTool) GetFunction(methodName string) interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if method, exists := m.methods[methodName]; exists {
		return method.Function
	}
	return nil
}

func (m *MCPTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	m.mu.RLock()
	tool, toolExists := m.tools[methodName]
	m.mu.RUnlock()

	if !toolExists {
		return nil, fmt.Errorf("tool %s not found", methodName)
	}

	// Converter JSON input para map
	var params map[string]interface{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal input: %w", err)
		}
	}

	// Chamar ferramenta usando ClientSession.CallTool
	result, err := m.session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      tool.Name,
		Arguments: params,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Processar resultado
	return m.processResult(result)
}
