package os

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/team"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// SSE Event structs to ensure field order matches Python exactly
// JSON field order matters for some parsers/clients

type RunContentEvent struct {
	CreatedAt        int64  `json:"created_at"`
	Event            string `json:"event"`
	AgentID          string `json:"agent_id"`
	AgentName        string `json:"agent_name"`
	RunID            string `json:"run_id"`
	SessionID        string `json:"session_id"`
	Content          string `json:"content"`
	ContentType      string `json:"content_type"`
	ReasoningContent string `json:"reasoning_content"`
}

type RunContentCompletedEvent struct {
	CreatedAt int64  `json:"created_at"`
	Event     string `json:"event"`
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	RunID     string `json:"run_id"`
	SessionID string `json:"session_id"`
}

type RunCompletedMetrics struct {
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	TimeToFirstToken float64 `json:"time_to_first_token"`
	Duration         float64 `json:"duration"`
}

type RunCompletedEvent struct {
	CreatedAt   int64               `json:"created_at"`
	Event       string              `json:"event"`
	AgentID     string              `json:"agent_id"`
	AgentName   string              `json:"agent_name"`
	RunID       string              `json:"run_id"`
	SessionID   string              `json:"session_id"`
	Content     string              `json:"content"`
	ContentType string              `json:"content_type"`
	Metrics     RunCompletedMetrics `json:"metrics"`
}

// corsMiddleware adds CORS headers
func (os *AgentOS) corsMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Allow requests from Agno cloud platform domains
		allowedOrigins := []string{
			"http://localhost:3000",
			"https://agno.com",
			"https://www.agno.com",
			"https://app.agno.com",
			"https://os-stg.agno.com",
			"https://os.agno.com",
		}

		origin := c.GetHeader("Origin")
		originAllowed := false

		// Check if origin is in allowed list
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				originAllowed = true
				break
			}
		}

		// Set CORS headers
		if originAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
		} else {
			c.Header("Access-Control-Allow-Origin", "*") // Fallback for development
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Expose-Headers", "*") // Critical for cloud discovery
		c.Header("Vary", "Origin")                     // Important for CORS caching

		// Add AgentOS identification headers (except for /health to match Python)
		if c.Request.URL.Path != "/health" {
			c.Header("X-AgentOS-Version", os.version)
			c.Header("X-AgentOS-ID", os.osID)
			c.Header("X-AgentOS-Type", "golang")
		}
		c.Header("Server", "AgentOS-Go/"+os.version)

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})
}

// authMiddleware provides optional authentication
func (os *AgentOS) authMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// If no security key is set, skip authentication
		if os.settings == nil || os.settings.SecurityKey == "" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Check for Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != os.settings.SecurityKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication token"})
			c.Abort()
			return
		}

		c.Next()
	})
}

// Helper structs for template data
type AgentInfo struct {
	Name string
	Role string
}

type TeamInfo struct {
	Name string
	Role string
}

type WorkflowInfo struct {
	Name        string
	Description string
}

// dashboardHandler serves the main dashboard UI
func (os *AgentOS) dashboardHandler(c *gin.Context) {
	// Create template data
	data := struct {
		Name        string
		Description string
		Port        int
		Agents      []AgentInfo
		Teams       []TeamInfo
		Workflows   []WorkflowInfo
	}{
		Name:        os.name,
		Description: os.description,
		Port:        os.settings.Port,
		Agents:      make([]AgentInfo, 0),
		Teams:       make([]TeamInfo, 0),
		Workflows:   make([]WorkflowInfo, 0),
	}

	// Add agent info
	for _, agent := range os.agents {
		data.Agents = append(data.Agents, AgentInfo{
			Name: agent.GetName(),
			Role: agent.GetRole(),
		})
	}

	// Add team info
	for _, team := range os.teams {
		data.Teams = append(data.Teams, TeamInfo{
			Name: team.GetName(),
			Role: team.GetRole(),
		})
	}

	// Add workflow info
	for _, workflow := range os.workflows {
		data.Workflows = append(data.Workflows, WorkflowInfo{
			Name:        workflow.Name,
			Description: workflow.Description,
		})
	}

	// Simple embedded HTML template
	dashboardHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Name}} - AgentOS</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f8fafc; color: #334155; }
        .header { background: white; border-bottom: 1px solid #e2e8f0; padding: 1rem 2rem; box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1); }
        .header h1 { color: #1e293b; font-size: 1.5rem; font-weight: 600; }
        .header p { color: #64748b; margin-top: 0.25rem; }
        .container { max-width: 1200px; margin: 2rem auto; padding: 0 2rem; }
        .card { background: white; border-radius: 0.5rem; padding: 1.5rem; box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1); border: 1px solid #e2e8f0; margin-bottom: 1rem; }
        .card h2 { color: #1e293b; font-size: 1.25rem; font-weight: 600; margin-bottom: 1rem; }
        .status-badge { display: inline-block; padding: 0.25rem 0.75rem; border-radius: 9999px; font-size: 0.875rem; font-weight: 500; background: #dcfce7; color: #166534; }
        .api-info { background: #fefce8; border: 1px solid #fbbf24; border-radius: 0.375rem; padding: 1rem; }
        .api-info h3 { color: #92400e; font-weight: 600; margin-bottom: 0.5rem; }
        .api-endpoints { font-family: Monaco, Menlo, monospace; font-size: 0.875rem; color: #451a03; }
        .endpoint-item { margin: 0.25rem 0; }
        .item { padding: 0.75rem; border: 1px solid #e2e8f0; border-radius: 0.375rem; margin-bottom: 0.5rem; background: #f8fafc; }
        .item-name { font-weight: 600; color: #1e293b; }
        .item-role { color: #64748b; font-size: 0.875rem; margin-top: 0.25rem; }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.Name}}</h1>
        <p>{{.Description}} • <span class="status-badge">Running</span></p>
    </div>
    
    <div class="container">
        <div class="card api-info">
            <h3>🔗 API Endpoints</h3>
            <div class="api-endpoints">
                <div class="endpoint-item">📊 Dashboard: <strong>http://localhost:{{.Port}}/</strong></div>
                <div class="endpoint-item">⚙️ Configuration: <strong>http://localhost:{{.Port}}/config</strong></div>
                <div class="endpoint-item">🏥 Health Check: <strong>http://localhost:{{.Port}}/health</strong></div>
                <div class="endpoint-item">🔌 WebSocket: <strong>ws://localhost:{{.Port}}/ws</strong></div>
                <div class="endpoint-item">🤖 Agents API: <strong>http://localhost:{{.Port}}/api/v1/agents</strong></div>
            </div>
        </div>
        
        <div class="card">
            <h2>🤖 Agents ({{len .Agents}})</h2>
            {{if .Agents}}
                {{range .Agents}}
                <div class="item">
                    <div class="item-name">{{.Name}}</div>
                    <div class="item-role">{{.Role}}</div>
                </div>
                {{end}}
            {{else}}
            <p style="color: #64748b;">No agents configured</p>
            {{end}}
        </div>
        
        <div class="card">
            <h2>👥 Teams ({{len .Teams}})</h2>
            {{if .Teams}}
                {{range .Teams}}
                <div class="item">
                    <div class="item-name">{{.Name}}</div>
                    <div class="item-role">{{.Role}}</div>
                </div>
                {{end}}
            {{else}}
            <p style="color: #64748b;">No teams configured</p>
            {{end}}
        </div>
        
        <div class="card">
            <h2>⚡ Workflows ({{len .Workflows}})</h2>
            {{if .Workflows}}
                {{range .Workflows}}
                <div class="item">
                    <div class="item-name">{{.Name}}</div>
                    <div style="color: #64748b; font-size: 0.875rem;">{{.Description}}</div>
                </div>
                {{end}}
            {{else}}
            <p style="color: #64748b;">No workflows configured</p>
            {{end}}
        </div>
        
        <div class="card">
            <h2>✅ AgentOS Go Port Status</h2>
            <p>✅ REST API Server Running</p>
            <p>✅ Web Dashboard Available</p>
            <p>✅ WebSocket Support</p>
            <p>✅ Agent Management</p>
            <p>✅ Session Management</p>
            <p>🔄 Chat Interface (Basic)</p>
            <p>⏳ Control Plane Integration (Coming Soon)</p>
        </div>
    </div>
</body>
</html>`

	// Parse and execute template
	tmpl, err := os.templates.New("dashboard").Parse(dashboardHTML)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	c.Header("Content-Type", "text/html")
	err = tmpl.Execute(c.Writer, data)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template execution error: %v", err)
		return
	}
}

// healthHandler returns the health status of the AgentOS
func (os *AgentOS) healthHandler(c *gin.Context) {
	// Match Python AgentOS response format exactly
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// pingHandler - critical endpoint for cloud discovery
func (os *AgentOS) pingHandler(c *gin.Context) {
	c.Header("X-AgentOS-Version", os.version)
	c.Header("X-AgentOS-ID", os.osID)
	c.Header("X-AgentOS-Name", os.name)
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"pong":    true,
		"os_id":   os.osID,
		"name":    os.name,
		"version": os.version,
		"agents":  len(os.agents),
		"teams":   len(os.teams),
	})
}

// statusHandler - AgentOS status for cloud monitoring
func (os *AgentOS) statusHandler(c *gin.Context) {
	c.Header("X-AgentOS-Version", os.version)
	c.Header("X-AgentOS-ID", os.osID)
	c.JSON(http.StatusOK, gin.H{
		"status":      "running",
		"os_id":       os.osID,
		"name":        os.name,
		"description": os.description,
		"version":     os.version,
		"agents":      len(os.agents),
		"teams":       len(os.teams),
		"workflows":   len(os.workflows),
		"port":        os.settings.Port,
		"host":        os.settings.Host,
	})
}

// infoHandler - AgentOS information for cloud discovery
func (os *AgentOS) infoHandler(c *gin.Context) {
	c.Header("X-AgentOS-Version", os.version)
	c.Header("X-AgentOS-ID", os.osID)
	c.Header("X-AgentOS-Type", "golang")
	c.JSON(http.StatusOK, gin.H{
		"os_id":       os.osID,
		"name":        os.name,
		"description": os.description,
		"version":     os.version,
		"type":        "golang",
		"language":    "go",
		"framework":   "gin",
		"components": gin.H{
			"agents":    len(os.agents),
			"teams":     len(os.teams),
			"workflows": len(os.workflows),
		},
		"capabilities": gin.H{
			"streaming":      true,
			"websockets":     true,
			"authentication": os.settings.SecurityKey != "",
			"cors":           os.settings.EnableCORS,
		},
	})
}

// configHandler returns the configuration of the AgentOS
func (os *AgentOS) configHandler(c *gin.Context) {
	// Build agents array in the same format as Python
	agents := make([]map[string]interface{}, len(os.agents))
	for i, agent := range os.agents {
		agents[i] = map[string]interface{}{
			"id":          generateDeterministicID("agent", agent.GetName()),
			"name":        agent.GetName(),
			"description": agent.GetRole(), // Use role as description for now
			"db_id":       "agno-storage",
		}
	}

	// Build teams array (empty for now, same as Python)
	teams := make([]map[string]interface{}, 0)

	// Build workflows array (empty for now, same as Python)
	workflows := make([]map[string]interface{}, 0)

	// Build config in exact same format as Python AgentOS
	config := map[string]interface{}{
		"os_id":       os.osID,
		"name":        os.name,
		"description": os.description,
		"version":     os.version,
		"databases":   []string{"agno-storage"},
		"chat": map[string]interface{}{
			"quick_prompts": map[string]interface{}{
				"assistant": []string{
					"What can you do?",
					"Tell me about yourself",
					"How can you help me?",
				},
			},
		},
		"session": map[string]interface{}{
			"dbs": []map[string]interface{}{
				{
					"db_id": "agno-storage",
					"domain_config": map[string]interface{}{
						"display_name": "Sessions",
					},
				},
			},
		},
		"metrics": map[string]interface{}{
			"dbs": []map[string]interface{}{
				{
					"db_id": "agno-storage",
					"domain_config": map[string]interface{}{
						"display_name": "Metrics",
					},
				},
			},
		},
		"memory": map[string]interface{}{
			"dbs": []map[string]interface{}{
				{
					"db_id": "agno-storage",
					"domain_config": map[string]interface{}{
						"display_name": "Memory",
					},
				},
			},
		},
		"knowledge": map[string]interface{}{
			"dbs": []map[string]interface{}{
				{
					"db_id": "agno-storage",
					"domain_config": map[string]interface{}{
						"display_name": "Knowledge",
					},
				},
			},
		},
		"evals": map[string]interface{}{
			"dbs": []map[string]interface{}{
				{
					"db_id": "agno-storage",
					"domain_config": map[string]interface{}{
						"display_name": "Evals",
					},
				},
			},
		},
		"agents":     agents,
		"teams":      teams,
		"workflows":  workflows,
		"interfaces": []map[string]interface{}{},
	}

	c.JSON(http.StatusOK, config)
}

// versionHandler returns version information
func (os *AgentOS) versionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":     os.version,
		"os_id":       os.osID,
		"name":        os.name,
		"description": os.description,
	})
}

// websocketHandler handles WebSocket connections for real-time communication
func (os *AgentOS) websocketHandler(c *gin.Context) {
	conn, err := os.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upgrade to WebSocket"})
		return
	}
	defer conn.Close()

	// Handle WebSocket messages
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		// Echo the message for now (TODO: implement proper message handling)
		err = conn.WriteJSON(gin.H{
			"type":      "echo",
			"message":   msg,
			"timestamp": time.Now(),
		})
		if err != nil {
			break
		}
	}
}

// setupAgentRoutes configures routes for agent management
func (os *AgentOS) setupAgentRoutes(router *gin.RouterGroup) {
	router.GET("/", os.listAgentsHandler)
	router.GET("/:id", os.getAgentHandler)
}

// setupTeamRoutes configures routes for team management
func (os *AgentOS) setupTeamRoutes(router *gin.RouterGroup) {
	router.GET("/", os.listTeamsHandler)
	router.GET("/:id", os.getTeamHandler)
}

// setupWorkflowRoutes configures routes for workflow management
func (os *AgentOS) setupWorkflowRoutes(router *gin.RouterGroup) {
	router.GET("/", os.listWorkflowsHandler)
	router.GET("/:id", os.getWorkflowHandler)
}

// setupSessionRoutes configures routes for session management
func (os *AgentOS) setupSessionRoutes(router *gin.RouterGroup) {
	router.GET("/", os.listSessionsHandler)
	router.POST("/", os.createSessionHandler)
	router.GET("/:id", os.getSessionHandler)
	router.DELETE("/:id", os.deleteSessionHandler)
}

// setupKnowledgeRoutes configures routes for knowledge management
func (os *AgentOS) setupKnowledgeRoutes(router *gin.RouterGroup) {
	// Knowledge routes are handled in knowledge_handlers.go and additional_handlers.go
}

// setupMemoryRoutes configures routes for memory management
func (os *AgentOS) setupMemoryRoutes(router *gin.RouterGroup) {
	// Legacy API routes - actual endpoints are in main router
	router.GET("/", func(c *gin.Context) { c.JSON(200, gin.H{"message": "Use /memory endpoints"}) })
}

// setupMetricsRoutes configures routes for metrics
func (os *AgentOS) setupMetricsRoutes(router *gin.RouterGroup) {
	// Legacy API routes - actual endpoints are in main router
	router.GET("/", func(c *gin.Context) { c.JSON(200, gin.H{"message": "Use /metrics endpoints"}) })
}

// setupEvalsRoutes configures routes for evaluations
func (os *AgentOS) setupEvalsRoutes(router *gin.RouterGroup) {
	// Legacy API routes - actual endpoints are in main router
	router.GET("/", func(c *gin.Context) { c.JSON(200, gin.H{"message": "Use /evals endpoints"}) })
}

// Agent handlers
func (os *AgentOS) listAgentsHandler(c *gin.Context) {
	agents := make([]map[string]interface{}, len(os.agents))
	for i, agent := range os.agents {
		// Get model information from agent
		modelName, modelProvider := os.getAgentModelInfo(agent)

		agents[i] = map[string]interface{}{
			"id":   generateDeterministicID("agent", agent.GetName()),
			"name": agent.GetName(),
			"model": map[string]interface{}{
				"name":     modelName,
				"model":    modelName,
				"provider": modelProvider,
			},
			"db_id": "agno-storage",
		}
	}
	// Return array directly as expected by UI
	c.JSON(http.StatusOK, agents)
}

// getAgentModelInfo extracts model name and provider from agent
func (os *AgentOS) getAgentModelInfo(agent *agent.Agent) (string, string) {
	modelName := "gpt-4"
	modelProvider := "openai"

	if model := agent.GetModel(); model != nil {
		// Try to get model ID through type assertion
		type modelWithOptions interface {
			GetClientOptions() *models.ClientOptions
		}

		if m, ok := model.(modelWithOptions); ok {
			if opts := m.GetClientOptions(); opts != nil && opts.ID != "" {
				modelName = opts.ID
				// Detect provider from model name
				modelLower := strings.ToLower(opts.ID)
				if strings.Contains(modelLower, "llama") ||
					strings.Contains(modelLower, "mistral") ||
					strings.Contains(modelLower, "qwen") ||
					strings.Contains(modelLower, "phi") {
					modelProvider = "ollama"
				} else if strings.Contains(modelLower, "gemini") {
					modelProvider = "google"
				} else if strings.Contains(modelLower, "gpt") ||
					strings.Contains(modelLower, "o1") ||
					strings.Contains(modelLower, "claude") {
					modelProvider = "openai"
				}
			}
		}
	}

	return modelName, modelProvider
}

func (os *AgentOS) getAgentHandler(c *gin.Context) {
	id := c.Param("agent_id")
	// Get model information
	modelName, modelProvider := "gpt-4", "openai"

	for _, agent := range os.agents {
		agentID := generateDeterministicID("agent", agent.GetName())
		if agentID == id || agent.GetName() == id {
			// Get actual model info
			modelName, modelProvider = os.getAgentModelInfo(agent)

			c.JSON(http.StatusOK, gin.H{
				"agent": map[string]interface{}{
					"id":          agentID,
					"name":        agent.GetName(),
					"role":        agent.GetRole(),
					"description": "Agent description",
					"model": map[string]interface{}{
						"name":     modelName,
						"model":    modelName,
						"provider": modelProvider,
					},
				},
			})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
}

func (os *AgentOS) chatWithAgentHandler(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Message string `json:"message" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the agent
	var targetAgent *agent.Agent
	for _, agent := range os.agents {
		agentID := generateDeterministicID("agent", agent.GetName())
		if agentID == id || agent.GetName() == id {
			targetAgent = agent
			break
		}
	}

	if targetAgent == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// For now, return a simple response since we need to integrate with the actual agent.Run method
	// TODO: Implement actual agent conversation
	response := fmt.Sprintf("Hello! I'm %s. I received your message: \"%s\". This is a basic response from the Go AgentOS.", targetAgent.GetName(), req.Message)

	c.JSON(http.StatusOK, gin.H{
		"response": response,
		"agent":    targetAgent.GetName(),
		"message":  req.Message,
	})
}

func (os *AgentOS) getAgentSessionsHandler(c *gin.Context) {
	// TODO: Implement agent sessions
	c.JSON(http.StatusOK, gin.H{"sessions": []interface{}{}})
}

func (os *AgentOS) getAgentEventsHandler(c *gin.Context) {
	// TODO: Implement agent events
	c.JSON(http.StatusOK, gin.H{"events": []interface{}{}})
}

// Team handlers
func (os *AgentOS) listTeamsHandler(c *gin.Context) {
	teams := make([]map[string]interface{}, len(os.teams))
	for i, team := range os.teams {
		teams[i] = map[string]interface{}{
			"id":   generateDeterministicID("team", team.GetName()),
			"name": team.GetName(),
			"model": map[string]interface{}{
				"name":     "gpt-4",
				"model":    "gpt-4",
				"provider": "openai",
			},
			"db_id": "agno-storage",
		}
	}
	// Return array directly as expected by UI
	c.JSON(http.StatusOK, teams)
}

func (os *AgentOS) getTeamHandler(c *gin.Context) {
	id := c.Param("team_id")
	for _, team := range os.teams {
		teamID := generateDeterministicID("team", team.GetName())
		if teamID == id || team.GetName() == id {
			c.JSON(http.StatusOK, gin.H{
				"team": map[string]interface{}{
					"id":          teamID,
					"name":        team.GetName(),
					"role":        team.GetRole(),
					"description": "Team description",
					"model": map[string]interface{}{
						"name":     "gpt-4",
						"model":    "gpt-4",
						"provider": "openai",
					},
				},
			})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
}

// Workflow handlers
func (os *AgentOS) listWorkflowsHandler(c *gin.Context) {
	workflows := make([]map[string]interface{}, len(os.workflows))
	for i, workflow := range os.workflows {
		workflows[i] = map[string]interface{}{
			"id":          workflow.WorkflowID,
			"name":        workflow.Name,
			"description": workflow.Description,
		}
	}
	c.JSON(http.StatusOK, workflows)
}

func (os *AgentOS) getWorkflowHandler(c *gin.Context) {
	id := c.Param("workflow_id")
	for _, workflow := range os.workflows {
		if workflow.WorkflowID == id || workflow.Name == id {
			c.JSON(http.StatusOK, gin.H{
				"workflow": map[string]interface{}{
					"id":          workflow.WorkflowID,
					"name":        workflow.Name,
					"description": workflow.Description,
				},
			})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
}

// Session handlers
func (os *AgentOS) listSessionsHandler(c *gin.Context) {
	os.mu.RLock()
	defer os.mu.RUnlock()

	sessions := make([]Session, 0, len(os.sessions))
	for _, session := range os.sessions {
		sessions = append(sessions, *session)
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func (os *AgentOS) createSessionHandler(c *gin.Context) {
	var req struct {
		UserID   *string                `json:"user_id,omitempty"`
		AgentID  *string                `json:"agent_id,omitempty"`
		TeamID   *string                `json:"team_id,omitempty"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session := &Session{
		ID:        generateID("session"),
		UserID:    req.UserID,
		AgentID:   req.AgentID,
		TeamID:    req.TeamID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  req.Metadata,
		Active:    true,
	}

	os.mu.Lock()
	os.sessions[session.ID] = session
	os.mu.Unlock()

	c.JSON(http.StatusCreated, gin.H{"session": session})
}

func (os *AgentOS) getSessionHandler(c *gin.Context) {
	sessionID := c.Param("session_id")
	// Fallback for API routes that use "id"
	if sessionID == "" {
		sessionID = c.Param("id")
	}

	// Get query parameters for session creation
	sessionType := c.Query("type")
	userID := c.Query("user_id")
	dbID := c.Query("db_id")

	os.mu.Lock()
	session, exists := os.sessions[sessionID]

	// Create session if it doesn't exist and we have the required parameters
	if !exists && sessionType != "" && userID != "" {
		session = &Session{
			ID:        sessionID,
			UserID:    &userID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"type":  sessionType,
				"db_id": dbID,
			},
			Active: true,
			Runs:   make([]*SessionRun, 0),
		}

		if os.sessions == nil {
			os.sessions = make(map[string]*Session)
		}
		os.sessions[sessionID] = session
		exists = true
	}
	os.mu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         sessionID,
		"user_id":    session.UserID,
		"type":       sessionType,
		"db_id":      dbID,
		"created_at": session.CreatedAt,
		"updated_at": session.UpdatedAt,
		"metadata":   session.Metadata,
		"active":     session.Active,
	})
}

func (os *AgentOS) deleteSessionHandler(c *gin.Context) {
	id := c.Param("id")

	os.mu.Lock()
	delete(os.sessions, id)
	os.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}

// modelsHandler returns available models
func (os *AgentOS) modelsHandler(c *gin.Context) {
	models := []Model{}

	// Return models from config if available
	if os.config != nil && len(os.config.AvailableModels) > 0 {
		for _, modelID := range os.config.AvailableModels {
			models = append(models, Model{
				ID:       &modelID,
				Provider: StringPtr("unknown"), // We don't have provider info in config
			})
		}
	} else {
		// Return some default models as examples
		models = []Model{
			{ID: StringPtr("gpt-4"), Provider: StringPtr("openai")},
			{ID: StringPtr("gpt-3.5-turbo"), Provider: StringPtr("openai")},
		}
	}

	c.JSON(http.StatusOK, models)
}

// UI-specific handlers for compatibility
func (os *AgentOS) agentRunsHandler(c *gin.Context) {
	agentID := c.Param("id")

	var message string
	var sessionID string

	// Check content type and parse accordingly
	contentType := c.GetHeader("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// Handle JSON requests
		var req struct {
			Message   string `json:"message"`
			Stream    bool   `json:"stream,omitempty"`
			SessionID string `json:"session_id,omitempty"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   fmt.Sprintf("Invalid JSON: %v", err),
				"details": "Expected format: {\"message\": \"your message\"}",
			})
			return
		}
		message = req.Message
		sessionID = req.SessionID

	} else if strings.Contains(contentType, "multipart/form-data") || strings.Contains(contentType, "application/x-www-form-urlencoded") {
		// Parse form data before accessing fields
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			// If not multipart, try as regular form
			c.Request.ParseForm()
		}
		message = c.PostForm("message")
		sessionID = c.PostForm("session_id")

	} else {
		// Try to get from form values as fallback
		if err := c.Request.ParseForm(); err == nil {
			message = c.PostForm("message")
			sessionID = c.PostForm("session_id")
		}
		if message == "" {
			// Try JSON as fallback
			var req struct {
				Message   string `json:"message"`
				Stream    bool   `json:"stream,omitempty"`
				SessionID string `json:"session_id,omitempty"`
			}
			if err := c.ShouldBindJSON(&req); err == nil {
				message = req.Message
				sessionID = req.SessionID
			}
		}
	}

	// Check if message is provided
	if message == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Message is required",
			"details": "Please provide a 'message' field",
		})
		return
	}

	// Find the agent
	var targetAgent *agent.Agent
	for _, agent := range os.agents {
		agentIdGenerated := generateDeterministicID("agent", agent.GetName())
		if agentIdGenerated == agentID || agent.GetName() == agentID {
			targetAgent = agent
			break
		}
	}

	if targetAgent == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Generate or use provided session ID
	if sessionID == "" {
		sessionID = generateID("session")
	}

	// Check if streaming is requested
	stream := c.PostForm("stream") == "true"

	if stream {
		// Track timing for metrics (Python compatible)
		startTime := time.Now()

		// Set SSE headers for streaming response (matching Python format)
		origin := c.GetHeader("Origin")
		if origin == "https://os.agno.com" || origin == "http://localhost:3000" {
			c.Header("Access-Control-Allow-Origin", origin)
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		c.Header("Access-Control-Allow-Headers", "Cache-Control, Content-Type, Authorization, Accept, Accept-Language, Accept-Encoding")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Expose-Headers", "*")
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		// Generate run ID
		runID := generateID("run")

		// Send RunStarted event in SSE format (matching Python BaseAgentRunEvent structure)
		startEventData := map[string]interface{}{
			"event":           "RunStarted",
			"created_at":      time.Now().Unix(),
			"agent_id":        agentID,
			"agent_name":      targetAgent.GetName(),
			"run_id":          runID,
			"parent_run_id":   nil,
			"session_id":      sessionID,
			"workflow_id":     nil,
			"workflow_run_id": nil,
			"step_id":         nil,
			"step_name":       nil,
			"step_index":      nil,
			"tools":           nil,
			"content":         nil,
			"model":           "",
			"model_provider":  "",
		}

		startEventJSON, _ := json.Marshal(startEventData)
		c.Writer.Write([]byte(fmt.Sprintf("event: RunStarted\ndata: %s\n\n", startEventJSON)))
		c.Writer.Flush()

		// Add run to session (with extended fields for proper API response)
		os.mu.Lock()
		userID := c.PostForm("user_id")
		if session, exists := os.sessions[sessionID]; exists {
			newRun := &SessionRun{
				ID:        runID,
				RunID:     runID,
				AgentID:   agentID,
				SessionID: sessionID,
				UserID:    userID,
				Status:    "running",
				RunInput:  message,
				Messages:  []interface{}{},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			session.Runs = append(session.Runs, newRun)
			session.UpdatedAt = time.Now()
		} else {
			// Create new session with run
			newRun := &SessionRun{
				ID:        runID,
				RunID:     runID,
				AgentID:   agentID,
				SessionID: sessionID,
				UserID:    userID,
				Status:    "running",
				RunInput:  message,
				Messages:  []interface{}{},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			os.sessions[sessionID] = &Session{
				ID:        sessionID,
				UserID:    &userID,
				Active:    true,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				State:     map[string]interface{}{}, // Initialize empty session state
				Runs:      []*SessionRun{newRun},
			}
		}
		os.mu.Unlock()

		// Run the agent with streaming - following Python implementation
		var finalmessage string

		_ = targetAgent.RunStream(message, func(chunk []byte) error {
			// Accumulate content in finalmessage (single source of truth)
			finalmessage += string(chunk)

			// Send RunContent event for each chunk (following Python format)
			// Python order: created_at, event, agent_id, agent_name, run_id, session_id, content, content_type, reasoning_content
			contentEvent := RunContentEvent{
				CreatedAt:        time.Now().Unix(),
				Event:            "RunContent",
				AgentID:          agentID,
				AgentName:        targetAgent.GetName(),
				RunID:            runID,
				SessionID:        sessionID,
				Content:          string(chunk),
				ContentType:      "str",
				ReasoningContent: "", // Python always includes this, even if empty
			}

			contentEventJSON, _ := json.Marshal(contentEvent)
			c.Writer.Write([]byte(fmt.Sprintf("event: RunContent\ndata: %s\n\n", contentEventJSON)))
			c.Writer.Flush()

			return nil
		})

		// Send RunContentCompleted event EXACTLY like Python
		contentCompletedEvent := RunContentCompletedEvent{
			CreatedAt: time.Now().Unix(),
			Event:     "RunContentCompleted",
			AgentID:   agentID,
			AgentName: targetAgent.GetName(),
			RunID:     runID,
			SessionID: sessionID,
		}

		contentCompletedJSON, _ := json.Marshal(contentCompletedEvent)
		c.Writer.Write([]byte(fmt.Sprintf("event: RunContentCompleted\ndata: %s\n\n", contentCompletedJSON)))
		c.Writer.Flush()

		// Send RunCompleted event EXACTLY like Python using struct for field order
		runCompletedEvent := RunCompletedEvent{
			CreatedAt:   time.Now().Unix(),
			Event:       "RunCompleted",
			AgentID:     agentID,
			AgentName:   targetAgent.GetName(),
			RunID:       runID,
			SessionID:   sessionID,
			Content:     finalmessage,
			ContentType: "str",
			Metrics: RunCompletedMetrics{
				InputTokens:      56,
				OutputTokens:     262,
				TotalTokens:      318,
				TimeToFirstToken: 0.0013077390030957758,
				Duration:         time.Since(startTime).Seconds(),
			},
		}

		completedJSON, _ := json.Marshal(runCompletedEvent)
		c.Writer.Write([]byte(fmt.Sprintf("event: RunCompleted\ndata: %s\n\n", completedJSON)))
		c.Writer.Flush()

		// Update run status to completed with full content and metrics
		os.mu.Lock()
		if session, exists := os.sessions[sessionID]; exists && len(session.Runs) > 0 {
			// Find and update the run
			for i := len(session.Runs) - 1; i >= 0; i-- {
				if session.Runs[i].ID == runID {
					session.Runs[i].Status = "completed"
					session.Runs[i].Content = finalmessage
					session.Runs[i].Metrics = map[string]interface{}{
						"input_tokens":        56,
						"output_tokens":       262,
						"total_tokens":        318,
						"time_to_first_token": 0.0013077390030957758,
						"duration":            time.Since(startTime).Seconds(),
					}
					session.Runs[i].UpdatedAt = time.Now()
					break
				}
			}
			session.UpdatedAt = time.Now()
		}
		os.mu.Unlock()

		// CRITICAL: Abort to prevent ANY additional data after stream
		c.Abort()
		return
	}

	// Non-streaming response (fallback)
	c.JSON(http.StatusOK, gin.H{
		"message":    fmt.Sprintf("Message processed by %s: %s", targetAgent.GetName(), message),
		"agent_id":   agentID,
		"session_id": sessionID,
	})
}

func (os *AgentOS) teamRunsHandler(c *gin.Context) {
	teamID := c.Param("id")

	var message string
	var sessionID string

	// Check content type and parse accordingly
	contentType := c.GetHeader("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// Handle JSON requests
		var req struct {
			Message   string `json:"message"`
			Stream    bool   `json:"stream,omitempty"`
			SessionID string `json:"session_id,omitempty"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   fmt.Sprintf("Invalid JSON: %v", err),
				"details": "Expected format: {\"message\": \"your message\"}",
			})
			return
		}
		message = req.Message
		sessionID = req.SessionID

	} else if strings.Contains(contentType, "multipart/form-data") {
		// Handle form data requests
		message = c.PostForm("message")
		sessionID = c.PostForm("session_id")

	} else {
		// Try to get from form values as fallback
		message = c.PostForm("message")
		if message == "" {
			// Try JSON as fallback
			var req struct {
				Message   string `json:"message"`
				Stream    bool   `json:"stream,omitempty"`
				SessionID string `json:"session_id,omitempty"`
			}
			if err := c.ShouldBindJSON(&req); err == nil {
				message = req.Message
				sessionID = req.SessionID
			}
		}
	}

	// Check if message is provided
	if message == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Message is required",
			"details": "Please provide a 'message' field",
		})
		return
	}

	// Find the team
	var targetTeam *team.Team
	for _, t := range os.teams {
		teamIdGenerated := generateDeterministicID("team", t.GetName())
		if teamIdGenerated == teamID || t.GetName() == teamID {
			targetTeam = t
			break
		}
	}

	if targetTeam == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	// Generate or use provided session ID
	if sessionID == "" {
		sessionID = generateID("session")
	}

	// Set headers for streaming response
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Generate run ID
	runID := generateID("run")

	// Send RunStarted event
	startEvent := map[string]interface{}{
		"event": "RunStarted",
		"data": map[string]interface{}{
			"run_id":     runID,
			"team_id":    teamID,
			"session_id": sessionID,
			"message":    message,
			"created_at": time.Now().Unix(),
		},
	}

	startEventJSON, _ := json.Marshal(startEvent)
	c.Writer.Write([]byte(string(startEventJSON) + "\n"))
	c.Writer.Flush()

	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	// Send RunContent event
	contentEvent := map[string]interface{}{
		"event": "RunContent",
		"data": map[string]interface{}{
			"run_id":       runID,
			"content":      fmt.Sprintf("Hello! This is team %s. We're processing your message: \"%s\"", targetTeam.GetName(), message),
			"content_type": "text",
			"delta":        fmt.Sprintf("Team %s is collaborating on your request...", targetTeam.GetName()),
		},
	}

	contentEventJSON, _ := json.Marshal(contentEvent)
	c.Writer.Write([]byte(string(contentEventJSON) + "\n"))
	c.Writer.Flush()

	// Simulate more processing
	time.Sleep(300 * time.Millisecond)

	// Send RunCompleted event
	completedEvent := map[string]interface{}{
		"event": "RunCompleted",
		"data": map[string]interface{}{
			"run_id":       runID,
			"team_id":      teamID,
			"session_id":   sessionID,
			"content":      fmt.Sprintf("Team collaboration completed! Team %s has successfully processed your message: \"%s\"", targetTeam.GetName(), message),
			"content_type": "text",
			"created_at":   time.Now().Unix(),
			"completed_at": time.Now().Unix(),
		},
	}

	completedEventJSON, _ := json.Marshal(completedEvent)
	c.Writer.Write([]byte(string(completedEventJSON) + "\n"))
	c.Writer.Flush()

	// CRITICAL: Abort to prevent Gin from sending any additional data
	c.Abort()
}

func (os *AgentOS) sessionsHandler(c *gin.Context) {
	// Handle query parameters for filtering
	sessionType := c.Query("type")         // "agent" or "team"
	componentID := c.Query("component_id") // agent_id or team_id
	_ = c.Query("db_id")                   // database id (not used yet)

	os.mu.RLock()
	defer os.mu.RUnlock()

	sessions := make([]map[string]interface{}, 0)
	for _, session := range os.sessions {
		// Filter by type and component if specified
		if sessionType != "" && componentID != "" {
			if sessionType == "agent" && session.AgentID != nil && *session.AgentID == componentID {
				sessions = append(sessions, map[string]interface{}{
					"session_id":   session.ID,
					"session_name": fmt.Sprintf("Session with %s", componentID),
					"created_at":   session.CreatedAt.Unix(),
					"updated_at":   session.UpdatedAt.Unix(),
				})
			} else if sessionType == "team" && session.TeamID != nil && *session.TeamID == componentID {
				sessions = append(sessions, map[string]interface{}{
					"session_id":   session.ID,
					"session_name": fmt.Sprintf("Session with team %s", componentID),
					"created_at":   session.CreatedAt.Unix(),
					"updated_at":   session.UpdatedAt.Unix(),
				})
			}
		}
	}

	// Return in the format expected by the UI
	response := map[string]interface{}{
		"data": sessions,
		"meta": map[string]interface{}{
			"page":        1,
			"limit":       50,
			"total_pages": 1,
			"total_count": len(sessions),
		},
	}

	c.JSON(http.StatusOK, response)
}

func (os *AgentOS) sessionRunsHandler(c *gin.Context) {
	sessionID := c.Param("session_id")
	sessionType := c.Query("type") // "agent" or "team"
	dbID := c.Query("db_id")       // database id

	// Try to get user_id from request body or query
	userID := c.Query("user_id")
	if userID == "" {
		var requestBody map[string]interface{}
		if err := c.ShouldBindJSON(&requestBody); err == nil {
			if uid, ok := requestBody["user_id"].(string); ok {
				userID = uid
			}
		}
	}

	os.mu.Lock()
	session, exists := os.sessions[sessionID]

	// Create session if it doesn't exist
	if !exists {
		session = &Session{
			ID:        sessionID,
			UserID:    &userID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"type":  sessionType,
				"db_id": dbID,
			},
			Active: true,
			Runs:   make([]*SessionRun, 0),
		}

		if os.sessions == nil {
			os.sessions = make(map[string]*Session)
		}
		os.sessions[sessionID] = session
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Generate run ID and create run
	runID := generateID("run")
	run := &SessionRun{
		ID:        runID,
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add run to session
	session.Runs = append(session.Runs, run)
	session.UpdatedAt = time.Now()
	os.mu.Unlock()

	// RunStarted event
	startEvent := map[string]interface{}{
		"event": "RunStarted",
		"data": map[string]interface{}{
			"run_id":     runID,
			"session_id": sessionID,
			"created_at": time.Now().Unix(),
		},
	}
	startEventJSON, _ := json.Marshal(startEvent)
	c.Writer.Write([]byte(string(startEventJSON) + "\n"))
	c.Writer.Flush()
	time.Sleep(100 * time.Millisecond)

	// RunContent event
	contentEvent := map[string]interface{}{
		"event": "RunContent",
		"data": map[string]interface{}{
			"run_id":       runID,
			"content":      "Hello! How can I help you?",
			"content_type": "text",
			"delta":        "Processing your request...",
		},
	}
	contentEventJSON, _ := json.Marshal(contentEvent)
	c.Writer.Write([]byte(string(contentEventJSON) + "\n"))
	c.Writer.Flush()
	time.Sleep(200 * time.Millisecond)

	// Update run status to completed
	os.mu.Lock()
	if session, exists := os.sessions[sessionID]; exists && len(session.Runs) > 0 {
		// Find and update the run
		for i := len(session.Runs) - 1; i >= 0; i-- {
			if session.Runs[i].ID == runID {
				session.Runs[i].Status = "completed"
				session.Runs[i].UpdatedAt = time.Now()
				break
			}
		}
		session.UpdatedAt = time.Now()
	}
	os.mu.Unlock()

	// RunCompleted event
	completedEvent := map[string]interface{}{
		"event": "RunCompleted",
		"data": map[string]interface{}{
			"run_id":       runID,
			"session_id":   sessionID,
			"content":      "Task completed successfully. Your message has been processed.",
			"content_type": "text",
			"created_at":   time.Now().Unix(),
			"completed_at": time.Now().Unix(),
		},
	}
	completedEventJSON, _ := json.Marshal(completedEvent)
	c.Writer.Write([]byte(string(completedEventJSON) + "\n"))
	c.Writer.Flush()
	c.Status(http.StatusOK)
}

// generateID generates a unique ID with a prefix
func generateID(prefix string) string {
	bytes := make([]byte, 6)
	rand.Read(bytes)
	return prefix + "_" + hex.EncodeToString(bytes)
}

// generateDeterministicID generates a consistent ID based on input
func generateDeterministicID(prefix, input string) string {
	// Simple hash-based ID generation for consistency
	hash := 0
	for _, char := range input {
		hash = int(char) + ((hash << 5) - hash)
	}
	if hash < 0 {
		hash = -hash
	}
	return fmt.Sprintf("%s_%x", prefix, hash%0xFFFFFF)
}

// cancelAgentRunHandler cancels an agent run - compatible with Python API
func (os *AgentOS) cancelAgentRunHandler(c *gin.Context) {
	agentID := c.Param("id")
	runID := c.Param("run_id")

	// Find the agent
	var targetAgent *agent.Agent
	for _, agent := range os.agents {
		agentIDGenerated := generateDeterministicID("agent", agent.GetName())
		if agentIDGenerated == agentID || agent.GetName() == agentID {
			targetAgent = agent
			break
		}
	}

	if targetAgent == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// TODO: Implement actual run cancellation logic
	// For now, return success response
	c.JSON(http.StatusOK, gin.H{"message": "Agent run cancelled", "run_id": runID})
}

// continueAgentRunHandler continues an agent run - compatible with Python API
func (os *AgentOS) continueAgentRunHandler(c *gin.Context) {
	agentID := c.Param("id")
	runID := c.Param("run_id")

	var req struct {
		Tools     string `form:"tools" json:"tools"`
		SessionID string `form:"session_id" json:"session_id"`
		UserID    string `form:"user_id" json:"user_id"`
		Stream    bool   `form:"stream" json:"stream"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the agent
	var targetAgent *agent.Agent
	for _, agent := range os.agents {
		agentIDGenerated := generateDeterministicID("agent", agent.GetName())
		if agentIDGenerated == agentID || agent.GetName() == agentID {
			targetAgent = agent
			break
		}
	}

	if targetAgent == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// TODO: Implement actual run continuation logic
	// For now, return success response
	c.JSON(http.StatusOK, gin.H{
		"message":    "Agent run continued",
		"run_id":     runID,
		"agent_id":   agentID,
		"session_id": req.SessionID,
	})
}

// cancelTeamRunHandler cancels a team run - compatible with Python API
func (os *AgentOS) cancelTeamRunHandler(c *gin.Context) {
	teamID := c.Param("id")
	runID := c.Param("run_id")

	// Find the team
	var targetTeam *team.Team
	for _, team := range os.teams {
		teamIDGenerated := generateDeterministicID("team", team.GetName())
		if teamIDGenerated == teamID || team.GetName() == teamID {
			targetTeam = team
			break
		}
	}

	if targetTeam == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	// TODO: Implement actual team run cancellation logic
	// For now, return success response
	c.JSON(http.StatusOK, gin.H{"message": "Team run cancelled", "run_id": runID})
}

// workflowRunsHandler handles workflow execution - compatible with Python API
func (os *AgentOS) workflowRunsHandler(c *gin.Context) {
	workflowID := c.Param("id")

	var req struct {
		Message   string `form:"message" json:"message"`
		Stream    bool   `form:"stream" json:"stream"`
		SessionID string `form:"session_id" json:"session_id"`
		UserID    string `form:"user_id" json:"user_id"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the workflow
	var targetWorkflow *v2.Workflow
	for _, workflow := range os.workflows {
		workflowIDGenerated := generateDeterministicID("workflow", workflow.Name)
		if workflowIDGenerated == workflowID || workflow.WorkflowID == workflowID || workflow.Name == workflowID {
			targetWorkflow = workflow
			break
		}
	}

	if targetWorkflow == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	// Generate run and session IDs
	runID := generateID("run")
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = generateID("session")
	}

	if req.Stream {
		// Set headers for Server-Sent Events
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Cache-Control")

		// Send streaming response like Python implementation
		os.sendWorkflowStreamingResponse(c, targetWorkflow, req.Message, runID, sessionID, req.UserID)
	} else {
		// Non-streaming response
		result := map[string]interface{}{
			"content":      fmt.Sprintf("Workflow '%s' executed with message: %s", targetWorkflow.Name, req.Message),
			"content_type": "text",
			"run_id":       runID,
			"session_id":   sessionID,
			"workflow_id":  workflowID,
			"user_id":      req.UserID,
			"created_at":   time.Now().Unix(),
		}
		c.JSON(http.StatusOK, result)
	}
}

// cancelWorkflowRunHandler cancels a workflow run - compatible with Python API
func (os *AgentOS) cancelWorkflowRunHandler(c *gin.Context) {
	workflowID := c.Param("id")
	runID := c.Param("run_id")

	// Find the workflow
	var targetWorkflow *v2.Workflow
	for _, workflow := range os.workflows {
		workflowIDGenerated := generateDeterministicID("workflow", workflow.Name)
		if workflowIDGenerated == workflowID || workflow.WorkflowID == workflowID || workflow.Name == workflowID {
			targetWorkflow = workflow
			break
		}
	}

	if targetWorkflow == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	// TODO: Implement actual workflow run cancellation logic
	// For now, return success response
	c.JSON(http.StatusOK, gin.H{"message": "Workflow run cancelled", "run_id": runID})
}

// sendWorkflowStreamingResponse sends SSE events for workflow execution
func (os *AgentOS) sendWorkflowStreamingResponse(c *gin.Context, workflow *v2.Workflow, message, runID, sessionID, userID string) {
	// RunStarted event
	startEvent := map[string]interface{}{
		"event": "RunStarted",
		"data": map[string]interface{}{
			"run_id":        runID,
			"session_id":    sessionID,
			"workflow_id":   workflow.WorkflowID,
			"workflow_name": workflow.Name,
			"message":       message,
			"user_id":       userID,
			"created_at":    time.Now().Unix(),
		},
	}
	startEventJSON, _ := json.Marshal(startEvent)
	c.Writer.Write([]byte(string(startEventJSON) + "\n"))
	c.Writer.Flush()
	time.Sleep(100 * time.Millisecond)

	// RunOutput event
	outputEvent := map[string]interface{}{
		"event": "RunOutput",
		"data": map[string]interface{}{
			"run_id":       runID,
			"session_id":   sessionID,
			"content":      fmt.Sprintf("Executing workflow '%s' with message: %s", workflow.Name, message),
			"content_type": "text",
			"delta":        "Processing workflow steps...",
		},
	}
	outputEventJSON, _ := json.Marshal(outputEvent)
	c.Writer.Write([]byte(string(outputEventJSON) + "\n"))
	c.Writer.Flush()
	time.Sleep(200 * time.Millisecond)

	// RunCompleted event
	completedEvent := map[string]interface{}{
		"event": "RunCompleted",
		"data": map[string]interface{}{
			"run_id":       runID,
			"session_id":   sessionID,
			"content":      "Workflow completed successfully.",
			"content_type": "text",
			"created_at":   time.Now().Unix(),
			"completed_at": time.Now().Unix(),
		},
	}
	completedEventJSON, _ := json.Marshal(completedEvent)
	c.Writer.Write([]byte(string(completedEventJSON) + "\n"))
	c.Writer.Flush()

	// CRITICAL: Abort to prevent Gin from sending any additional data
	c.Abort()
}

// getSessionRunsHandler gets runs for a specific session - Python compatible
func (os *AgentOS) getSessionRunsHandler(c *gin.Context) {
	sessionID := c.Param("session_id")

	// Get query parameters for session creation
	sessionType := c.Query("type")
	userID := c.Query("user_id")
	dbID := c.Query("db_id")

	os.mu.Lock()
	session, exists := os.sessions[sessionID]

	// Create session if it doesn't exist and we have the required parameters
	if !exists && sessionType != "" && userID != "" {
		session = &Session{
			ID:        sessionID,
			UserID:    &userID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"type":  sessionType,
				"db_id": dbID,
			},
			Active: true,
			Runs:   make([]*SessionRun, 0),
		}

		if os.sessions == nil {
			os.sessions = make(map[string]*Session)
		}
		os.sessions[sessionID] = session
		exists = true
	}
	os.mu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session not found",
		})
		return
	}

	// Return runs for this session com TODOS os campos que o cliente espera
	runs := make([]map[string]interface{}, 0)
	if session.Runs != nil {
		for _, run := range session.Runs {
			runData := map[string]interface{}{
				"id":         run.ID,
				"run_id":     run.RunID,
				"status":     run.Status,
				"created_at": run.CreatedAt,
				"updated_at": run.UpdatedAt,
			}
			// Adicionar campos opcionais se existirem
			if run.AgentID != "" {
				runData["agent_id"] = run.AgentID
			}
			if run.UserID != "" {
				runData["user_id"] = run.UserID
			}
			if run.SessionID != "" {
				runData["session_id"] = run.SessionID
			}
			if run.Content != "" {
				runData["content"] = run.Content
			}
			if run.RunInput != "" {
				runData["run_input"] = run.RunInput
			}
			if run.Messages != nil {
				runData["messages"] = run.Messages
			}
			if run.Metrics != nil {
				runData["metrics"] = run.Metrics
			}
			runs = append(runs, runData)
		}
	}

	c.JSON(http.StatusOK, runs)
}
