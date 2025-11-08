#!/bin/bash

# Blog Post Generator - Quick Start Script
# This script generates sample blog posts to demonstrate the workflow

echo "=== Blog Post Generator - Demo ==="
echo ""

# Check if OLLAMA_API_KEY is set
if [ -z "$OLLAMA_API_KEY" ]; then
    echo "❌ Error: OLLAMA_API_KEY environment variable is not set"
    echo ""
    echo "Please set it with:"
    echo "  export OLLAMA_API_KEY='your-api-key-here'"
    exit 1
fi

echo "✅ OLLAMA_API_KEY is set"
echo ""

# Sample topics
topics=(
    "Introduction to Go Programming for Beginners"
    "Building Microservices with Go"
    "AI Agent Orchestration Patterns"
)

echo "📚 Generating ${#topics[@]} sample blog posts..."
echo ""

for topic in "${topics[@]}"; do
    echo "📝 Generating: $topic"
    go run main.go "$topic"
    echo ""
    echo "---"
    echo ""
    sleep 2
done

echo "✨ Demo completed!"
echo "📂 Check the blog_posts/ directory for generated content"
