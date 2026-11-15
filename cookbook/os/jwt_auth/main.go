// JWT authentication and RBAC scopes for AgentOS.
//
// Shows: JWTConfig (HS256 shared secret) with Authorization enabled, so every
// route requires the scopes from DefaultScopeMappings (agents:read,
// agents:<id>:run, sessions:read, agent_os:admin, ...). The caller's identity
// is put in the request context as a RunAuthorization that tools and custom
// handlers read with agnoos.RunAuthorizationFromContext.
//
// Needs: nothing external at startup (the agents only call Ollama when run).
// The program prints two tokens to use:
//
//	go run ./cookbook/os/jwt_auth
//	curl -H "Authorization: Bearer $LIMITED" localhost:7777/agents     # only "support" is listed
//	curl -H "Authorization: Bearer $LIMITED" localhost:7777/sessions   # 403 Insufficient permissions
//	curl -H "Authorization: Bearer $ADMIN"   localhost:7777/sessions   # 200
//	curl -H "Authorization: Bearer $LIMITED" localhost:7777/whoami     # identity from the context
//
// Env: JWT_SECRET (default "change-me").
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	agnoos "github.com/devalexandre/agno-golang/agno/os"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "change-me"
	}
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	support, _ := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Model: model, Name: "support"})
	billing, _ := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Model: model, Name: "billing"})

	app, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{
		OSID:     "jwt-os",
		Agents:   []*agent.Agent{support, billing},
		Settings: &agnoos.AgentOSSettings{Host: "0.0.0.0", Port: 7777, EnableCORS: true, Timeout: time.Minute},
		JWT: &agnoos.JWTConfig{
			VerificationKeys: []string{secret},
			Algorithm:        "HS256",
			Authorization:    true, // enforce route scopes
			VerifyAudience:   true, // aud must be the OS id
			UserIsolation:    true, // non-admins only see their own sessions/memories
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	sign := func(sub string, scopes []string) string {
		tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": sub, "aud": "jwt-os", "scopes": scopes, "exp": time.Now().Add(time.Hour).Unix(),
		}).SignedString([]byte(secret))
		return tok
	}
	fmt.Println("LIMITED=" + sign("alice", []string{"agents:support:read", "agents:support:run"}))
	fmt.Println("ADMIN=" + sign("root", []string{agnoos.ScopeAdmin}))

	router := app.GetApp()
	router.GET("/whoami", func(c *gin.Context) {
		auth := agnoos.RunAuthorizationFromContext(c.Request.Context())
		c.JSON(200, gin.H{
			"identity":        auth.Context,
			"can_run_billing": auth.Require("agents", "run", "billing") == nil,
			"can_run_support": auth.Require("agents", "run", "support") == nil,
		})
	})
	log.Fatal(router.Run(":7777"))
}
