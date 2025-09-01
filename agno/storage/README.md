# Agno-Golang Storage System

Persistent storage system for Agno-Golang, compatible with Python Agno standards.

## Overview

The storage system provides data persistence for:
- **Agent Sessions**: Storage of conversations and individual agent state
- **Team Sessions**: Persistence of team collaborations and message history
- **Workflow Sessions**: Storage of workflow executions (V1 and V2)

## Arquitetura

```
agno/storage/
├── contracts.go      # Interfaces e contratos principais
└── sqlite/
    └── sqlite.go      # Implementação SQLite
```

### Main Components

#### `contracts.go`
- **Storage Interface**: Contrato principal para operações CRUD
- **Session Types**: Estruturas para diferentes tipos de sessão
- **StorageMode**: Constantes para diferentes modos de armazenamento

#### `sqlite/sqlite.go`
- **SqliteStorage**: Implementação completa do storage SQLite
- **Schema Management**: Versionamento e upgrade automático de schema
- **Multi-mode Support**: Suporte para diferentes tipos de sessão

## Modos de Storage

```go
const (
    AgentMode      StorageMode = "agent"
    TeamMode       StorageMode = "team"
    WorkflowMode   StorageMode = "workflow"
    WorkflowV2Mode StorageMode = "workflow_v2"
)
```

## Basic Usage

### 1. SQLite Storage Configuration

```go
import (
    "github.com/devalexandre/agno-golang/agno/storage"
    "github.com/devalexandre/agno-golang/agno/storage/sqlite"
)

// Create storage instance
dbFile := "my_app.db"
sqliteStorage, err := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{
    TableName:         "sessions",
    DBFile:            &dbFile,
    SchemaVersion:     1,
    AutoUpgradeSchema: true,
    Mode:              storage.TeamMode,
})

// Create tables
err = sqliteStorage.Create()
```

### 2. Integration with Team System

```go
import (
    "github.com/devalexandre/agno-golang/agno/team"
)

// Configure team with storage
teamConfig := team.TeamConfig{
    // ... outras configurações
    Storage:   sqliteStorage,
    SessionID: "my-session-001",
    UserID:    "user-123",
}

myTeam := team.NewTeam(teamConfig)

// Interactions will be automatically persisted
response, err := myTeam.Run("Analyze this data...")
```

### 3. Manual Storage Operations

```go
// Create a session
session := &storage.TeamSession{
    Session: storage.Session{
        SessionID:   "session-001",
        UserID:      "user-123",
        SessionData: map[string]interface{}{
            "created_by": "application",
        },
        CreatedAt: time.Now().Unix(),
        UpdatedAt: time.Now().Unix(),
    },
    TeamID: "strategic-team",
    TeamData: map[string]interface{}{
        "messages": []map[string]interface{}{
            {"role": "user", "content": "Hello"},
            {"role": "assistant", "content": "Hi there!"},
        },
    },
}

// Save (Upsert)
savedSession, err := sqliteStorage.Upsert(session)

// Read
loadedSession, err := sqliteStorage.Read("session-001", stringPtr("user-123"))

// List
sessions, err := sqliteStorage.List(stringPtr("user-123"), nil, nil)

// Delete
err = sqliteStorage.Delete("session-001", stringPtr("user-123"))
```

## Schema do Banco de Dados

### Tabela Base (todos os modos)
```sql
CREATE TABLE sessions (
    session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    memory TEXT,
    session_data TEXT,
    extra_data TEXT,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER DEFAULT (strftime('%s', 'now'))
);
```

### Extensions by Mode

#### Team Mode
```sql
-- Additional columns:
team_id TEXT NOT NULL,
team_data TEXT,
team_session_id TEXT
```

#### Workflow Mode
```sql
-- Additional columns:
workflow_id TEXT NOT NULL,
workflow_data TEXT
```

#### Agent Mode
```sql
-- Additional columns:
agent_id TEXT NOT NULL,
agent_data TEXT
```

## Recursos Avançados

### 1. Schema Versioning
- Automatic schema with versioning
- Automatic upgrade of old versions
- Backward compatibility

### 2. JSON Serialization
- Complex data stored as JSON
- Automatic deserialization to Go structs
- Compatibility with Python Agno

### 3. Indexing
```sql
-- Automatic indexes created:
CREATE INDEX idx_{table}_user_id ON {table} (user_id);
CREATE INDEX idx_{table}_created_at ON {table} (created_at);
-- Mode-specific indexes (team_id, workflow_id, etc.)
```

### 4. UPSERT Operations
- Automatic Insert or Update based on primary key
- Preservation of created_at on updates
- Automatic update of updated_at

## Compatibility with Python Agno

This system is designed to be compatible with Python Agno:

- **Identical data structures**
- **Compatible database schema**
- **Equivalent storage modes**
- **Compatible JSON serialization**

## Practical Examples

### Team with Persistence
```bash
# Run complete example
go run examples/team/sqlite_storage/main.go
```

### Workflow with Storage (TODO)
```bash
# Future example
go run examples/workflow/sqlite_storage/main.go
```

## Advanced Configurations

### Custom Table Names
```go
config := sqlite.SqliteStorageConfig{
    TableName: "my_custom_sessions",
    // ...
}
```

### Different Databases by Mode
```go
// Team storage
teamDB := "teams.db"
teamStorage, _ := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{
    DBFile: &teamDB,
    Mode:   storage.TeamMode,
})

// Agent storage
agentDB := "agents.db"
agentStorage, _ := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{
    DBFile: &agentDB,
    Mode:   storage.AgentMode,
})
```

## Performance and Optimization

### Indexing
- Automatic indexes on user_id, created_at
- Mode-specific indexes (team_id, workflow_id)
- Query optimization for common operations

### Connection Pooling
- SQLite with optimized configuration
- WAL mode for better concurrency
- Configurable timeout

### Memory Usage
- Lazy loading of large data
- Streaming for batch operations
- Automatic cleanup of old sessions (future)

## Roadmap

### Future Implementations
- [ ] **PostgreSQL Storage**: Para aplicações enterprise
- [ ] **Redis Storage**: Para cache e sessões temporárias
- [ ] **Cloud Storage**: AWS RDS, Google Cloud SQL
- [ ] **Backup/Restore**: Ferramentas de backup automático
- [ ] **Migration Tools**: Migração entre diferentes storages
- [ ] **Analytics**: Métricas e analytics de uso

### Advanced Features
- [ ] **Encryption**: Criptografia de dados sensíveis
- [ ] **Compression**: Compressão de dados grandes
- [ ] **Partitioning**: Particionamento de dados por data
- [ ] **Replication**: Replicação para alta disponibilidade

## Troubleshooting

### Common Issues

#### Storage doesn't save data
```go
// Check if storage and/or memory are configured
if team.storage == nil && team.memory == nil {
    // Storage will not be executed
}
```

#### Schema error
```go
// Enable auto-upgrade
config.AutoUpgradeSchema = true
```

#### Slow performance
```go
// Check indexes
sqlite3 myapp.db ".schema"
// Should show automatic indexes
```

## Contribution

To contribute to the storage system:

1. **Tests**: Add tests for new features
2. **Documentation**: Update this documentation
3. **Compatibility**: Maintain compatibility with Python Agno
4. **Performance**: Optimizations are always welcome

## License

MIT License - compatible with the main Agno project.
