package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// ============================================================================
// ADVANCED EXAMPLE: Tools with Complex Struct Parameters
// ============================================================================

// User represents a person with detailed information
type User struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Age      int      `json:"age"`
	Skills   []string `json:"skills"`
	Active   bool     `json:"active"`
	JoinDate string   `json:"join_date"`
}

// BookingRequest represents a complex booking operation
type BookingRequest struct {
	CustomerName    string   `json:"customer_name"`
	Email           string   `json:"email"`
	CheckIn         string   `json:"check_in"`
	CheckOut        string   `json:"check_out"`
	RoomType        string   `json:"room_type"`
	Guests          int      `json:"guests"`
	SpecialRequests []string `json:"special_requests"`
}

// BookingResponse represents booking confirmation
type BookingResponse struct {
	BookingID    string  `json:"booking_id"`
	Status       string  `json:"status"`
	ConfirmEmail string  `json:"confirm_email"`
	TotalPrice   float64 `json:"total_price"`
}

// Location represents geographic information
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
}

// WeatherQuery for complex weather search
type WeatherQuery struct {
	Location  Location `json:"location"`
	DateRange string   `json:"date_range"`
	Metrics   []string `json:"metrics"`
}

// ============================================================================
// TOOL FUNCTIONS WITH COMPLEX STRUCT PARAMETERS
// ============================================================================

// createUserProfile takes a complex User struct and creates a profile
func createUserProfile(user User) (string, error) {
	status := "Active"
	if !user.Active {
		status = "Inactive"
	}

	return fmt.Sprintf(
		"✓ User Profile Created:\n"+
			"  Name: %s\n"+
			"  Email: %s\n"+
			"  Age: %d\n"+
			"  Skills: %v\n"+
			"  Status: %s\n"+
			"  Joined: %s",
		user.Name, user.Email, user.Age, user.Skills,
		status, user.JoinDate,
	), nil
}

// bookHotel processes a complex booking request
func bookHotel(request BookingRequest) (BookingResponse, error) {
	// Simulate booking logic
	bookingID := fmt.Sprintf("BK-%d", time.Now().UnixMilli()%100000)

	// Calculate total price (simulation)
	checkIn, _ := time.Parse("2006-01-02", request.CheckIn)
	checkOut, _ := time.Parse("2006-01-02", request.CheckOut)
	nights := checkOut.Sub(checkIn).Hours() / 24

	pricePerNight := map[string]float64{
		"standard": 100,
		"deluxe":   150,
		"suite":    250,
	}[request.RoomType]

	totalPrice := nights * pricePerNight

	return BookingResponse{
		BookingID:    bookingID,
		Status:       "Confirmed",
		ConfirmEmail: request.Email,
		TotalPrice:   totalPrice,
	}, nil
}

// searchWeather searches weather with complex nested structs
func searchWeather(query WeatherQuery) (string, error) {
	metricsStr := fmt.Sprintf("%v", query.Metrics)
	return fmt.Sprintf(
		"🌤️ Weather Search Results:\n"+
			"  Location: %s, %s\n"+
			"  Coordinates: %.2f, %.2f\n"+
			"  Period: %s\n"+
			"  Metrics: %s",
		query.Location.City, query.Location.Country,
		query.Location.Latitude, query.Location.Longitude,
		query.DateRange, metricsStr,
	), nil
}

// processMultipleUsers handles a slice of complex objects
func processMultipleUsers(users []User) (string, error) {
	if len(users) == 0 {
		return "No users provided", nil
	}

	result := fmt.Sprintf("Processed %d users:\n", len(users))
	for i, user := range users {
		result += fmt.Sprintf("  %d. %s (Age: %d, Active: %v)\n",
			i+1, user.Name, user.Age, user.Active)
	}
	return result, nil
}

// ============================================================================
// SIMPLE TOOLS (for comparison)
// ============================================================================

func add(a int, b int) (int, error) {
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

	// Create an Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("================================================================================")
	fmt.Println("ADVANCED EXAMPLE: Tools with Complex Struct Parameters")
	fmt.Println("================================================================================\n")

	// ========================================================================
	// Example 1: Simple Tools (baseline)
	// ========================================================================
	fmt.Println("📌 Section 1: Simple Tools (baseline)")
	fmt.Println("--------------------------------------------------------------------------------")

	simpleTool1 := tools.NewToolFromFunction(add, "Add two numbers")
	simpleTool2 := tools.NewToolFromFunction(greet, "Greet someone")

	fmt.Println("✓ Created simple tools:")
	fmt.Println("  - add: Takes 2 integers")
	fmt.Println("  - greet: Takes 1 string\n")

	// ========================================================================
	// Example 2: Tools with Struct Parameters
	// ========================================================================
	fmt.Println("📌 Section 2: Tools with Struct Parameters")
	fmt.Println("--------------------------------------------------------------------------------")

	userProfileTool := tools.NewToolFromFunction(
		createUserProfile,
		"Create a user profile from detailed user information including name, email, age, skills, and status",
	)

	fmt.Println("✓ Created user profile tool:")
	fmt.Println("  - Takes User struct with:")
	fmt.Println("    • ID: integer")
	fmt.Println("    • Name: string")
	fmt.Println("    • Email: string")
	fmt.Println("    • Age: integer")
	fmt.Println("    • Skills: array of strings")
	fmt.Println("    • Active: boolean")
	fmt.Println("    • JoinDate: string\n")

	// ========================================================================
	// Example 3: Tools with Nested Structs
	// ========================================================================
	fmt.Println("📌 Section 3: Tools with Nested Structs")
	fmt.Println("--------------------------------------------------------------------------------")

	weatherTool := tools.NewToolFromFunction(
		searchWeather,
		"Search weather information for a location with specific metrics and date range",
	)

	fmt.Println("✓ Created weather search tool:")
	fmt.Println("  - Takes WeatherQuery struct containing:")
	fmt.Println("    • Location: nested struct with latitude, longitude, city, country")
	fmt.Println("    • DateRange: string")
	fmt.Println("    • Metrics: array of strings")
	fmt.Println("  - Handles complex nested data structures\n")

	// ========================================================================
	// Example 4: Tools with Complex Return Types
	// ========================================================================
	fmt.Println("📌 Section 4: Tools with Complex Return Types")
	fmt.Println("--------------------------------------------------------------------------------")

	hotelBookingTool := tools.NewToolFromFunction(
		bookHotel,
		"Book a hotel room with customer details, dates, room type and special requests. Returns booking confirmation with ID and total price.",
	)

	fmt.Println("✓ Created hotel booking tool:")
	fmt.Println("  - Takes BookingRequest struct with:")
	fmt.Println("    • CustomerName, Email, CheckIn, CheckOut")
	fmt.Println("    • RoomType: 'standard', 'deluxe', or 'suite'")
	fmt.Println("    • Guests: number of guests")
	fmt.Println("    • SpecialRequests: array of strings")
	fmt.Println("  - Returns BookingResponse struct with:")
	fmt.Println("    • BookingID, Status, ConfirmEmail, TotalPrice\n")

	// ========================================================================
	// Example 5: Tools with Array of Structs
	// ========================================================================
	fmt.Println("📌 Section 5: Tools with Array Parameters")
	fmt.Println("--------------------------------------------------------------------------------")

	multiUserTool := tools.NewToolFromFunction(
		processMultipleUsers,
		"Process multiple user records at once",
	)

	fmt.Println("✓ Created multi-user processing tool:")
	fmt.Println("  - Takes array of User structs")
	fmt.Println("  - Processes all users and returns summary\n")

	// ========================================================================
	// Setup Agent with All Tools
	// ========================================================================
	fmt.Println("📌 Section 6: Agent Integration")
	fmt.Println("--------------------------------------------------------------------------------")

	toolsList := []toolkit.Tool{
		simpleTool1,
		simpleTool2,
		userProfileTool,
		weatherTool,
		hotelBookingTool,
		multiUserTool,
	}

	fmt.Printf("✓ Created agent with %d tools\n\n", len(toolsList))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "Advanced Assistant",
		Instructions: `You are an advanced assistant that handles complex data structures.
You can:
- Create user profiles with detailed information
- Book hotels with specific requirements
- Search weather with location and metric data
- Process multiple users at once
- Perform simple calculations
- Greet users

When users ask you to perform tasks, use the appropriate tools with all the required information.
Always provide detailed responses.`,
		Tools: toolsList,
	})

	if err != nil {
		log.Fatal(err)
	}

	// ========================================================================
	// Demonstrate Tools with Example Calls
	// ========================================================================
	fmt.Println("📌 Section 7: Example Tool Executions")
	fmt.Println("--------------------------------------------------------------------------------\n")

	// Example 1: Simple tool
	fmt.Println("Example 1: Simple math tool")
	fmt.Println("Query: 'What is 10 plus 15?'")
	ag.PrintResponse("What is 10 plus 15?", true, true)
	fmt.Println()

	// Example 2: Struct parameter tool
	fmt.Println("Example 2: User profile creation")
	fmt.Println("Query: Create a profile for John, 28 years old, with Go/Python/Rust skills")
	ag.PrintResponse(
		"Create a profile for John, 28 years old, with Go, Python, and Rust skills, "+
			"email john@example.com, active status, joined 2024-01-15",
		true, true,
	)
	fmt.Println()

	// Example 3: Nested struct tool
	fmt.Println("Example 3: Weather search with nested location")
	fmt.Println("Query: Search weather in São Paulo, Brazil for next week")
	ag.PrintResponse(
		"Search weather in São Paulo, Brazil at coordinates -23.55, -46.63 "+
			"for next week with temperature and humidity metrics",
		true, true,
	)
	fmt.Println()

	// Example 4: Complex return type
	fmt.Println("Example 4: Hotel booking")
	fmt.Println("Query: Book a deluxe hotel room for 3 guests")
	ag.PrintResponse(
		"Book a deluxe hotel room for 3 guests from 2024-12-10 to 2024-12-13 "+
			"for John Doe, john@example.com, with special requests for late checkout",
		true, true,
	)

	fmt.Println("\n" + "================================================================================")
	fmt.Println("✅ Advanced Tools Example Complete!")
	fmt.Println("================================================================================")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("✓ Structs work seamlessly as tool parameters")
	fmt.Println("✓ Nested structs are fully supported")
	fmt.Println("✓ Complex return types are handled automatically")
	fmt.Println("✓ Arrays of structs work as parameters")
	fmt.Println("✓ Type conversion happens automatically")
	fmt.Println("✓ Same simple API as Python - no boilerplate!")
}
