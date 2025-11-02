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
	"github.com/devalexandre/agno-golang/agno/knowledge"
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
	knowledge []interface{} // Direct knowledge bases passed to AgentOS (like Python)

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

	// Database storage (like Python's self.dbs and self.knowledge_dbs)
	dbs          map[string]interface{} // All databases from agents/teams/workflows
	knowledgeDbs map[string]interface{} // Databases specifically used for knowledge

	// Knowledge base storage
	knowledgeDocs      map[string]*KnowledgeDocument
	knowledgeByDB      map[string][]*KnowledgeDocument
	knowledgeInstances []*KnowledgeInstance // Auto-discovered knowledge instances from agents/teams/direct

	// WebSocket upgrader for real-time communication
	upgrader websocket.Upgrader
}

// knowledgeToInterface converts []knowledge.Knowledge to []interface{}
func knowledgeToInterface(knowledge []knowledge.Knowledge) []interface{} {
	if knowledge == nil {
		return nil
	}
	result := make([]interface{}, len(knowledge))
	for i, k := range knowledge {
		result[i] = k
	}
	return result
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
			Port:       8080,
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
		osID:               options.OSID,
		name:               name,
		description:        description,
		version:            version,
		agents:             options.Agents,
		teams:              options.Teams,
		workflows:          options.Workflows,
		knowledge:          knowledgeToInterface(options.Knowledge), // Store direct knowledge bases
		config:             config,
		settings:           settings,
		sessions:           make(map[string]*Session),
		events:             make([]Event, 0),
		interfaces:         options.Interfaces,
		ctx:                ctx,
		cancel:             cancel,
		enableMCP:          options.EnableMCP,
		telemetry:          options.Telemetry,
		middleware:         options.Middleware,
		dbs:                make(map[string]interface{}), // Initialize databases map
		knowledgeDbs:       make(map[string]interface{}), // Initialize knowledge databases map
		knowledgeDocs:      make(map[string]*KnowledgeDocument),
		knowledgeByDB:      make(map[string][]*KnowledgeDocument),
		knowledgeInstances: make([]*KnowledgeInstance, 0),
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

	// Auto-discover databases from agents, teams, workflows, and knowledge (like Python)
	os.autoDiscoverDatabases()

	// Auto-discover knowledge instances from agents, teams, and direct OS knowledge
	os.autoDiscoverKnowledgeInstances()

	return nil
}

// autoDiscoverKnowledgeInstances discovers knowledge instances from agents, teams, and direct OS knowledge
// This matches Python's _auto_discover_knowledge_instances() behavior
func (os *AgentOS) autoDiscoverKnowledgeInstances() {
	seen := make(map[string]bool) // Track by DB ID to avoid duplicates

	// Helper function to add knowledge if not duplicate
	addKnowledgeIfNotDuplicate := func(k knowledge.Knowledge) {
		if k == nil {
			return
		}

		// Get ContentsDB - only add if it exists
		contentsDB := k.GetContentsDB()
		if contentsDB == nil {
			return
		}

		// Get the DB ID
		dbID := contentsDB.GetID()

		// Skip if already seen (deduplicate)
		if seen[dbID] {
			return
		}
		seen[dbID] = true

		// Add knowledge instance
		os.knowledgeInstances = append(os.knowledgeInstances, &KnowledgeInstance{
			Knowledge:  k,
			ContentsDB: contentsDB,
			DBID:       dbID,
		})
	}

	// 1. Collect from agents
	if os.agents != nil {
		for _, agent := range os.agents {
			agentKnowledge := agent.GetKnowledge()
			if agentKnowledge != nil {
				addKnowledgeIfNotDuplicate(agentKnowledge)
			}
		}
	}

	// 2. Collect from teams (future support)
	if os.teams != nil {
		for _, team := range os.teams {
			// Teams don't have knowledge field yet in Go implementation
			// But keeping for future compatibility
			_ = team
		}
	}

	// 3. Collect from direct OS knowledge (like Python: for knowledge_base in self.knowledge or [])
	if os.knowledge != nil {
		for _, kb := range os.knowledge {
			// Type assert to knowledge.Knowledge interface
			if k, ok := kb.(knowledge.Knowledge); ok {
				addKnowledgeIfNotDuplicate(k)
			}
		}
	}
}

// autoDiscoverDatabases auto-discovers databases from agents, teams, workflows, and knowledge
// This matches Python's _auto_discover_databases() behavior
func (os *AgentOS) autoDiscoverDatabases() {
	dbs := make(map[string]interface{})
	knowledgeDbs := make(map[string]interface{})

	// Helper function to register DB with validation
	registerDBWithValidation := func(registeredDBs map[string]interface{}, db interface{}) error {
		// Get DB ID using type assertion
		var dbID string
		if dbWithID, ok := db.(interface{ GetID() string }); ok {
			dbID = dbWithID.GetID()
		} else {
			return fmt.Errorf("database does not have GetID method")
		}

		// Check if DB with this ID already exists
		if existingDB, exists := registeredDBs[dbID]; exists {
			// Validate compatibility
			if !os.areDBInstancesCompatible(existingDB, db) {
				return fmt.Errorf(
					"database ID conflict detected: two different database instances have the same ID '%s'. "+
						"Database instances with the same ID must point to the same database with identical configuration",
					dbID,
				)
			}
		}

		registeredDBs[dbID] = db
		return nil
	}

	// 1. Collect from agents
	if os.agents != nil {
		for _, agent := range os.agents {
			// Agent's knowledge ContentsDB
			if agentKnowledge := agent.GetKnowledge(); agentKnowledge != nil {
				if contentsDB := agentKnowledge.GetContentsDB(); contentsDB != nil {
					if err := registerDBWithValidation(knowledgeDbs, contentsDB); err != nil {
						log.Printf("Warning: %v", err)
					}
				}
			}
		}
	}

	// 2. Collect from teams (future support)
	// Teams don't have knowledge field yet in Go implementation

	// 3. Collect from workflows (future support)
	// Workflows don't have DB field exposed yet

	// 4. Collect from direct OS knowledge (like Python: for knowledge_base in self.knowledge or [])
	if os.knowledge != nil {
		for _, kb := range os.knowledge {
			if k, ok := kb.(knowledge.Knowledge); ok {
				if contentsDB := k.GetContentsDB(); contentsDB != nil {
					if err := registerDBWithValidation(knowledgeDbs, contentsDB); err != nil {
						log.Printf("Warning: %v", err)
					}
				}
			}
		}
	}

	// 5. Collect from interfaces (future support)
	// Interfaces don't have GetAgent/GetTeam methods exposed yet

	os.dbs = dbs
	os.knowledgeDbs = knowledgeDbs
}

// areDBInstancesCompatible checks if two database instances are compatible
// This matches Python's _are_db_instances_compatible() behavior
func (os *AgentOS) areDBInstancesCompatible(db1, db2 interface{}) bool {
	// If they're the same object reference, they're compatible
	if db1 == db2 {
		return true
	}

	// Check if they're the same type
	if fmt.Sprintf("%T", db1) != fmt.Sprintf("%T", db2) {
		return false
	}

	// Check db_url if exists
	type HasURL interface {
		GetURL() string
	}
	if dbURL1, ok1 := db1.(HasURL); ok1 {
		if dbURL2, ok2 := db2.(HasURL); ok2 {
			if dbURL1.GetURL() != dbURL2.GetURL() {
				return false
			}
		}
	}

	// Check db_file if exists
	type HasFile interface {
		GetFile() string
	}
	if dbFile1, ok1 := db1.(HasFile); ok1 {
		if dbFile2, ok2 := db2.(HasFile); ok2 {
			if dbFile1.GetFile() != dbFile2.GetFile() {
				return false
			}
		}
	}

	// Check table names
	type HasTableNames interface {
		GetSessionTableName() string
		GetMemoryTableName() string
		GetMetricsTableName() string
		GetEvalTableName() string
		GetKnowledgeTableName() string
	}
	if db1Tables, ok1 := db1.(HasTableNames); ok1 {
		if db2Tables, ok2 := db2.(HasTableNames); ok2 {
			if db1Tables.GetSessionTableName() != db2Tables.GetSessionTableName() ||
				db1Tables.GetMemoryTableName() != db2Tables.GetMemoryTableName() ||
				db1Tables.GetMetricsTableName() != db2Tables.GetMetricsTableName() ||
				db1Tables.GetEvalTableName() != db2Tables.GetEvalTableName() ||
				db1Tables.GetKnowledgeTableName() != db2Tables.GetKnowledgeTableName() {
				return false
			}
		}
	}

	return true
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
	router.GET("/agents/:agent_id", os.getAgentHandler) // Python compatible
	router.HEAD("/agents/:agent_id", os.getAgentHandler)
	router.GET("/teams", os.listTeamsHandler)
	router.HEAD("/teams", os.listTeamsHandler)
	router.GET("/teams/:team_id", os.getTeamHandler) // Python compatible
	router.HEAD("/teams/:team_id", os.getTeamHandler)
	router.GET("/workflows", os.listWorkflowsHandler)
	router.HEAD("/workflows", os.listWorkflowsHandler)
	router.GET("/workflows/:workflow_id", os.getWorkflowHandler) // Python compatible
	router.HEAD("/workflows/:workflow_id", os.getWorkflowHandler)

	// Configuration endpoint - public (same as Python)
	router.GET("/config", os.configHandler)
	router.HEAD("/config", os.configHandler)

	// Models endpoint - public (same as Python)
	router.GET("/models", os.modelsHandler)
	router.HEAD("/models", os.modelsHandler)

	// Agent and Team operations - needed by UI (compatible with Python API)
	router.POST("/agents/:id/runs", os.agentRunsHandler)
	router.POST("/agents/:id/runs/:run_id/cancel", os.cancelAgentRunHandler)
	router.POST("/agents/:id/runs/:run_id/continue", os.continueAgentRunHandler)
	router.POST("/teams/:id/runs", os.teamRunsHandler)
	router.POST("/teams/:id/runs/:run_id/cancel", os.cancelTeamRunHandler)
	router.POST("/workflows/:id/runs", os.workflowRunsHandler)
	router.POST("/workflows/:id/runs/:run_id/cancel", os.cancelWorkflowRunHandler)

	// Sessions - needed by UI and Python compatibility
	router.GET("/sessions", os.sessionsHandler)
	router.GET("/sessions/:session_id", os.getSessionHandler) // Get individual session
	router.POST("/sessions/:session_id/runs", os.sessionRunsHandler)
	router.GET("/sessions/:session_id/runs", os.getSessionRunsHandler) // Get session runs
	protected := router.Group("/")
	protected.Use(os.authMiddleware())
	{
		// Version endpoint
		protected.GET("/version", os.versionHandler)
	}

	// Knowledge routes - compatible with Python API (uses /content not /documents)
	router.GET("/knowledge/content", os.listKnowledgeContentHandler)
	router.POST("/knowledge/content", os.createKnowledgeContentHandler)
	router.GET("/knowledge/content/:content_id", os.getKnowledgeContentHandler)
	router.PATCH("/knowledge/content/:content_id", os.updateKnowledgeContentHandler)
	router.DELETE("/knowledge/content/:content_id", os.deleteKnowledgeContentHandler)
	router.DELETE("/knowledge/content", os.deleteAllKnowledgeContentHandler)
	router.GET("/knowledge/content/:content_id/status", os.getKnowledgeContentStatusHandler)
	router.GET("/knowledge/config", os.getKnowledgeConfigHandler)
	router.POST("/knowledge/search", os.searchKnowledgeHandler)

	// Legacy knowledge/documents endpoints for backward compatibility
	router.GET("/knowledge/documents", os.listKnowledgeContentHandler)
	router.POST("/knowledge/documents", os.createKnowledgeContentHandler)
	router.GET("/knowledge/documents/:document_id", os.getKnowledgeContentHandler)
	router.PATCH("/knowledge/documents/:document_id", os.updateKnowledgeContentHandler)
	router.DELETE("/knowledge/documents/:document_id", os.deleteKnowledgeContentHandler)
	router.DELETE("/knowledge/documents", os.deleteAllKnowledgeContentHandler)
	router.GET("/knowledge/conversations", os.getKnowledgeConversationsHandler)

	// Memory routes - compatible with Python API
	router.POST("/memory/add", os.addMemoryHandler)
	router.DELETE("/memory/:memory_id", os.deleteMemoryHandler)
	router.DELETE("/memory", os.deleteAllMemoriesHandler)
	router.GET("/memory", os.getMemoriesHandler)
	router.GET("/memory/entities", os.getMemoryEntitiesHandler)
	router.GET("/memory/conversations", os.getMemoryConversationsHandler)
	router.PATCH("/memory/conversations/:conversation_id", os.updateMemoryConversationHandler)
	router.GET("/memory/run_ids/:run_id", os.getMemoriesByRunIDHandler)

	// Metrics routes - compatible with Python API
	router.GET("/metrics", os.getMetricsHandler)
	router.POST("/metrics", os.createMetricsHandler)

	// Evals routes - compatible with Python API
	router.GET("/evals", os.listEvalsHandler)
	router.POST("/evals", os.createEvalHandler)
	router.GET("/evals/:eval_id", os.getEvalHandler)
	router.POST("/evals/:eval_id/run", os.runEvalHandler)

	// WebSocket endpoints
	router.GET("/ws", os.websocketHandler)
	router.GET("/workflows/ws", os.websocketHandler) // Frontend compatibility
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
