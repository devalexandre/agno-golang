#!/bin/bash
# Test Ollama Cloud authentication

echo "Testing Ollama Cloud with curl..."
curl -s https://ollama.com/api/chat \
  -H "Authorization: Bearer $OLLAMA_API_KEY" \
  -d '{
    "model":"deepseek-v3.1:671b-cloud",
    "messages": [{
      "role": "user",
      "content": "Say hi in 5 words"
    }],
    "stream": false
  }' | jq -r '.message.content'

echo ""
echo "Testing agno-golang..."
cd "$(dirname "$0")"
go run ./cookbook/agents/ollama-cloud/main.go
