# Quick Start Guide - Agent Examples

## 🎯 Choose Your Example Based on Your Needs

### Need to validate input/output?
👉 **[hooks](./hooks/)** - Pre and Post-hooks for validation and transformation  
👉 **[guardrails](./guardrails/)** - Safety checks and policy enforcement

### Need rich context (date/time/location)?
👉 **[context_building](./context_building/)** - Enriched context building

### Need to change agent personality?
👉 **[custom_system_message](./custom_system_message/)** - Custom personas

### Need resilience against failures?
👉 **[retries_example](./retries_example/)** - Retry with configurable strategies  
👉 **[retry_backoff](./retry_backoff/)** - Exponential backoff

### Need to track conversations?
👉 **[session_state_example](./session_state_example/)** - Multi-user persistent state  
👉 **[session_management](./session_management/)** - Session tracking with DB

### Need memory & persistence?
👉 **[memory_example](./memory_example/)** - User/agent memories and summaries

### Need knowledge base & RAG?
👉 **[knowledge_filters_example](./knowledge_filters_example/)** - Metadata-based search  
👉 **[knowledge_pdf](./knowledge_pdf/)** - PDF processing with vectors  
👉 **[update_knowledge](./update_knowledge/)** - Dynamic knowledge updates

### Need context control?
👉 **[context_control_example](./context_control_example/)** - Dependencies and history control

### Need debugging & monitoring?
👉 **[metadata_debug_example](./metadata_debug_example/)** - Request tracking and debug mode  
👉 **[metadata_test](./metadata_test/)** - Production monitoring

### Need default tools?
👉 **[read_chat_history](./read_chat_history/)** - Agent reads chat history  
👉 **[read_toolcall_history](./read_toolcall_history/)** - Tool execution tracking

### Need to process images/audio/video?
👉 **[media_support](./media_support/)** - Multimodal support

### Need advanced features?
👉 **[agentic_state](./agentic_state/)** - Tool-modifiable state  
👉 **[input_and_output](./input_and_output/)** - Structured schemas  
👉 **[ollama-cloud](./ollama-cloud/)** - Cloud deployments

## ⚡ Quick Execution

```bash
# Navigate to example folder
cd cookbook/agents/<example_name>

# Run
go run main.go
```

## 📊 Feature Matrix

| Example | Hooks | Context | Retry | Session | Memory | Knowledge | Media | Debug |
|---------|:-----:|:-------:|:-----:|:-------:|:------:|:---------:|:-----:|:-----:|
| **hooks** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **context_building** | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **custom_system_message** | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **guardrails** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **session_state_example** | ❌ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **knowledge_filters_example** | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| **context_control_example** | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **metadata_debug_example** | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ |
| **retries_example** | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **retry_backoff** | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **session_management** | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **memory_example** | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| **knowledge_pdf** | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| **update_knowledge** | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| **read_chat_history** | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **read_toolcall_history** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| **media_support** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ |
| **agentic_state** | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **input_and_output** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **ollama-cloud** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

## 🔗 Useful Links

- **[Complete README](./README.md)** - Detailed documentation for all examples
- **[Run Options Guide](../../docs/RUN_OPTIONS.md)** - Complete Run() options reference
- **[Feature Parity](../../docs/FEATURE_PARITY.md)** - Go vs Python comparison
- **[API Reference](../../README.md)** - Project documentation

## 💡 Tips

### Combining Features

All examples can be combined! Here's a production-ready configuration:

```go
agent, err := agent.NewAgent(agent.AgentConfig{
    Context: ctx,
    Model:   model,
    Name:    "Production Agent",
    
    // Hooks for validation
    PreHooks:  []func(context.Context, interface{}) error{validateInput},
    PostHooks: []func(context.Context, *models.RunResponse) error{logOutput},
    
    // Context building
    AddNameToContext:     true,
    AddDatetimeToContext: true,
    
    // Retry config
    DelayBetweenRetries: 2,
    ExponentialBackoff:  true,
    
    // Memory & Storage
    DB:            db,
    MemoryManager: memoryManager,
    Knowledge:     knowledgeBase,
})

// Run with options
response, err := agent.Run(
    "Your message",
    agent.WithSessionID("session_123"),
    agent.WithUserID("user_456"),
    agent.WithSessionState(state),
    agent.WithKnowledgeFilters(filters),
    agent.WithMetadata(metadata),
    agent.WithRetries(5),
    agent.WithDebugMode(true),
)
```

### Quick Setup

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull models
ollama pull llama3.2:latest
ollama pull gemma:2b

# Start Qdrant (for knowledge examples)
docker run -p 6333:6333 qdrant/qdrant:latest

# Run any example
cd cookbook/agents/session_state_example
go run main.go
```

## 📦 Requirements

- **Go:** 1.23 or higher
- **Ollama:** Local or Cloud
- **Qdrant:** For knowledge base examples (Docker)

## 🚀 Getting Started

1. **Choose an example** from the list above based on your needs
2. **Navigate** to the example folder
3. **Read** the example's README.md for specific details
4. **Run** with `go run main.go`
5. **Modify** the code to fit your use case

## 📚 Learn More

- Start with **[session_state_example](./session_state_example/)** for basic state management
- Try **[knowledge_filters_example](./knowledge_filters_example/)** for RAG
- Explore **[metadata_debug_example](./metadata_debug_example/)** for production monitoring
- Combine features for advanced use cases

---

**Last Updated:** November 8, 2025  
**Total Examples:** 20  
**Go Version:** 1.23+
