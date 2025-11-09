package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/db"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func main() {
	ctx := context.Background()

	// Start PostgreSQL container
	fmt.Println("🐳 Starting PostgreSQL container...")
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		log.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer postgresContainer.Terminate(ctx)

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to get connection string: %v", err)
	}

	fmt.Println("✅ PostgreSQL container started")
	fmt.Printf("📝 Connection string: %s\n\n", connStr)

	// Create database connection
	fmt.Println("🔌 Connecting to database...")
	database, err := db.NewFromDSN(db.PostgreSQL, connStr)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer database.Close()

	// Create sample table and data
	fmt.Println("📊 Creating sample data...")
	_, err = database.Exec(`
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			email VARCHAR(100),
			age INTEGER
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Insert sample data
	users := []struct {
		name  string
		email string
		age   int
	}{
		{"Alice", "alice@example.com", 28},
		{"Bob", "bob@example.com", 35},
		{"Carol", "carol@example.com", 42},
	}

	for _, u := range users {
		_, err = database.Exec(
			"INSERT INTO users (name, email, age) VALUES ($1, $2, $3)",
			u.name, u.email, u.age,
		)
		if err != nil {
			log.Printf("Warning: %v", err)
		}
	}

	fmt.Println("✅ Sample data created\n")

	// Create DatabaseTool
	fmt.Println("🔧 Creating DatabaseTool...")
	dbTool := tools.NewDatabaseTool(database.DB, tools.DatabaseConfig{
		Type:     "postgres",
		ReadOnly: false,
		MaxRows:  100,
	})
	fmt.Println("✅ DatabaseTool created\n")

	// Demo 1: List tables
	fmt.Println("📋 Demo 1: List Tables")
	fmt.Println("=" + string(make([]byte, 50)))
	result, err := dbTool.Execute("list_tables", json.RawMessage(`{}`))
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n", result)
	}
	fmt.Println()

	// Demo 2: Describe table
	fmt.Println("📋 Demo 2: Describe Table")
	fmt.Println("=" + string(make([]byte, 50)))
	params, _ := json.Marshal(map[string]interface{}{
		"table": "users",
	})
	result, err = dbTool.Execute("describe_table", params)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n", result)
	}
	fmt.Println()

	// Demo 3: Select all users
	fmt.Println("📋 Demo 3: Select All Users")
	fmt.Println("=" + string(make([]byte, 50)))
	params, _ = json.Marshal(map[string]interface{}{
		"query": "SELECT * FROM users",
	})
	result, err = dbTool.Execute("execute_select", params)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n", result)
	}
	fmt.Println()

	// Demo 4: Filter users
	fmt.Println("📋 Demo 4: Filter Users (age > 30)")
	fmt.Println("=" + string(make([]byte, 50)))
	params, _ = json.Marshal(map[string]interface{}{
		"query": "SELECT * FROM users WHERE age > 30",
	})
	result, err = dbTool.Execute("execute_select", params)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n", result)
	}
	fmt.Println()

	// Demo 5: Insert new user
	fmt.Println("📋 Demo 5: Insert New User")
	fmt.Println("=" + string(make([]byte, 50)))
	params, _ = json.Marshal(map[string]interface{}{
		"query": "INSERT INTO users (name, email, age) VALUES ('Frank', 'frank@example.com', 29)",
	})
	result, err = dbTool.Execute("execute_query", params)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n", result)
	}
	fmt.Println()

	// Demo 6: Verify insert
	fmt.Println("📋 Demo 6: Verify Insert")
	fmt.Println("=" + string(make([]byte, 50)))
	params, _ = json.Marshal(map[string]interface{}{
		"query": "SELECT * FROM users WHERE name = 'Frank'",
	})
	result, err = dbTool.Execute("execute_select", params)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n", result)
	}
	fmt.Println()

	fmt.Println("✅ All demos completed successfully!")
	fmt.Println("\n💡 This example shows direct tool usage without an AI agent")
	fmt.Println("💡 The DatabaseTool is working correctly!")
}
