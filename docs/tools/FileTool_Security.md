# FileTool - Security System

## 🛡️ Overview

The **FileTool** implements a security system that **disables write operations by default**. This is a protection measure to prevent accidental modifications to the file system.

## 🔒 Default Behavior

### Allowed Operations (Always)
- ✅ **ReadFile**: Reading files
- ✅ **GetFileInfo**: File/directory information  
- ✅ **ListDirectory**: Directory listing
- ✅ **SearchFiles**: File search

### Restricted Operations (Disabled by default)
- ❌ **WriteFile**: Writing/creating files
- ❌ **CreateDirectory**: Creating directories
- ❌ **DeleteFile**: Deleting files/directories

## 🔧 How to Use

### 1. Default FileTool (Read-Only)
```go
import "github.com/devalexandre/agno-golang/agno/tools"

// Create FileTool with write disabled
fileTool := tools.NewFileTool()
fmt.Println(fileTool.IsWriteEnabled()) // false

// Read operations work normally
content, err := fileTool.ReadFile(ReadFileParams{Path: "/etc/hostname"})

// Write operations fail with security message
result, err := fileTool.WriteFile(WriteFileParams{
    Path: "/tmp/test.txt", 
    Content: "test"
})
// Returns: "write operations are disabled for security"
```

### 2. Enabling Write Manually
```go
// Create default FileTool
fileTool := tools.NewFileTool()

// Enable write when necessary
fileTool.EnableWrite()
fmt.Println(fileTool.IsWriteEnabled()) // true

// Now write operations work
result, err := fileTool.WriteFile(WriteFileParams{
    Path: "/tmp/test.txt", 
    Content: "test"
})

// Disable again if necessary
fileTool.DisableWrite()
```

### 3. FileTool with Pre-enabled Write
```go
// Create FileTool with write already enabled
fileTool := tools.NewFileToolWithWrite()
fmt.Println(fileTool.IsWriteEnabled()) // true

// All operations work immediately
result, err := fileTool.WriteFile(WriteFileParams{
    Path: "/tmp/test.txt", 
    Content: "test"
})
```

## 📊 Control Methods

### Status Check
```go
enabled := fileTool.IsWriteEnabled() // bool
```

### Enable/Disable
```go
fileTool.EnableWrite()   // Enable write
fileTool.DisableWrite()  // Disable write
```

### Constructors
```go
// Write disabled (default)
fileTool := tools.NewFileTool()

// Write enabled
fileTool := tools.NewFileToolWithWrite()
```

## 🛠️ Usage with Agents

### Read-Only Agent (Safe)
```go
agent := agent.NewAgent(model)
agent.AddTool(tools.NewFileTool()) // Read-only

// Agent can read files but not modify
agent.PrintResponse("Read the contents of /etc/hostname", false, true)
```

### Agent with Write (Caution)
```go
agent := agent.NewAgent(model)
agent.AddTool(tools.NewFileToolWithWrite()) // Write enabled

// Agent can modify files
agent.PrintResponse("Create a file called test.txt with 'Hello World'", false, true)
```

### Dynamic Control
```go
fileTool := tools.NewFileTool()
agent.AddTool(fileTool)

// Enable write only when necessary
fileTool.EnableWrite()
agent.PrintResponse("Create a backup file", false, true)

// Desabilitar novamente
fileTool.DisableWrite()
```

## ⚠️ Error Messages

When write operations are attempted with write disabled:

```json
{
  "path": "/tmp/test.txt",
  "success": false,
  "error": "write operations are disabled for security. Use EnableWrite() to enable them",
  "operation": "WriteFile"
}
```

## 🎯 Use Cases

### Development/Testing (Safe)
```go
// For development, use default FileTool
fileTool := tools.NewFileTool()
// Agent can analyze files but not modify anything
```

### Production with Control
```go
// In production, enable write only when necessary
fileTool := tools.NewFileTool()

if allowFileWrites {
    fileTool.EnableWrite()
}
```

### Automation/Scripts
```go
// For automation scripts that need to modify files
fileTool := tools.NewFileToolWithWrite()
// All operations enabled from the start
```

## 🏆 Benefits

1. **Security by Default**: Prevents accidental modifications
2. **Granular Control**: Enable write only when necessary
3. **Audit**: Clear when write is enabled or not
4. **Flexibility**: Multiple ways to control behavior
5. **Transparency**: Clear messages about restrictions

---

**💡 Tip**: For maximum security in production, always use `NewFileTool()` and enable write only temporarily when necessary.
