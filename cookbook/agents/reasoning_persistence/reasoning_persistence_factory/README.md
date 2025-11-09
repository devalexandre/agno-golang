# Reasoning Persistence Factory Example

This example demonstrates how to use the factory pattern to create reasoning persistence instances for different database backends without worrying about the underlying library implementation.

## Overview

The factory pattern abstracts database selection, allowing users to:
- Choose their database type (SQLite, PostgreSQL, MySQL, etc.)
- Pass only database configuration
- Let the factory handle the implementation details

## Running the Example

```bash
cd cookbook/agents/reasoning_persistence_factory
go run main.go
```

## What This Example Shows

### 1. SQLite Persistence
The simplest option for local development:
```go
config := &reasoning.DatabaseConfig{
    Type:     reasoning.DatabaseTypeSQLite,
    Database: "/tmp/agno_reasoning.db",
}
persistence, err := reasoning.NewReasoningPersistence(config)
```

### 2. PostgreSQL Persistence
For production environments:
```go
config := &reasoning.DatabaseConfig{
    Type:     reasoning.DatabaseTypePostgreSQL,
    Host:     "localhost",
    Port:     5432,
    User:     "postgres",
    Password: "password",
    Database: "agno",
    SSLMode:  "disable",
}
persistence, err := reasoning.NewReasoningPersistence(config)
```

### 3. MySQL Persistence
Alternative relational database:
```go
config := &reasoning.DatabaseConfig{
    Type:     reasoning.DatabaseTypeMySQL,
    Host:     "localhost",
    Port:     3306,
    User:     "root",
    Password: "password",
    Database: "agno",
}
persistence, err := reasoning.NewReasoningPersistence(config)
```

### 4. Environment-Based Configuration
Load configuration from environment variables:
```go
config := &reasoning.DatabaseConfig{
    Type:     reasoning.DatabaseType(os.Getenv("DB_TYPE")),
    Host:     os.Getenv("DB_HOST"),
    Port:     parsePort(os.Getenv("DB_PORT")),
    User:     os.Getenv("DB_USER"),
    Password: os.Getenv("DB_PASSWORD"),
    Database: os.Getenv("DB_NAME"),
    SSLMode:  os.Getenv("DB_SSL_MODE"),
}
persistence, err := reasoning.NewReasoningPersistence(config)
```

### 5. Error Handling
Demonstrates proper error handling for various scenarios:
- Nil configuration
- Unsupported database types
- Missing required configuration
- Invalid parameters

## Key Features

✅ **Simple API** - Just pass a config and get a persistence instance
✅ **Type Safety** - DatabaseType constants prevent typos
✅ **Validation** - Configuration is validated before creating persistence
✅ **Extensible** - Easy to add new database types
✅ **Error Handling** - Clear error messages for debugging
✅ **Environment Support** - Load configuration from environment variables

## Configuration Options

### DatabaseConfig Fields

```go
type DatabaseConfig struct {
    Type                DatabaseType  // Required: database type
    Host                string        // Server address
    Port                int           // Server port
    User                string        // Database user
    Password            string        // Database password
    Database            string        // Database name or path
    SSLMode             string        // SSL mode (PostgreSQL)
    MaxConnections      int           // Max connection pool size
    MaxIdleConnections  int           // Max idle connections
}
```

### Supported Database Types

```go
const (
    DatabaseTypeSQLite     DatabaseType = "sqlite"
    DatabaseTypePostgreSQL DatabaseType = "postgresql"
    DatabaseTypeMySQL      DatabaseType = "mysql"
    DatabaseTypeMariaDB    DatabaseType = "mariadb"
    DatabaseTypeOracle     DatabaseType = "oracle"
    DatabaseTypeSQLServer  DatabaseType = "sqlserver"
)
```

## Usage with Agents

```go
// Create persistence
config := &reasoning.DatabaseConfig{
    Type:     reasoning.DatabaseTypePostgreSQL,
    Host:     "localhost",
    Port:     5432,
    User:     "postgres",
    Password: "password",
    Database: "agno",
}

persistence, err := reasoning.NewReasoningPersistence(config)
if err != nil {
    log.Fatal(err)
}

// Create agent with reasoning persistence
ag := agent.New(
    agent.WithModel("gpt-4"),
    agent.WithReasoningPersistence(persistence),
)

// Run agent
ctx := context.Background()
response, err := ag.Run(ctx, "Your question here")
```

## Environment Variables

Set these environment variables to configure persistence from environment:

```bash
export DB_TYPE=postgresql
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=password
export DB_NAME=agno
export DB_SSL_MODE=disable
```

Then use:
```go
persistence, err := createFromEnvironment()
```

## Error Scenarios

The example demonstrates handling of:

1. **Nil Configuration**
   ```
   Error: database config is nil
   ```

2. **Unsupported Database Type**
   ```
   Error: unsupported database type: unsupported
   ```

3. **Missing Required PostgreSQL Configuration**
   ```
   Error: host, port and database are required for PostgreSQL
   ```

4. **Missing SQLite Database Path**
   ```
   Error: database path is required for SQLite
   ```

## Next Steps

1. **Implement Database Drivers** - Add actual implementations for each database type
2. **Add Connection Pooling** - Configure connection pool sizes
3. **Add Migrations** - Create database schema automatically
4. **Add Tests** - Test with multiple database backends
5. **Add Monitoring** - Track connection pool metrics

## Related Documentation

- [Persistence Factory Documentation](../../../agno/reasoning/PERSISTENCE_FACTORY.md)
- [Reasoning Documentation](../../../agno/reasoning/README.md)
- [ksql Documentation](https://pkg.go.dev/github.com/vingarcia/ksql)

## Files

- `main.go` - Example implementation with multiple database configurations
- `README.md` - This file

## Notes

- The factory currently returns "not implemented" errors for non-SQLite databases
- Full implementations require the appropriate database drivers:
  - SQLite: `github.com/mattn/go-sqlite3`
  - PostgreSQL/MySQL/etc: `github.com/vingarcia/ksql`
- The factory pattern allows easy addition of new database types
- Configuration validation happens before attempting to create persistence
