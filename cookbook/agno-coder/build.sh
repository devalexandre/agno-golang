#!/bin/bash
# Build script for agno-coder

cd "$(dirname "$0")"
echo "Building agno-coder..."
go build -o agno-coder main.go validation.go

if [ $? -eq 0 ]; then
    echo "✓ Build successful!"
    echo ""
    echo "Usage examples:"
    echo "  ./agno-coder --analyze main.go"
    echo "  ./agno-coder --prompt 'Your instruction' --path ./path"
else
    echo "✗ Build failed"
    exit 1
fi
