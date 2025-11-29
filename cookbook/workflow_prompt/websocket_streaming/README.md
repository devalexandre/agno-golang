# WebSocket Streaming Example

This example demonstrates how to use **WebSocket streaming** in workflows for real-time event delivery.

## Concept

WebSocket Support allows you to:
1. Stream workflow events in real-time
2. Monitor workflow progress as it happens
3. Build real-time dashboards and UIs
4. Get immediate feedback on workflow execution

## How It Works

```
Workflow → Event → WebSocket Handler → Real-time Updates
```

1. Workflow emits events during execution
2. WebSocket handler receives events
3. Events are sent to connected clients
4. Real-time updates displayed to users

## Running the Example

```bash
go run main.go
```

## Configuration

```go
// Create a WebSocket handler
wsHandler := v2.NewDefaultWebSocketHandler(func(event *v2.WorkflowRunResponseEvent) error {
    // Handle event (send to client, log, etc.)
    fmt.Printf("Event: %s\n", event.Event)
    return nil
})

// Configure workflow with WebSocket
workflow := v2.NewWorkflow(
    v2.WithWebSocketHandler(wsHandler),
    // ... other options
)
```

## Available Handlers

### 1. DefaultWebSocketHandler
- Stores events in memory
- Calls a callback function for each event
- Good for testing and simple use cases

### 2. JSONWebSocketHandler
- Serializes events to JSON
- Sends to a writer function
- Good for actual WebSocket connections

## Custom Handler

You can implement your own `WebSocketHandler`:

```go
type WebSocketHandler interface {
    SendEvent(event *WorkflowRunResponseEvent) error
    Close() error
}
```

## Use Cases

- Real-time workflow monitoring
- Live dashboards
- Progress indicators
- Event logging and analytics
- Multi-user collaboration tools

## Benefits

- **Real-time Updates**: See workflow progress as it happens
- **Better UX**: Provide immediate feedback to users
- **Debugging**: Monitor workflow execution in detail
- **Scalability**: Handle multiple concurrent workflows
