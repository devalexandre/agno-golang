# AgentOS for Go

AgentOS is a comprehensive framework for building and managing AI agent systems in Go. This is a port of the Python AgentOS to Go, providing similar functionality with Go's performance and type safety benefits.

## Features

- **Agent Management**: Create, configure, and manage AI agents
- **Team Coordination**: Organize agents into teams for collaborative work
- **Workflow Orchestration**: Define and execute complex workflows
- **REST API**: Full HTTP API for external integrations
- **WebSocket Support**: Real-time communication capabilities
- **Session Management**: Handle user sessions and conversations
- **Configuration System**: Flexible configuration for different domains
- **Extensible Architecture**: Plugin-based interfaces for custom functionality

## Quick Start

### Installation

Add the AgentOS to your Go project:

```bash
go get github.com/devalexandre/agno-golang
```

### Basic Usage

```go
package main

import (
    "log"
    
    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models/openai/chat"
    agentOS "github.com/devalexandre/agno-golang/agno/os"
)

func main() {
    // Create an assistant agent
    assistant, err := agent.NewAgent(agent.AgentOptions{
        Name:         "Assistant",
        Description:  "A helpful AI assistant",
        Instructions: []string{"You are a helpful AI assistant."},
        Model: &chat.OpenAIChat{
            ID: "gpt-3.5-turbo",
        },
        Markdown: true,
    })
    if err != nil {
        log.Fatalf("Failed to create assistant agent: %v", err)
    }

    // Create the AgentOS instance
    osInstance, err := agentOS.NewAgentOS(agentOS.AgentOSOptions{
        OSID:        "my-first-os",
        Name:        stringPtr("My First AgentOS"),
        Description: stringPtr("My first AgentOS in Go"),
        Agents:      []*agent.Agent{assistant},
        Settings: &agentOS.AgentOSSettings{
            Port:       7777,
            Host:       "0.0.0.0",
            Debug:      true,
            EnableCORS: true,
        },
    })
    if err != nil {
        log.Fatalf("Failed to create AgentOS: %v", err)
    }

    // Start the AgentOS server
    log.Println("Starting AgentOS...")
    if err := osInstance.Serve(); err != nil {
        log.Fatalf("Failed to start AgentOS: %v", err)
    }
}

func stringPtr(s string) *string {
    return &s
}
```

### Running Your AgentOS

```bash
go run main.go
```

Access your running instance:
- **App Interface**: `http://localhost:7777`
- **Configuration**: `http://localhost:7777/config`
- **Health Check**: `http://localhost:7777/health`
- **API Documentation**: `http://localhost:7777/api/v1`

## API Endpoints

The AgentOS provides a comprehensive REST API:

### Core Endpoints
- `GET /health` - Health check
- `GET /config` - Configuration information
- `GET /version` - Version information
- `GET /ws` - WebSocket endpoint

### Agent Management
- `GET /api/v1/agents` - List all agents
- `GET /api/v1/agents/:id` - Get specific agent
- `POST /api/v1/agents/:id/chat` - Chat with agent
- `GET /api/v1/agents/:id/sessions` - Get agent sessions
- `GET /api/v1/agents/:id/events` - Get agent events

### Team Management
- `GET /api/v1/teams` - List all teams
- `GET /api/v1/teams/:id` - Get specific team
- `POST /api/v1/teams/:id/chat` - Chat with team
- `GET /api/v1/teams/:id/sessions` - Get team sessions
- `GET /api/v1/teams/:id/events` - Get team events

### Workflow Management
- `GET /api/v1/workflows` - List all workflows
- `GET /api/v1/workflows/:id` - Get specific workflow
- `POST /api/v1/workflows/:id/run` - Run workflow
- `GET /api/v1/workflows/:id/sessions` - Get workflow sessions
- `GET /api/v1/workflows/:id/events` - Get workflow events

### Session Management
- `GET /api/v1/sessions` - List sessions
- `POST /api/v1/sessions` - Create session
- `GET /api/v1/sessions/:id` - Get session
- `DELETE /api/v1/sessions/:id` - Delete session
- `GET /api/v1/sessions/:id/messages` - Get session messages
- `POST /api/v1/sessions/:id/messages` - Add session message

### Domain Management
- `GET /api/v1/knowledge` - Knowledge management
- `GET /api/v1/memory` - Memory management
- `GET /api/v1/metrics` - Metrics and analytics
- `GET /api/v1/evals` - Evaluation management

## Configuration

### AgentOS Options

```go
type AgentOSOptions struct {
    OSID         string                // Required: Unique OS identifier
    Name         *string               // Optional: Display name
    Description  *string               // Optional: Description
    Version      *string               // Optional: Version string
    Agents       []*agent.Agent        // Optional: List of agents
    Teams        []*team.Team          // Optional: List of teams
    Workflows    []*v2.Workflow        // Optional: List of workflows
    Interfaces   []AgentOSInterface    // Optional: Custom interfaces
    Config       *AgentOSConfig        // Optional: Domain configurations
    Settings     *AgentOSSettings      // Optional: Server settings
    EnableMCP    bool                  // Optional: Enable MCP support
    Telemetry    bool                  // Optional: Enable telemetry
    Middleware   []interface{}         // Optional: Custom middleware
    CustomRoutes []interface{}         // Optional: Custom routes
}
```

### Server Settings

```go
type AgentOSSettings struct {
    Port        int           // Server port (default: 7777)
    Host        string        // Server host (default: "0.0.0.0")
    Reload      bool          // Enable hot reload (default: false)
    Debug       bool          // Enable debug mode (default: false)
    LogLevel    string        // Log level (default: "info")
    Timeout     time.Duration // Request timeout (default: 30s)
    EnableCORS  bool          // Enable CORS (default: true)
    EnableMCP   bool          // Enable MCP (default: false)
    Telemetry   bool          // Enable telemetry (default: false)
}
```

## Advanced Features

### Domain Configuration

Configure different domains of your AgentOS:

```go
config := &agentOS.AgentOSConfig{
    AvailableModels: []string{"gpt-3.5-turbo", "gpt-4"},
    Chat: &agentOS.ChatConfig{
        QuickPrompts: map[string][]string{
            "assistant": {"Hello", "Help me", "Explain"},
        },
    },
    Knowledge: &agentOS.KnowledgeConfig{
        DomainConfig: agentOS.KnowledgeDomainConfig{
            DomainConfig: agentOS.DomainConfig{
                DisplayName: stringPtr("Knowledge Base"),
            },
        },
    },
}
```

### Custom Interfaces

Implement custom interfaces for extending functionality:

```go
type MyCustomInterface struct {
    name string
}

func (i *MyCustomInterface) GetID() string { return "custom-interface" }
func (i *MyCustomInterface) GetName() string { return i.name }
func (i *MyCustomInterface) Initialize() error { return nil }
func (i *MyCustomInterface) Shutdown() error { return nil }
```

### WebSocket Integration

The AgentOS provides WebSocket support for real-time communication:

```javascript
const ws = new WebSocket('ws://localhost:7777/ws');

ws.onopen = function() {
    console.log('Connected to AgentOS');
    ws.send(JSON.stringify({
        type: 'chat',
        agent_id: 'assistant',
        message: 'Hello!'
    }));
};

ws.onmessage = function(event) {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};
```

## Testing

Run the test suite:

```bash
go test ./agno/os/...
```

Run tests with coverage:

```bash
go test -cover ./agno/os/...
```

## Examples

See the `examples/` directory for more comprehensive examples:

- `examples/basic_agentos/` - Basic AgentOS setup
- `examples/multi_agent/` - Multi-agent system
- `examples/team_collaboration/` - Team-based workflows
- `examples/custom_interfaces/` - Custom interface implementation

## Comparison with Python AgentOS

| Feature | Python AgentOS | Go AgentOS | Notes |
|---------|----------------|------------|-------|
| Performance | ✓ | ✓✓ | Go provides better performance |
| Type Safety | ✓ | ✓✓ | Go's strong typing prevents runtime errors |
| Memory Usage | ✓ | ✓✓ | Lower memory footprint |
| Concurrency | ✓ | ✓✓ | Go's goroutines are more efficient |
| Ecosystem | ✓✓ | ✓ | Python has more AI/ML libraries |
| Deployment | ✓ | ✓✓ | Single binary deployment |

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run the test suite
6. Submit a pull request

## License

This project is licensed under the same license as the main agno-golang project.