# Agno Workflow V2

A powerful and flexible workflow orchestration system for Go, inspired by Python's workflow patterns. This system allows you to build complex workflows with sequential, parallel, conditional, and loop execution patterns.

## Features

- **Sequential Execution**: Execute steps one after another
- **Parallel Execution**: Run multiple steps concurrently
- **Conditional Logic**: Branch execution based on conditions
- **Loop Support**: Iterate over steps with various loop conditions
- **Router Pattern**: Route to different paths based on input
- **Event System**: Monitor workflow execution with events
- **Storage Support**: Persist workflow sessions
- **Metrics Collection**: Track execution metrics and performance
- **Agent Integration**: Seamlessly integrate with Agno agents and teams

## Installation

```bash
go get github.com/devalexandre/agno-golang/agno/workflow/v2
```

## Quick Start

### Basic Sequential Workflow

```go
package main

import (
    "context"
    "fmt"
    "strings"
    
    v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
    // Define step functions
    step1 := func(input *v2.StepInput) (*v2.StepOutput, error) {
        message := input.GetMessageAsString()
        return &v2.StepOutput{
            Content:  strings.ToUpper(message),
            StepName: "step1",
        }, nil
    }
    
    step2 := func(input *v2.StepInput) (*v2.StepOutput, error) {
        // Access previous step content
        previous := fmt.Sprintf("%v", input.PreviousStepContent)
        return &v2.StepOutput{
            Content:  fmt.Sprintf("Processed: %s", previous),
            StepName: "step2",
        }, nil
    }
    
    // Create workflow
    workflow := v2.NewWorkflow(
        v2.WithWorkflowName("My Workflow"),
        v2.WithWorkflowSteps([]interface{}{step1, step2}),
    )
    
    // Run workflow
    ctx := context.Background()
    response, err := workflow.Run(ctx, "hello world")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    // Result will be: "Processed: HELLO WORLD"
    fmt.Printf("Result: %v\n", response.Content)
    fmt.Printf("Status: %s\n", response.Status)
}
```

## Workflow Components

### 1. Condition

Execute different paths based on conditions:

```go
condition := v2.NewCondition(
    v2.WithConditionName("validation_check"),
    v2.WithIf(func(input *v2.StepInput) bool {
        return len(input.GetMessageAsString()) > 5
    }),
    v2.WithThen(func(input *v2.StepInput) (*v2.StepOutput, error) {
        return &v2.StepOutput{Content: "Long message"}, nil
    }),
    v2.WithElse(func(input *v2.StepInput) (*v2.StepOutput, error) {
        return &v2.StepOutput{Content: "Short message"}, nil
    }),
)
```

### 2. Parallel

Execute multiple steps concurrently:

```go
parallel := v2.NewParallel(
    v2.WithParallelName("concurrent_tasks"),
    v2.WithParallelSteps(task1, task2, task3),
    v2.WithMaxConcurrency(2),
    v2.WithCombineOutputs(true),
)
```

### 3. Loop

Iterate over steps with various conditions:

```go
// Loop N times
loopN := v2.NewLoop(
    v2.WithLoopName("repeat_task"),
    v2.WithLoopSteps(task),
    v2.WithLoopCondition(v2.ForN(5)),
    v2.WithMaxIterations(5), // Safety limit
)

// Loop while condition is true
loopWhile := v2.NewLoop(
    v2.WithLoopName("conditional_loop"),
    v2.WithLoopSteps(task),
    v2.WithLoopCondition(v2.While(func(i int, lastOutput *v2.StepOutput) bool {
        // Continue while i < 10 and content is not "stop"
        if lastOutput != nil {
            return i < 10 && lastOutput.Content != "stop"
        }
        return i < 10
    })),
    v2.WithMaxIterations(20), // Safety limit to prevent infinite loops
)
```

### 4. Router

Route to different paths based on input:

```go
router := v2.NewRouter(
    v2.WithRouterName("request_router"),
    v2.WithRouteFunc(func(input *v2.StepInput) string {
        message := input.GetMessageAsString()
        if strings.Contains(message, "error") {
            return "error"
        }
        return "success"
    }),
    v2.WithRoute("error", errorHandler),
    v2.WithRoute("success", successHandler),
    v2.WithDefaultRoute(defaultHandler),
)
```

## Agent and Team Integration

Integrate Agno agents and teams into your workflows:

```go
import (
    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/team"
    "github.com/devalexandre/agno-golang/agno/tools"
)

// Create agents
webAgent := agent.NewAgent(agent.AgentConfig{
    Name:  "Web Agent",
    Model: openAIModel, // Your OpenAI model instance
    Tools: []toolkit.Tool{
        tools.NewDuckDuckGoTools(),
    },
    Role: "Search the web for information",
})

dataAgent := agent.NewAgent(agent.AgentConfig{
    Name:  "Data Agent",
    Model: openAIModel,
    Role:  "Analyze and process data",
})

// Create team
researchTeam := team.NewTeam(
    team.WithName("Research Team"),
    team.WithMembers([]*agent.Agent{webAgent, dataAgent}),
)

// Convert to workflow steps
agentStep := func(input *v2.StepInput) (*v2.StepOutput, error) {
    ctx := context.Background()
    result, err := webAgent.Run(ctx, input.GetMessageAsString())
    if err != nil {
        return nil, err
    }
    return &v2.StepOutput{
        Content:  result,
        StepName: "web_agent",
    }, nil
}

teamStep := func(input *v2.StepInput) (*v2.StepOutput, error) {
    ctx := context.Background()
    result, err := researchTeam.Run(ctx, input.GetMessageAsString())
    if err != nil {
        return nil, err
    }
    return &v2.StepOutput{
        Content:  result,
        StepName: "research_team",
    }, nil
}

// Use in workflow
workflow := v2.NewWorkflow(
    v2.WithWorkflowName("Research Workflow"),
    v2.WithWorkflowSteps([]interface{}{
        agentStep,
        teamStep,
    }),
)
```

## Storage and Sessions

Persist workflow sessions using SQLite storage:

```go
// Create storage
storage, err := storage.NewSQLiteStorage(
    storage.WithTableName("workflow_sessions"),
    storage.WithDBFile("workflow.db"),
)
if err != nil {
    panic(err)
}
defer storage.Close()

// Create workflow with storage
workflow := v2.NewWorkflow(
    v2.WithWorkflowName("Persistent Workflow"),
    v2.WithStorage(storage),
    v2.WithSessionID("session-001"),
    v2.WithWorkflowSteps([]interface{}{step1, step2}),
)
```

## Event Handling

Monitor workflow execution with events:

```go
workflow := v2.NewWorkflow(
    v2.WithWorkflowName("Event Workflow"),
    v2.WithWorkflowSteps([]interface{}{step1, step2}),
)

// Register event handlers
workflow.OnEvent(v2.StepStartedEvent, func(event *v2.WorkflowRunResponseEvent) {
    fmt.Printf("Step started: %v\n", event.Metadata["step_name"])
})

workflow.OnEvent(v2.StepCompletedEvent, func(event *v2.WorkflowRunResponseEvent) {
    fmt.Printf("Step completed: %v\n", event.Metadata["step_name"])
})

workflow.OnEvent(v2.WorkflowCompletedEvent, func(event *v2.WorkflowRunResponseEvent) {
    fmt.Println("Workflow completed!")
})
```

## Metrics and Monitoring

Track workflow execution metrics:

```go
response, err := workflow.Run(ctx, "input")
if err != nil {
    panic(err)
}

// Get metrics
metrics := workflow.GetMetrics()
fmt.Printf("Steps executed: %d\n", metrics.StepsExecuted)
fmt.Printf("Steps succeeded: %d\n", metrics.StepsSucceeded)
fmt.Printf("Steps failed: %d\n", metrics.StepsFailed)
fmt.Printf("Duration: %dms\n", metrics.DurationMs)

// Get specific step output
stepOutput := workflow.GetStepOutput("step_name")
if stepOutput != nil {
    fmt.Printf("Step content: %v\n", stepOutput.Content)
}
```

## Advanced Example: Blog Post Creation Workflow

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
    // Step 1: Prepare research query
    prepareResearchQuery := func(input *v2.StepInput) (*v2.StepOutput, error) {
        topic := input.GetMessageAsString()
        query := fmt.Sprintf("Latest information about %s including trends, statistics, and expert opinions", topic)
        return &v2.StepOutput{
            Content:  query,
            StepName: "prepare_query",
            Metadata: map[string]interface{}{
                "original_topic": topic,
            },
        }, nil
    }
    
    // Step 2: Research team gathers information
    researchTeamStep := func(input *v2.StepInput) (*v2.StepOutput, error) {
        query := input.GetMessageAsString()
        // In a real implementation, this would call your research team
        researchData := fmt.Sprintf("Research results for: %s", query)
        return &v2.StepOutput{
            Content:  researchData,
            StepName: "research",
        }, nil
    }
    
    // Step 3: Analyze research
    analyzeResearch := func(input *v2.StepInput) (*v2.StepOutput, error) {
        research := fmt.Sprintf("%v", input.PreviousStepContent)
        analysis := fmt.Sprintf("Key insights from research: %s", research)
        return &v2.StepOutput{
            Content:  analysis,
            StepName: "analysis",
        }, nil
    }
    
    // Step 4: Write blog post
    writerAgentStep := func(input *v2.StepInput) (*v2.StepOutput, error) {
        analysis := input.GetStepContent("analysis")
        topic := input.GetStepOutput("prepare_query").Metadata["original_topic"]
        
        blogPost := fmt.Sprintf("# Blog Post: %v\n\n%v", topic, analysis)
        return &v2.StepOutput{
            Content:  blogPost,
            StepName: "writer",
        }, nil
    }
    
    // Step 5: Format and prepare for publishing
    formatAndPublish := func(input *v2.StepInput) (*v2.StepOutput, error) {
        blogPost := fmt.Sprintf("%v", input.PreviousStepContent)
        formatted := fmt.Sprintf("FORMATTED FOR PUBLICATION:\n%s\n\n[Ready to publish]", blogPost)
        return &v2.StepOutput{
            Content:  formatted,
            StepName: "publisher",
        }, nil
    }
    
    // Create workflow
    contentWorkflow := v2.NewWorkflow(
        v2.WithWorkflowName("Blog Post Workflow"),
        v2.WithWorkflowDescription("Create blog posts from research"),
        v2.WithWorkflowSteps([]interface{}{
            prepareResearchQuery,
            researchTeamStep,
            analyzeResearch,
            writerAgentStep,
            formatAndPublish,
        }),
        v2.WithDebugMode(true),
    )
    
    // Run workflow
    ctx := context.Background()
    response, err := contentWorkflow.Run(ctx, "AI trends in 2024")
    if err != nil {
        log.Fatal(err)
    }
    
    // Display results
    fmt.Printf("Workflow Status: %s\n", response.Status)
    fmt.Printf("Final Output:\n%v\n", response.Content)
    
    // Get metrics
    metrics := contentWorkflow.GetMetrics()
    fmt.Printf("\nMetrics:\n")
    fmt.Printf("- Steps Executed: %d\n", metrics.StepsExecuted)
    fmt.Printf("- Duration: %dms\n", metrics.DurationMs)
}
```

## Step Input/Output Utilities

The `StepInput` type provides useful methods for accessing data:

```go
func myStep(input *v2.StepInput) (*v2.StepOutput, error) {
    // Get message as string
    message := input.GetMessageAsString()
    
    // Get previous step content
    previous := input.PreviousStepContent
    
    // Get specific step output
    stepData := input.GetStepOutput("previous_step_name")
    
    // Get all previous content
    allContent := input.GetAllPreviousContent()
    
    // Access metadata
    metadata := input.AdditionalData["key"]
    
    return &v2.StepOutput{
        Content: "processed",
        StepName: "my_step",
        Metadata: map[string]interface{}{
            "processed_at": time.Now(),
        },
    }, nil
}
```

## Configuration Options

### Workflow Options

- `WithWorkflowName(name string)` - Set workflow name
- `WithWorkflowDescription(desc string)` - Set workflow description
- `WithWorkflowSteps(steps []interface{})` - Set workflow steps
- `WithStorage(storage Storage)` - Enable session persistence
- `WithSessionID(id string)` - Set session ID
- `WithDebugMode(debug bool)` - Enable debug mode
- `WithStreaming(stream bool)` - Enable streaming mode
- `WithEventStorage(store bool)` - Store events

### Step Options

- `WithName(name string)` - Set step name
- `WithAgent(agent Agent)` - Use agent as executor
- `WithTeam(team Team)` - Use team as executor
- `WithExecutor(fn ExecutorFunc)` - Use function as executor
- `WithMaxRetries(n int)` - Set max retry attempts
- `WithTimeout(seconds int)` - Set step timeout
- `WithSkipOnFailure(skip bool)` - Skip on failure

## Error Handling

Workflows provide robust error handling:

```go
response, err := workflow.Run(ctx, input)
if err != nil {
    // Check error type
    switch err {
    case context.Canceled:
        fmt.Println("Workflow was cancelled")
    case context.DeadlineExceeded:
        fmt.Println("Workflow timed out")
    default:
        fmt.Printf("Workflow failed: %v\n", err)
    }
}

// Check workflow status
if response.Status == v2.RunStatusFailed {
    fmt.Printf("Workflow failed: %v\n", response.Metadata["error"])
}
```

## Testing

The workflow system includes comprehensive tests. Run them with:

```bash
go test ./agno/workflow/v2/... -v
```

## Complete Examples

For more complete examples, check the test file `workflow_test.go` which includes:

- Basic sequential workflows
- Conditional execution patterns
- Parallel processing examples
- Loop implementations
- Router patterns
- Complex multi-component workflows
- Event handling demonstrations
- Metrics collection examples

## License

This project is part of the Agno-Golang framework.