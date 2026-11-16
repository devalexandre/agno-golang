// Unified CORS / origin policy.
//
// Shows: AgentOSSettings.CORSAllowedOrigins (exact origins, "*" for any) and
// CORSOriginPatterns (full-match regexes) on top of the Agno defaults
// (localhost:3000, os.agno.com, ...). The same policy drives CORS headers,
// preflight answers (400 for a disallowed origin, as Starlette does), the
// WebSocket origin check and auth error responses. A disallowed origin gets
// no Access-Control-Allow-Origin header.
//
// Needs: nothing external.
//
//	go run ./cookbook/os/cors_policy
//	curl -i -H "Origin: https://app.example.com" localhost:7777/health   # allowed by pattern
//	curl -i -H "Origin: https://evil.test" localhost:7777/health         # no ACAO header
//	curl -i -X OPTIONS -H "Origin: https://evil.test" -H "Access-Control-Request-Method: POST" localhost:7777/agents  # 400
package main

import (
	"log"
	"time"

	agnoos "github.com/devalexandre/agno-golang/agno/os"
)

func main() {
	app, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{
		OSID: "cors-os",
		Settings: &agnoos.AgentOSSettings{
			Host: "0.0.0.0", Port: 7777, EnableCORS: true, Timeout: time.Minute,
			CORSAllowedOrigins: []string{"https://dashboard.internal"},
			CORSOriginPatterns: []string{`https://[a-z0-9-]+\.example\.com`},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(app.Serve())
}
