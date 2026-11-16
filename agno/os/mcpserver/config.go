// Package mcpserver serves an AgentOS (or any component.Registry) as an MCP
// server over the streamable HTTP transport. It is the Go port of Python's
// agno/os/mcp.py, agno/os/config.py:MCPConfig and
// agno/os/middleware/mcp_routing.py.
//
// The default tool surface (8 tools) is tagged by domain:
//
//	core      get_agentos_config, run_agent, run_team, run_workflow, continue_run, cancel_run
//	session   get_sessions, get_session_runs
//	lifecycle continue_run, cancel_run (dual-tagged with core)
//
// New(reg, nil) serves every default tool, like Python's AgentOS(mcp=True).
// With a Config, default tools are opt-in (Config.DefaultTools), components can
// be published as individually named tools (Config.Expose) and
// Config.LifecycleTools adds continue_run/cancel_run bounded to the exposed
// components.
package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Built-in tool tags (Python MCP_BUILTIN_TAGS).
const (
	TagCore      = "core"
	TagSession   = "session"
	TagLifecycle = "lifecycle"
)

// Result modes for the run tools.
const (
	ResultTrimmed = "trimmed"
	ResultFull    = "full"
)

// Default bounds applied to tool arguments.
const (
	DefaultMaxMessageLength = 100_000
	DefaultMaxIDLength      = 256
	DefaultMaxPageSize      = 100
	DefaultMaxRequirements  = 100
	DefaultToolRunTimeout   = 120 * time.Second
	defaultPath             = "/mcp"
	toolNameMaxLength       = 128
)

// Identity is the authenticated caller of an MCP request.
type Identity struct {
	UserID string
	Scopes []string
}

// Exposure publishes one component as its own MCP tool ("chief" instead of
// run_agent(agent_id="chief")). Python: MCPConfig(tools=[agent]) /
// agent.as_tool(name=..., description=...).
type Exposure struct {
	Component component.Component
	// Name overrides the tool name (default: the component id).
	Name string
	// Description overrides the model-facing description (default: the
	// component's description, else its name).
	Description string
}

// CustomTool is a user-provided MCP tool registered next to the built-ins.
// Handler receives the validated arguments; the authenticated caller is
// available through IdentityFromContext.
type CustomTool struct {
	Tool    *mcp.Tool
	Handler mcp.ToolHandler
}

// Config configures the MCP server. The zero value publishes no tools (see
// DefaultTools); pass a nil *Config to New for the full default surface.
type Config struct {
	// Name / Version / Instructions are sent in the initialize response.
	Name         string
	Version      string
	Instructions string
	// OSID and Description feed get_agentos_config and the server card.
	OSID        string
	Description string

	// DefaultTools opts in to the 8 built-in tools.
	DefaultTools bool
	// LifecycleTools adds continue_run/cancel_run for exposed components.
	LifecycleTools bool
	// IncludeTags / ExcludeTags scope the enabled default tools by tag.
	// A nil IncludeTags means "every tag but lifecycle"; an empty, non-nil
	// slice means no built-in tool.
	IncludeTags []string
	ExcludeTags []string

	// Expose publishes components as named tools.
	Expose []Exposure
	// Tools are custom tools (see FromToolkit for Agno toolkits).
	Tools []CustomTool

	// ResultMode is "trimmed" (default) or "full".
	ResultMode string

	// Sessions backs the session tools. Nil disables them.
	Sessions component.SessionStore

	// Bounds for tool arguments. Zero means the default.
	MaxMessageLength int
	MaxIDLength      int
	MaxPageSize      int

	// Path is the transport path (default "/mcp"); PathAliases are extra
	// transport paths; RootHost serves "/", "/server-card" and "/server/tools"
	// on that exact Host.
	Path        string
	PathAliases []string
	RootHost    string

	// DisableServerCard turns off {path}/server-card and the browser redirect.
	DisableServerCard bool
	// ServerCardURL is the public endpoint URL advertised verbatim by the card.
	ServerCardURL string
	// DisableToolRunEndpoint turns off POST {path}/server/tools/{name}/run.
	DisableToolRunEndpoint bool
	// ToolRunTimeout bounds one tool-run endpoint call (default 120s).
	ToolRunTimeout time.Duration

	// Authenticate resolves the caller (nil: anonymous). An error is a 401.
	Authenticate func(r *http.Request) (*Identity, error)
	// Authorize is the per-request gate: given the authenticated user id
	// ("" when anonymous) return false to reject with 401 before any tool
	// runs. Python: MCPConfig.authorize.
	Authorize func(userID string) bool
	// Authenticated tells the server that an outer layer authenticates
	// requests, so it is not "open" (affects default host checks and the card).
	Authenticated bool

	// AllowedHosts enables DNS-rebinding protection (Host/Origin checks) with
	// localhost always allowed. Nil on an open server means localhost-only.
	AllowedHosts   []string
	AllowedOrigins []string
	// DisableHostCheck opts an open server out of the localhost-only default.
	DisableHostCheck bool

	// Stateless serves the transport without MCP sessions.
	Stateless bool
	// JSONResponse answers with application/json instead of SSE.
	JSONResponse bool
}

var (
	pathRe     = regexp.MustCompile(`^/(?:[A-Za-z0-9_-]+/)*[A-Za-z0-9_-]+$`)
	hostRe     = regexp.MustCompile(`^(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)*[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	toolNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
)

// Validate checks the routing settings (Python MCPConfig._validate_routing).
func (c *Config) Validate() error {
	paths := c.paths()
	seen := map[string]bool{}
	for _, p := range paths {
		if !pathRe.MatchString(p) {
			return fmt.Errorf("mcp: path %q must be a non-root absolute path with plain segments; use RootHost for /", p)
		}
		if seen[p] {
			return fmt.Errorf("mcp: duplicate path %q", p)
		}
		seen[p] = true
	}
	for _, p := range paths {
		for _, suffix := range []string{"/server-card", "/server"} {
			for _, other := range paths {
				if other == p+suffix || strings.HasPrefix(other, p+suffix+"/") {
					return fmt.Errorf("mcp: transport, server-card and server paths must not overlap (%s, %s)", p, other)
				}
			}
		}
	}
	if c.RootHost != "" && !hostRe.MatchString(c.RootHost) {
		return fmt.Errorf("mcp: RootHost must be an exact hostname without a scheme, port or wildcard")
	}
	switch c.ResultMode {
	case "", ResultTrimmed, ResultFull:
	default:
		return fmt.Errorf("mcp: ResultMode must be %q or %q", ResultTrimmed, ResultFull)
	}
	for _, tag := range append(append([]string{}, c.IncludeTags...), c.ExcludeTags...) {
		if tag != TagCore && tag != TagSession && tag != TagLifecycle {
			return fmt.Errorf("mcp: unknown tool tag %q", tag)
		}
	}
	return nil
}

func (c *Config) paths() []string {
	p := c.Path
	if p == "" {
		p = defaultPath
	}
	return append([]string{p}, c.PathAliases...)
}

func (c *Config) primaryPath() string { return c.paths()[0] }

func orDefault(v, d int) int {
	if v <= 0 {
		return d
	}
	return v
}

// enabledTags resolves which built-in tags register (Python _enabled_builtin_tags).
func enabledTags(cfg *Config, explicit bool) map[string]bool {
	all := func() map[string]bool { return map[string]bool{TagCore: true, TagSession: true} }
	if !explicit {
		return all()
	}
	enabled := map[string]bool{}
	if cfg.DefaultTools {
		if cfg.IncludeTags != nil {
			for _, t := range cfg.IncludeTags {
				enabled[t] = true
			}
		} else {
			enabled = all()
		}
		for _, t := range cfg.ExcludeTags {
			delete(enabled, t)
		}
	}
	excludesLifecycle := false
	for _, t := range cfg.ExcludeTags {
		if t == TagLifecycle {
			excludesLifecycle = true
		}
	}
	if len(cfg.Expose) > 0 && cfg.LifecycleTools && !excludesLifecycle {
		enabled[TagLifecycle] = true
	}
	return enabled
}

type identityKey struct{}

// IdentityFromContext returns the authenticated caller of a tool call, if any.
func IdentityFromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(identityKey{}).(*Identity)
	return id
}

// WithIdentity attaches an identity to ctx (used by tests and custom transports).
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}
