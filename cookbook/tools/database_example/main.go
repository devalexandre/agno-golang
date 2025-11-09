package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/db"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func main() {
	ctx := context.Background()

	// Get API key from environment
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// Start PostgreSQL container with testcontainers
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
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate container: %v", err)
		}
	}()

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to get connection string: %v", err)
	}

	fmt.Println("✅ PostgreSQL container started")
	fmt.Printf("📝 Connection string: %s\n\n", connStr)

	// Create database connection using agno/db
	fmt.Println("🔌 Connecting to database using agno/db...")
	database, err := db.NewFromDSN(db.PostgreSQL, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	fmt.Println("✅ Connected to database")
	fmt.Println()

	// Create sample table and data
	fmt.Println("📊 Creating sample table and data...")
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(100) UNIQUE NOT NULL,
			age INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Insert sample data
	sampleUsers := []struct {
		name  string
		email string
		age   int
	}{
		{"Alice Johnson", "alice@example.com", 28},
		{"Bob Smith", "bob@example.com", 35},
		{"Carol Williams", "carol@example.com", 42},
		{"David Brown", "david@example.com", 31},
		{"Eve Davis", "eve@example.com", 26},
	}

	for _, user := range sampleUsers {
		_, err = database.Exec(
			"INSERT INTO users (name, email, age) VALUES ($1, $2, $3) ON CONFLICT (email) DO NOTHING",
			user.name, user.email, user.age,
		)
		if err != nil {
			log.Printf("Warning: Failed to insert user %s: %v", user.name, err)
		}
	}

	fmt.Println("✅ Sample data created")
	fmt.Println()

	// Create DatabaseTool using agno/db (follows ExaTool pattern)
	fmt.Println("🔧 Creating DatabaseTool...")
	dbTool := tools.NewDatabaseTool(database.DB, tools.DatabaseConfig{
		Type:     "postgres",
		ReadOnly: false, // Allow all operations for this example
		MaxRows:  100,
	})

	fmt.Println("✅ DatabaseTool created")
	fmt.Println()

	// Create Ollama Cloud model with tool support
	fmt.Println("🤖 Creating Ollama Cloud model...")
	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("gpt-oss:20b-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama Cloud model: %v", err)
	}

	fmt.Println("✅ Model created")
	fmt.Println()

	// Create agent with DatabaseTool
	fmt.Println("🤖 Creating agent with DatabaseTool...")

	// Debug: Print available methods
	fmt.Println("📋 Available methods in DatabaseTool:")
	for methodName := range dbTool.GetMethods() {
		fmt.Printf("  - %s\n", methodName)
	}
	fmt.Println()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Model:         ollamaModel,
		Tools:         []toolkit.Tool{dbTool},
		ShowToolsCall: true,
		Debug:         false,
		Instructions: `You are a helpful database assistant with access to structured database tools. 

		Important guidelines: 

			Always select the most appropriate tool for the user’s request.  
			To retrieve data, use execute_select with a valid SELECT query.  
			To inspect a table’s schema, use describe_table.  
			To count rows, use execute_select with SELECT COUNT(*) FROM ....  
			To insert, update, or delete data, use execute_query with the corresponding SQL statement.  
			Always include the full result from the tool in your response—never omit or summarize it arbitrarily.  
			Format output clearly: use tables, lists, or structured text so results are easy to read.
			If uncertain about the user’s intent, ask clarifying questions instead of making assumptions.`,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("✅ Agent created")
	fmt.Println()

	// Example queries
	queries := []string{
		"List all tables in the database",
		"Describe the structure of the users table",
		"Show me all users in the database",
		"Find users older than 30 years",
		"Count how many users we have",
		"Add a new user named 'Frank Miller' with email 'frank@example.com' and age 29",
		"Show me the user we just added",
	}

	fmt.Println("🚀 Running example queries...")
	fmt.Println()
	fmt.Println("=" + string(make([]byte, 78)) + "=")
	fmt.Println()

	for i, query := range queries {
		fmt.Printf("Query %d: %s\n", i+1, query)
		fmt.Println("-" + string(make([]byte, 78)) + "-")

		response, err := ag.Run(query)
		if err != nil {
			log.Printf("❌ Error: %v\n", err)
			fmt.Println()
			continue
		}

		fmt.Printf("🤖 Response:\n%s\n", response.TextContent)
		fmt.Println()
		fmt.Println("=" + string(make([]byte, 78)) + "=")
		fmt.Println()
	}

	fmt.Println("✅ All queries completed successfully!")
	fmt.Println("\n💡 Tip: You can modify this example to test different database operations")
	fmt.Println("💡 The PostgreSQL container will be automatically cleaned up on exit")
}
