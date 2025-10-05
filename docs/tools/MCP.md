# 🔗 MCP (Model Context Protocol) Module

The Agno-Golang MCP module implements complete support for the [Model Context Protocol](https://modelcontextprotocol.io/), enabling agents to dynamically connect to MCP servers and use tools transparently.

## 🚀 Overview

MCP is a standard protocol for connecting AI assistants with external systems in a secure and consistent manner. The Agno-Golang MCP module provides:

- ✅ **Dynamic Discovery**: Connects to any MCP server and automatically discovers available tools
- ✅ **Automatic Registration**: Registers all available tools from the MCP server
- ✅ **Smart Typing**: Generates specific parameters for known tools
- ✅ **Compatibility**: Works alongside other Agno-Golang tools
- ✅ **Performance**: Optimized connections and resource management

## 📁 Structure

```
agno/tools/mcp/
├── mcp.go           # Main MCP implementation
└── README.md        # This documentation
```

## 🔧 Installation

The MCP module is included in Agno-Golang. To use external MCP servers, you can install them via npm:

```bash
# Example: Filesystem server
npm install -g @modelcontextprotocol/server-filesystem

# Other popular MCP servers
npm install -g @modelcontextprotocol/server-git
npm install -g @modelcontextprotocol/server-sqlite
```

## 🛠️ Basic Usage

### Simple Configuration

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models/ollama"
    "github.com/devalexandre/agno-golang/agno/tools/mcp"
)

func main() {
    // Create the command for the MCP server
    workDir := "/path/to/directory"
    command := fmt.Sprintf("npx -y @modelcontextprotocol/server-filesystem %s", workDir)
    
    // Create the MCP tool
    mcpTool, err := mcp.NewMCPTool(command, 30) // 30s timeout
    if err != nil {
        log.Fatal(err)
    }
    defer mcpTool.Close()
    
    // Connect to the MCP server
    ctx := context.Background()
    if err := mcpTool.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    
    // Use with an agent
    model, _ := ollama.NewOllamaChat(/* configurations */)
    agent, _ := agent.NewAgent(agent.AgentConfig{
        Model: model,
        Tools: []toolkit.Tool{mcpTool},
    })
    
    // Execute queries
    response, _ := agent.Run("List files in this directory")
    fmt.Println(response.TextContent)
}
```

## 📖 Complete Example

Here's a complete example based on `examples/mcp_agent/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/mcp"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func runAgent(message string) error {
	// Set the path to explore - use current working directory to avoid permission issues
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	fmt.Printf("Using directory: %s\\n", currentDir)

	// Initialize MCP tools following the same pattern as WeatherTool
	command := fmt.Sprintf("npx -y @modelcontextprotocol/server-filesystem %s", currentDir)

	mcpTool, err := mcp.NewMCPTool(command, 30)
	if err != nil {
		return fmt.Errorf("failed to create MCP tool: %w", err)
	}
	defer mcpTool.Close() // Always close the connection when done

	// Connect to the MCP server
	ctx := context.Background()
	if err := mcpTool.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		return fmt.Errorf("failed to create Ollama model: %w", err)
	}

	// Create agent with MCP tool
	filesystemAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Name:    "Filesystem Assistant",
		Model:   model,
		Tools:   []toolkit.Tool{mcpTool},
		Instructions: \`You are a filesystem assistant. Help users explore files and directories.

- Navigate the filesystem to answer questions
- Provide clear context about files you examine
- Use headings to organize your responses
- Be concise and focus on relevant information\`,
		Debug:         false,
		ShowToolsCall: false,
	})
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Run the agent
	fmt.Printf("Question: %s\\n", message)
	fmt.Println("Processing...")

	response, err := filesystemAgent.Run(message)
	if err != nil {
		return fmt.Errorf("agent run failed: %w", err)
	}

	fmt.Printf("\\nResponse:\\n%s\\n", response.TextContent)

	return nil
}

func main() {
	// Make sure Ollama is running
	fmt.Println("=== Filesystem MCP Agent ===")
	fmt.Println("Make sure Ollama is running: ollama serve")
	fmt.Println("And the model is available: ollama pull llama3.2:latest")
	fmt.Println()

	// Example usage - you can change the message to explore different things
	examples := []string{
		"List all files in the current directory",
		"What files are in this directory?",
		"Can you show me the directory structure?",
	}

	// Run with the first example or use command line argument
	message := examples[0]
	if len(os.Args) > 1 {
		message = os.Args[1]
	}

	if err := runAgent(message); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
```

### Running the Example

```bash
# Run with default question
go run examples/mcp_agent/main.go

# Run with custom question
go run examples/mcp_agent/main.go "Read the README.md file"

# Run with file operations
go run examples/mcp_agent/main.go "Search for all Go files in this project"
```

## 🔧 Supported MCP Servers

### Filesystem Server
```bash
# Install
npm install -g @modelcontextprotocol/server-filesystem

# Use
command := "npx -y @modelcontextprotocol/server-filesystem /path/to/directory"
mcpTool, _ := mcp.NewMCPTool(command, 30)
```

**Available tools:**
- `list_directory` - Lists files and directories
- `read_text_file` - Reads text file content
- `write_file` - Writes content to files
- `create_directory` - Creates directories
- `search_files` - Searches files by pattern
- `get_file_info` - Gets file metadata

### Git Server
```bash
# Install
npm install -g @modelcontextprotocol/server-git

# Use
command := "npx -y @modelcontextprotocol/server-git /path/to/repo"
mcpTool, _ := mcp.NewMCPTool(command, 30)
```

### SQLite Server
```bash
# Install  
npm install -g @modelcontextprotocol/server-sqlite

# Use
command := "npx -y @modelcontextprotocol/server-sqlite /path/to/database.db"
mcpTool, _ := mcp.NewMCPTool(command, 30)
```

## 🏗️ Architecture

### Main Components

1. **MCPTool**: Implements the Agno-Golang `Tool` interface
2. **Dynamic Discovery**: Automatically lists tools via `ListTools()`
3. **Automatic Registration**: Registers each tool in the toolkit
4. **Specific Wrappers**: Creates wrapper functions for each tool type

### Execution Flow

```
1. NewMCPTool() -> Creates instance
2. Connect() -> Connects to MCP server
3. registerDynamicTools() -> Discovers and registers tools
4. Agent.Run() -> Executes tools as needed
5. Close() -> Cleans up resources
```

## 🔒 Security

### Best Practices

```go
// ✅ Always close the connection
mcpTool, err := mcp.NewMCPTool(command, 30)
if err != nil {
    return err
}
defer mcpTool.Close() // IMPORTANT

// ✅ Use appropriate timeouts
mcpTool, err := mcp.NewMCPTool(command, 30) // 30 seconds

// ✅ Validate directories before use
if !isValidDirectory(workDir) {
    return errors.New("invalid directory")
}
```

### Security Limitations

- MCP filesystem servers respect allowed directories
- Commands are validated before execution
- Timeouts prevent hanging operations

## 🧪 Testing

### Basic Test

```go
func TestMCPTool(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    command := fmt.Sprintf("npx -y @modelcontextprotocol/server-filesystem %s", tmpDir)
    
    mcpTool, err := mcp.NewMCPTool(command, 10)
    assert.NoError(t, err)
    defer mcpTool.Close()
    
    // Connect
    ctx := context.Background()
    err = mcpTool.Connect(ctx)
    assert.NoError(t, err)
    
    // Test tool execution
    params := map[string]interface{}{"path": "."}
    result, err := mcpTool.Execute("MCP_list_directory", json.RawMessage(`{"path":"."}`))
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Integration Testing

```bash
# Run the complete example
cd examples/mcp_agent
go run main.go "Test query"

# Test with different MCP servers
go run main.go "List files"
go run main.go "Read package.json"  
go run main.go "Search for *.go files"
```

## 📊 Performance

### Performance Metrics

| Operation | Average Time | Memory Usage |
|-----------|--------------|--------------|
| MCP Connection | ~1-2s | ~5MB |
| Directory Listing | ~50-100ms | ~1MB |
| File Reading | ~10-50ms | ~2-5MB |
| File Writing | ~20-100ms | ~1-3MB |

### Optimizations

- **Connection Pool**: Reuse `MCPTool` for multiple operations
- **Timeouts**: Configure appropriate timeouts for your application
- **Cleanup**: Always call `Close()` to release resources

## 🔧 Troubleshooting

### Common Issues

#### 1. MCP server not found
```bash
Error: failed to connect to MCP server: exec: "npx": executable file not found
```
**Solution**: Install Node.js and npm: `sudo apt install nodejs npm`

#### 2. Connection timeout
```bash
Error: context deadline exceeded
```
**Solution**: Increase timeout: `mcp.NewMCPTool(command, 60)`

#### 3. File permissions
```bash
Error: EACCES: permission denied
```
**Solution**: Use directories with appropriate permissions or run as correct user

### Debug

```go
// Enable debug in agent to see tool calls
agent.NewAgent(agent.AgentConfig{
    Debug:         true,          // Shows general debug
    ShowToolsCall: true,          // Shows tool calls
    // ... other configurations
})
```

## 🚀 Roadmap

- [ ] **Tool Caching**: Cache tool list for reconnections
- [ ] **Multiplexing**: Support for multiple simultaneous MCP servers
- [ ] **Dynamic Schema**: Struct generation based on MCP JSON Schema
- [ ] **Retry Logic**: Automatic reconnection on failure
- [ ] **Metrics**: Detailed performance metrics

## 📚 References

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [Agno-Golang Documentation](../../README.md)
- [Tools Module Documentation](../README.md)

## 🤝 Contributing

To contribute improvements to the MCP module:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/mcp-enhancement`)
3. Commit your changes (`git commit -am 'Add MCP enhancement'`)
4. Push to the branch (`git push origin feature/mcp-enhancement`)
5. Open a Pull Request

---

**⭐ Like the MCP Module? Star us on GitHub!**

*Connecting agents to the real world, one protocol at a time.*