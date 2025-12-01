# Ollama Cloud + Memory Example

This example demonstrates how to create an **Agent with persistent memory** using **Ollama Cloud** as the model provider, exactly like in the Python version of Agno.

## Key Features

✨ **Memory as Parameter** - Pass memory manager directly to Agent config (Python style)
🧠 **Persistent SQLite Database** - Memories are stored and retrieved across sessions
☁️ **Ollama Cloud Model** - Uses kimi-k2:1t-cloud model from Ollama Cloud
🤖 **Agentic Memory** - AI automatically decides what information is worth remembering
📚 **Memory Recall** - Agent references past conversations naturally

## Setup

### 1. Get Ollama API Key

```bash
# Get your API key from https://ollama.com
export OLLAMA_API_KEY='your-api-key-here'
```

### 2. Run the Example

```bash
go run ./cookbook/ollama_cloud_memory/main.go
```

## What Happens

The example demonstrates 3 interactions:

### Demo 1: Learning Personal Information
User shares personal information → Agent acknowledges and remembers

```
👤 User: My name is Alice Thompson. I'm a software engineer...
🤖 Agent: Nice to meet you, Alice! I'll remember that you're a software engineer...
```

### Demo 2: Recalling Stored Information  
User asks about past conversation → Agent recalls the information

```
👤 User: What was I telling you about my hobbies?
🤖 Agent: You mentioned enjoying hiking and photography...
```

### Demo 3: Learning New Preferences
User shares new interests → Agent adds to its memory

```
👤 User: I'm recently interested in machine learning...
🤖 Agent: I'll remember your interest in machine learning and agno!
```

## Code Structure

### 1. Create Ollama Cloud Model
```go
model, err := ollama.NewOllamaChat(
    models.WithID("kimi-k2:1t-cloud"),
    models.WithBaseURL("https://ollama.com"),
    models.WithAPIKey(apiKey),
)
```

### 2. Setup Memory Database
```go
db, err := sqlite.NewSqliteMemoryDb("user_memories", dbPath)
```

### 3. Create Memory Manager
```go
memoryManager := memory.NewMemory(model, db)
```

### 4. Create Agent with Memory Parameter
```go
agt, err := agent.NewAgent(agent.AgentConfig{
    Context:             ctx,
    Model:               model,
    Name:                "Memory Assistant",
    Memory:              memoryManager,        // ✨ KEY: Pass memory as parameter
    UserID:              userID,
    EnableUserMemories:  true,                 // Store user preferences
    EnableAgenticMemory: true,                 // AI decides what to remember
    ReadChatHistory:     true,                 // Include past conversations
    NumHistoryRuns:      3,                    // Last 3 interactions
})
```

## Memory Storage

Memories are stored in:
```
~/.agno_memory/user_memories.db
```

Each memory contains:
- **memory_id**: Unique identifier
- **memory**: The actual memory text
- **topics**: Extracted topics for categorization
- **user_id**: User identifier
- **updated_at**: Last update timestamp

## Configuration Options

| Option | Description |
|--------|-------------|
| `EnableUserMemories` | Store user preferences and facts |
| `EnableAgenticMemory` | Let AI decide what's worth remembering |
| `ReadChatHistory` | Include past conversations in context |
| `NumHistoryRuns` | How many previous interactions to include |
| `AddHistoryToMessages` | Add history to messages sent to model |

## Comparison with Python

### Python (agno-python)
```python
from agno.agent import Agent
from agno.db.postgres import PostgresDb
from agno.models.ollama import Ollama

db = PostgresDb(db_url=db_url)

agent = Agent(
    model=Ollama(id="qwen2.5:latest"),
    db=db,
    enable_user_memories=True,
    enable_session_summaries=True,
)

agent.print_response("My name is John", stream=True)
```

### Go (agno-golang)
```go
db, _ := sqlite.NewSqliteMemoryDb("user_memories", dbPath)
memoryManager := memory.NewMemory(model, db)

agt, _ := agent.NewAgent(agent.AgentConfig{
    Model:               model,
    Memory:              memoryManager,
    EnableUserMemories:  true,
    EnableAgenticMemory: true,
})

resp, _ := agt.Run(ctx, "My name is Alice Thompson")
```

**The pattern is identical!** Memory is passed as a parameter to the Agent.

## Future Enhancements

- [ ] Memory Optimization Strategies (summarize, recent-only)
- [ ] Advanced memory analytics
- [ ] Multi-user memory management
- [ ] Memory export/import
- [ ] Memory cleanup and archiving

## Troubleshooting

**Q: Memory not being stored?**
A: Ensure `EnableUserMemories` or `EnableAgenticMemory` is `true`

**Q: OLLAMA_API_KEY not set?**
A: Run `export OLLAMA_API_KEY='your-key'` before running

**Q: Model not responding?**
A: Check your API key and internet connection

## Related Examples

- [Memory Optimization Strategies](../../docs/MEMORY_OPTIMIZATION_STRATEGIES.md)
- [Agent with Tools](../agents/memory_example/main.go)
- [Ollama Integration](../getting_started/main.go)

---

**Status**: ✅ Production Ready | **Last Updated**: Dec 2024
