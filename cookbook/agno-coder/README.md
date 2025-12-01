# 🤖 Agno Coder CLI

A powerful AI-powered CLI for code analysis, planning, and execution. Simply describe what you want to do in natural language, and Agno Coder will find the files, analyze them, and execute the task.

## ✨ Features

- **Natural Language Interface**: Just describe what you want in plain text
- **Smart File Discovery**: Automatically finds and reads relevant files
- **Code Analysis**: Security reviews, best practices, performance analysis
- **Implementation Planning**: Creates detailed step-by-step plans
- **Auto-Execution**: Implements changes with validation and debugging loops

## 🚀 Quick Start

### Build

```bash
cd cookbook/agno-coder
./build.sh
```

Or manually:

```bash
go build -o agno-coder .
```

### Usage

```bash
agno-coder "<your prompt>"
```

That's it! Just describe what you want to do, including any file paths or directories.

## 📝 Examples

### Code Review

```bash
# Review a specific file for security issues
agno-coder "Review this code for security issues and best practices cookbook/agno-coder/main.go"

# Analyze all Go files in a directory
agno-coder "Analyze all Go files in ./agno/tools for potential improvements"
```

### Code Modifications

```bash
# Add error handling
agno-coder "Add error handling to all functions in ./cmd"

# Refactor code
agno-coder "Refactor the authentication module in auth/ to use interfaces"

# Add new features
agno-coder "Create a new REST endpoint for user management in api/handlers/"
```

### Code Search

```bash
# Find patterns
agno-coder "Find all TODO comments in the project and list them"

# Search for specific code
agno-coder "Find all places where we use deprecated functions"
```

### Documentation

```bash
# Generate docs
agno-coder "Add documentation comments to all exported functions in ./pkg"

# Create README
agno-coder "Create a README.md for the project in ./myproject"
```

## 🔧 How It Works

Agno Coder uses a multi-agent workflow:

1. **CodeAnalyzer**: Parses your request, finds relevant files using system tools (find, grep, cat, ls), reads file contents, and provides structured analysis.

2. **CodePlanner**: Creates a detailed implementation plan based on the analysis.

3. **CodeExecutor**: Implements the changes step-by-step.

4. **CodeValidator**: Verifies the implementation works correctly.

5. **CodeDebugger**: If validation fails, analyzes errors and provides fix strategies.

The workflow includes an automatic retry loop that continues until the implementation is successful or max iterations are reached.

## 🛠️ Available Tools

The agents have access to:

### FileTool
- `ReadFile`: Read file contents
- `WriteFile`: Write/modify files
- `ListDirectory`: List directory contents
- `SearchFiles`: Search for files by pattern
- `CreateDirectory`: Create directories
- `DeleteFile`: Delete files/directories

### ShellTool
- `Execute`: Run shell commands (find, grep, cat, ls, etc.)
- `ListFiles`: List files in current directory
- `GetCurrentDirectory`: Get current working directory
- `SystemInfo`: Get system information

## ⚙️ Configuration

The CLI uses OpenRouter API by default. Set your API key:

```bash
export OPENROUTER_API_KEY="your-api-key"
```

Or modify the model configuration in `main.go` to use a different provider.

## 📋 Requirements

- Go 1.21+
- OpenRouter API key (or configure another LLM provider)

## 🔒 Security Note

The CLI has write access enabled by default to allow code modifications. Use with caution and always review changes before committing.

## 📄 License

MIT License
