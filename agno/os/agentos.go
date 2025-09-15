package os

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/team"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// AgentOS represents the main AgentOS instance
type AgentOS struct {
	// Core configuration
	osID        string
	name        string
	description string
	version     string

	// Components
	agents    []*agent.Agent
	teams     []*team.Team
	workflows []*v2.Workflow

	// Configuration and settings
	config   *AgentOSConfig
	settings *AgentOSSettings

	// Runtime state
	server     *http.Server
	router     *gin.Engine
	sessions   map[string]*Session
	events     []Event
	interfaces []AgentOSInterface
	templates  *template.Template

	// Control
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex // Features
	enableMCP  bool
	telemetry  bool
	middleware []interface{}

	// WebSocket upgrader for real-time communication
	upgrader websocket.Upgrader
}

// NewAgentOS creates a new AgentOS instance
func NewAgentOS(options AgentOSOptions) (*AgentOS, error) {
	if options.OSID == "" {
		return nil, fmt.Errorf("os_id is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Set default settings if not provided
	settings := options.Settings
	if settings == nil {
		settings = &AgentOSSettings{
			Port:       7777,
			Host:       "0.0.0.0",
			Reload:     false,
			Debug:      false,
			LogLevel:   "info",
			Timeout:    30 * time.Second,
			EnableCORS: true,
			EnableMCP:  options.EnableMCP,
			Telemetry:  options.Telemetry,
		}
	}

	// Set default config if not provided
	config := options.Config
	if config == nil {
		config = &AgentOSConfig{}
	}

	name := options.OSID
	if options.Name != nil {
		name = *options.Name
	}

	description := ""
	if options.Description != nil {
		description = *options.Description
	}

	version := "1.0.0"
	if options.Version != nil {
		version = *options.Version
	}

	os := &AgentOS{
		osID:        options.OSID,
		name:        name,
		description: description,
		version:     version,
		agents:      options.Agents,
		teams:       options.Teams,
		workflows:   options.Workflows,
		config:      config,
		settings:    settings,
		sessions:    make(map[string]*Session),
		events:      make([]Event, 0),
		interfaces:  options.Interfaces,
		ctx:         ctx,
		cancel:      cancel,
		enableMCP:   options.EnableMCP,
		telemetry:   options.Telemetry,
		middleware:  options.Middleware,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
	}

	// Initialize components
	if err := os.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize AgentOS: %w", err)
	}

	return os, nil
}

// initialize initializes all AgentOS components
func (os *AgentOS) initialize() error {
	// Note: Agents, teams, and workflows are already initialized when created
	// We just need to verify they exist and optionally configure them

	// For agents, we can access their names via GetName() method
	if os.agents != nil {
		for _, agent := range os.agents {
			// Agents are already initialized when created with NewAgent
			// We could add any additional setup here if needed
			_ = agent.GetName() // Just verify the agent is valid
		}
	}

	// For teams, we can access their names via GetName() method
	if os.teams != nil {
		for _, team := range os.teams {
			// Teams are already initialized when created with NewTeam
			// We could add any additional setup here if needed
			_ = team.GetName() // Just verify the team is valid
		}
	}

	// For workflows, they have public fields
	if os.workflows != nil {
		for _, workflow := range os.workflows {
			if workflow.WorkflowID == "" {
				workflow.WorkflowID = generateID(workflow.Name)
			}
		}
	}

	// Initialize interfaces
	if os.interfaces != nil {
		for _, iface := range os.interfaces {
			if err := iface.Initialize(); err != nil {
				return fmt.Errorf("failed to initialize interface %s: %w", iface.GetName(), err)
			}
		}
	}

	// Load HTML templates
	templatePath := filepath.Join("agno", "os", "web", "templates", "*.html")
	templates, err := template.ParseGlob(templatePath)
	if err != nil {
		// If templates don't exist, create them in memory
		templates = template.New("dashboard")
		// We'll embed the template content directly for now
		log.Printf("Warning: Could not load templates from %s, using embedded templates", templatePath)
	}
	os.templates = templates

	return nil
}

// GetApp creates and returns the HTTP router/app
func (os *AgentOS) GetApp() *gin.Engine {
	if os.settings.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Enable CORS if configured
	if os.settings.EnableCORS {
		router.Use(os.corsMiddleware())
	}

	// Add custom middleware if provided
	for _, middleware := range os.middleware {
		if mw, ok := middleware.(gin.HandlerFunc); ok {
			router.Use(mw)
		}
	}

	// Setup routes
	os.setupRoutes(router)

	os.router = router
	return router
}

// setupRoutes configures all the routes for the AgentOS
func (os *AgentOS) setupRoutes(router *gin.Engine) {
	// Dashboard UI (root path) - public
	router.GET("/", os.dashboardHandler)

	// Health check endpoint - public
	router.GET("/health", os.healthHandler)
	router.HEAD("/health", os.healthHandler)

	// AgentOS discovery endpoint - critical for cloud detection
	router.GET("/ping", os.pingHandler)
	router.HEAD("/ping", os.pingHandler)
	router.GET("/status", os.statusHandler)
	router.HEAD("/status", os.statusHandler)
	router.GET("/info", os.infoHandler)
	router.HEAD("/info", os.infoHandler)

	// Public endpoints for cloud platform compatibility (listing agents, teams, workflows)
	router.GET("/agents", os.listAgentsHandler)
	router.HEAD("/agents", os.listAgentsHandler)
	router.GET("/teams", os.listTeamsHandler)
	router.HEAD("/teams", os.listTeamsHandler)
	router.GET("/workflows", os.listWorkflowsHandler)
	router.HEAD("/workflows", os.listWorkflowsHandler)

	// Configuration endpoint - public (same as Python)
	router.GET("/config", os.configHandler)
	router.HEAD("/config", os.configHandler)

	// Models endpoint - public (same as Python)
	router.GET("/models", os.modelsHandler)
	router.HEAD("/models", os.modelsHandler)

	// Agent and Team operations - needed by UI
	router.POST("/agents/:id/runs", os.agentRunsHandler)
	router.POST("/teams/:id/runs", os.teamRunsHandler)

	// Sessions - needed by UI
	router.GET("/sessions", os.sessionsHandler)
	router.POST("/sessions/:id/runs", os.sessionRunsHandler)

	// Protected endpoints that require authentication
	protected := router.Group("/")
	protected.Use(os.authMiddleware())
	{
		// Version endpoint
		protected.GET("/version", os.versionHandler)
	}

	// Base routes for agents, teams, workflows
	api := router.Group("/api/v1")
	api.Use(os.authMiddleware()) // Apply authentication to API routes
	{
		// Agent routes
		if len(os.agents) > 0 {
			agents := api.Group("/agents")
			os.setupAgentRoutes(agents)
		}

		// Team routes
		if len(os.teams) > 0 {
			teams := api.Group("/teams")
			os.setupTeamRoutes(teams)
		}

		// Workflow routes
		if len(os.workflows) > 0 {
			workflows := api.Group("/workflows")
			os.setupWorkflowRoutes(workflows)
		}

		// Session routes
		sessions := api.Group("/sessions")
		os.setupSessionRoutes(sessions)

		// Knowledge routes
		knowledge := api.Group("/knowledge")
		os.setupKnowledgeRoutes(knowledge)

		// Memory routes
		memory := api.Group("/memory")
		os.setupMemoryRoutes(memory)

		// Metrics routes
		metrics := api.Group("/metrics")
		os.setupMetricsRoutes(metrics)

		// Evals routes
		evals := api.Group("/evals")
		os.setupEvalsRoutes(evals)
	}

	// WebSocket endpoint
	router.GET("/ws", os.websocketHandler)
}

// Serve starts the AgentOS server
func (os *AgentOS) Serve() error {
	app := os.GetApp()

	os.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", os.settings.Host, os.settings.Port),
		Handler:      app,
		ReadTimeout:  os.settings.Timeout,
		WriteTimeout: os.settings.Timeout,
	}

	// Determine cloud endpoint based on environment
	cloudEndpoint := "https://os.agno.com/"
	if os.settings.Debug {
		cloudEndpoint = "https://os-stg.agno.com/"
	}

	// Determine protocol
	protocol := "http"
	if os.settings.EnableTLS {
		protocol = "https"
	}

	log.Printf("🚀 AgentOS '%s' starting on %s:%d", os.name, os.settings.Host, os.settings.Port)
	log.Printf("📊 Dashboard: %s://%s:%d", protocol, os.settings.Host, os.settings.Port)
	log.Printf("⚙️  Configuration: %s://%s:%d/config", protocol, os.settings.Host, os.settings.Port)
	log.Printf("☁️  Cloud Platform: %s", cloudEndpoint)
	log.Printf("🔑 Security Key: %s", func() string {
		if os.settings.SecurityKey != "" {
			return "configured"
		}
		return "not set (authentication disabled)"
	}())

	// Note: Cloud will auto-discover this AgentOS instance
	// Same as Python AgentOS - no manual registration needed

	// Start server with TLS if enabled
	if os.settings.EnableTLS {
		if os.settings.CertFile == "" || os.settings.KeyFile == "" {
			return fmt.Errorf("TLS enabled but cert_file or key_file not provided")
		}
		log.Printf("🔒 Starting HTTPS server with TLS certificate: %s", os.settings.CertFile)
		return os.server.ListenAndServeTLS(os.settings.CertFile, os.settings.KeyFile)
	}
	
	return os.server.ListenAndServe()
}

// Shutdown gracefully shuts down the AgentOS
func (os *AgentOS) Shutdown() error {
	log.Println("🛑 Shutting down AgentOS...")

	// Cancel context
	if os.cancel != nil {
		os.cancel()
	}

	// Shutdown interfaces
	if os.interfaces != nil {
		for _, iface := range os.interfaces {
			if err := iface.Shutdown(); err != nil {
				log.Printf("Error shutting down interface %s: %v", iface.GetName(), err)
			}
		}
	}

	// Shutdown server
	if os.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return os.server.Shutdown(ctx)
	}

	return nil
}

// GetOSID returns the OS ID
func (os *AgentOS) GetOSID() string {
	return os.osID
}

// GetName returns the OS name
func (os *AgentOS) GetName() string {
	return os.name
}

// GetDescription returns the OS description
func (os *AgentOS) GetDescription() string {
	return os.description
}

// GetVersion returns the OS version
func (os *AgentOS) GetVersion() string {
	return os.version
}

// GetAgents returns all agents
func (os *AgentOS) GetAgents() []*agent.Agent {
	return os.agents
}

// GetTeams returns all teams
func (os *AgentOS) GetTeams() []*team.Team {
	return os.teams
}

// GetWorkflows returns all workflows
func (os *AgentOS) GetWorkflows() []*v2.Workflow {
	return os.workflows
}

// GetConfig returns the OS configuration
func (os *AgentOS) GetConfig() *AgentOSConfig {
	return os.config
}

// GetSettings returns the OS settings
func (os *AgentOS) GetSettings() *AgentOSSettings {
	return os.settings
}

// registerWithCloud attempts to register this AgentOS instance with the cloud platform
func (os *AgentOS) registerWithCloud(cloudEndpoint string) {
	if os.settings.SecurityKey == "" {
		log.Printf("⚠️  No SecurityKey configured, skipping cloud registration")
		return
	}

	// Wait a bit for the server to start
	time.Sleep(2 * time.Second)

	// Prepare registration data
	registrationData := map[string]interface{}{
		"os_id":        os.osID,
		"name":         os.name,
		"description":  os.description,
		"version":      os.version,
		"host":         os.settings.Host,
		"port":         os.settings.Port,
		"local_url":    fmt.Sprintf("http://%s:%d", os.settings.Host, os.settings.Port),
		"agents":       len(os.agents),
		"teams":        len(os.teams),
		"workflows":    len(os.workflows),
		"security_key": os.settings.SecurityKey,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(registrationData)
	if err != nil {
		log.Printf("❌ Failed to marshal registration data: %v", err)
		return
	}

	// Attempt registration
	registrationURL := cloudEndpoint + "api/v1/register"
	req, err := http.NewRequest("POST", registrationURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Failed to create registration request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.settings.SecurityKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to register with cloud platform: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		log.Printf("✅ Successfully registered with cloud platform at %s", cloudEndpoint)
	} else {
		log.Printf("⚠️  Cloud registration returned status %d", resp.StatusCode)
	}
}
