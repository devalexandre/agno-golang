# Quick Start Guide - 24 Agno Go Tools

## Installation & Setup

### Prerequisites
- Go 1.20 or higher
- Git
- Basic understanding of Go packages

### Clone & Setup

```bash
# Navigate to project
cd /home/devalexandre/projects/devalexandre/agno-golang

# Verify Go installation
go version

# Build all tools
go build ./agno/tools
```

---

## Running Tests

### All Tests
```bash
go test ./agno/tools -v
```

### Phase-Specific Tests
```bash
# Phase 1 Tests
go test ./agno/tools -v -run "Phase1"

# Phase 2 Tests
go test ./agno/tools -v -run "Phase2"

# Phase 3 Tests
go test ./agno/tools -v -run "Phase3"
```

### Specific Tool Tests
```bash
# Docker
go test ./agno/tools -v -run "Docker"

# Kubernetes
go test ./agno/tools -v -run "Kubernetes"

# Message Queue
go test ./agno/tools -v -run "MessageQueue"

# Cache
go test ./agno/tools -v -run "Cache"

# Monitoring
go test ./agno/tools -v -run "Monitoring"
```

### With Coverage
```bash
go test ./agno/tools -cover
go test ./agno/tools -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Basic Usage Examples

### 1. Docker Container Manager

```go
package main

import (
    "fmt"
    "github.com/devalexandre/agno-golang/agno/tools"
)

func main() {
    docker := tools.NewDockerContainerManager()
    
    // Pull an image
    result, err := docker.PullImage(tools.PullImageParams{
        ImageName: "ubuntu:latest",
        Registry:  "docker.io",
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Pull result: %+v\n", result)
    
    // List containers
    result, err = docker.ListContainers(struct{}{})
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Containers: %+v\n", result)
}
```

### 2. Kubernetes Operations

```go
package main

import (
    "fmt"
    "github.com/devalexandre/agno-golang/agno/tools"
)

func main() {
    k8s := tools.NewKubernetesOperationsTool()
    
    // Apply manifest
    result, err := k8s.ApplyManifest(tools.ApplyManifestParams{
        Namespace: "default",
        Manifest:  "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test",
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Apply result: %+v\n", result)
    
    // Get pods
    result, err = k8s.GetPods(tools.GetPodsParams{
        Namespace: "default",
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Pods: %+v\n", result)
}
```

### 3. Cache Manager

```go
package main

import (
    "fmt"
    "github.com/devalexandre/agno-golang/agno/tools"
)

func main() {
    cache := tools.NewCacheManagerTool()
    
    // Set cache value
    result, err := cache.SetCache(tools.SetCacheParams{
        Key:   "user:123",
        Value: "John Doe",
        TTL:   3600,
        Tags:  []string{"users", "important"},
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Set result: %+v\n", result)
    
    // Get cache value
    result, err = cache.GetCache(tools.GetCacheParams{
        Key: "user:123",
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Get result: %+v\n", result)
    
    // Get cache stats
    result, err = cache.GetCacheStats(tools.GetStatsParams{
        Detailed: true,
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Cache stats: %+v\n", result)
}
```

### 4. Message Queue Manager

```go
package main

import (
    "fmt"
    "github.com/devalexandre/agno-golang/agno/tools"
)

func main() {
    queue := tools.NewMessageQueueManagerTool()
    
    // Create queue
    result, err := queue.CreateQueue(tools.CreateQueueParams{
        QueueName: "orders",
        QueueType: "Standard",
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Queue created: %+v\n", result)
    
    // Publish message
    result, err = queue.PublishMessage(tools.PublishMessageParams{
        QueueName:   "orders",
        MessageBody: `{"order_id": "123", "amount": 99.99}`,
        Attributes: map[string]string{
            "priority": "high",
        },
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Message published: %+v\n", result)
}
```

### 5. Monitoring & Alerts

```go
package main

import (
    "fmt"
    "github.com/devalexandre/agno-golang/agno/tools"
)

func main() {
    monitoring := tools.NewMonitoringAlertsTool()
    
    // Record metric
    result, err := monitoring.RecordMetric(tools.RecordMetricParams{
        MetricName: "cpu_usage",
        Value:      75.5,
        Unit:       "percent",
        Tags: map[string]string{
            "server": "prod-01",
        },
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Metric recorded: %+v\n", result)
    
    // Create alert
    result, err = monitoring.CreateAlert(tools.CreateAlertParams{
        AlertName:  "High CPU Usage",
        MetricName: "cpu_usage",
        Condition:  "above",
        Threshold:  80.0,
        Severity:   "critical",
        NotifyTo:   []string{"admin@example.com"},
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Alert created: %+v\n", result)
    
    // Get metrics
    result, err = monitoring.GetMetrics(tools.GetMetricsParams{
        MetricName: "cpu_usage",
        TimeRange:  60,
    })
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Metrics: %+v\n", result)
}
```

---

## File Structure

```
agno/tools/
├── docker_container_manager.go        # Docker tool
├── kubernetes_operations_tool.go      # Kubernetes tool
├── message_queue_manager_tool.go      # Message queue tool
├── cache_manager_tool.go              # Cache tool
├── monitoring_alerts_tool.go          # Monitoring tool
│
├── [10 more Phase 2 tools]
├── [9 Phase 1 tools]
│
└── Tests
    ├── phase1_tests.go
    ├── phase2_first_wave_test.go
    ├── phase2_second_wave_test.go
    └── phase3_tools_test.go
```

---

## Documentation Files

- **EXECUTIVE_SUMMARY.md** - Project overview and status
- **24_TOOLS_COMPLETE_GUIDE.md** - Complete guide for all tools
- **PHASE3_TOOLS_DOCUMENTATION.md** - Detailed Phase 3 documentation

---

## Common Commands

```bash
# Build
go build ./agno/tools

# Test all
go test ./agno/tools -v

# Test with coverage
go test ./agno/tools -cover

# Format code
go fmt ./agno/tools/...

# Check for errors
go vet ./agno/tools/...

# Get dependencies
go mod tidy

# List all tests
go test -list ./agno/tools
```

---

## Troubleshooting

### Build Errors
```bash
# Clean build
go clean ./agno/tools
go build ./agno/tools
```

### Test Failures
```bash
# Run specific test with verbose output
go test ./agno/tools -v -run TestName

# Show test output even if passed
go test ./agno/tools -v -run TestName
```

### Import Issues
```bash
# Update modules
go mod tidy

# Verify all dependencies
go mod verify
```

---

## Best Practices

1. **Always instantiate tools before use**
   ```go
   tool := tools.NewToolName()
   ```

2. **Check for errors**
   ```go
   result, err := tool.Method(params)
   if err != nil {
       // Handle error
   }
   ```

3. **Use structured parameters**
   ```go
   params := tools.ParamStruct{
       Field1: "value1",
       Field2: "value2",
   }
   ```

4. **Handle JSON responses**
   ```go
   result, _ := tool.Method(params)
   fmt.Printf("%+v\n", result) // Pretty print
   ```

---

## Performance Tips

- Cache frequently accessed data
- Use batch operations when available
- Monitor resource usage with the Monitoring tool
- Profile code with the Performance Profiler tool
- Use appropriate TTL values for cached data

---

## Next Steps

1. Read the complete guide: `24_TOOLS_COMPLETE_GUIDE.md`
2. Review Phase 3 documentation: `PHASE3_TOOLS_DOCUMENTATION.md`
3. Run the test suite: `go test ./agno/tools -v`
4. Integrate tools into your project
5. Customize as needed for your use case

---

## Support & Documentation

- **Code Examples**: Check test files for usage patterns
- **API Reference**: See method signatures in source files
- **Data Types**: Review type definitions in each tool file
- **Error Handling**: Check error returns and messages

---

**Happy coding with Agno Go Tools! 🚀**
