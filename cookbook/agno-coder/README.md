# 🤖 Agno Coder

A powerful CLI tool for code analysis, planning, and execution using AI-powered workflows with Qwen Coder.

## ✨ Features

- 🔍 **Smart Code Analysis** - Analyze Go code with detailed feedback on architecture, security, performance, and maintainability
- 🎯 **Custom Prompts** - Pass custom instructions for any coding task
- 📁 **File & Folder Support** - Analyze individual files or entire directories
- ✅ **Environment Validation** - Automatic dependency checking before execution
- 🔄 **Multi-Step Workflow** - Analysis → Planning → Execution with memory
- 🛠️ **Tool Integration** - Built-in file and shell tools for code manipulation
- 📊 **Formatted Output** - Clean, readable markdown output (no JSON!)

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/devalexandre/agno-golang
cd agno-golang/cookbook/agno-coder

# Build
go build -o agno-coder main.go validation.go

# Or run directly
go run main.go validation.go --help
```

### Prerequisites

**Required:**
- Go 1.21+
- OLLAMA_API_KEY environment variable

**Optional:**
- Git (for version control operations)
- Go tools (for compilation, testing)

### Environment Setup

```bash
# Set your Ollama API key
export OLLAMA_API_KEY=your_api_key_here
```

## 📖 Usage

### Basic Commands

```bash
# Analyze a file
agno-coder --analyze path/to/file.go

# Implement a feature
agno-coder --implement "Add error handling to main function"

# General task
agno-coder --task "Refactor authentication module"
```

### Custom Prompts (New! 🎉)

Pass custom instructions for any task:

```bash
# Custom prompt without path (general task)
agno-coder --prompt "Explain how dependency injection works in Go"

# Custom prompt with file
agno-coder --prompt "Add comprehensive error handling" --path main.go

# Custom prompt with folder
agno-coder --prompt "Review code quality and suggest improvements" --path ./api

# Complex refactoring
agno-coder --prompt "Refactor to use interfaces and add unit tests" --path ./services
```

## 💡 Use Cases

### 1. Code Review

```bash
agno-coder --prompt "Review this code for security issues and best practices" --path ./handlers
```

### 2. Performance Optimization

```bash
agno-coder --prompt "Identify performance bottlenecks and suggest optimizations" --path ./database
```

### 3. Refactoring

```bash
agno-coder --prompt "Suggest refactoring opportunities to improve maintainability" --path ./legacy-code
```

### 4. Documentation

```bash
agno-coder --prompt "Generate comprehensive documentation with examples" --path ./pkg/client
```

### 5. Security Audit

```bash
agno-coder --prompt "Find potential security vulnerabilities (SQL injection, XSS, etc)" --path ./api
```

### 6. Add Features

```bash
agno-coder --prompt "Add structured logging using logrus" --path ./cmd
```

## 🎓 Best Practices

### Be Specific in Prompts

❌ **Bad**: `--prompt "improve code"`

✅ **Good**: `--prompt "Improve error handling by adding context and wrapping errors using fmt.Errorf"`

### Use Appropriate Scope

❌ **Bad**: `--path .` (too broad)

✅ **Good**: `--path ./api/handlers` (focused scope)

### Combine Analysis with Action

```bash
# First analyze
agno-coder --analyze main.go

# Then refactor based on analysis
agno-coder --prompt "Refactor based on previous analysis" --path main.go
```

## 🔧 How It Works

The CLI uses a three-agent workflow:

1. **CodeAnalyzer** - Analyzes code structure, patterns, and issues
2. **CodePlanner** - Creates executable implementation plans
3. **CodeExecutor** - Executes the plan using available tools

Each agent has access to:
- **FileTool** - Read/write files, list directories, search code
- **ShellTool** - Execute system commands, get system info

## 🎯 Command Reference

| Flag | Description | Example |
|------|-------------|---------|
| `--analyze` | Analyze a file | `--analyze main.go` |
| `--implement` | Implement a feature | `--implement "Add logging"` |
| `--task` | General task | `--task "Refactor module"` |
| `--prompt` | Custom instruction | `--prompt "Your instruction"` |
| `--path` | File or folder path | `--path ./api` |

## 🛡️ Environment Validation

The CLI automatically validates your environment before running:

**Required Checks:**
- ✅ OLLAMA_API_KEY configured

**Optional Checks:**
- ⚠️ Git installed
- ⚠️ Go installed

Example output:
```
Checking environment...
✓ OLLAMA_API_KEY: configured
✓ Git: git version 2.39.2
✓ Go: go version go1.21.0 linux/amd64

Initializing models... ✓ Ready
```

## 📊 Examples

### Example 1: Add Error Handling

```bash
agno-coder --prompt "Add comprehensive error handling with context" --path ./api/handlers/user.go
```

### Example 2: Security Review

```bash
agno-coder --prompt "Perform security audit and identify vulnerabilities" --path ./auth
```

### Example 3: Generate Tests

```bash
agno-coder --prompt "Generate unit tests with >80% coverage" --path ./services/payment.go
```

### Example 4: Refactor for Performance

```bash
agno-coder --prompt "Optimize database queries and add connection pooling" --path ./repository
```

## 🐛 Troubleshooting

### Error: OLLAMA_API_KEY not set

```bash
export OLLAMA_API_KEY=your_key_here
```

### Error: Path not found

Make sure the path exists and is relative to your current directory.

### Tool Call Errors

If you see "unknown action" errors, ensure you're using the latest version:

```bash
go build -o agno-coder main.go validation.go
```

## 🔄 Recent Updates

### v2.0 (Phase 2 Complete)
- ✅ Custom prompt support with `--prompt` flag
- ✅ File and folder analysis with `--path` flag
- ✅ Environment validation
- ✅ Improved error messages
- ✅ Fixed JSON output issue (clean text output)

### v1.0 (Phase 1)
- ✅ Basic analyze/implement/task commands
- ✅ Multi-agent workflow
- ✅ Tool integration (FileTool, ShellTool)
- ✅ Memory and context management

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📝 License

This project is part of the Agno framework.

## 🔗 Links

- [Agno Framework](https://github.com/devalexandre/agno-golang)
- [Documentation](https://docs.agno.com)
- [Issues](https://github.com/devalexandre/agno-golang/issues)

## 💬 Support

For questions and support, please open an issue on GitHub.

---

Made with ❤️ using [Agno Framework](https://github.com/devalexandre/agno-golang) and Qwen Coder
