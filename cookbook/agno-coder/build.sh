#!/bin/bash
# Build script for agno-coder

cd "$(dirname "$0")"
echo "Building agno-coder..."
go build -o agno-coder main.go validation.go
#copy to /home/devalexandre/.local/bin
cp agno-coder /home/devalexandre/.local/bin/agno-coder

if [ $? -eq 0 ]; then
    echo "✓ Build successful!"
    echo "Copied to /home/devalexandre/.local/bin/agno-coder"
    echo ""
    echo "Usage examples:"
    echo "  ./agno-coder --analyze main.go"
    echo "  ./agno-coder --prompt 'Your instruction' --path ./path"
else
    echo "✗ Build failed"
    exit 1
fi
