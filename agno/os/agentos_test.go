package os

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/chat"
)

func TestNewAgentOS(t *testing.T) {
	tests := []struct {
		name    string
		options AgentOSOptions
		wantErr bool
	}{
		{
			name: "valid options",
			options: AgentOSOptions{
				OSID:        "test-os",
				Name:        StringPtr("Test OS"),
				Description: StringPtr("Test Description"),
			},
			wantErr: false,
		},
		{
			name: "missing OS ID",
			options: AgentOSOptions{
				Name: StringPtr("Test OS"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os, err := NewAgentOS(tt.options)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, os)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, os)
				assert.Equal(t, tt.options.OSID, os.GetOSID())
			}
		})
	}
}

func TestAgentOS_GetApp(t *testing.T) {
	// Create a test agent
	testAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Name:    "TestAgent",
		Model: func() models.AgnoModelInterface {
			model, _ := chat.NewOpenAIChat()
			return model
		}(),
	})
	require.NoError(t, err)

	// Create AgentOS instance
	os, err := NewAgentOS(AgentOSOptions{
		OSID:   "test-os",
		Agents: []*agent.Agent{testAgent},
		Settings: &AgentOSSettings{
			Debug: true,
		},
	})
	require.NoError(t, err)

	// Get the app
	app := os.GetApp()
	assert.NotNil(t, app)
	assert.IsType(t, &gin.Engine{}, app)
}

func TestAgentOS_HealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create AgentOS instance
	os, err := NewAgentOS(AgentOSOptions{
		OSID: "test-os",
	})
	require.NoError(t, err)

	// Create test router
	router := gin.New()
	router.GET("/health", os.healthHandler)

	// Create test request
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
	// Health endpoint doesn't include version (matches Python AgentOS)
}

func TestAgentOS_ConfigHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create AgentOS instance
	os, err := NewAgentOS(AgentOSOptions{
		OSID:        "test-os",
		Name:        StringPtr("Test OS"),
		Description: StringPtr("Test Description"),
		Version:     StringPtr("1.0.0"),
	})
	require.NoError(t, err)

	// Create test router
	router := gin.New()
	router.GET("/config", os.configHandler)

	// Create test request
	req, _ := http.NewRequest("GET", "/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test-os", response["os_id"])
	assert.Equal(t, "Test OS", response["name"])
	assert.Equal(t, "Test Description", response["description"])
	assert.Equal(t, "1.0.0", response["version"])
}

func TestAgentOS_ListAgentsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create test agents
	agent1, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Name:    "Agent1",
		Model: func() models.AgnoModelInterface {
			model, _ := chat.NewOpenAIChat()
			return model
		}(),
	})
	require.NoError(t, err)

	agent2, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Name:    "Agent2",
		Model: func() models.AgnoModelInterface {
			model, _ := chat.NewOpenAIChat()
			return model
		}(),
	})
	require.NoError(t, err)

	// Create AgentOS instance
	os, err := NewAgentOS(AgentOSOptions{
		OSID:   "test-os",
		Agents: []*agent.Agent{agent1, agent2},
	})
	require.NoError(t, err)

	// Create test router
	router := gin.New()
	router.GET("/agents", os.listAgentsHandler)

	// Create test request
	req, _ := http.NewRequest("GET", "/agents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var agents []interface{}
	err = json.Unmarshal(w.Body.Bytes(), &agents)
	assert.NoError(t, err)
	assert.Len(t, agents, 2)
}

func TestAgentOS_CreateSessionHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create AgentOS instance
	os, err := NewAgentOS(AgentOSOptions{
		OSID: "test-os",
	})
	require.NoError(t, err)

	// Create test router
	router := gin.New()
	router.POST("/sessions", os.createSessionHandler)

	// Create test request
	req, _ := http.NewRequest("POST", "/sessions", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Note: This test will fail with bad request because we didn't set the body correctly
	// but it tests the handler setup
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatConfig_Validation(t *testing.T) {
	// Test ChatConfig with valid quick prompts
	config := ChatConfig{
		QuickPrompts: map[string][]string{
			"agent1": {"prompt1", "prompt2", "prompt3"},
			"agent2": {"prompt1", "prompt2"},
		},
	}

	// This would normally be validated by a validation library
	// but for now we just test the structure
	assert.NotNil(t, config.QuickPrompts)
	assert.Len(t, config.QuickPrompts["agent1"], 3)
	assert.Len(t, config.QuickPrompts["agent2"], 2)
}

func TestGenerateID(t *testing.T) {
	id1 := generateID("test")
	id2 := generateID("test")

	// IDs should be different
	assert.NotEqual(t, id1, id2)

	// IDs should start with prefix
	assert.Contains(t, id1, "test_")
	assert.Contains(t, id2, "test_")

	// IDs should have proper length (prefix + underscore + 12 hex chars)
	assert.True(t, len(id1) >= len("test_")+12)
}

func TestAgentOSConfig_DefaultValues(t *testing.T) {
	config := AgentOSConfig{}

	// Test default values (nil for optional fields)
	assert.Nil(t, config.AvailableModels)
	assert.Nil(t, config.Chat)
	assert.Nil(t, config.Evals)
	assert.Nil(t, config.Knowledge)
	assert.Nil(t, config.Memory)
	assert.Nil(t, config.Metrics)
	assert.Nil(t, config.Session)
}

func TestAgentOSSettings_DefaultValues(t *testing.T) {
	options := AgentOSOptions{
		OSID: "test",
	}

	os, err := NewAgentOS(options)
	require.NoError(t, err)

	settings := os.GetSettings()
	assert.Equal(t, 8080, settings.Port) // Python uses 8080 by default
	assert.Equal(t, "0.0.0.0", settings.Host)
	assert.Equal(t, false, settings.Reload)
	assert.Equal(t, false, settings.Debug)
	assert.Equal(t, "info", settings.LogLevel)
	assert.Equal(t, true, settings.EnableCORS)
}

// Helper function is now in conversions.go
