# Pacote agno/db

O pacote `agno/db` fornece uma abstração simples e unificada para trabalhar com múltiplos bancos de dados SQL em Go. Ele elimina a necessidade de conhecer os detalhes de cada driver e connection string, permitindo que você se concentre na lógica da aplicação.

## Bancos de Dados Suportados

- **PostgreSQL** - Banco de dados relacional robusto e rico em recursos
- **MySQL** - Banco de dados relacional popular e amplamente usado
- **SQLite** - Banco de dados leve e embutido, ideal para desenvolvimento e aplicações pequenas
- **MariaDB** - Fork do MySQL com melhorias de performance

## Instalação

```bash
go get github.com/devalexandre/agno-golang/agno/db
```

## Uso Básico

### PostgreSQL

```go
import "github.com/devalexandre/agno-golang/agno/db"

// Criar conexão
database, err := db.New(db.Config{
    Type:     db.PostgreSQL,
    Host:     "localhost",
    Port:     5432,
    User:     "postgres",
    Password: "password",
    Database: "mydb",
    SSLMode:  "disable", // ou "require", "verify-ca", "verify-full"
})
if err != nil {
    log.Fatal(err)
}
defer database.Close()

// Usar como *sql.DB normal
rows, err := database.Query("SELECT * FROM users")
```

### MySQL

```go
database, err := db.New(db.Config{
    Type:     db.MySQL,
    Host:     "localhost",
    Port:     3306,
    User:     "root",
    Password: "password",
    Database: "mydb",
})
if err != nil {
    log.Fatal(err)
}
defer database.Close()
```

### MariaDB

```go
database, err := db.New(db.Config{
    Type:     db.MariaDB,
    Host:     "localhost",
    Port:     3306,
    User:     "root",
    Password: "password",
    Database: "mydb",
})
if err != nil {
    log.Fatal(err)
}
defer database.Close()
```

### SQLite

```go
database, err := db.New(db.Config{
    Type:     db.SQLite,
    Database: "./mydb.db", // Caminho do arquivo
})
if err != nil {
    log.Fatal(err)
}
defer database.Close()
```

## Usando DSN (Data Source Name)

Se você já tem uma connection string pronta, pode usar o campo `DSN` no `Config` ou a função `NewFromDSN`:

### Opção 1: Usando Config com DSN

```go
// PostgreSQL
database, err := db.New(db.Config{
    Type: db.PostgreSQL,
    DSN:  "postgres://user:password@localhost:5432/mydb?sslmode=disable",
})

// MySQL
database, err := db.New(db.Config{
    Type: db.MySQL,
    DSN:  "user:password@tcp(localhost:3306)/mydb?parseTime=true",
})

// SQLite
database, err := db.New(db.Config{
    Type: db.SQLite,
    DSN:  "file:mydb.db?cache=shared&mode=rwc",
})
```

### Opção 2: Usando NewFromDSN

```go
// PostgreSQL
database, err := db.NewFromDSN(
    db.PostgreSQL,
    "postgres://user:password@localhost:5432/mydb?sslmode=disable",
)

// MySQL
database, err := db.NewFromDSN(
    db.MySQL,
    "user:password@tcp(localhost:3306)/mydb?parseTime=true",
)

// SQLite
database, err := db.NewFromDSN(
    db.SQLite,
    "./mydb.db",
)
```

## Configurações Avançadas

```go
database, err := db.New(db.Config{
    Type:         db.PostgreSQL,
    Host:         "localhost",
    Port:         5432,
    User:         "postgres",
    Password:     "password",
    Database:     "mydb",
    SSLMode:      "require",
    MaxOpenConns: 25,  // Máximo de conexões abertas (padrão: 10)
    MaxIdleConns: 10,  // Máximo de conexões ociosas (padrão: 5)
})
```

## Métodos Auxiliares

O pacote fornece métodos auxiliares para verificar o tipo de banco:

```go
database, _ := db.New(db.Config{
    Type:     db.PostgreSQL,
    Database: "mydb",
    // ...
})

// Verificar tipo de banco
if database.IsPostgreSQL() {
    fmt.Println("Usando PostgreSQL")
}

if database.IsMySQL() {
    fmt.Println("Usando MySQL ou MariaDB")
}

if database.IsSQLite() {
    fmt.Println("Usando SQLite")
}

// Obter tipo
dbType := database.GetType()
fmt.Printf("Tipo: %s\n", dbType)

// Obter configuração completa
config := database.GetConfig()
fmt.Printf("Host: %s, Port: %d\n", config.Host, config.Port)
```

## Exemplo Completo

```go
package main

import (
    "database/sql"
    "fmt"
    "log"

    "github.com/devalexandre/agno-golang/agno/db"
)

func main() {
    // Criar conexão
    database, err := db.New(db.Config{
        Type:     db.PostgreSQL,
        Host:     "localhost",
        Port:     5432,
        User:     "postgres",
        Password: "password",
        Database: "mydb",
        SSLMode:  "disable",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()

    // Criar tabela
    _, err = database.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            name VARCHAR(100),
            email VARCHAR(100)
        )
    `)
    if err != nil {
        log.Fatal(err)
    }

    // Inserir dados
    result, err := database.Exec(
        "INSERT INTO users (name, email) VALUES ($1, $2)",
        "João Silva",
        "joao@example.com",
    )
    if err != nil {
        log.Fatal(err)
    }

    id, _ := result.LastInsertId()
    fmt.Printf("Usuário criado com ID: %d\n", id)

    // Consultar dados
    rows, err := database.Query("SELECT id, name, email FROM users")
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    for rows.Next() {
        var id int
        var name, email string
        if err := rows.Scan(&id, &name, &email); err != nil {
            log.Fatal(err)
        }
        fmt.Printf("ID: %d, Nome: %s, Email: %s\n", id, name, email)
    }
}
```

## Integração com DatabaseTool

O pacote `agno/db` pode ser usado com o `DatabaseTool` para criar ferramentas de banco de dados para agentes:

```go
import (
    "github.com/devalexandre/agno-golang/agno/db"
    "github.com/devalexandre/agno-golang/agno/tools"
)

// Criar conexão
database, err := db.New(db.Config{
    Type:     db.PostgreSQL,
    Host:     "localhost",
    Port:     5432,
    User:     "postgres",
    Password: "password",
    Database: "mydb",
})
if err != nil {
    log.Fatal(err)
}

// Criar DatabaseTool usando a conexão
dbTool := tools.NewDatabaseToolWithDB(database.DB, tools.DatabaseConfig{
    ReadOnly: true,
    MaxRows:  100,
})

// Usar com agente
agent := agent.NewAgent(model, agent.WithTools(dbTool))
```

## Tratamento de Erros

```go
database, err := db.New(db.Config{
    Type:     db.PostgreSQL,
    Host:     "localhost",
    Port:     5432,
    User:     "postgres",
    Password: "wrong_password",
    Database: "mydb",
})
if err != nil {
    // Erro ao conectar ou configurar
    log.Printf("Erro ao criar conexão: %v\n", err)
    return
}
defer database.Close()

// Testar conexão
if err := database.Ping(); err != nil {
    log.Printf("Erro ao pingar banco: %v\n", err)
    return
}
```

## Notas Importantes

1. **Drivers**: Os drivers necessários já estão importados no pacote:
   - PostgreSQL: `github.com/lib/pq`
   - MySQL/MariaDB: `github.com/go-sql-driver/mysql`
   - SQLite: `github.com/mattn/go-sqlite3`

2. **Pool de Conexões**: O pacote configura automaticamente um pool de conexões com valores padrão sensatos (10 conexões abertas, 5 ociosas).

3. **Teste de Conexão**: O construtor `New()` automaticamente testa a conexão com `Ping()` antes de retornar.

4. **Compatibilidade**: O tipo `*db.DB` embute `*sql.DB`, então você pode usar todos os métodos padrão do `database/sql`.

5. **DSN vs Campos**: Quando `DSN` é fornecido, os campos `Host`, `Port`, `User`, `Password`, `Database` e `SSLMode` são ignorados.

## Referências

- [database/sql Documentation](https://pkg.go.dev/database/sql)
- [PostgreSQL Driver](https://github.com/lib/pq)
- [MySQL Driver](https://github.com/go-sql-driver/mysql)
- [SQLite Driver](https://github.com/mattn/go-sqlite3)
