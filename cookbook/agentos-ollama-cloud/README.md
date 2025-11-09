# AgentOS with Ollama Cloud 🚀

Complete example of AgentOS using **Ollama Cloud** (kimi-k2:1t-cloud model) with multiple agents, tools, and full API support including file uploads, WebSocket, and continue run capabilities.

## 🌟 Features

- ✅ **3 Specialized Agents:**
  - 🔍 **Researcher** - Web search with DuckDuckGo
  - ✍️ **Writer** - Content creation with file operations
  - 🤖 **Assistant** - General-purpose AI assistant

- ✅ **Full AgentOS API:**
  - File uploads (images, audio, video, documents)
  - Continue run for human-in-the-loop workflows
  - WebSocket for real-time workflow execution
  - Session management
  - Streaming and non-streaming responses

- ✅ **Ollama Cloud Integration:**
  - kimi-k2:1t-cloud model
  - Cloud-hosted, no local GPU needed
  - Fast and reliable

## 📋 Prerequisites

1. **Ollama Cloud API Key** - Get it from [https://ollamacloud.link](https://ollamacloud.link)
2. **Go 1.23+** - Install from [https://golang.org/dl/](https://golang.org/dl/)

## 🚀 Quick Start

### 1. Set up environment variables

```bash
# Required: Ollama Cloud API key
export OLLAMA_CLOUD_API_KEY="your-api-key-here"

# Optional: Security key for authentication
export SECURITY_KEY="your-secret-key"

# Optional: Custom port (default: 8080)
export PORT=8080
```

### 2. Run the server

```bash
cd cookbook/agentos-ollama-cloud
go run main.go
```

You should see:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🚀 AgentOS with Ollama Cloud Starting...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 Server: http://localhost:8080
🤖 Model: kimi-k2:1t-cloud (Ollama Cloud)
👥 Agents: 3 (Researcher, Writer, Assistant)
🔧 Tools: DuckDuckGo Search, File Operations
🔓 Security: Disabled (no authentication)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## 📚 API Usage Examples

### List all agents

```bash
curl http://localhost:8080/agents | jq
```

**Response:**
```json
[
  {
    "id": "agent_researcher",
    "name": "Researcher",
    "role": "Research Specialist",
    "model": {
      "name": "kimi-k2:1t-cloud",
      "provider": "ollama"
    }
  },
  {
    "id": "agent_writer",
    "name": "Writer",
    "role": "Content Writer"
  },
  {
    "id": "agent_assistant",
    "name": "Assistant",
    "role": "AI Assistant"
  }
]
```

### Execute an agent (non-streaming)

```bash
curl -X POST http://localhost:8080/agents/agent_researcher/runs \
  -F 'message=What are the latest developments in AI?' \
  -F 'stream=false'
```

**Response:**
```json
{
  "run_id": "run_abc123",
  "agent_id": "agent_researcher",
  "content": "Based on my search, here are the latest AI developments...",
  "session_id": "session_xyz",
  "created_at": 1699459200
}
```

### Execute with streaming (SSE)

```bash
curl -X POST http://localhost:8080/agents/agent_assistant/runs \
  -F 'message=Tell me a joke' \
  -F 'stream=true'
```

**Response (Server-Sent Events):**
```
event: RunStarted
data: {"event":"RunStarted","run_id":"run_123","agent_id":"agent_assistant",...}

event: RunContent
data: {"event":"RunContent","content":"Why did the programmer...","run_id":"run_123",...}

event: RunCompleted
data: {"event":"RunCompleted","content":"Why did the programmer quit his job?...","run_id":"run_123",...}
```

### Upload files (images, documents)

```bash
curl -X POST http://localhost:8080/agents/agent_writer/runs \
  -F 'message=Analyze this image and document' \
  -F 'images=@photo.jpg' \
  -F 'files=@report.pdf' \
  -F 'stream=false'
```

### Continue a run (human-in-the-loop)

```bash
# 1. Start a run that requires approval
curl -X POST http://localhost:8080/agents/agent_researcher/runs \
  -F 'message=Search and summarize AI news'

# 2. Continue with updated tools
curl -X POST http://localhost:8080/agents/agent_researcher/runs/run_123/continue \
  -F 'tools=[{"name":"search","status":"approved","result":"..."}]' \
  -F 'session_id=session_xyz' \
  -F 'stream=true'
```

### WebSocket for workflows

```javascript
const ws = new WebSocket('ws://localhost:8080/workflows/ws');

ws.onopen = () => {
  // Authenticate (if SECURITY_KEY is set)
  ws.send(JSON.stringify({
    action: 'authenticate',
    token: 'your-security-key'
  }));
  
  // Start workflow
  ws.send(JSON.stringify({
    action: 'start-workflow',
    workflow_id: 'workflow_id',
    message: 'Execute this workflow',
    session_id: 'session_123'
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event:', data.event);
  console.log('Data:', data);
};
```

### Session management

```bash
# List all sessions
curl http://localhost:8080/sessions

# Get specific session
curl http://localhost:8080/sessions/session_123

# Get session runs
curl http://localhost:8080/sessions/session_123/runs
```

## 🔧 Available Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/` | API root information |
| `GET` | `/agents` | List all agents |
| `GET` | `/agents/:id` | Get agent details |
| `POST` | `/agents/:id/runs` | Execute agent (supports file uploads) |
| `POST` | `/agents/:id/runs/:run_id/continue` | Continue agent run |
| `POST` | `/agents/:id/runs/:run_id/cancel` | Cancel agent run |
| `GET` | `/sessions` | List sessions |
| `GET` | `/sessions/:id` | Get session details |
| `GET` | `/sessions/:id/runs` | Get session runs |
| `WS` | `/workflows/ws` | WebSocket for workflows |
| `GET` | `/config` | AgentOS configuration |
| `GET` | `/models` | List available models |

## 🛠️ Agent Capabilities

### Researcher Agent
- **Tools:** DuckDuckGo web search
- **Best for:** Information gathering, fact-checking, research
- **Example:** `"What are the best practices for Go concurrency?"`

### Writer Agent
- **Tools:** File read/write operations
- **Best for:** Content creation, document generation, file management
- **Example:** `"Write a blog post about AI and save it to post.md"`

### Assistant Agent
- **Tools:** None (general conversation)
- **Best for:** General questions, explanations, conversations
- **Example:** `"Explain quantum computing in simple terms"`

## 🔒 Security

Enable authentication by setting `SECURITY_KEY` environment variable:

```bash
export SECURITY_KEY="your-secret-key-here"
go run main.go
```

All requests will require authentication. For WebSocket connections, send authentication message first:

```javascript
ws.send(JSON.stringify({
  action: 'authenticate',
  token: 'your-secret-key-here'
}));
```

## 📊 Monitoring & Debugging

Enable debug mode in code:

```go
Settings: &agentOS.AgentOSSettings{
    Debug: true,  // Enable debug logging
    // ...
}
```

Or set debug on agents:

```go
agent.NewAgent(agent.AgentConfig{
    Debug: true,  // Show detailed logs
    ShowToolsCall: true,  // Show tool execution details
    // ...
})
```

## 🐳 Docker Deployment (Optional)

Create `Dockerfile`:

```dockerfile
FROM golang:1.23-alpine

WORKDIR /app
COPY . .
RUN go build -o agentos main.go

EXPOSE 8080
CMD ["./agentos"]
```

Build and run:

```bash
docker build -t agentos-ollama-cloud .
docker run -p 8080:8080 \
  -e OLLAMA_CLOUD_API_KEY="your-key" \
  agentos-ollama-cloud
```

## 📝 Customization

### Add more agents

```go
customAgent, err := agent.NewAgent(agent.AgentConfig{
    Context:      ctx,
    Name:         "CustomAgent",
    Role:         "Specialist",
    Instructions: "Your custom instructions...",
    Model:        ollamaModel,
    Tools:        []toolkit.Tool{/* your tools */},
})

// Add to AgentOS
osInstance, err := agentOS.NewAgentOS(agentOS.AgentOSOptions{
    Agents: []*agent.Agent{researcher, writer, assistant, customAgent},
    // ...
})
```

### Add custom tools

```go
import "github.com/devalexandre/agno-golang/agno/tools"

// Add weather tool
weatherTool := tools.NewWeatherTool()

agent.NewAgent(agent.AgentConfig{
    Tools: []toolkit.Tool{weatherTool},
    // ...
})
```

### Change model

```go
// Use different Ollama Cloud model
ollamaModel, err := ollama.NewOllamaChat(
    models.WithID("llama3.2:latest-cloud"),  // Different model
    models.WithBaseURL("https://api.ollamacloud.link"),
    models.WithAPIKey(apiKey),
)
```

## 🧪 Testing

Test with different agents:

```bash
# Test Researcher
curl -X POST http://localhost:8080/agents/agent_researcher/runs \
  -F 'message=Search for Go best practices' \
  -F 'stream=false'

# Test Writer
curl -X POST http://localhost:8080/agents/agent_writer/runs \
  -F 'message=Write a haiku about coding' \
  -F 'stream=false'

# Test Assistant
curl -X POST http://localhost:8080/agents/agent_assistant/runs \
  -F 'message=Explain REST APIs' \
  -F 'stream=false'
```

## 📖 Additional Resources

- [AgentOS Documentation](../../docs/)
- [Agent Configuration](../../docs/agent/)
- [Tools Documentation](../../docs/tools/)
- [Ollama Cloud](https://ollamacloud.link)

## 🐛 Troubleshooting

**Problem:** `OLLAMA_CLOUD_API_KEY environment variable is required`  
**Solution:** Set the API key: `export OLLAMA_CLOUD_API_KEY="your-key"`

**Problem:** `Failed to create Ollama Cloud model`  
**Solution:** Check API key is valid and Ollama Cloud service is accessible

**Problem:** `Port already in use`  
**Solution:** Change port: `export PORT=8081` or kill existing process

**Problem:** Agent not responding  
**Solution:** Check Ollama Cloud credits/limits, enable debug mode

## 📄 License

MIT License - see [../../LICENSE](../../LICENSE)

---

**Built with ❤️ using Agno Framework**
