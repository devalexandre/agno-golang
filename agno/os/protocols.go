package os

import (
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devalexandre/agno-golang/agno/os/a2aserver"
	"github.com/devalexandre/agno-golang/agno/os/agui"
	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/os/mcpserver"
	"github.com/devalexandre/agno-golang/agno/storage"
)

// ProtocolOptions mounts protocol interfaces on the AgentOS router.
//
//	MCP  - serve the AgentOS as an MCP server (Python AgentOS(mcp=...)).
//	       AgentOSOptions.EnableMCP with a nil MCP serves every default tool.
//	A2A  - serve every agent/team/workflow over A2A (Python a2a_interface=True).
//	AGUI - serve one agent or team over AG-UI (Python interfaces=[AGUI(...)]).
type ProtocolOptions struct {
	MCP  *mcpserver.Config
	A2A  *a2aserver.Config
	AGUI *AGUIOptions
	// Extra are additional components (e.g. remote A2A agents or remote
	// AgentOS components) served by the protocol interfaces.
	Extra []component.Component
}

// AGUIOptions configures the AG-UI mount.
type AGUIOptions struct {
	agui.Config
	// Component is the agent or team to serve (default: the first agent).
	Component component.Component
}

// Registry returns the AgentOS components as a protocol-neutral registry.
func (os *AgentOS) Registry() *component.Registry {
	reg := component.RegistryFrom(os.agents, os.teams, os.workflows)
	for _, c := range os.protocols.Extra {
		reg.Add(c)
	}
	return reg
}

// sessionStore exposes the discovered storage.DB databases to the MCP session tools.
func (os *AgentOS) sessionStore() component.SessionStore {
	os.mu.RLock()
	defer os.mu.RUnlock()
	dbs := map[string]storage.DB{}
	for id, db := range os.dbs {
		if sdb, ok := db.(storage.DB); ok {
			dbs[id] = sdb
		}
	}
	if len(dbs) == 0 {
		return nil
	}
	return component.NewStorageSessionStore(dbs)
}

// bearerMatches reports whether r carries the AgentOS security key.
func (os *AgentOS) bearerMatches(r *http.Request) bool {
	key := os.settings.SecurityKey
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(key)) == 1
}

func (os *AgentOS) mountProtocols(router *gin.Engine) {
	reg := os.Registry()
	secured := os.settings != nil && os.settings.SecurityKey != ""

	if os.protocols.MCP != nil || os.enableMCP {
		var cfg *mcpserver.Config
		if os.protocols.MCP != nil {
			c := *os.protocols.MCP
			cfg = &c
		} else {
			cfg = &mcpserver.Config{DefaultTools: true} // EnableMCP: the full default surface
		}
		if cfg.OSID == "" {
			cfg.OSID = os.osID
		}
		if cfg.Name == "" {
			cfg.Name = os.name
		}
		if cfg.Description == "" {
			cfg.Description = os.description
		}
		if cfg.Version == "" {
			cfg.Version = os.version
		}
		if cfg.Sessions == nil {
			cfg.Sessions = os.sessionStore()
		}
		if secured && cfg.Authenticate == nil {
			cfg.Authenticate = func(r *http.Request) (*mcpserver.Identity, error) {
				if !os.bearerMatches(r) {
					return nil, errors.New("invalid or missing bearer token")
				}
				return &mcpserver.Identity{UserID: r.Header.Get("X-User-ID")}, nil
			}
		}
		srv, err := mcpserver.New(reg, cfg)
		if err != nil {
			log.Printf("AgentOS: MCP server disabled: %v", err)
		} else {
			h := gin.WrapH(srv)
			for _, p := range srv.Paths() {
				router.Any(p, h)
				router.Any(p+"/*mcp", h)
			}
			if host := srv.RootHost(); host != "" {
				router.Use(func(c *gin.Context) {
					if strings.EqualFold(strings.Split(c.Request.Host, ":")[0], host) {
						p := c.Request.URL.Path
						if p == "/" || p == "/server-card" || strings.HasPrefix(p, "/server/tools/") {
							srv.ServeHTTP(c.Writer, c.Request)
							c.Abort()
							return
						}
					}
					c.Next()
				})
			}
		}
	}

	if os.protocols.A2A != nil {
		cfg := *os.protocols.A2A
		if secured && cfg.Authorize == nil {
			cfg.Authorize = func(r *http.Request, _ component.Kind, _ string) error {
				if !os.bearerMatches(r) {
					return errors.New("invalid or missing bearer token")
				}
				return nil
			}
		}
		srv, err := a2aserver.New(reg, &cfg)
		if err != nil {
			log.Printf("AgentOS: A2A interface disabled: %v", err)
		} else {
			router.Any(srv.Prefix()+"/*a2a", gin.WrapH(srv))
		}
	}

	if opts := os.protocols.AGUI; opts != nil {
		comp := opts.Component
		if comp == nil {
			if agents := reg.List(component.KindAgent); len(agents) > 0 {
				comp = agents[0]
			} else if teams := reg.List(component.KindTeam); len(teams) > 0 {
				comp = teams[0]
			}
		}
		srv, err := agui.New(comp, &opts.Config)
		if err != nil {
			log.Printf("AgentOS: AG-UI interface disabled: %v", err)
		} else {
			h := gin.WrapH(srv)
			router.POST(srv.Prefix()+"/agui", h)
			router.OPTIONS(srv.Prefix()+"/agui", h)
			if srv.Prefix() != "" { // GET /status at the root is the AgentOS status route
				router.GET(srv.Prefix()+"/status", h)
			}
		}
	}
}
