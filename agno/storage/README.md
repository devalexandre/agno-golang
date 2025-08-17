# Agno-Golang Storage System

Sistema de armazenamento persistente para o Agno-Golang, compatível com os padrões do Python Agno.

## Visão Geral

O sistema de storage fornece persistência de dados para:
- **Sessões de Agente**: Armazenamento de conversas e estado de agentes individuais
- **Sessões de Team**: Persistência de colaborações em equipe e histórico de mensagens
- **Sessões de Workflow**: Armazenamento de execuções de workflow (V1 e V2)

## Arquitetura

```
agno/storage/
├── contracts.go      # Interfaces e contratos principais
└── sqlite/
    └── sqlite.go      # Implementação SQLite
```

### Components Principais

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

## Uso Básico

### 1. Configuração do Storage SQLite

```go
import (
    "github.com/devalexandre/agno-golang/agno/storage"
    "github.com/devalexandre/agno-golang/agno/storage/sqlite"
)

// Criar instância de storage
dbFile := "my_app.db"
sqliteStorage, err := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{
    TableName:         "sessions",
    DBFile:            &dbFile,
    SchemaVersion:     1,
    AutoUpgradeSchema: true,
    Mode:              storage.TeamMode,
})

// Criar tabelas
err = sqliteStorage.Create()
```

### 2. Integração com Team System

```go
import (
    "github.com/devalexandre/agno-golang/agno/team"
)

// Configurar team com storage
teamConfig := team.TeamConfig{
    // ... outras configurações
    Storage:   sqliteStorage,
    SessionID: "my-session-001",
    UserID:    "user-123",
}

myTeam := team.NewTeam(teamConfig)

// As interações serão automaticamente persistidas
response, err := myTeam.Run("Analyze this data...")
```

### 3. Operações Manuais de Storage

```go
// Criar uma sessão
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

// Salvar (Upsert)
savedSession, err := sqliteStorage.Upsert(session)

// Ler
loadedSession, err := sqliteStorage.Read("session-001", stringPtr("user-123"))

// Listar
sessions, err := sqliteStorage.List(stringPtr("user-123"), nil, nil)

// Deletar
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

### Extensões por Modo

#### Team Mode
```sql
-- Colunas adicionais:
team_id TEXT NOT NULL,
team_data TEXT,
team_session_id TEXT
```

#### Workflow Mode
```sql
-- Colunas adicionais:
workflow_id TEXT NOT NULL,
workflow_data TEXT
```

#### Agent Mode
```sql
-- Colunas adicionais:
agent_id TEXT NOT NULL,
agent_data TEXT
```

## Recursos Avançados

### 1. Versionamento de Schema
- Schema automático com versionamento
- Upgrade automático de versões antigas
- Compatibilidade backward

### 2. Serialização JSON
- Dados complexos armazenados como JSON
- Deserialização automática para structs Go
- Compatibilidade com Python Agno

### 3. Indexação
```sql
-- Índices automáticos criados:
CREATE INDEX idx_{table}_user_id ON {table} (user_id);
CREATE INDEX idx_{table}_created_at ON {table} (created_at);
-- Índices específicos por modo (team_id, workflow_id, etc.)
```

### 4. Operações UPSERT
- Insert ou Update automático baseado em chave primária
- Preservação de created_at em updates
- Atualização automática de updated_at

## Compatibilidade com Python Agno

Este sistema é projetado para ser compatível com o Python Agno:

- **Estruturas de dados idênticas**
- **Schema de banco compatível**
- **Modos de storage equivalentes**
- **Serialização JSON compatível**

## Exemplos Práticos

### Team com Persistência
```bash
# Executar exemplo completo
go run examples/team/sqlite_storage/main.go
```

### Workflow com Storage (TODO)
```bash
# Exemplo futuro
go run examples/workflow/sqlite_storage/main.go
```

## Configurações Avançadas

### Custom Table Names
```go
config := sqlite.SqliteStorageConfig{
    TableName: "my_custom_sessions",
    // ...
}
```

### Diferentes Databases por Modo
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

## Performance e Otimização

### Indexação
- Índices automáticos em user_id, created_at
- Índices específicos por modo (team_id, workflow_id)
- Query optimization para operações comuns

### Connection Pooling
- SQLite com configuração otimizada
- WAL mode para melhor concorrência
- Timeout configurável

### Memory Usage
- Lazy loading de dados grandes
- Streaming para operações batch
- Cleanup automático de sessões antigas (futuro)

## Roadmap

### Futuras Implementações
- [ ] **PostgreSQL Storage**: Para aplicações enterprise
- [ ] **Redis Storage**: Para cache e sessões temporárias
- [ ] **Cloud Storage**: AWS RDS, Google Cloud SQL
- [ ] **Backup/Restore**: Ferramentas de backup automático
- [ ] **Migration Tools**: Migração entre diferentes storages
- [ ] **Analytics**: Métricas e analytics de uso

### Recursos Avançados
- [ ] **Encryption**: Criptografia de dados sensíveis
- [ ] **Compression**: Compressão de dados grandes
- [ ] **Partitioning**: Particionamento de dados por data
- [ ] **Replication**: Replicação para alta disponibilidade

## Troubleshooting

### Problemas Comuns

#### Storage não salva dados
```go
// Verificar se storage e/ou memory estão configurados
if team.storage == nil && team.memory == nil {
    // Storage não será executado
}
```

#### Erro de schema
```go
// Habilitar auto-upgrade
config.AutoUpgradeSchema = true
```

#### Performance lenta
```go
// Verificar índices
sqlite3 myapp.db ".schema"
// Deve mostrar índices automáticos
```

## Contribuição

Para contribuir com o sistema de storage:

1. **Testes**: Adicionar testes para novos recursos
2. **Documentação**: Atualizar esta documentação
3. **Compatibilidade**: Manter compatibilidade com Python Agno
4. **Performance**: Otimizações são sempre bem-vindas

## Licença

MIT License - compatível com o projeto Agno principal.
