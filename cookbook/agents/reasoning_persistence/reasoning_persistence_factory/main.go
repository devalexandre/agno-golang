package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func main() {
	// Example 1: Using SQLite (simplest option)
	fmt.Println("=== Example 1: SQLite ===")
	sqlitePersistence, err := createSQLitePersistence()
	if err != nil {
		log.Printf("SQLite error: %v\n", err)
	} else {
		fmt.Printf("SQLite persistence created: %T\n", sqlitePersistence)
	}

	// Example 2: Using PostgreSQL
	fmt.Println("\n=== Example 2: PostgreSQL ===")
	postgresPersistence, err := createPostgreSQLPersistence()
	if err != nil {
		log.Printf("PostgreSQL error: %v\n", err)
	} else {
		fmt.Printf("PostgreSQL persistence created: %T\n", postgresPersistence)
	}

	// Example 3: Using MySQL
	fmt.Println("\n=== Example 3: MySQL ===")
	mysqlPersistence, err := createMySQLPersistence()
	if err != nil {
		log.Printf("MySQL error: %v\n", err)
	} else {
		fmt.Printf("MySQL persistence created: %T\n", mysqlPersistence)
	}

	// Example 4: Using environment variables
	fmt.Println("\n=== Example 4: Environment-based Configuration ===")
	envPersistence, err := createFromEnvironment()
	if err != nil {
		log.Printf("Environment-based error: %v\n", err)
	} else {
		fmt.Printf("Environment-based persistence created: %T\n", envPersistence)
	}

	// Example 5: Demonstrating error handling
	fmt.Println("\n=== Example 5: Error Handling ===")
	demonstrateErrorHandling()
}

// createSQLitePersistence creates a SQLite persistence instance
func createSQLitePersistence() (reasoning.ReasoningPersistence, error) {
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeSQLite,
		Database: "/tmp/agno_reasoning.db",
	}

	return reasoning.NewReasoningPersistence(config)
}

// createPostgreSQLPersistence creates a PostgreSQL persistence instance
func createPostgreSQLPersistence() (reasoning.ReasoningPersistence, error) {
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypePostgreSQL,
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "password",
		Database: "agno",
		SSLMode:  "disable",
	}

	return reasoning.NewReasoningPersistence(config)
}

// createMySQLPersistence creates a MySQL persistence instance
func createMySQLPersistence() (reasoning.ReasoningPersistence, error) {
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeMySQL,
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "password",
		Database: "agno",
	}

	return reasoning.NewReasoningPersistence(config)
}

// createFromEnvironment creates persistence from environment variables
func createFromEnvironment() (reasoning.ReasoningPersistence, error) {
	// Get database type from environment, default to SQLite
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

// parsePort converts a string to an integer port
func parsePort(portStr string) int {
	if portStr == "" {
		return 0
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return port
}

// demonstrateErrorHandling shows how to handle errors from the factory
func demonstrateErrorHandling() {
	// Test 1: Nil config
	fmt.Println("Test 1: Nil config")
	_, err := reasoning.NewReasoningPersistence(nil)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	}

	// Test 2: Unsupported database type
	fmt.Println("Test 2: Unsupported database type")
	config := &reasoning.DatabaseConfig{
		Type: reasoning.DatabaseType("unsupported"),
	}
	_, err = reasoning.NewReasoningPersistence(config)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	}

	// Test 3: Missing required PostgreSQL config
	fmt.Println("Test 3: Missing required PostgreSQL config")
	config = &reasoning.DatabaseConfig{
		Type: reasoning.DatabaseTypePostgreSQL,
		// Missing Host, Port, Database
	}
	_, err = reasoning.NewReasoningPersistence(config)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	}

	// Test 4: Missing SQLite database path
	fmt.Println("Test 4: Missing SQLite database path")
	config = &reasoning.DatabaseConfig{
		Type: reasoning.DatabaseTypeSQLite,
		// Missing Database
	}
	_, err = reasoning.NewReasoningPersistence(config)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	}
}

// Example of using persistence with an agent (pseudo-code)
func exampleWithAgent(ctx context.Context) error {
	// Create persistence
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeSQLite,
		Database: "/tmp/agno_reasoning.db",
	}

	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		return fmt.Errorf("failed to create persistence: %w", err)
	}

	// Use persistence to save reasoning steps
	step := reasoning.ReasoningStepRecord{
		RunID:           "run-123",
		AgentID:         "agent-1",
		StepNumber:      1,
		Title:           "Initial Analysis",
		Reasoning:       "Analyzing the problem...",
		Action:          "search",
		Result:          "Found relevant information",
		Confidence:      0.95,
		ReasoningTokens: 150,
		InputTokens:     50,
		OutputTokens:    100,
		Duration:        1000,
	}

	if err := persistence.SaveReasoningStep(ctx, step); err != nil {
		return fmt.Errorf("failed to save reasoning step: %w", err)
	}

	// Retrieve reasoning history
	history, err := persistence.GetReasoningHistory(ctx, "run-123")
	if err != nil {
		return fmt.Errorf("failed to get reasoning history: %w", err)
	}

	fmt.Printf("Retrieved history with %d steps\n", len(history.Steps))

	return nil
}
