# Session Summarization Example

This example demonstrates how to use the Memory Manager's session summarization feature to compress long conversations into concise summaries.

## Features Demonstrated

- **Session Summarization**: Automatically generate summaries of conversation sessions
- **Memory Persistence**: Store and retrieve session summaries from SQLite
- **Context Compression**: Reduce context window usage while maintaining conversation continuity
- **Summary Retrieval**: Access previous session summaries for context

## What is Session Summarization?

Session summarization is a memory management technique that:
- Compresses long conversations into concise summaries
- Reduces token usage in context windows
- Maintains conversation continuity across sessions
- Enables long-term memory without overwhelming the model

## Prerequisites

- Go 1.21 or higher
- Ollama Cloud API key

## Setup

1. Set your Ollama Cloud API key:
```bash
export OLLAMA_API_KEY=your_api_key_here
```

2. Run the example:
```bash
go run main.go
```

## How It Works

### 1. Memory Manager Setup
```go
memoryDB, err := memorysqlite.NewSqliteMemoryDb("user_memories", "session_summary.db")
memoryManager := memory.NewMemory(ollamaModel, memoryDB)
```

### 2. Conversation Simulation
The example simulates a multi-turn conversation about trip planning to Japan.

### 3. Summary Creation
```go
summary, err := memoryManager.CreateSessionSummary(ctx, userID, sessionID, conversationMessages)
```

The AI automatically:
- Analyzes the conversation
- Identifies main topics
- Extracts key decisions
- Creates a concise summary

### 4. Summary Retrieval
```go
retrievedSummary, err := memoryManager.GetSessionSummary(ctx, userID, sessionID)
```

### 5. Using Summaries in New Conversations
Summaries can be injected as context for future conversations:
```go
contextPrompt := fmt.Sprintf(`Previous conversation summary:
%s

Current question: %s`, retrievedSummary.Summary, newQuery)
```

## Benefits

- **Reduced Token Usage**: Summaries use fewer tokens than full conversation history
- **Better Context Management**: Focus on key information
- **Long-term Memory**: Maintain context across multiple sessions
- **Improved Relevance**: AI responses are more focused and relevant

## Output

The example will:
1. Simulate a conversation about trip planning
2. Create a session summary
3. Store the summary in SQLite
4. Retrieve and display the summary
5. Demonstrate using the summary in a new conversation

## Database

Session summaries are stored in `session_summary.db` with the following structure:
- User ID
- Session ID
- Summary text
- Timestamps

## Use Cases

- **Customer Support**: Summarize support tickets
- **Chatbots**: Maintain conversation context
- **Meeting Notes**: Generate meeting summaries
- **Documentation**: Create conversation documentation
- **Analytics**: Analyze conversation patterns

## Related Examples

- `memory_example/` - Basic memory management
- `session_management/` - Session state management
- `read_chat_history/` - Reading conversation history
