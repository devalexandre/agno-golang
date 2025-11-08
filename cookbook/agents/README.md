# Agent Cookbook Examples

This folder contains practical examples of using Agno Agent with different configurations and features.

## � Quick Start

New to Agno agents? Start here: **[QUICK_START.md](./QUICK_START.md)**

For detailed Run() options documentation: **[docs/RUN_OPTIONS.md](../../docs/RUN_OPTIONS.md)**

## �📚 Available Examples

### Core Configuration Examples

#### 1. [Hooks](./hooks/)
Demonstrates the hooks system (pre-hooks and post-hooks) for:
- Input validation
- Output validation
- Data transformation
- Logging and auditing
- Security and filters

**Main features:**
- `PreHooks`: Execute before agent processing
- `PostHooks`: Execute after response generation
- Multi-layer validation
- Custom error handling

#### 2. [Context Building](./context_building/)
Demonstrates enriched context building with:
- Agent name in context
- Current date/time
- Location and timezone
- Custom additional context

**Main features:**
- `AddNameToContext`: Adds identity to agent
- `AddDatetimeToContext`: Temporal awareness
- `AddLocationToContext`: Geographic awareness
- `TimezoneIdentifier`: Support for different timezones
- `AdditionalContext`: Custom context

#### 3. [Custom System Message](./custom_system_message/)
Demonstrates complete agent persona customization with:
- Pirate assistant
- Shakespearean assistant
- Technical expert

**Main features:**
- `SystemMessage`: Custom system message
- `SystemMessageRole`: System message role
- `BuildContext`: Control over context building
- Total control over behavior and style

#### 4. [Guardrails](./guardrails/)
Demonstrates input/output validation and safety:
- Content filtering
- Safety checks
- Custom validators
- Policy enforcement

**Main features:**
- Input validation before processing
- Output filtering after generation
- Custom guardrail functions
- Security and compliance

### Run Options Examples

#### 5. [Session State Example](./session_state_example/) 💾
Demonstrates session state persistence:
- Multiple user sessions with SQLite
- State isolation between users
- Persistent state across conversations
- Session history tracking

**Main features:**
- `WithSessionID()`: Session identification
- `WithSessionState()`: Persistent state map
- `WithAddSessionStateToContext()`: Include state in context
- `WithAddHistoryToContext()`: Include chat history
- Multi-user session management

#### 6. [Knowledge Filters Example](./knowledge_filters_example/) 🔍
Demonstrates knowledge base filtering with metadata:
- 6 documents with rich metadata (language, category, level)
- Single and multiple filter combinations
- Language-based filtering (en/pt)
- Category and level filtering
- Empty results handling

**Main features:**
- `WithKnowledgeFilters()`: Metadata-based search
- Multiple filter combinations
- Qdrant vector database
- Dynamic knowledge retrieval

#### 7. [Context Control Example](./context_control_example/) 🎛️
Demonstrates context building control:
- External dependencies injection
- History inclusion control
- Dependencies in context
- Stateless vs stateful conversations

**Main features:**
- `WithDependencies()`: External resources
- `WithAddHistoryToContext()`: History control
- `WithAddDependenciesToContext()`: Dependencies control
- `WithAddSessionStateToContext()`: State control

#### 8. [Metadata & Debug Example](./metadata_debug_example/) 🐛
Demonstrates metadata tracking and debugging:
- Request tracking with unique IDs
- A/B testing support
- SLA monitoring
- Debug mode for inspection

**Main features:**
- `WithMetadata()`: Custom metadata tracking
- `WithDebugMode()`: Detailed debug output
- Request correlation
- Performance tracking

#### 9. [Retries Example](./retries_example/) 🔄
Demonstrates retry strategies for resilience:
- No retries (fail fast)
- 3 retries (balanced)
- 5 retries (resilient)
- 10 retries (maximum resilience)

**Main features:**
- `WithRetries()`: Configurable retry count
- Exponential backoff support
- Transient failure handling
- Error recovery strategies

### Storage & Persistence Examples

#### 10. [Session Management](./session_management/)
Demonstrates session management with database storage:
- Conversation tracking
- User identification
- Persistent context
- Session metadata

**Main features:**
- `WithSessionID()`: Session identification
- `WithUserID()`: User identification
- Maintained conversation history
- `WithMetadata()`: Custom metadata

#### 11. [Memory Example](./memory_example/) 🧠
Demonstrates memory management:
- User memories
- Agentic memories
- Session summaries
- Memory persistence

**Main features:**
- Memory Manager integration
- User-specific memories
- Agent memories
- Automatic summarization

### Knowledge Base Examples

#### 12. [Knowledge PDF](./knowledge_pdf/) 📄
Demonstrates PDF document processing:
- PDF loading and parsing
- Vector embeddings
- Semantic search
- RAG (Retrieval Augmented Generation)

**Main features:**
- PDF document support
- Knowledge base integration
- Vector database (Qdrant)
- Semantic similarity search

#### 13. [Update Knowledge](./update_knowledge/) 📚
Demonstrates dynamic knowledge updates:
- Add new knowledge at runtime
- Update existing knowledge
- Knowledge versioning
- Default tool usage

**Main features:**
- `update_knowledge` default tool
- Dynamic knowledge management
- Vector database updates
- Real-time knowledge refresh

### Default Tools Examples

#### 14. [Read Chat History](./read_chat_history/) 💬
Demonstrates chat history access:
- Agent reads own chat history
- Conversation analysis
- History-based responses
- Context awareness

**Main features:**
- `read_chat_history` default tool
- Historical context
- Conversation continuity
- Self-referential capabilities

#### 15. [Read Tool Call History](./read_toolcall_history/) 🛠️
Demonstrates tool call tracking:
- Tool execution history
- Tool usage patterns
- Debug and monitoring
- Performance analysis

**Main features:**
- `read_tool_call_history` default tool
- Tool execution tracking
- Usage analytics
- Debugging support

### Multimodal Examples

#### 16. [Media Support](./media_support/)
Demonstrates support for multiple media types:
- Images (analysis and comparison)
- Audio (transcription)
- Video (description)
- Files (document analysis)
- Media combination

**Main features:**
- `WithImages()`: Image input
- `WithAudio()`: Audio input
- `WithVideos()`: Video input
- `WithFiles()`: File input
- `StoreMedia`: Reference persistence
- `SendMediaToModel`: Include media in requests

### Advanced Examples

#### 17. [Agentic State](./agentic_state/) 🤖
Demonstrates agent-modifiable state:
- Tools can modify agent state
- State persistence
- State-driven behavior
- Dynamic agent configuration

**Main features:**
- Tool-modifiable state
- State propagation
- Dynamic behavior changes
- Advanced state management

#### 18. [Input and Output Schemas](./input_and_output/) 📋
Demonstrates structured input/output:
- Input validation with schemas
- Structured JSON output
- Type-safe responses
- Schema enforcement

**Main features:**
- `InputSchema`: Input validation
- `OutputSchema`: Structured output
- `OutputModel`: Separate parsing model
- Type safety

#### 19. [Metadata Test](./metadata_test/) 🧪
Demonstrates metadata in production scenarios:
- Production request tracking
- Performance monitoring
- User analytics
- Error tracking

**Main features:**
- Real-world metadata usage
- Performance metrics
- User tracking
- Production debugging

### Cloud Examples

#### 20. [Ollama Cloud](./ollama-cloud/) ☁️
Demonstrates Ollama Cloud integration:
- Remote model execution
- Cloud model selection
- API authentication
- Scalable deployments

**Main features:**
- Ollama Cloud support
- Remote model access
- Cloud-native architecture
- Production scalability

## 🚀 How to Run

Each example can be run individually:

```bash
# Core Configuration Examples
cd cookbook/agents/hooks && go run main.go
cd cookbook/agents/context_building && go run main.go
cd cookbook/agents/custom_system_message && go run main.go
cd cookbook/agents/guardrails && go run main.go

# Run Options Examples
cd cookbook/agents/session_state_example && go run main.go
cd cookbook/agents/knowledge_filters_example && go run main.go
cd cookbook/agents/context_control_example && go run main.go
cd cookbook/agents/metadata_debug_example && go run main.go
cd cookbook/agents/retries_example && go run main.go

# Storage & Persistence
cd cookbook/agents/session_management && go run main.go
cd cookbook/agents/memory_example && go run main.go

# Knowledge Base
cd cookbook/agents/knowledge_pdf && go run main.go
cd cookbook/agents/update_knowledge && go run main.go

# Default Tools
cd cookbook/agents/read_chat_history && go run main.go
cd cookbook/agents/read_toolcall_history && go run main.go

# Multimodal
cd cookbook/agents/media_support && go run main.go

# Advanced
cd cookbook/agents/agentic_state && go run main.go
cd cookbook/agents/input_and_output && go run main.go
cd cookbook/agents/ollama-cloud && go run main.go
```

## 📋 Requirements

- Go 1.23 or higher
- Ollama running (local or cloud)

### Local Ollama Setup

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Download models
ollama pull llama3.2:latest
ollama pull gemma:2b  # For embeddings

# Start Ollama
ollama serve
```

### Cloud Ollama Setup

For cloud examples, configure your Ollama Cloud endpoint:
```go
model := ollama.New(ollama.WithBaseURL("https://ollama.com"))
```

### Vector Database (for Knowledge examples)

```bash
# Start Qdrant with Docker
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant:latest
```

## 🎯 Use Cases by Category

### 🔒 Validation and Security
- **Hooks**: Content filters, input validation, usage auditing
- **Guardrails**: Safety checks, policy enforcement

### 🧠 Context Management
- **Context Building**: Time-sensitive responses, geographic location
- **Context Control**: History management, dependencies injection
- **Session State**: Persistent state across conversations

### 🎨 Behavior Customization
- **Custom System Message**: Specific personas, tone and style
- **Agentic State**: Dynamic behavior changes

### 🔄 Reliability & Resilience
- **Retries**: Handle transient failures, configure retry strategies
- **Retry Backoff**: Exponential backoff for rate limiting

### 💬 Conversations & Memory
- **Session Management**: Maintain context, track conversations
- **Memory Example**: User memories, agent memories, summaries
- **Read Chat History**: Historical context awareness

### 📚 Knowledge & RAG
- **Knowledge Filters**: Metadata-based search, language filtering
- **Knowledge PDF**: Document processing, semantic search
- **Update Knowledge**: Dynamic knowledge management

### 🎥 Multimodal Processing
- **Media Support**: Images, audio, video, files
- **Multiple modalities**: Combined media analysis

### 🐛 Debugging & Monitoring
- **Metadata & Debug**: Request tracking, A/B testing, SLA monitoring
- **Read Tool Call History**: Tool execution tracking
- **Metadata Test**: Production monitoring

### ☁️ Production & Scale
- **Ollama Cloud**: Cloud deployments, remote models
- **Input/Output Schemas**: Type-safe APIs, validation

## 🔧 Advanced Configuration

### Combining Features

You can combine multiple features in a single agent:

```go
agent, err := agent.NewAgent(agent.AgentConfig{
    Context: ctx,
    Model:   model,
    
    // Core Configuration
    Name:        "Advanced Agent",
    Description: "Agent with multiple features",
    
    // Hooks
    PreHooks:  []func(context.Context, interface{}) error{validateInput},
    PostHooks: []func(context.Context, *models.RunResponse) error{logOutput},
    
    // Context Building
    AddNameToContext:     true,
    AddDatetimeToContext: true,
    TimezoneIdentifier:   "America/Sao_Paulo",
    
    // Custom Message
    SystemMessage: "You are a helpful assistant...",
    
    // Retry Config
    DelayBetweenRetries: 2,
    ExponentialBackoff:  true,
    
    // Memory & Storage
    DB:            db,
    MemoryManager: memoryManager,
    
    // Knowledge Base
    Knowledge: knowledge,
    
    // Media Options
    StoreMedia:       true,
    SendMediaToModel: true,
    
    // Input/Output Schemas
    InputSchema:  inputSchema,
    OutputSchema: outputSchema,
    OutputModel:  separateModel,
    
    // Reasoning
    Reasoning:      true,
    ReasoningModel: reasoningModel,
    
    // Semantic Compression
    SemanticCompression: true,
    SemanticModel:       semanticModel,
})

// Run with multiple options
response, err := agent.Run(
    "Your message here",
    agent.WithSessionID("session_123"),
    agent.WithUserID("user_456"),
    agent.WithSessionState(state),
    agent.WithKnowledgeFilters(filters),
    agent.WithDependencies(deps),
    agent.WithMetadata(metadata),
    agent.WithRetries(5),
    agent.WithAddHistoryToContext(true),
    agent.WithAddDependenciesToContext(true),
    agent.WithAddSessionStateToContext(true),
    agent.WithDebugMode(true),
)
```

## 📊 Feature Matrix

| Example | Hooks | Context | Retry | Session | Memory | Knowledge | Media | Debug |
|---------|:-----:|:-------:|:-----:|:-------:|:------:|:---------:|:-----:|:-----:|
| hooks | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| context_building | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| custom_system_message | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| guardrails | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| session_state_example | ❌ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| knowledge_filters_example | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| context_control_example | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| metadata_debug_example | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ |
| retries_example | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| session_management | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| memory_example | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| knowledge_pdf | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| update_knowledge | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| read_chat_history | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| read_toolcall_history | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| media_support | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ |
| agentic_state | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| input_and_output | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| ollama-cloud | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

## 📖 Complete Documentation

For detailed documentation:
- **[Quick Start Guide](./QUICK_START.md)** - Fast introduction for beginners
- **[Run Options Reference](../../docs/RUN_OPTIONS.md)** - Complete Run() options guide
- **[Feature Parity](../../docs/FEATURE_PARITY.md)** - Go vs Python comparison
- **[Agent Documentation](../../docs/agent/)** - Detailed API docs
- **[Main README](../../README.md)** - Project overview

## 🤝 Contributing

Want to add a new example? Please:
1. Create a new folder in `cookbook/agents/`
2. Add a `main.go` with the example
3. Include a README.md explaining the example
4. Update this README with your example
5. Submit a Pull Request

### Example Template

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
    ctx := context.Background()
    
    // Create model
    model := ollama.New(ollama.ID("llama3.2:latest"))
    
    // Create agent
    myAgent, err := agent.NewAgent(agent.AgentConfig{
        Context: ctx,
        Model:   model,
        Name:    "Example Agent",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Run
    response, err := myAgent.Run("Your message")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(response.TextContent)
}
```

## 📝 License

All examples follow the same license as the main project.

---

**Last Updated:** November 8, 2025  
**Total Examples:** 20  
**Go Version:** 1.23+
