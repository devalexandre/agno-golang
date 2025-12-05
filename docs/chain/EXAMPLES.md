# ChainTool Examples

Practical examples demonstrating ChainTool features.

## Example 1: Basic ChainTool Pipeline

```go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, _ := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)

	// Step 1: Create tools
	step1 := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			return strings.ToUpper(input), nil
		},
		"Convert to uppercase",
	)

	step2 := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			return fmt.Sprintf("[%s]", input), nil
		},
		"Add brackets",
	)

	// Step 2: Create agent with ChainTool enabled
	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:           model,
		Tools:           []toolkit.Tool{step1, step2},
		EnableChainTool: true,
	})

	// Step 3: Run - chain executes automatically
	response, _ := agent.Run("hello")
	// Output: "[HELLO]"
}
```

## Example 2: Error Handling with RollbackToPrevious

```go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, _ := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)

	validate := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			return fmt.Sprintf("VALIDATED[%s]", input), nil
		},
		"Validate input",
	)

	optionalTransform := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			// Might fail
			if len(input) < 10 {
				return "", fmt.Errorf("too short")
			}
			return strings.ToUpper(input), nil
		},
		"Optional transform",
	)

	enrich := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			return fmt.Sprintf("ENRICHED{%s}", input), nil
		},
		"Enrich data",
	)

	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:           model,
		Tools:           []toolkit.Tool{validate, optionalTransform, enrich},
		EnableChainTool: true,
		ChainToolErrorConfig: &agent.ChainToolErrorConfig{
			Strategy:   agent.RollbackToPrevious,  // Skip failed tool
			MaxRetries: 1,
		},
	})

	// If optionalTransform fails, enrich uses validate's result
	response, _ := agent.Run("short")
	// Still succeeds even though optionalTransform failed
}
```

## Example 3: Caching Results

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, _ := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)

	expensiveOperation := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			time.Sleep(2 * time.Second) // Expensive
			return fmt.Sprintf("PROCESSED[%s]", input), nil
		},
		"Expensive processing",
	)

	// Create cache
	cache := agent.NewMemoryCache(5*time.Minute, 100)

	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:              model,
		Tools:              []toolkit.Tool{expensiveOperation},
		EnableChainTool:    true,
		ChainToolCache:     cache,  // Enable caching
	})

	// First run: 2 seconds
	agent.Run("data")

	// Second run: instant (cached)
	agent.Run("data")
}
```

## Example 4: Dynamic Tool Addition

```go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, _ := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)

	basicTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			return strings.ToUpper(input), nil
		},
		"Basic processing",
	)

	// Create agent with basic tool
	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:           model,
		Tools:           []toolkit.Tool{basicTool},
		EnableChainTool: true,
	})

	// Run with basic tool
	agent.Run("data")

	// Add advanced tool later
	advancedTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			return fmt.Sprintf("[%s]", input), nil
		},
		"Advanced processing",
	)

	agent.AddTool(advancedTool)

	// Run with both tools
	agent.Run("data")
}
```

## Example 5: Progressive Enhancement

```go
package main

func setupAgent(userLevel string) *agent.Agent {
	model, _ := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)

	baseTool := createBaseTool()

	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:           model,
		Tools:           []toolkit.Tool{baseTool},
		EnableChainTool: true,
	})

	// Add tools based on user level
	if userLevel == "premium" {
		agent.AddTool(createAdvancedTool())
		agent.AddTool(createAnalyticsTool())
	}

	if userLevel == "enterprise" {
		agent.AddTool(createCustomizationTool())
		agent.AddTool(createIntegrationTool())
	}

	return agent
}

func main() {
	basicAgent := setupAgent("free")       // 1 tool
	premiumAgent := setupAgent("premium")  // 3 tools
	enterpriseAgent := setupAgent("enterprise") // 5 tools
}
```

## Example 6: A/B Testing

```go
package main

import (
	"context"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, _ := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)

	// Algorithm A
	algorithmA := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			return fmt.Sprintf("RESULT_A[%s]", input), nil
		},
		"Processing algorithm A",
	)

	// Algorithm B
	algorithmB := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			return fmt.Sprintf("RESULT_B[%s]", input), nil
		},
		"Processing algorithm B",
	)

	// Create two agents
	var algorithm toolkit.Tool
	if experimentGroup == "A" {
		algorithm = algorithmA
	} else {
		algorithm = algorithmB
	}

	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:           model,
		Tools:           []toolkit.Tool{algorithm},
		EnableChainTool: true,
	})

	response, _ := agent.Run("data")
	// Compare results between A and B
}
```

## Example 7: Feature Flags

```go
package main

import (
	"context"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

type FeatureFlags struct {
	EnableAdvancedProcessing bool
	EnableAnalytics          bool
	EnableOptimization       bool
}

func main() {
	flags := FeatureFlags{
		EnableAdvancedProcessing: true,
		EnableAnalytics:          false,
		EnableOptimization:       true,
	}

	model, _ := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)

	baseTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			return fmt.Sprintf("PROCESSED[%s]", input), nil
		},
		"Base processing",
	)

	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:           model,
		Tools:           []toolkit.Tool{baseTool},
		EnableChainTool: true,
	})

	// Add tools based on feature flags
	if flags.EnableAdvancedProcessing {
		agent.AddTool(createAdvancedTool())
	}

	if flags.EnableAnalytics {
		agent.AddTool(createAnalyticsTool())
	}

	if flags.EnableOptimization {
		agent.AddTool(createOptimizationTool())
	}

	agent.Run("data")
}
```

## Example 8: Conditional Tool Addition

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/agent"
)

func setupAgent(dataType string) *agent.Agent {
	model, _ := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)

	baseTool := createBaseTool()

	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:           model,
		Tools:           []toolkit.Tool{baseTool},
		EnableChainTool: true,
	})

	// Add tools based on data type
	switch dataType {
	case "json":
		agent.AddTool(createJsonParserTool())
	case "xml":
		agent.AddTool(createXmlParserTool())
	case "csv":
		agent.AddTool(createCsvParserTool())
	case "image":
		agent.AddTool(createImageProcessorTool())
	}

	return agent
}

func main() {
	jsonAgent := setupAgent("json")
	xmlAgent := setupAgent("xml")
	imageAgent := setupAgent("image")
}
```

## Example 9: Tool Swapping

```go
package main

import (
	"context"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/agent"
)

func main() {
	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:           model,
		Tools:           []toolkit.Tool{createOldImplementationTool()},
		EnableChainTool: true,
	})

	// Use old implementation
	agent.Run("data")

	// Swap to new implementation
	agent.RemoveTool("Old Implementation")
	agent.AddTool(createNewImplementationTool())

	// Use new implementation
	agent.Run("data")
}
```

## Example 10: Error Recovery with Retries

```go
package main

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/devalexandre/agno-golang/agno/agent"
)

func main() {
	model, _ := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)

	unreliableTool := tools.NewToolFromFunction(
		func(ctx context.Context, input string) (string, error) {
			// 50% chance of failure
			if rand.Float32() < 0.5 {
				return "", fmt.Errorf("temporary failure")
			}
			return fmt.Sprintf("SUCCESS[%s]", input), nil
		},
		"Unreliable tool",
	)

	agent, _ := agent.NewAgent(agent.AgentConfig{
		Model:           model,
		Tools:           []toolkit.Tool{unreliableTool},
		EnableChainTool: true,
		ChainToolErrorConfig: &agent.ChainToolErrorConfig{
			Strategy:   agent.RollbackToStart,  // Retry from beginning
			MaxRetries: 3,  // Try up to 3 times
		},
	})

	// Will retry automatically on failure
	agent.Run("data")
}
```

---

**See Also:**
- [ChainTool Documentation](./README.md)
- [Dynamic Tools Documentation](./DYNAMIC_TOOLS.md)
- [Error Handling Strategies](./README.md#error-handling-strategies)
