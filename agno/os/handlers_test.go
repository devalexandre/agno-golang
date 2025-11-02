package os

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/team"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// setupTestAgentOS creates a test AgentOS instance
func setupTestAgentOS() (*AgentOS, error) {
	// Create test agents
	testAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Name:    "test-agent",
		Role:    "Test Agent for validation",
	})
	if err != nil {
		return nil, err
	}

	// Create test teams
	testTeam := team.NewTeam(team.TeamConfig{
		Context: context.Background(),
		Name:    "test-team",
		Role:    "Test Team for validation",
	})

	// Create test workflows
	testWorkflow := &v2.Workflow{
		WorkflowID:  "test-workflow",
		Name:        "test-workflow",
		Description: "Test Workflow for validation",
	}

	options := AgentOSOptions{
		OSID:        "test-os",
		Name:        StringPtr("Test AgentOS"),
		Description: StringPtr("Test AgentOS for endpoint validation"),
		Version:     StringPtr("1.0.0-test"),
		Agents:      []*agent.Agent{testAgent},
		Teams:       []*team.Team{testTeam},
		Workflows:   []*v2.Workflow{testWorkflow},
		Settings: &AgentOSSettings{
			Port:       8080,
			Host:       "localhost",
			Debug:      true,
			EnableCORS: true,
		},
	}

	return NewAgentOS(options)
}

// TestHealthEndpoint tests the health check endpoint
func TestHealthEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

// TestPingEndpoint tests the ping endpoint for cloud discovery
func TestPingEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, true, response["pong"])
	assert.Equal(t, "test-os", response["os_id"])
	assert.Equal(t, "Test AgentOS", response["name"])
	assert.Equal(t, "1.0.0-test", response["version"])
	assert.Equal(t, float64(1), response["agents"])
	assert.Equal(t, float64(1), response["teams"])
}

// TestStatusEndpoint tests the status endpoint
func TestStatusEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "running", response["status"])
	assert.Equal(t, "test-os", response["os_id"])
	assert.Equal(t, "Test AgentOS", response["name"])
	assert.Equal(t, float64(8080), response["port"])
}

// TestInfoEndpoint tests the info endpoint
func TestInfoEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/info", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test-os", response["os_id"])
	assert.Equal(t, "golang", response["type"])
	assert.Equal(t, "go", response["language"])

	capabilities, ok := response["capabilities"].(map[string]interface{})
	require.True(t, ok, "capabilities should be a map")
	assert.Equal(t, true, capabilities["streaming"])
	assert.Equal(t, true, capabilities["websockets"])
}

// TestConfigEndpoint tests the config endpoint
func TestConfigEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test-os", response["os_id"])

	databases := response["databases"].([]interface{})
	assert.Contains(t, databases, "agno-storage")

	agents := response["agents"].([]interface{})
	assert.Len(t, agents, 1)

	agent := agents[0].(map[string]interface{})
	assert.Equal(t, "test-agent", agent["name"])
	assert.Equal(t, "agno-storage", agent["db_id"])
}

// TestListAgentsEndpoint tests the list agents endpoint
func TestListAgentsEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agents", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var agents []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &agents)
	require.NoError(t, err)

	assert.Len(t, agents, 1)
	assert.Equal(t, "test-agent", agents[0]["name"])
	assert.Contains(t, agents[0], "id")
	assert.Contains(t, agents[0], "model")
	assert.Equal(t, "agno-storage", agents[0]["db_id"])
}

// TestGetAgentEndpoint tests the get agent endpoint
func TestGetAgentEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test with agent name
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agents/test-agent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	agent := response["agent"].(map[string]interface{})
	assert.Equal(t, "test-agent", agent["name"])
	assert.Contains(t, agent, "id")
	assert.Contains(t, agent, "model")

	// Test with non-existent agent
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/agents/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestListTeamsEndpoint tests the list teams endpoint
func TestListTeamsEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/teams", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var teams []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &teams)
	require.NoError(t, err)

	assert.Len(t, teams, 1)
	assert.Equal(t, "test-team", teams[0]["name"])
	assert.Contains(t, teams[0], "id")
	assert.Contains(t, teams[0], "model")
	assert.Equal(t, "agno-storage", teams[0]["db_id"])
}

// TestGetTeamEndpoint tests the get team endpoint
func TestGetTeamEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test with team name
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/teams/test-team", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	team := response["team"].(map[string]interface{})
	assert.Equal(t, "test-team", team["name"])
	assert.Contains(t, team, "id")
	assert.Contains(t, team, "model")

	// Test with non-existent team
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/teams/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestListWorkflowsEndpoint tests the list workflows endpoint
func TestListWorkflowsEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/workflows", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var workflows []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &workflows)
	require.NoError(t, err)

	assert.Len(t, workflows, 1)
	assert.Equal(t, "test-workflow", workflows[0]["name"])
	assert.Equal(t, "test-workflow", workflows[0]["id"])
	assert.Equal(t, "Test Workflow for validation", workflows[0]["description"])
}

// TestGetWorkflowEndpoint tests the get workflow endpoint
func TestGetWorkflowEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test with workflow ID
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/workflows/test-workflow", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	workflow := response["workflow"].(map[string]interface{})
	assert.Equal(t, "test-workflow", workflow["name"])
	assert.Equal(t, "test-workflow", workflow["id"])

	// Test with non-existent workflow
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/workflows/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestModelsEndpoint tests the models endpoint
func TestModelsEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/models", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var models []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &models)
	require.NoError(t, err)

	assert.Greater(t, len(models), 0)
	// Should have default models
	found := false
	for _, model := range models {
		if model["id"] == "gpt-4" {
			found = true
			assert.Equal(t, "openai", model["provider"])
			break
		}
	}
	assert.True(t, found, "Should contain gpt-4 model")
}

// TestSessionsEndpoint tests session management endpoints
func TestSessionsEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test list sessions (initially empty)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sessions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var sessions []interface{}
	err = json.Unmarshal(w.Body.Bytes(), &sessions)
	require.NoError(t, err)
	assert.Len(t, sessions, 0)

	// Test create session
	sessionData := map[string]interface{}{
		"user_id":  "test-user",
		"agent_id": "test-agent",
		"metadata": map[string]interface{}{"test": true},
	}

	sessionJSON, _ := json.Marshal(sessionData)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/sessions", bytes.NewBuffer(sessionJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 201, w.Code)

	var createResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &createResponse)
	require.NoError(t, err)

	session := createResponse["session"].(map[string]interface{})
	sessionID := session["id"].(string)
	assert.NotEmpty(t, sessionID)
	assert.Equal(t, "test-user", session["user_id"])
	assert.Equal(t, "test-agent", session["agent_id"])

	// Test get session
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", fmt.Sprintf("/sessions/%s", sessionID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var getResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &getResponse)
	require.NoError(t, err)

	retrievedSession := getResponse["session"].(map[string]interface{})
	assert.Equal(t, sessionID, retrievedSession["id"])

	// Test delete session
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("/sessions/%s", sessionID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	// Verify session is deleted
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", fmt.Sprintf("/sessions/%s", sessionID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestKnowledgeContentEndpoints tests knowledge content management
func TestKnowledgeContentEndpoints(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test get knowledge config
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/knowledge/config?db_id=agno-storage", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var configResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &configResponse)
	require.NoError(t, err)
	assert.Contains(t, configResponse, "readers")

	// Test list content without db_id (no knowledge instances available)
	// Since the test agent doesn't have knowledge with ContentsDB, this should return 404
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/knowledge/content?db_id=agno-storage", nil)
	router.ServeHTTP(w, req)

	// Should return 404 because no agents have knowledge configured
	assert.Equal(t, 404, w.Code)

	var errorResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Contains(t, errorResponse, "error")

	// Test upload content with multipart form (should fail - no DB configured)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add form fields
	writer.WriteField("name", "Test Document")
	writer.WriteField("description", "Test document description")
	writer.WriteField("metadata", `{"type": "test"}`)
	writer.WriteField("text_content", "This is test content for the document")

	writer.Close()

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/knowledge/content?db_id=agno-storage", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	// Should return 404 because no agents have knowledge configured
	assert.Equal(t, 404, w.Code)

	// Test get content by ID (should fail - no DB)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/knowledge/content/test123?db_id=agno-storage", nil)
	router.ServeHTTP(w, req)

	// Should return 404 because no agents have knowledge configured
	assert.Equal(t, 404, w.Code)

	// Test delete content (should fail - no DB)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/knowledge/content/test123?db_id=agno-storage", nil)
	router.ServeHTTP(w, req)

	// Should return 404 because no agents have knowledge configured
	assert.Equal(t, 404, w.Code)
}

// TestKnowledgeVectorSearch tests vector search endpoint
func TestKnowledgeVectorSearch(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	searchRequest := map[string]interface{}{
		"query":   "test search query",
		"db_id":   "agno-storage",
		"filters": map[string]interface{}{},
		"meta": map[string]interface{}{
			"limit": 10,
			"page":  1,
		},
	}

	searchJSON, _ := json.Marshal(searchRequest)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/knowledge/search", bytes.NewBuffer(searchJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Check paginated response structure
	assert.Contains(t, response, "data")
	assert.Contains(t, response, "meta")

	// Check meta contains pagination info
	meta := response["meta"].(map[string]interface{})
	assert.Contains(t, meta, "page")
	assert.Contains(t, meta, "limit")
	assert.Contains(t, meta, "total_pages")
	assert.Contains(t, meta, "total_count")
}

// TestMemoryEndpoints tests memory management endpoints
func TestMemoryEndpoints(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test list memories (initially empty)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/memories?db_id=agno-storage", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var listResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &listResponse)
	require.NoError(t, err)

	data := listResponse["data"].([]interface{})
	assert.Len(t, data, 0)

	// Test create memory
	memoryData := map[string]interface{}{
		"memory":  "This is a test memory",
		"topics":  []string{"test", "validation"},
		"user_id": "test-user",
	}

	memoryJSON, _ := json.Marshal(memoryData)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/memories", bytes.NewBuffer(memoryJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var createResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &createResponse)
	require.NoError(t, err)

	memoryID := createResponse["memory_id"].(string)
	assert.NotEmpty(t, memoryID)
	assert.Equal(t, "This is a test memory", createResponse["memory"])

	// Test get memory by ID
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", fmt.Sprintf("/memories/%s", memoryID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var getResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &getResponse)
	require.NoError(t, err)
	assert.Equal(t, memoryID, getResponse["memory_id"])

	// Test list memories after creation
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/memories?user_id=test-user", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &listResponse)
	require.NoError(t, err)

	data = listResponse["data"].([]interface{})
	assert.Len(t, data, 1)

	// Test memory topics
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/memory_topics", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var topics []string
	err = json.Unmarshal(w.Body.Bytes(), &topics)
	require.NoError(t, err)
	assert.Contains(t, topics, "test")
	assert.Contains(t, topics, "validation")

	// Test user memory stats
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/user_memory_stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var statsResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &statsResponse)
	require.NoError(t, err)

	statsData := statsResponse["data"].([]interface{})
	assert.Len(t, statsData, 1)

	// Test delete memory
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("/memories/%s", memoryID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 204, w.Code)

	// Verify memory is deleted
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", fmt.Sprintf("/memories/%s", memoryID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestMetricsEndpoints tests metrics endpoints
func TestMetricsEndpoints(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test get metrics
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var metricsResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &metricsResponse)
	require.NoError(t, err)
	assert.Contains(t, metricsResponse, "metrics")

	// Test metrics refresh
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/metrics/refresh", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 204, w.Code)

	// Test with date filters
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/metrics?starting_date=2024-01-01&ending_date=2024-12-31", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

// TestEvalsEndpoints tests evaluation endpoints
func TestEvalsEndpoints(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test get eval runs (initially empty)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/eval-runs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var evalResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &evalResponse)
	require.NoError(t, err)

	data := evalResponse["data"].([]interface{})
	assert.Len(t, data, 0)

	// Test with query parameters
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/eval-runs?agent_id=test-agent&limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	// Test get non-existent eval run
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/eval-runs/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestCORSHeaders tests CORS headers are properly set
func TestCORSHeaders(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test OPTIONS request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "https://os.agno.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, 204, w.Code)
	assert.Equal(t, "https://os.agno.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")

	// Test with unallowed origin
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "https://malicious.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin")) // Fallback
}

// TestAgentOSHeaders tests AgentOS identification headers
func TestAgentOSHeaders(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test that AgentOS headers are set (except for /health)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "1.0.0-test", w.Header().Get("X-AgentOS-Version"))
	assert.Equal(t, "test-os", w.Header().Get("X-AgentOS-ID"))
	assert.Equal(t, "golang", w.Header().Get("X-AgentOS-Type"))
	assert.Contains(t, w.Header().Get("Server"), "AgentOS-Go/")

	// Test that headers are not set for /health
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Header().Get("X-AgentOS-Version"))
	assert.Empty(t, w.Header().Get("X-AgentOS-ID"))
	assert.Empty(t, w.Header().Get("X-AgentOS-Type"))
}

// TestAgentRunsEndpoint tests agent runs with streaming response
func TestAgentRunsEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test agent runs with JSON payload
	runData := map[string]interface{}{
		"message":    "Hello test agent",
		"session_id": "test-session",
	}

	runJSON, _ := json.Marshal(runData)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/agents/test-agent/runs", bytes.NewBuffer(runJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

	// Verify streaming response contains expected events
	body := w.Body.String()
	assert.Contains(t, body, "RunStarted")
	assert.Contains(t, body, "RunContent")
	assert.Contains(t, body, "RunCompleted")
	assert.Contains(t, body, "Hello test agent")

	// Test with form data
	formData := "message=Hello+form+agent&session_id=form-session"
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/agents/test-agent/runs", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	// Test with missing message
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/agents/test-agent/runs", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	// Test with non-existent agent
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/agents/non-existent/runs", bytes.NewBuffer(runJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestTeamRunsEndpoint tests team runs with streaming response
func TestTeamRunsEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test team runs
	runData := map[string]interface{}{
		"message":    "Hello test team",
		"session_id": "team-session",
	}

	runJSON, _ := json.Marshal(runData)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/teams/test-team/runs", bytes.NewBuffer(runJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

	// Verify streaming response contains expected events
	body := w.Body.String()
	assert.Contains(t, body, "RunStarted")
	assert.Contains(t, body, "RunContent")
	assert.Contains(t, body, "RunCompleted")
	assert.Contains(t, body, "Hello test team")

	// Test with non-existent team
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/teams/non-existent/runs", bytes.NewBuffer(runJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestPaginationAndFiltering tests pagination and filtering across endpoints
func TestPaginationAndFiltering(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test knowledge content pagination
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/knowledge/content?db_id=agno-storage&page=1&limit=5", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	meta := response["meta"].(map[string]interface{})
	assert.Equal(t, float64(1), meta["page"])
	assert.Equal(t, float64(5), meta["limit"])

	// Test memory pagination
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/memories?page=2&limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	meta = response["meta"].(map[string]interface{})
	assert.Equal(t, float64(2), meta["page"])
	assert.Equal(t, float64(10), meta["limit"])

	// Test eval runs with filters
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/eval-runs?agent_id=test-agent&sort_by=created_at&sort_order=desc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

// TestErrorHandling tests various error scenarios
func TestErrorHandling(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test invalid JSON in session creation
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sessions", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	// Test invalid JSON in memory creation
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/memories", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	// Test invalid JSON in vector search
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/knowledge/search", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	// Test knowledge content with invalid db_id
	searchRequest := map[string]interface{}{
		"query": "test",
		"db_id": "non-existent-db",
	}
	searchJSON, _ := json.Marshal(searchRequest)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/knowledge/search", bytes.NewBuffer(searchJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestVersionEndpoint tests the version endpoint
func TestVersionEndpoint(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/version", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "1.0.0-test", response["version"])
	assert.Equal(t, "test-os", response["os_id"])
	assert.Equal(t, "Test AgentOS", response["name"])
}

// TestWebSocketUpgrade tests WebSocket upgrade
func TestWebSocketUpgrade(t *testing.T) {
	os, err := setupTestAgentOS()
	require.NoError(t, err)

	router := os.GetApp()

	// Test WebSocket endpoint exists (will fail upgrade without proper headers)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws", nil)
	router.ServeHTTP(w, req)

	// Should return 400 because it's not a valid WebSocket upgrade request
	assert.Equal(t, 400, w.Code)
}
