#!/bin/bash

# AgentOS Ollama Cloud Demo Script
# This script demonstrates the key features of AgentOS

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

BASE_URL="http://localhost:8080"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🚀 AgentOS Ollama Cloud - Demo Script"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Wait for server to be ready
echo -e "${BLUE}⏳ Waiting for AgentOS to be ready...${NC}"
until curl -s $BASE_URL > /dev/null 2>&1; do
    echo -n "."
    sleep 1
done
echo -e "${GREEN}✓ Server is ready!${NC}"
echo ""

# 1. List agents
echo -e "${YELLOW}━━━ 1. List Available Agents ━━━${NC}"
echo "curl $BASE_URL/agents | jq"
echo ""
AGENTS=$(curl -s $BASE_URL/agents | jq -r '.[].name')
echo -e "${GREEN}Available agents:${NC}"
echo "$AGENTS" | while read agent; do echo "  • $agent"; done
echo ""

# Get first agent ID for testing
AGENT_ID=$(curl -s $BASE_URL/agents | jq -r '.[0].id')
echo -e "${GREEN}Using agent: $AGENT_ID${NC}"
echo ""

# 2. Simple agent run (non-streaming)
echo -e "${YELLOW}━━━ 2. Execute Agent (Non-streaming) ━━━${NC}"
echo "curl -X POST $BASE_URL/agents/$AGENT_ID/runs \\"
echo "  -F 'message=Tell me a short joke about programming' \\"
echo "  -F 'stream=false'"
echo ""
RESPONSE=$(curl -s -X POST $BASE_URL/agents/$AGENT_ID/runs \
  -F 'message=Tell me a short joke about programming' \
  -F 'stream=false')
echo -e "${GREEN}Response:${NC}"
echo "$RESPONSE" | jq
echo ""

# 3. Get run ID from response
RUN_ID=$(echo "$RESPONSE" | jq -r '.run_id // empty')
SESSION_ID=$(echo "$RESPONSE" | jq -r '.session_id // empty')

if [ -n "$SESSION_ID" ]; then
    # 4. List sessions
    echo -e "${YELLOW}━━━ 3. List Sessions ━━━${NC}"
    echo "curl $BASE_URL/sessions | jq"
    echo ""
    curl -s $BASE_URL/sessions | jq
    echo ""

    # 5. Get session details
    echo -e "${YELLOW}━━━ 4. Get Session Details ━━━${NC}"
    echo "curl $BASE_URL/sessions/$SESSION_ID | jq"
    echo ""
    curl -s $BASE_URL/sessions/$SESSION_ID | jq
    echo ""
fi

# 6. Agent run with streaming
echo -e "${YELLOW}━━━ 5. Execute Agent (Streaming) ━━━${NC}"
echo "curl -N -X POST $BASE_URL/agents/$AGENT_ID/runs \\"
echo "  -F 'message=Count from 1 to 3' \\"
echo "  -F 'stream=true'"
echo ""
echo -e "${GREEN}Streaming response:${NC}"
curl -N -X POST $BASE_URL/agents/$AGENT_ID/runs \
  -F 'message=Count from 1 to 3' \
  -F 'stream=true' 2>/dev/null | head -20
echo ""
echo ""

# 7. Test Researcher agent (if available)
RESEARCHER_ID=$(curl -s $BASE_URL/agents | jq -r '.[] | select(.name=="Researcher") | .id')
if [ -n "$RESEARCHER_ID" ]; then
    echo -e "${YELLOW}━━━ 6. Test Researcher Agent (DuckDuckGo Search) ━━━${NC}"
    echo "curl -X POST $BASE_URL/agents/$RESEARCHER_ID/runs \\"
    echo "  -F 'message=What is Go programming language?' \\"
    echo "  -F 'stream=false'"
    echo ""
    RESEARCH_RESPONSE=$(curl -s -X POST $BASE_URL/agents/$RESEARCHER_ID/runs \
      -F 'message=What is Go programming language?' \
      -F 'stream=false')
    echo -e "${GREEN}Research result:${NC}"
    echo "$RESEARCH_RESPONSE" | jq '.content' -r | head -20
    echo "..."
    echo ""
fi

# 8. Test file upload (if we have a test file)
echo -e "${YELLOW}━━━ 7. Test File Upload ━━━${NC}"
# Create a test file
echo "This is a test document for AgentOS demo." > /tmp/test_document.txt
echo "curl -X POST $BASE_URL/agents/$AGENT_ID/runs \\"
echo "  -F 'message=Summarize this document' \\"
echo "  -F 'files=@/tmp/test_document.txt' \\"
echo "  -F 'stream=false'"
echo ""
FILE_RESPONSE=$(curl -s -X POST $BASE_URL/agents/$AGENT_ID/runs \
  -F 'message=Summarize this document' \
  -F 'files=@/tmp/test_document.txt' \
  -F 'stream=false')
echo -e "${GREEN}File upload result:${NC}"
echo "$FILE_RESPONSE" | jq
rm /tmp/test_document.txt
echo ""

# 9. Get AgentOS config
echo -e "${YELLOW}━━━ 8. Get AgentOS Configuration ━━━${NC}"
echo "curl $BASE_URL/config | jq"
echo ""
curl -s $BASE_URL/config | jq
echo ""

# 10. Get available models
echo -e "${YELLOW}━━━ 9. Get Available Models ━━━${NC}"
echo "curl $BASE_URL/models | jq"
echo ""
curl -s $BASE_URL/models | jq
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ Demo completed successfully!${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${BLUE}💡 Next steps:${NC}"
echo "  • Try different agents and tools"
echo "  • Upload images, audio, or other files"
echo "  • Test WebSocket connection for workflows"
echo "  • Explore continue run for human-in-the-loop"
echo ""
echo -e "${BLUE}📚 Documentation:${NC} See README.md for more examples"
echo ""
