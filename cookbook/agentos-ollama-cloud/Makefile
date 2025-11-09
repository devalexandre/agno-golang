.PHONY: help run build demo clean test install

# Default target
help:
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "🚀 AgentOS Ollama Cloud - Makefile Commands"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "Available commands:"
	@echo "  make run          - Run AgentOS server"
	@echo "  make build        - Build binary"
	@echo "  make demo         - Run demo script (server must be running)"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make install      - Install dependencies"
	@echo ""
	@echo "Setup:"
	@echo "  1. Copy .env.example to .env"
	@echo "  2. Add your OLLAMA_CLOUD_API_KEY"
	@echo "  3. Run 'make run'"
	@echo ""

# Run the server
run:
	@echo "🚀 Starting AgentOS with Ollama Cloud..."
	@if [ ! -f .env ]; then \
		echo "⚠️  .env file not found. Creating from .env.example..."; \
		cp .env.example .env; \
		echo "📝 Please edit .env and add your OLLAMA_CLOUD_API_KEY"; \
		exit 1; \
	fi
	@export $$(cat .env | xargs) && go run main.go

# Build binary
build:
	@echo "🔨 Building AgentOS binary..."
	@go build -o agentos-ollama-cloud main.go
	@echo "✅ Built: agentos-ollama-cloud"

# Run demo script
demo:
	@echo "🎬 Running demo script..."
	@./demo.sh

# Run tests
test:
	@echo "🧪 Running tests..."
	@go test -v

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -f agentos-ollama-cloud
	@rm -f *.log
	@echo "✅ Cleaned"

# Install dependencies
install:
	@echo "📦 Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies installed"

# Development mode with auto-reload (requires air)
dev:
	@if ! command -v air > /dev/null; then \
		echo "📦 Installing air for auto-reload..."; \
		go install github.com/cosmtrek/air@latest; \
	fi
	@export $$(cat .env | xargs) && air

# Docker build
docker-build:
	@echo "🐳 Building Docker image..."
	@docker build -t agentos-ollama-cloud .
	@echo "✅ Docker image built: agentos-ollama-cloud"

# Docker run
docker-run:
	@echo "🐳 Running Docker container..."
	@docker run -p 8080:8080 --env-file .env agentos-ollama-cloud
