# Reasoning Persistence Factory

A factory pattern implementation that abstracts database selection for reasoning persistence, allowing users to choose their database without worrying about the underlying library implementation.

## Overview

The factory pattern provides a simple, unified interface for configuring reasoning persistence across multiple database backends:

- **SQLite** - Local file-based database
- **PostgreSQL** - Enterprise-grade relational database
- **MySQL** - Popular open-source relational database
- **MariaDB** - MySQL-compatible database
- **Oracle** - Enterprise database system
- **SQL Server** - Microsoft's enterprise database

## Quick Start

### SQLite (Simplest Option)

```go
package main

import (
	"log"
	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func main() {
	// Configure SQLite
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeSQLite,
		Database: "/path/to/agno.db",
	}

	// Create persistence instance
	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatal(err)
	}

	// Use persistence...
	_ = persistence
}
```

### PostgreSQL

```go
package main

import (
	"log"
	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func main() {
	// Configure PostgreSQL
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypePostgreSQL,
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "password",
		Database: "agno",
		SSLMode:  "disable", // or "require", "verify-ca", "verify-full"
	}

	// Create persistence instance
	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatal(err)
	}

	// Use persistence...
	_ = persistence
}
```

### MySQL

```go
package main

import (
	"log"
	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func main() {
	// Configure MySQL
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeMySQL,
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "password",
		Database: "agno",
	}

	// Create persistence instance
	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatal(err)
	}

	// Use persistence...
	_ = persistence
}
```

## Configuration Options

### DatabaseConfig Structure

```go
type DatabaseConfig struct {
	// Type is the database type (required)
	Type DatabaseType

	// Host is the server address (not needed for SQLite)
	Host string

	// Port is the server port (not needed for SQLite)
	Port int

	// User is the database user
	User string

	// Password is the database password
	Password string

	// Database is the database name (or path for SQLite)
	Database string

	// SSLMode is the SSL mode for PostgreSQL
	// Options: disable, require, verify-ca, verify-full
	SSLMode string

	// MaxConnections is the maximum number of connections
	MaxConnections int

	// MaxIdleConnections is the maximum number of idle connections
	MaxIdleConnections int
}
```

### Database Types

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

## Usage with Agent

```go
package main

import (
	"context"
	"log"
	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func main() {
	// Configure persistence
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
	response, err := ag.Run(ctx, "What is 2 + 2?")
	if err != nil {
		log.Fatal(err)
	}

	log.Println(response)
}
```

## Environment-Based Configuration

```go
package main

import (
	"log"
	"os"
	"strconv"
	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func getReasoningPersistence() (reasoning.ReasoningPersistence, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		dbType = "sqlite"
	}

	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseType(dbType),
		Host:     os.Getenv("DB_HOST"),
		Port:     parsePort(os.Getenv("DB_PORT")),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Database: os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSL_MODE"),
	}

	return reasoning.NewReasoningPersistence(config)
}

func parsePort(portStr string) int {
	if portStr == "" {
		return 0
	}
	port, _ := strconv.Atoi(portStr)
	return port
}

func main() {
	persistence, err := getReasoningPersistence()
	if err != nil {
		log.Fatal(err)
	}

	_ = persistence
}
```

## Implementation Details

### How It Works

1. **Factory Function**: `NewReasoningPersistence()` accepts a `DatabaseConfig`
2. **Type Switching**: Routes to the appropriate database implementation
3. **Abstraction**: Users don't need to know about ksql or specific drivers
4. **Extensibility**: New database types can be added by implementing the `ReasoningPersistence` interface

### Current Implementation Status

- ✅ Factory pattern structure
- ✅ Database type constants
- ✅ Configuration validation
- ⏳ SQLite implementation (requires `github.com/mattn/go-sqlite3`)
- ⏳ PostgreSQL implementation (requires `github.com/vingarcia/ksql`)
- ⏳ MySQL implementation (requires `github.com/vingarcia/ksql`)
- ⏳ MariaDB implementation (requires `github.com/vingarcia/ksql`)
- ⏳ Oracle implementation (requires `github.com/vingarcia/ksql`)
- ⏳ SQL Server implementation (requires `github.com/vingarcia/ksql`)

## Adding New Database Support

To add support for a new database:

1. Create a new function in `persistence_factory.go`:

```go
func newNewDatabasePersistence(config *DatabaseConfig) (ReasoningPersistence, error) {
	// Validation
	if config.Host == "" || config.Port == 0 || config.Database == "" {
		return nil, fmt.Errorf("host, port and database are required")
	}

	// Implementation
	// ...

	return persistence, nil
}
```

2. Add a new constant to `DatabaseType`:

```go
const (
	DatabaseTypeNewDatabase DatabaseType = "newdatabase"
)
```

3. Add a case to the switch statement in `NewReasoningPersistence()`:

```go
case DatabaseTypeNewDatabase:
	return newNewDatabasePersistence(config)
```

## Error Handling

The factory provides clear error messages:

```go
persistence, err := reasoning.NewReasoningPersistence(config)
if err != nil {
	// Handle specific errors
	switch err.Error() {
	case "database config is nil":
		log.Println("Configuration is missing")
	case "unsupported database type: unknown":
		log.Println("Database type not supported")
	default:
		log.Printf("Error: %v\n", err)
	}
}
```

## Best Practices

1. **Use environment variables** for database configuration in production
2. **Validate configuration** before creating persistence
3. **Handle errors** appropriately
4. **Close connections** when done (if applicable)
5. **Use connection pooling** for better performance
6. **Test with multiple databases** if supporting multiple backends

## Related Files

- `persistence.go` - Core interface and SQLite implementation
- `persistence_ksql.go` - ksql-based implementation template
- `persistence_factory.go` - Factory pattern implementation
- `persistence_test.go` - Unit tests

## See Also

- [Reasoning Documentation](./README.md)
- [ksql Documentation](https://pkg.go.dev/github.com/vingarcia/ksql)
- [SQLite Documentation](https://www.sqlite.org/)
