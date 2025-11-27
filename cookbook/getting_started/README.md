# Getting Started with Agno-Golang

Welcome to the Getting Started guide for Agno-Golang! This collection of examples will walk you through the core features of the framework, from basic agents to advanced memory management.

## 🚀 Prerequisites

1. **Go 1.21+** installed
2. **Ollama** running locally with llama3.2:latest model
   ```bash
   # Install Ollama
   curl -fsSL https://ollama.com/install.sh | sh
   
   # Pull the model
   ollama pull llama3.2:latest
   
   # Start Ollama (if not running)
   ollama serve
   ```

## 📚 Examples Overview

### 01 - Basic Agent
**File:** `01_basic_agent/main.go`

Learn how to create your first AI agent with personality and instructions.

```bash
cd 01_basic_agent && go run main.go
```

**What you'll learn:**
- Creating an agent with Ollama
- Setting instructions and personality
- Using PrintResponse for output
- Enabling markdown rendering

---

### 02 - Agent with Tools
**File:** `02_agent_with_tools/main.go`

Add web search capabilities to your agent using DuckDuckGo.

```bash
cd 02_agent_with_tools && go run main.go
```

**What you'll learn:**
- Adding tools to agents
- Using DuckDuckGo search
- Tool calling and responses
- Creating a news reporter agent

---

### 03 - Agent with Knowledge
**File:** `03_agent_with_knowledge/main.go`

**Prerequisites:** Qdrant running on localhost:6333
```bash
docker run -p 6333:6333 qdrant/qdrant
```

Build a RAG (Retrieval Augmented Generation) agent with vector database.

```bash
cd 03_agent_with_knowledge && go run main.go
```

**What you'll learn:**
- Setting up knowledge base
- Using Qdrant vector database
- Adding documents to knowledge
- RAG-powered responses

---

### 04 - Write Your Own Tool
**File:** `04_write_your_own_tool/main.go`

Create custom tools for your agents (Hacker News API example).

```bash
cd 04_write_your_own_tool && go run main.go
```

**What you'll learn:**
- Creating custom tool functions
- Tool parameter definitions
- Integrating external APIs
- Tool response handling

---

### 05 - Structured Output
**File:** `05_structured_output/main.go`

Get structured JSON responses from your agents.

```bash
cd 05_structured_output && go run main.go
```

**What you'll learn:**
- Defining output schemas
- Using OutputModel
- Parsing structured responses
- Type-safe agent outputs

---

### 06 - Agent with Storage
**File:** `06_agent_with_storage/main.go`

Enable conversation history and persistence.

```bash
cd 06_agent_with_storage && go run main.go
```

**What you'll learn:**
- Setting up storage
- Conversation history
- Session management
- Multi-turn conversations

---

### 07 - Agent State
**File:** `07_agent_state/main.go`

Manage agent state for stateful applications (game example).

```bash
cd 07_agent_state && go run main.go
```

**What you'll learn:**
- Enabling agentic state
- Setting and getting state
- State persistence
- Building stateful applications

---

### 08 - Agent Context
**File:** `08_agent_context/main.go`

Provide rich context for personalized responses.

```bash
cd 08_agent_context && go run main.go
```

**What you'll learn:**
- Adding context data
- User profiles
- Datetime and timezone context
- Personalized recommendations

---

### 09 - Agent Session
**File:** `09_agent_session/main.go`

Manage multiple sessions for the same user.

```bash
cd 09_agent_session && go run main.go
```

**What you'll learn:**
- Session IDs and User IDs
- Multi-session management
- Session continuity
- Cross-session context

---

### 10 - User Memories and Summaries
**File:** `10_user_memories_and_summaries/main.go`

Enable long-term memory and conversation summarization.

```bash
cd 10_user_memories_and_summaries && go run main.go
```

**What you'll learn:**
- Memory management
- User memory storage
- Session summarization
- Personalized long-term interactions

---

## 🎯 Quick Start

Run all examples in sequence:

```bash
#!/bin/bash
for i in {01..10}; do
    dir=$(ls -d ${i}_* 2>/dev/null | head -1)
    if [ -d "$dir" ]; then
        echo "Running $dir..."
        cd "$dir" && go run main.go
        cd ..
        echo "---"
    fi
done
```

## 📖 Learning Path

**Beginner:**
1. Start with `01_basic_agent`
2. Add tools with `02_agent_with_tools`
3. Learn storage with `06_agent_with_storage`

**Intermediate:**
4. Create custom tools with `04_write_your_own_tool`
5. Use structured output with `05_structured_output`
6. Manage context with `08_agent_context`

**Advanced:**
7. Implement RAG with `03_agent_with_knowledge`
8. Handle state with `07_agent_state`
9. Manage sessions with `09_agent_session`
10. Add memory with `10_user_memories_and_summaries`

## 🔧 Configuration

All examples use the same Ollama configuration:

```go
model, err := ollama.NewOllamaChat(
    models.WithID("llama3.2:latest"),
    models.WithBaseURL("http://localhost:11434"),
)
```

To use a different model:
```go
models.WithID("llama3.1:latest")  // or any other Ollama model
```

## 🎨 Features Demonstrated

- ✅ Beautiful terminal output with lipgloss
- ✅ Markdown rendering with glamour
- ✅ Emoji support
- ✅ Streaming responses
- ✅ Tool calling
- ✅ RAG (Retrieval Augmented Generation)
- ✅ Memory management
- ✅ Session handling
- ✅ State management
- ✅ Structured outputs

## 🐛 Troubleshooting

### Ollama not responding
```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Restart Ollama
pkill ollama && ollama serve
```

### Model not found
```bash
# Pull the model
ollama pull llama3.2:latest

# List available models
ollama list
```

### Qdrant connection error (Example 03)
```bash
# Start Qdrant with Docker
docker run -p 6333:6333 qdrant/qdrant

# Or use in-memory vector DB instead
```

## 📚 Next Steps

After completing these examples, explore:

- **Agents/** - Advanced agent patterns
- **Tools/** - More tool examples
- **Workflows/** - Multi-agent workflows
- **Teams/** - Agent collaboration

## 🤝 Contributing

Found an issue or want to improve an example? Contributions are welcome!

---

**Happy coding! 🚀**

*All examples use Ollama for local, cost-free development*
