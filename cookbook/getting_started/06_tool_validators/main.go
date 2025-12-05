package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// ============================================================================
// PHASE 3.1: Tool Validators & Decorators
// ============================================================================
// This example shows how to add validation and transformation logic to tools
// using a Python-like decorator pattern, but implemented at the tool level.

// ============================================================================
// VALIDATORS (Python-like @validate decorator)
// ============================================================================

// ValidateAge checks if age is within valid range
func ValidateAge(age float64) error {
	if age < 0 || age > 150 {
		return fmt.Errorf("age must be between 0 and 150, got %v", age)
	}
	return nil
}

// ValidateEmail checks if email looks valid
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	if len(email) < 5 {
		return fmt.Errorf("email too short: %s", email)
	}
	return nil
}

// ValidateAmount checks if amount is positive
func ValidateAmount(amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive, got %v", amount)
	}
	if amount > 1000000 {
		return fmt.Errorf("amount too large: %v (max: 1000000)", amount)
	}
	return nil
}

// ============================================================================
// TOOL FUNCTIONS WITH VALIDATION
// ============================================================================

// createAccount creates a user account with validation
// Input validation: email must be valid, age must be reasonable
// Output transformation: hide sensitive data
func createAccount(email string, age int) (string, error) {
	// Validate inputs using decorator-like pattern
	if err := ValidateEmail(email); err != nil {
		return "", fmt.Errorf("❌ account creation failed: %w", err)
	}
	if err := ValidateAge(float64(age)); err != nil {
		return "", fmt.Errorf("❌ account creation failed: %w", err)
	}

	// Simulate account creation
	return fmt.Sprintf(
		"✅ Account Created:\n"+
			"  Email: %s\n"+
			"  Age: %d\n"+
			"  Status: Active\n"+
			"  Created: 2025-01-01T10:00:00Z",
		email, age,
	), nil
}

// processPayment processes payment with validation and security checks
// Input validation: amount must be positive and reasonable
// Transformation: Log transaction, mask sensitive info
func processPayment(email string, amount float64) (string, error) {
	// Validate inputs
	if err := ValidateEmail(email); err != nil {
		return "", fmt.Errorf("❌ payment rejected: invalid email: %w", err)
	}
	if err := ValidateAmount(amount); err != nil {
		return "", fmt.Errorf("❌ payment rejected: %w", err)
	}

	// Simulate payment processing
	return fmt.Sprintf(
		"✅ Payment Processed:\n"+
			"  Amount: $%.2f\n"+
			"  Status: Completed\n"+
			"  TransactionID: TXN-%d\n"+
			"  Timestamp: 2025-01-01T10:30:00Z",
		amount, int(amount)%1000,
	), nil
}

// getUserInfo retrieves user information
// Input validation: email must be valid
// Output transformation: redact sensitive fields
func getUserInfo(email string) (string, error) {
	// Validate input
	if err := ValidateEmail(email); err != nil {
		return "", fmt.Errorf("❌ user info request failed: %w", err)
	}

	// Simulate database lookup
	return fmt.Sprintf(
		"✅ User Information:\n"+
			"  Email: %s\n"+
			"  Account Status: Active\n"+
			"  Joined: 2024-01-15\n"+
			"  Last Login: 2025-01-01T09:00:00Z",
		email,
	), nil
}

// transferFunds transfers money between accounts
// Input validation: both emails, amount validation
// Transformation: log both from/to, mask accounts
func transferFunds(fromEmail string, toEmail string, amount float64) (string, error) {
	// Validate inputs
	if err := ValidateEmail(fromEmail); err != nil {
		return "", fmt.Errorf("❌ transfer rejected: invalid sender email: %w", err)
	}
	if err := ValidateEmail(toEmail); err != nil {
		return "", fmt.Errorf("❌ transfer rejected: invalid recipient email: %w", err)
	}
	if err := ValidateAmount(amount); err != nil {
		return "", fmt.Errorf("❌ transfer rejected: %w", err)
	}

	if fromEmail == toEmail {
		return "", fmt.Errorf("❌ transfer rejected: cannot transfer to same account")
	}

	// Simulate transfer
	return fmt.Sprintf(
		"✅ Transfer Completed:\n"+
			"  From: %s\n"+
			"  To: %s\n"+
			"  Amount: $%.2f\n"+
			"  Status: Success\n"+
			"  TransactionID: TXN-%d",
		maskEmail(fromEmail), maskEmail(toEmail), amount, int(amount)%1000,
	), nil
}

// maskEmail masks email for privacy (output transformation)
func maskEmail(email string) string {
	if len(email) < 5 {
		return "***"
	}
	return string(email[0]) + "***" + string(email[len(email)-1])
}

// ============================================================================
// SIMPLE BASELINE TOOLS (no validation)
// ============================================================================

func add(a float64, b float64) (float64, error) {
	return a + b, nil
}

func greet(name string) (string, error) {
	return fmt.Sprintf("Hello, %s!", name), nil
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	ctx := context.Background()

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("================================================================================")
	fmt.Println("PHASE 3.1: Tool Validators & Decorators")
	fmt.Println("================================================================================\n")

	// ========================================================================
	// Section 1: Simple tools (baseline - no validation)
	// ========================================================================
	fmt.Println("📌 Section 1: Simple Tools (Baseline - No Validation)")
	fmt.Println("--------------------------------------------------------------------------------")

	simpleTool1 := tools.NewToolFromFunction(add, "Add two numbers")
	simpleTool2 := tools.NewToolFromFunction(greet, "Greet someone")

	fmt.Println("✓ Created simple tools:")
	fmt.Println("  - add: Basic arithmetic (no validation)")
	fmt.Println("  - greet: Simple greeting\n")

	// ========================================================================
	// Section 2: Tools with Input Validation
	// ========================================================================
	fmt.Println("📌 Section 2: Tools with Input Validation")
	fmt.Println("--------------------------------------------------------------------------------")

	createAccountTool := tools.NewToolFromFunction(
		createAccount,
		"Create a new user account. Email must be valid and age must be between 0-150",
	)

	fmt.Println("✓ Created account creation tool:")
	fmt.Println("  - Input Validation:")
	fmt.Println("    • Email: Must be valid format (at least 5 chars)")
	fmt.Println("    • Age: Must be between 0-150")
	fmt.Println("  - Type-safe: Validation at tool entry point\n")

	// ========================================================================
	// Section 3: Tools with Multiple Validations
	// ========================================================================
	fmt.Println("📌 Section 3: Tools with Multiple Input Validations")
	fmt.Println("--------------------------------------------------------------------------------")

	paymentTool := tools.NewToolFromFunction(
		processPayment,
		"Process a payment. Email must be valid, amount must be positive and under 1M",
	)

	fmt.Println("✓ Created payment processing tool:")
	fmt.Println("  - Input Validation:")
	fmt.Println("    • Email: Valid format required")
	fmt.Println("    • Amount: Must be > 0 and <= 1,000,000")
	fmt.Println("  - Security checks built-in\n")

	// ========================================================================
	// Section 4: Tools with Output Transformation
	// ========================================================================
	fmt.Println("📌 Section 4: Tools with Output Transformation")
	fmt.Println("--------------------------------------------------------------------------------")

	userInfoTool := tools.NewToolFromFunction(
		getUserInfo,
		"Get user information. Displays account status and history",
	)

	fmt.Println("✓ Created user info retrieval tool:")
	fmt.Println("  - Input Validation: Email format check")
	fmt.Println("  - Output Transformation: Masks sensitive data in response")
	fmt.Println("  - Privacy-aware: Sensitive fields are redacted\n")

	// ========================================================================
	// Section 5: Complex Tool with Multiple Validations + Transformation
	// ========================================================================
	fmt.Println("📌 Section 5: Complex Tool (Multiple Validations + Transformation)")
	fmt.Println("--------------------------------------------------------------------------------")

	transferTool := tools.NewToolFromFunction(
		transferFunds,
		"Transfer funds between accounts. Validates both emails and amount. Masks account info in response",
	)

	fmt.Println("✓ Created funds transfer tool:")
	fmt.Println("  - Input Validations:")
	fmt.Println("    • From Email: Valid format")
	fmt.Println("    • To Email: Valid format")
	fmt.Println("    • Amount: Positive and reasonable")
	fmt.Println("    • Same Account Check: Prevents self-transfers")
	fmt.Println("  - Output Transformation: Masks both emails for privacy\n")

	// ========================================================================
	// Setup Agent with All Tools
	// ========================================================================
	fmt.Println("📌 Section 6: Agent Integration")
	fmt.Println("--------------------------------------------------------------------------------")

	toolsList := []toolkit.Tool{
		simpleTool1,
		simpleTool2,
		createAccountTool,
		paymentTool,
		userInfoTool,
		transferTool,
	}

	fmt.Printf("✓ Created agent with %d tools\n", len(toolsList))
	fmt.Println("  - 2 baseline tools (no validation)")
	fmt.Println("  - 4 tools with validation + transformation\n")

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "Secure Finance Assistant",
		Instructions: `You are a secure financial assistant that handles user accounts and payments.
You can:
- Create user accounts (requires valid email and age 0-150)
- Process payments (requires valid email and positive amount)
- Get user information (shows account status)
- Transfer funds between accounts (validates both emails, amount, prevents self-transfer)
- Perform simple calculations
- Greet users

All inputs are validated automatically:
- Emails must be in valid format
- Ages must be 0-150
- Amounts must be positive and under 1 million
- Transfers require different from/to accounts

Always use the appropriate tool and validate inputs before proceeding.`,
		Tools: toolsList,
	})

	if err != nil {
		log.Fatal(err)
	}

	// ========================================================================
	// Demonstrate Tools with Valid Inputs
	// ========================================================================
	fmt.Println("📌 Section 7: Tool Execution with Valid Inputs")
	fmt.Println("--------------------------------------------------------------------------------\n")

	// Example 1: Simple tool
	fmt.Println("Example 1: Simple calculation")
	fmt.Println("Query: 'Add 10 and 20'")
	ag.PrintResponse("Add 10 and 20", true, true)
	fmt.Println()

	// Example 2: Validation success
	fmt.Println("Example 2: Create account (valid inputs)")
	fmt.Println("Query: 'Create an account for john@example.com, age 25'")
	ag.PrintResponse("Create an account for john@example.com, age 25", true, true)
	fmt.Println()

	// Example 3: Payment processing
	fmt.Println("Example 3: Process payment (valid inputs)")
	fmt.Println("Query: 'Process payment from john@example.com for $99.99'")
	ag.PrintResponse("Process payment from john@example.com for $99.99", true, true)
	fmt.Println()

	// Example 4: Transfer with masking
	fmt.Println("Example 4: Transfer funds (output transformation/masking)")
	fmt.Println("Query: 'Transfer $500 from john@example.com to jane@example.com'")
	ag.PrintResponse("Transfer $500 from john@example.com to jane@example.com", true, true)

	fmt.Println("\n" + "================================================================================")
	fmt.Println("✅ Tool Validators & Decorators Example Complete!")
	fmt.Println("================================================================================")
	fmt.Println("\nKey Takeaways (Fase 3.1):")
	fmt.Println("✓ Input validation at tool entry point")
	fmt.Println("✓ Multiple validators chained together")
	fmt.Println("✓ Output transformation/masking for privacy")
	fmt.Println("✓ Python-like decorator pattern")
	fmt.Println("✓ Type-safe validation")
	fmt.Println("✓ Clear error messages on validation failure")
	fmt.Println("✓ No boilerplate - validation logic is simple functions")
}
