package main

import (
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/utils"
	"github.com/devalexandre/agno-golang/agno/utils/terminal"
)

func main() {
	fmt.Println("=== Terminal Panel System Demo ===\n")

	// Enable markdown mode
	utils.SetMarkdownMode(true)

	// Demo 1: Thinking Panel
	fmt.Println("1. Thinking Panel:")
	utils.ThinkingPanel("Processing your request...")
	time.Sleep(1 * time.Second)

	// Demo 2: Response Panel
	fmt.Println("\n2. Response Panel with Markdown:")
	start := time.Now()
	time.Sleep(500 * time.Millisecond)
	responseContent := `# Breaking News! 🗽

**Times Square** is buzzing with excitement today!

## Key Points:
- 🎉 Massive celebration underway
- 🎭 Street performers everywhere
- 📸 Tourists capturing memories

> "This is the energy that makes NYC special!" - Local Reporter

Back to you in the studio! 📺`
	utils.ResponsePanel(responseContent, nil, start, true)

	// Demo 3: Tool Call Panel
	fmt.Println("\n3. Tool Call Panel:")
	utils.ToolCallPanelWithArgs("search_news", map[string]interface{}{
		"query":    "Times Square breaking news",
		"location": "New York",
		"limit":    5,
	})

	// Demo 4: Debug Panel
	fmt.Println("\n4. Debug Panel:")
	utils.DebugPanel("System Message:\nYou are NewsReporter.\n\nCurrent date and time: Tuesday, November 26, 2025 at 6:18 PM\n\n<instructions>\nYou are an enthusiastic news reporter...\n</instructions>")

	// Demo 5: Success Panel
	fmt.Println("\n5. Success Panel:")
	utils.SuccessPanel("Agent initialized successfully!")

	// Demo 6: Warning Panel
	fmt.Println("\n6. Warning Panel:")
	utils.WarningPanel("Rate limit approaching: 80% of quota used")

	// Demo 7: Error Panel
	fmt.Println("\n7. Error Panel:")
	utils.ErrorPanel(fmt.Errorf("connection timeout: failed to reach API endpoint"))

	// Demo 8: Info Panel
	fmt.Println("\n8. Info Panel:")
	utils.InfoPanel("Using model: llama3.2:latest\nTokens used: 1,234")

	// Demo 9: Custom Panel
	fmt.Println("\n9. Custom Panel:")
	renderer := utils.GetRenderer()
	customPanel := renderer.RenderCustom("🚀", "Custom Panel", "This is a custom panel with your own emoji and color!", terminal.ResponseColor)
	fmt.Println(customPanel)

	// Demo 10: Reasoning Panel
	fmt.Println("\n10. Reasoning Panel:")
	utils.ReasoningPanel("Step 1: Analyze the query\nStep 2: Search for relevant information\nStep 3: Synthesize the response\nStep 4: Format the output")

	fmt.Println("\n=== Demo Complete ===")
}
