package os

import (
	"context"
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

	// Connection tracking
	connectionID := generateID("ws_conn")
	isAuthenticated := false
	requiresAuth := os.settings.SecurityKey != "" // Require auth if security key is set

	// Send connection confirmation
	conn.WriteJSON(gin.H{
		"event":         "connected",
		"connection_id": connectionID,
		"message":       "Connected to workflow events. Please authenticate to continue.",
		"requires_auth": requiresAuth,
	})

	// If auth not required, mark as authenticated
	if !requiresAuth {
		isAuthenticated = true
	}

	// Handle WebSocket messages
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		// Get action from message (support both "action" and "type" fields for compatibility)
		action, ok := msg["action"].(string)
		if !ok {
			if typ, ok := msg["type"].(string); ok {
				action = typ
			}
		}

		// Handle authentication
		if action == "authenticate" {
			token, ok := msg["token"].(string)
			if !ok || token == "" {
				conn.WriteJSON(gin.H{
					"event": "auth_error",
					"error": "Token is required",
				})
				continue
			}

			// Validate token against security key
			if token == os.settings.SecurityKey || !requiresAuth {
				isAuthenticated = true
				conn.WriteJSON(gin.H{
					"event":   "authenticated",
					"message": "Authentication successful. You can now send commands.",
				})
			} else {
				conn.WriteJSON(gin.H{
					"event": "auth_error",
					"error": "Invalid token",
				})
			}
			continue
		}

		// Check authentication for other actions
		if requiresAuth && !isAuthenticated {
			conn.WriteJSON(gin.H{
				"event": "auth_required",
				"error": "Authentication required. Send authenticate action with valid token.",
			})
			continue
		}

		// Handle authenticated actions
		switch action {
		case "ping":
			conn.WriteJSON(gin.H{"event": "pong"})

		case "start-workflow", "run_workflow":
			// Execute workflow
			workflowID, _ := msg["workflow_id"].(string)
			workflowMessage, _ := msg["message"].(string)
			sessionID, _ := msg["session_id"].(string)
			// userID from websocket message (future use)
			_, _ = msg["user_id"].(string)

			if workflowID == "" {
				conn.WriteJSON(gin.H{
					"event": "error",
					"error": "workflow_id is required",
				})
				continue
			}

			// Find the workflow
			var targetWorkflow *v2.Workflow
			for _, wf := range os.workflows {
				wfID := generateDeterministicID("workflow", wf.Name)
				if wfID == workflowID || wf.Name == workflowID {
					targetWorkflow = wf
					break
				}
			}

			if targetWorkflow == nil {
				conn.WriteJSON(gin.H{
					"event": "error",
					"error": fmt.Sprintf("Workflow %s not found", workflowID),
				})
				continue
			}

			// Generate session ID if not provided
			if sessionID == "" {
				sessionID = generateID("session")
			}

			// Execute workflow with WebSocket streaming
			runID := generateID("run")

			// Send RunStarted event
			conn.WriteJSON(gin.H{
				"event":       "RunStarted",
				"created_at":  time.Now().Unix(),
				"workflow_id": workflowID,
				"run_id":      runID,
				"session_id":  sessionID,
			})

			// Run workflow (simplified version - in production should use proper streaming)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						conn.WriteJSON(gin.H{
							"event":  "error",
							"error":  fmt.Sprintf("Workflow execution failed: %v", r),
							"run_id": runID,
						})
					}
				}()

				// Execute workflow with context
				ctx := context.Background()
				result, err := targetWorkflow.Run(ctx, workflowMessage)
				if err != nil {
					conn.WriteJSON(gin.H{
						"event":  "error",
						"error":  err.Error(),
						"run_id": runID,
					})
					return
				}

				// Send content event
				content := ""
				if result != nil && result.Content != nil {
					if str, ok := result.Content.(string); ok {
						content = str
					} else {
						// Convert to JSON string if not a string
						if jsonBytes, err := json.Marshal(result.Content); err == nil {
							content = string(jsonBytes)
						}
					}
				}

				conn.WriteJSON(gin.H{
					"event":       "RunContent",
					"created_at":  time.Now().Unix(),
					"workflow_id": workflowID,
					"run_id":      runID,
					"session_id":  sessionID,
					"content":     content,
				})

				// Send RunCompleted event
				conn.WriteJSON(gin.H{
					"event":       "RunCompleted",
					"created_at":  time.Now().Unix(),
					"workflow_id": workflowID,
					"run_id":      runID,
					"session_id":  sessionID,
					"content":     content,
				})
			}()

		default:
			// Echo back for unknown actions
			conn.WriteJSON(gin.H{
				"event":            "echo",
				"original_message": msg,
			})
		}
	}
}

// Agent handlers
func (os *AgentOS) listAgentsHandler(c *gin.Context) {
	filter := accessibleResourceIDs(c)
	agents := make([]map[string]interface{}, 0, len(os.agents))
	for _, agent := range os.agents {
		if !resourceVisible(filter, generateDeterministicID("agent", agent.GetName()), agent.GetName(), agent.GetID()) {
			continue
		}
		// Get model information from agent
		modelName, modelProvider := os.getAgentModelInfo(agent)

		agents = append(agents, map[string]interface{}{
			"id":   generateDeterministicID("agent", agent.GetName()),
			"name": agent.GetName(),
			"model": map[string]interface{}{
				"name":     modelName,
				"model":    modelName,
				"provider": modelProvider,
			},
			"db_id": os.defaultDBID(),
		})
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
	filter := accessibleResourceIDs(c)
	teams := make([]map[string]interface{}, 0, len(os.teams))
	for _, team := range os.teams {
		if !resourceVisible(filter, generateDeterministicID("team", team.GetName()), team.GetName()) {
			continue
		}
		teams = append(teams, map[string]interface{}{
			"id":   generateDeterministicID("team", team.GetName()),
			"name": team.GetName(),
			"model": map[string]interface{}{
				"name":     "gpt-4",
				"model":    "gpt-4",
				"provider": "openai",
			},
			"db_id": os.defaultDBID(),
		})
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
	filter := accessibleResourceIDs(c)
	workflows := make([]map[string]interface{}, 0, len(os.workflows))
	for _, workflow := range os.workflows {
		if !resourceVisible(filter, workflow.WorkflowID, workflow.Name, generateDeterministicID("workflow", workflow.Name)) {
			continue
		}
		workflows = append(workflows, map[string]interface{}{
			"id":          workflow.WorkflowID,
			"name":        workflow.Name,
			"description": workflow.Description,
		})
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

// runCompletedMetrics builds run metrics from the agent response. Token counts
// are reported only when the model/agent supplied them; they are never invented.
func runCompletedMetrics(resp *models.RunResponse, start time.Time) RunCompletedMetrics {
	m := RunCompletedMetrics{Duration: time.Since(start).Seconds()}
	if resp == nil || resp.Metrics == nil {
		return m
	}
	m.InputTokens = metricInt(resp.Metrics["input_tokens"])
	m.OutputTokens = metricInt(resp.Metrics["output_tokens"])
	m.TotalTokens = metricInt(resp.Metrics["total_tokens"])
	if m.TotalTokens == 0 {
		m.TotalTokens = m.InputTokens + m.OutputTokens
	}
	if v, ok := resp.Metrics["time_to_first_token"].(float64); ok {
		m.TimeToFirstToken = v
	}
	return m
}

func metricInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func metricsMap(m RunCompletedMetrics) map[string]interface{} {
	return map[string]interface{}{
		"input_tokens":        m.InputTokens,
		"output_tokens":       m.OutputTokens,
		"total_tokens":        m.TotalTokens,
		"time_to_first_token": m.TimeToFirstToken,
		"duration":            m.Duration,
	}
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

	// Parse tools from JSON string
	var toolsData []map[string]interface{}
	if req.Tools != "" {
		if err := json.Unmarshal([]byte(req.Tools), &toolsData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid JSON in tools field",
				"details": err.Error(),
			})
			return
		}
	}

	// For now, continue run is implemented as a new run with the updated tools context
	// In the future, this should use actual run state restoration

	if req.Stream {
		// Set SSE headers for streaming
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

		// Send RunStarted event
		startEventData := map[string]interface{}{
			"event":      "RunStarted",
			"created_at": time.Now().Unix(),
			"agent_id":   agentID,
			"agent_name": targetAgent.GetName(),
			"run_id":     runID,
			"session_id": req.SessionID,
		}
		startEventJSON, _ := json.Marshal(startEventData)
		c.Writer.Write([]byte(fmt.Sprintf("event: RunStarted\ndata: %s\n\n", startEventJSON)))
		c.Writer.Flush()

		// Build continuation message with tools context
		continueMessage := fmt.Sprintf("Continuing from previous run. Updated tools: %s", req.Tools)

		// Run the agent
		var finalmessage string
		_ = targetAgent.RunStream(continueMessage, func(chunk []byte) error {
			finalmessage += string(chunk)

			// Send RunContent event
			contentEvent := RunContentEvent{
				CreatedAt:        time.Now().Unix(),
				Event:            "RunContent",
				AgentID:          agentID,
				AgentName:        targetAgent.GetName(),
				RunID:            runID,
				SessionID:        req.SessionID,
				Content:          string(chunk),
				ContentType:      "str",
				ReasoningContent: "",
			}

			contentEventJSON, _ := json.Marshal(contentEvent)
			c.Writer.Write([]byte(fmt.Sprintf("event: RunContent\ndata: %s\n\n", contentEventJSON)))
			c.Writer.Flush()

			return nil
		})

		// Send RunContentCompleted event
		contentCompletedEvent := RunContentCompletedEvent{
			CreatedAt: time.Now().Unix(),
			Event:     "RunContentCompleted",
			AgentID:   agentID,
			AgentName: targetAgent.GetName(),
			RunID:     runID,
			SessionID: req.SessionID,
		}
		contentCompletedJSON, _ := json.Marshal(contentCompletedEvent)
		c.Writer.Write([]byte(fmt.Sprintf("event: RunContentCompleted\ndata: %s\n\n", contentCompletedJSON)))
		c.Writer.Flush()

		// Send RunCompleted event
		runCompletedEvent := RunCompletedEvent{
			CreatedAt:   time.Now().Unix(),
			Event:       "RunCompleted",
			AgentID:     agentID,
			AgentName:   targetAgent.GetName(),
			RunID:       runID,
			SessionID:   req.SessionID,
			Content:     finalmessage,
			ContentType: "str",
		}
		runCompletedJSON, _ := json.Marshal(runCompletedEvent)
		c.Writer.Write([]byte(fmt.Sprintf("event: RunCompleted\ndata: %s\n\n", runCompletedJSON)))
		c.Writer.Flush()

	} else {
		// Non-streaming response
		continueMessage := fmt.Sprintf("Continuing from previous run. Updated tools: %s", req.Tools)

		runOpts := []agent.RunOption{}
		if req.SessionID != "" {
			runOpts = append(runOpts, agent.WithSessionID(req.SessionID))
		}
		if req.UserID != "" {
			runOpts = append(runOpts, agent.WithUserID(req.UserID))
		}

		// Convert runOpts to interface{} slice
		interfaceOpts := make([]interface{}, len(runOpts))
		for i, opt := range runOpts {
			interfaceOpts[i] = opt
		}
		response, err := targetAgent.Run(continueMessage, interfaceOpts...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to continue run",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"run_id":     runID,
			"agent_id":   agentID,
			"session_id": req.SessionID,
			"content":    response.TextContent,
		})
	}
}
