# 🤖 Agent Module

The Agent module provides AI agent implementations for Agno-Golang, supporting multiple AI model providers with tool integration, memory management, and knowledge base access.

## 🚀 Features

### ✅ Core Capabilities
- **Multi-Model Support**: OpenAI, Ollama, Gemini, and custom model providers
- **Tool Integration**: Function calling and tool execution framework
- **Memory Management**: Conversation history and context preservation
- **Knowledge Base Access**: Built-in knowledge search capabilities
- **Streaming Support**: Real-time response streaming
- **Role-Based Behavior**: Customizable agent roles and instructions
- **Error Handling**: Robust error recovery and retry mechanisms

## 🔧 Architecture

```
┌─────────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│   User Input        │───▶│   AI Agent       │───▶│   Model Provider    │
│                     │    │                  │    │   (OpenAI/Ollama)   │
└─────────────────────┘    └──────────────────┘    └─────────────────────┘
                                    │                         │
                           ┌──────────────────┐              │
                           │   Tools          │◄─────────────┘
                           │   (Functions)    │
                           └──────────────────┘
                                    │
                           ┌──────────────────┐
                           │   Knowledge Base │
                           │   (Search)       │
                           └──────────────────┘
```

## 📖 Interface Definition

### Core Agent Interface
```go
type Agent interface {
    // Message processing
    SendMessage(ctx context.Context, message string) (*AgentResponse, error)
    SendMessageStream(ctx context.Context, message string) (<-chan AgentResponse, error)
    
    // Configuration
    GetConfig() AgentConfig
    UpdateConfig(config AgentConfig) error
    
    // Tools and capabilities
    AddTool(tool Tool) error
    RemoveTool(toolName string) error
    GetTools() []Tool
    
    // Memory management
    GetMemory() []Message
    ClearMemory() error
    SetMemory(messages []Message) error
}
```

### Agent Configuration
```go
type AgentConfig struct {
    Name         string                 `json:"name"`
    Role         string                 `json:"role"`
    Instructions string                 `json:"instructions"`
    Model        models.Model           `json:"model"`
    Tools        []Tool                 `json:"tools"`
    Memory       []Message              `json:"memory"`
    Temperature  float64                `json:"temperature"`
    MaxTokens    int                    `json:"max_tokens"`
    Metadata     map[string]interface{} `json:"metadata"`
}
```

### Agent Response
```go
type AgentResponse struct {
    ID          string                 `json:"id"`
    Content     string                 `json:"content"`
    Role        string                 `json:"role"`
    ToolCalls   []ToolCall             `json:"tool_calls,omitempty"`
    Metadata    map[string]interface{} `json:"metadata"`
    Timestamp   time.Time              `json:"timestamp"`
    TokenUsage  TokenUsage             `json:"token_usage"`
}
```

## 🤖 OpenAI Agent

### Configuration
```go
type OpenAIAgentConfig struct {
    APIKey      string
    Model       string        // Default: "gpt-4"
    BaseURL     string        // Optional custom endpoint
    Temperature float64       // 0.0 to 2.0
    MaxTokens   int
    Timeout     time.Duration
}
```

### Usage Example
```go
package main

import (
    "context"
    "fmt"
    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models/openai"
)

func main() {
    ctx := context.Background()
    
    // Create OpenAI model
    model := openai.NewOpenAI(openai.OpenAIConfig{
        APIKey:      os.Getenv("OPENAI_API_KEY"),
        Model:       "gpt-4",
        Temperature: 0.7,
        MaxTokens:   2000,
    })
    
    // Create agent configuration
    config := agent.AgentConfig{
        Name:         "Assistant",
        Role:         "Helpful AI Assistant",
        Instructions: "You are a helpful AI assistant. Always provide accurate and helpful information.",
        Model:        model,
        Temperature:  0.7,
        MaxTokens:    2000,
    }
    
    // Create agent
    agent := agent.NewAgent(config)
    
    // Send message
    response, err := agent.SendMessage(ctx, "Hello! Can you help me understand AI concepts?")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Agent: %s\n", response.Content)
    
    // Stream response
    stream, err := agent.SendMessageStream(ctx, "Tell me about machine learning")
    if err != nil {
        panic(err)
    }
    
    for chunk := range stream {
        fmt.Print(chunk.Content)
    }
    fmt.Println()
}
```

## 🦙 Ollama Agent

### Configuration
```go
type OllamaAgentConfig struct {
    Host    string // Default: "localhost"
    Port    int    // Default: 11434
    Model   string // e.g., "llama2", "codellama"
    Timeout time.Duration
}
```

### Usage Example
```go
// Create Ollama model
model := ollama.NewOllama(ollama.OllamaConfig{
    Host:    "localhost",
    Port:    11434,
    Model:   "llama2",
    Timeout: 60 * time.Second,
})

// Create agent with Ollama
config := agent.AgentConfig{
    Name:         "Local Assistant",
    Role:         "Local AI Assistant",
    Instructions: "You are a helpful local AI assistant running on Ollama.",
    Model:        model,
}

agent := agent.NewAgent(config)
```

## 🛠️ Tool Integration

### Defining Tools
```go
type Tool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`
    Function    ToolFunction           `json:"-"`
}

type ToolFunction func(ctx context.Context, args map[string]interface{}) (interface{}, error)
```

### Built-in Tools

#### Weather Tool
```go
func WeatherTool() agent.Tool {
    return agent.Tool{
        Name:        "get_weather",
        Description: "Get current weather information for a location",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type":        "string",
                    "description": "The city and state, e.g. San Francisco, CA",
                },
            },
            "required": []string{"location"},
        },
        Function: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
            location := args["location"].(string)
            // Implement weather API call
            return getWeatherData(location)
        },
    }
}
```

#### Search Tool
```go
func SearchTool() agent.Tool {
    return agent.Tool{
        Name:        "web_search",
        Description: "Search the web for information",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "query": map[string]interface{}{
                    "type":        "string",
                    "description": "The search query",
                },
                "limit": map[string]interface{}{
                    "type":        "integer",
                    "description": "Number of results to return",
                    "default":     5,
                },
            },
            "required": []string{"query"},
        },
        Function: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
            query := args["query"].(string)
            limit := int(args["limit"].(float64))
            // Implement web search
            return performWebSearch(query, limit)
        },
    }
}
```

#### Knowledge Base Tool
```go
func KnowledgeBaseTool(kb knowledge.Knowledge) agent.Tool {
    return agent.Tool{
        Name:        "search_knowledge_base",
        Description: "Search the knowledge base for relevant information",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "query": map[string]interface{}{
                    "type":        "string",
                    "description": "The search query",
                },
                "limit": map[string]interface{}{
                    "type":        "integer",
                    "description": "Number of results to return",
                    "default":     5,
                },
            },
            "required": []string{"query"},
        },
        Function: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
            query := args["query"].(string)
            limit := int(args["limit"].(float64))
            
            results, err := kb.Search(ctx, query, limit, nil)
            if err != nil {
                return nil, err
            }
            
            // Format results for agent
            var formattedResults []string
            for _, result := range results {
                formattedResults = append(formattedResults, result.Document.Content)
            }
            
            return formattedResults, nil
        },
    }
}
```

### Adding Tools to Agent
```go
// Add individual tools
agent.AddTool(WeatherTool())
agent.AddTool(SearchTool())

// Add knowledge base tool
if knowledgeBase != nil {
    agent.AddTool(KnowledgeBaseTool(knowledgeBase))
}

// Create agent with tools
config := agent.AgentConfig{
    Name:         "Multi-tool Agent",
    Role:         "AI Assistant with Tools",
    Instructions: "You are an AI assistant with access to tools. Use them when needed to provide accurate information.",
    Model:        model,
    Tools: []agent.Tool{
        WeatherTool(),
        SearchTool(),
        KnowledgeBaseTool(knowledgeBase),
    },
}

agent := agent.NewAgent(config)
```

## 💾 Memory Management

### Conversation Memory
```go
type Message struct {
    ID        string                 `json:"id"`
    Role      string                 `json:"role"`      // "user", "assistant", "system"
    Content   string                 `json:"content"`
    ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
    Metadata  map[string]interface{} `json:"metadata"`
    Timestamp time.Time              `json:"timestamp"`
}
```

### Memory Operations
```go
// Get conversation history
messages := agent.GetMemory()
for _, msg := range messages {
    fmt.Printf("%s: %s\n", msg.Role, msg.Content)
}

// Clear memory
err := agent.ClearMemory()

// Set specific memory
customMemory := []agent.Message{
    {
        Role:    "system",
        Content: "You are an expert in AI and machine learning.",
    },
    {
        Role:    "user",
        Content: "Previous conversation context...",
    },
}
agent.SetMemory(customMemory)
```

### Persistent Memory
```go
type PersistentAgent struct {
    *agent.BaseAgent
    storage MemoryStorage
}

func NewPersistentAgent(config agent.AgentConfig, storage MemoryStorage) *PersistentAgent {
    baseAgent := agent.NewAgent(config)
    
    // Load existing memory
    memory, _ := storage.LoadMemory(config.Name)
    baseAgent.SetMemory(memory)
    
    return &PersistentAgent{
        BaseAgent: baseAgent,
        storage:   storage,
    }
}

func (p *PersistentAgent) SendMessage(ctx context.Context, message string) (*agent.AgentResponse, error) {
    response, err := p.BaseAgent.SendMessage(ctx, message)
    if err != nil {
        return nil, err
    }
    
    // Save memory after each interaction
    p.storage.SaveMemory(p.GetConfig().Name, p.GetMemory())
    
    return response, nil
}
```

## 🌊 Streaming Support

### Real-time Streaming
```go
func handleStreamingResponse(agent agent.Agent, userInput string) {
    ctx := context.Background()
    
    stream, err := agent.SendMessageStream(ctx, userInput)
    if err != nil {
        log.Printf("Error starting stream: %v", err)
        return
    }
    
    fmt.Print("Agent: ")
    var fullResponse strings.Builder
    
    for chunk := range stream {
        if chunk.Content != "" {
            fmt.Print(chunk.Content)
            fullResponse.WriteString(chunk.Content)
        }
        
        // Handle tool calls in streaming
        if len(chunk.ToolCalls) > 0 {
            for _, toolCall := range chunk.ToolCalls {
                fmt.Printf("\n[Using tool: %s]\n", toolCall.Function.Name)
            }
        }
    }
    
    fmt.Println()
    log.Printf("Full response: %s", fullResponse.String())
}
```

### Async Processing
```go
func processAsync(agent agent.Agent, messages []string) {
    results := make(chan *agent.AgentResponse, len(messages))
    errors := make(chan error, len(messages))
    
    for _, msg := range messages {
        go func(message string) {
            ctx := context.Background()
            response, err := agent.SendMessage(ctx, message)
            if err != nil {
                errors <- err
                return
            }
            results <- response
        }(msg)
    }
    
    // Collect results
    for i := 0; i < len(messages); i++ {
        select {
        case response := <-results:
            fmt.Printf("Response: %s\n", response.Content)
        case err := <-errors:
            fmt.Printf("Error: %v\n", err)
        case <-time.After(30 * time.Second):
            fmt.Println("Timeout waiting for response")
            return
        }
    }
}
```

## 🧪 Testing

### Unit Tests
```go
func TestAgent(t *testing.T) {
    // Create mock model
    model := models.NewMockModel()
    
    config := agent.AgentConfig{
        Name:         "Test Agent",
        Role:         "Test Assistant",
        Instructions: "You are a test assistant",
        Model:        model,
    }
    
    agent := agent.NewAgent(config)
    
    ctx := context.Background()
    response, err := agent.SendMessage(ctx, "Hello")
    
    assert.NoError(t, err)
    assert.NotEmpty(t, response.Content)
    assert.Equal(t, "assistant", response.Role)
}
```

### Integration Tests
```go
func TestAgentWithTools(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Setup real model
    model := openai.NewOpenAI(openai.OpenAIConfig{
        APIKey: os.Getenv("OPENAI_API_KEY"),
        Model:  "gpt-3.5-turbo",
    })
    
    config := agent.AgentConfig{
        Name:         "Tool Agent",
        Role:         "Assistant with Tools",
        Instructions: "Use tools when needed",
        Model:        model,
        Tools:        []agent.Tool{WeatherTool()},
    }
    
    agent := agent.NewAgent(config)
    
    ctx := context.Background()
    response, err := agent.SendMessage(ctx, "What's the weather in New York?")
    
    assert.NoError(t, err)
    assert.Contains(t, response.Content, "weather")
}
```

## 🔧 Custom Agents

### Specialized Agent
```go
type DocumentAnalyst struct {
    *agent.BaseAgent
    knowledgeBase knowledge.Knowledge
}

func NewDocumentAnalyst(model models.Model, kb knowledge.Knowledge) *DocumentAnalyst {
    config := agent.AgentConfig{
        Name:         "Document Analyst",
        Role:         "Document Analysis Specialist",
        Instructions: "You are an expert at analyzing documents. Use the knowledge base to find relevant information.",
        Model:        model,
        Tools:        []agent.Tool{KnowledgeBaseTool(kb)},
    }
    
    return &DocumentAnalyst{
        BaseAgent:     agent.NewAgent(config),
        knowledgeBase: kb,
    }
}

func (d *DocumentAnalyst) AnalyzeDocument(ctx context.Context, query string) (*agent.AgentResponse, error) {
    enhancedQuery := fmt.Sprintf("Analyze the following query using the available documents: %s", query)
    return d.SendMessage(ctx, enhancedQuery)
}

func (d *DocumentAnalyst) SummarizeDocuments(ctx context.Context, topic string) (*agent.AgentResponse, error) {
    query := fmt.Sprintf("Search for information about '%s' and provide a comprehensive summary", topic)
    return d.SendMessage(ctx, query)
}
```

### Multi-Agent System
```go
type MultiAgentSystem struct {
    agents map[string]agent.Agent
    router AgentRouter
}

func NewMultiAgentSystem() *MultiAgentSystem {
    return &MultiAgentSystem{
        agents: make(map[string]agent.Agent),
        router: NewAgentRouter(),
    }
}

func (m *MultiAgentSystem) AddAgent(name string, agent agent.Agent) {
    m.agents[name] = agent
}

func (m *MultiAgentSystem) ProcessMessage(ctx context.Context, message string) (*agent.AgentResponse, error) {
    // Route to appropriate agent
    agentName := m.router.RouteMessage(message)
    
    selectedAgent, exists := m.agents[agentName]
    if !exists {
        return nil, fmt.Errorf("agent not found: %s", agentName)
    }
    
    return selectedAgent.SendMessage(ctx, message)
}
```

## 🔧 Configuration Examples

### Development Setup
```go
// Simple development agent
model := openai.NewOpenAI(openai.OpenAIConfig{
    APIKey:      os.Getenv("OPENAI_API_KEY"),
    Model:       "gpt-3.5-turbo",
    Temperature: 0.7,
})

config := agent.AgentConfig{
    Name:         "Dev Assistant",
    Role:         "Development Helper",
    Instructions: "Help with development questions",
    Model:        model,
}

agent := agent.NewAgent(config)
```

### Production Setup
```go
// Production agent with full features
model := openai.NewOpenAI(openai.OpenAIConfig{
    APIKey:      os.Getenv("OPENAI_API_KEY"),
    Model:       "gpt-4",
    Temperature: 0.3,
    MaxTokens:   4000,
    Timeout:     60 * time.Second,
})

config := agent.AgentConfig{
    Name:         "Production Assistant",
    Role:         "Expert AI Assistant",
    Instructions: "You are an expert AI assistant with access to tools and knowledge bases. Always provide accurate, helpful, and well-sourced information.",
    Model:        model,
    Tools: []agent.Tool{
        WeatherTool(),
        SearchTool(),
        KnowledgeBaseTool(knowledgeBase),
    },
    Temperature: 0.3,
    MaxTokens:   4000,
}

// Add persistent memory
storage := NewMemoryStorage("./agent_memory")
agent := NewPersistentAgent(config, storage)
```

## 🔧 Troubleshooting

### Common Issues

#### 1. Model API Errors
```go
func handleAgentErrors(agent agent.Agent, message string) (*agent.AgentResponse, error) {
    maxRetries := 3
    for attempt := 0; attempt < maxRetries; attempt++ {
        ctx := context.Background()
        response, err := agent.SendMessage(ctx, message)
        
        if err == nil {
            return response, nil
        }
        
        // Handle specific error types
        if strings.Contains(err.Error(), "rate limit") {
            backoff := time.Duration(attempt+1) * 2 * time.Second
            time.Sleep(backoff)
            continue
        }
        
        if strings.Contains(err.Error(), "context length") {
            // Truncate conversation history
            agent.ClearMemory()
            continue
        }
        
        return nil, err
    }
    
    return nil, fmt.Errorf("failed after %d attempts", maxRetries)
}
```

#### 2. Tool Execution Errors
```go
func SafeToolExecution(tool agent.Tool, ctx context.Context, args map[string]interface{}) (interface{}, error) {
    // Add timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // Execute with panic recovery
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Tool %s panicked: %v", tool.Name, r)
        }
    }()
    
    result, err := tool.Function(ctx, args)
    return result, err
}
```

#### 3. Memory Management
```go
func optimizeMemory(agent agent.Agent, maxMessages int) {
    memory := agent.GetMemory()
    
    if len(memory) > maxMessages {
        // Keep system message and recent messages
        var optimizedMemory []agent.Message
        
        // Always keep system messages
        for _, msg := range memory {
            if msg.Role == "system" {
                optimizedMemory = append(optimizedMemory, msg)
            }
        }
        
        // Keep recent messages
        recentStart := len(memory) - maxMessages + len(optimizedMemory)
        if recentStart < 0 {
            recentStart = 0
        }
        
        for i := recentStart; i < len(memory); i++ {
            if memory[i].Role != "system" {
                optimizedMemory = append(optimizedMemory, memory[i])
            }
        }
        
        agent.SetMemory(optimizedMemory)
    }
}
```

---

The Agent module provides a powerful and flexible foundation for building AI agents in Agno-Golang, supporting multiple models, tools, and advanced features while maintaining simplicity and extensibility.
