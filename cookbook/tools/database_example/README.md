# Database Tool Example

This example demonstrates how to use the DatabaseTool with an AI agent to interact with a PostgreSQL database.

## Overview

The example:
1. Starts a PostgreSQL container using testcontainers
2. Creates a sample `users` table with test data
3. Creates a DatabaseTool connected to the database
4. Creates an AI agent with the DatabaseTool
5. Runs example queries through the agent

## Prerequisites

```bash
# Install required dependencies
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
```

## Environment Variables

```bash
export OLLAMA_API_KEY="your-api-key"
```

**Note:** This example uses `gpt-oss:20b-cloud` which has excellent tool calling support.

## Running the Example

### With AI Agent (Recommended for learning)
```bash
go run cookbook/tools/database_example/main.go
```

### Direct Tool Usage (Recommended for testing)
```bash
go run cookbook/tools/database_simple/main.go
```

## How It Works

### DatabaseTool Features

The DatabaseTool provides the following operations:

1. **list_tables** - List all tables in the database
2. **describe_table** - Show table structure (columns, types, constraints)
3. **execute_select** - Execute SELECT queries
4. **execute_query** - Execute any SQL query (INSERT, UPDATE, DELETE)
5. **get_table_info** - Get comprehensive table information
6. **execute_transaction** - Execute multiple queries in a transaction

### Example Usage

```go
// Create database connection
database, err := db.NewFromDSN(db.PostgreSQL, connStr)

// Create DatabaseTool
dbTool := tools.NewDatabaseToolWithDB(database.DB, tools.DatabaseConfig{
    Type:     "postgres",  // postgres, mysql, sqlite3
    ReadOnly: false,       // Set to true to allow only SELECT queries
    MaxRows:  100,         // Maximum rows to return
})

// Use with agent
agent, err := agent.NewAgent(agent.AgentConfig{
    Model: model,
    Tools: []toolkit.Tool{dbTool},
    Instructions: "You are a database assistant...",
})

// Or use directly
result, err := dbTool.Execute("list_tables", json.RawMessage(`{}`))
```

## Important Notes

### AI Agent Behavior

This example uses `gpt-oss:20b-cloud` which has excellent tool calling support. The agent should automatically call the appropriate database tools based on your queries.

**Important:** Make sure you're using a model with tool calling support:
- ✅ `gpt-oss:20b-cloud` - Recommended (used in this example)
- ✅ `deepseek-v3.1:671b-cloud` - Good tool support
- ✅ `qwen2.5:72b-cloud` - Good tool support
- ❌ `kimi-k2:1t-cloud` - Limited tool support

**Example of good queries:**
- ✅ "List all tables in the database"
- ✅ "Execute: SELECT * FROM users"
- ✅ "Show me the users table structure"

**Example of queries that may not trigger tools:**
- ❌ "What can you do?"
- ❌ "Tell me about the database"
- ❌ "How many users are there?" (too indirect)

### Direct Tool Usage

For reliable, predictable behavior, use the tool directly:

```go
// List tables
result, err := dbTool.Execute("list_tables", json.RawMessage(`{}`))

// Describe table
params, _ := json.Marshal(map[string]interface{}{
    "table": "users",
})
result, err = dbTool.Execute("describe_table", params)

// Execute query
params, _ = json.Marshal(map[string]interface{}{
    "query": "SELECT * FROM users WHERE age > 30",
})
result, err = dbTool.Execute("execute_select", params)
```

See `cookbook/tools/database_simple/main.go` for a complete working example.

## Database Support

The DatabaseTool supports:

- ✅ **PostgreSQL** - Full support
- ✅ **MySQL** - Full support
- ✅ **SQLite** - Full support
- ✅ **MariaDB** - Full support (via MySQL driver)

## Security Features

1. **Read-Only Mode** - Set `ReadOnly: true` to prevent modifications
2. **Row Limits** - Automatic LIMIT clause to prevent large result sets
3. **SQL Injection Protection** - Use parameterized queries
4. **Transaction Support** - Atomic operations with rollback on error

## Configuration Options

```go
tools.DatabaseConfig{
    Type:         "postgres",  // Database type
    ReadOnly:     false,       // Allow modifications
    MaxRows:      1000,        // Max rows per query
    MaxOpenConns: 10,          // Connection pool size
    MaxIdleConns: 5,           // Idle connections
}
```

## Troubleshooting

### Agent not calling tools

**Solution 1:** Use more direct commands
```go
response, err := agent.Run(ctx, "Execute this SQL: SELECT * FROM users")
```

**Solution 2:** Use the tool directly (see `database_simple` example)

**Solution 3:** Try a different model that's better at tool calling

### Database type not detected

**Solution:** Explicitly set the type in config
```go
tools.DatabaseConfig{
    Type: "postgres",  // or "mysql", "sqlite3"
}
```

### Connection errors

**Solution:** Check your connection string format:
- PostgreSQL: `postgres://user:pass@host:port/db?sslmode=disable`
- MySQL: `user:pass@tcp(host:port)/db`
- SQLite: `file:path/to/db.sqlite`

## Related Examples

- `cookbook/tools/database_simple/` - Direct tool usage without agent
- `agno/db/README.md` - Database package documentation
- `docs/tools/README.md` - General tools documentation

## Learn More

- [DatabaseTool Source](../../agno/tools/database_tool.go)
- [agno/db Package](../../agno/db/)
- [Tool Interface](../../agno/tools/toolkit/contracts.go)
