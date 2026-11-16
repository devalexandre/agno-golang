package os

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// DefaultCORSOrigins are always allowed (Python AgnoAPISettings.cors_origin_list).
var DefaultCORSOrigins = []string{
	"http://localhost:3000",
	"https://agno.com",
	"https://www.agno.com",
	"https://app.agno.com",
	"https://os-stg.agno.com",
	"https://os.agno.com",
}

// OriginPolicy is the browser origin policy shared by CORS, auth error
// responses and streaming endpoints (Python agno/os/middleware/cors.py).
// "*" in Origins allows every origin.
type OriginPolicy struct {
	Origins  []string
	Patterns []string
	compiled []*regexp.Regexp
}

// NewOriginPolicy builds a policy from exact origins and full-match regex patterns.
func NewOriginPolicy(origins, patterns []string) *OriginPolicy {
	p := &OriginPolicy{Origins: origins, Patterns: patterns}
	for _, pat := range patterns {
		if re, err := regexp.Compile("^(?:" + pat + ")$"); err == nil {
			p.compiled = append(p.compiled, re)
		}
	}
	return p
}

// Allows reports whether a browser origin may access the API.
func (p *OriginPolicy) Allows(origin string) bool {
	if p == nil || origin == "" {
		return false
	}
	for _, o := range p.Origins {
		if o == "*" || o == origin {
			return true
		}
	}
	for _, re := range p.compiled {
		if re.MatchString(origin) {
			return true
		}
	}
	return false
}

// buildOriginPolicy resolves the configured origins plus the Agno defaults.
func (os *AgentOS) buildOriginPolicy() *OriginPolicy {
	origins := append([]string{}, os.settings.CORSAllowedOrigins...)
	for _, d := range DefaultCORSOrigins {
		if !containsString(origins, d) {
			origins = append(origins, d)
		}
	}
	return NewOriginPolicy(origins, os.settings.CORSOriginPatterns)
}

// corsMiddleware applies the unified origin policy. Disallowed origins get no
// Access-Control-Allow-Origin header (and a 400 preflight), as Starlette's
// CORSMiddleware does.
func (os *AgentOS) corsMiddleware() gin.HandlerFunc {
	policy := os.originPolicy
	return func(c *gin.Context) {
		// AgentOS identification headers (not on /health, like Python).
		if c.Request.URL.Path != "/health" {
			c.Header("X-AgentOS-Version", os.version)
			c.Header("X-AgentOS-ID", os.osID)
			c.Header("X-AgentOS-Type", "golang")
		}
		c.Header("Server", "AgentOS-Go/"+os.version)

		origin := c.GetHeader("Origin")
		allowed := policy.Allows(origin)
		if origin != "" {
			c.Writer.Header().Add("Vary", "Origin")
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Expose-Headers", "*")
		}
		isPreflight := c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != ""
		if c.Request.Method == http.MethodOptions {
			if origin != "" && !allowed && isPreflight {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			if allowed {
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
				reqHeaders := c.GetHeader("Access-Control-Request-Headers")
				if reqHeaders == "" {
					reqHeaders = "Content-Type, Authorization, Cache-Control, Accept"
				}
				c.Header("Access-Control-Allow-Headers", strings.TrimSpace(reqHeaders))
				c.Header("Access-Control-Max-Age", "600")
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
