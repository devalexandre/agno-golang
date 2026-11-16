package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolHandler is the internal handler: args are already validated against the
// tool's input schema (defaults applied).
type toolHandler func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, error)

type toolDef struct {
	tool     *mcp.Tool
	tags     []string
	resolved *jsonschema.Resolved
	handler  toolHandler
}

// Server is an MCP server over a component registry.
type Server struct {
	cfg      Config
	explicit bool
	reg      *component.Registry
	mcp      *mcp.Server
	tools    map[string]*toolDef
	order    []string
	enabled  map[string]bool
	// lifecycleTargets bounds continue_run/cancel_run to exposed components
	// when the pair is registered only by riding along with exposures.
	lifecycleTargets map[string]bool
	transport        http.Handler
}

// New builds an MCP server over reg. A nil cfg serves every default tool.
func New(reg *component.Registry, cfg *Config) (*Server, error) {
	explicit := cfg != nil
	if cfg == nil {
		cfg = &Config{}
	}
	if reg == nil {
		reg = component.NewRegistry()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	s := &Server{cfg: *cfg, explicit: explicit, reg: reg, tools: map[string]*toolDef{}}
	if s.cfg.ResultMode == "" {
		s.cfg.ResultMode = ResultTrimmed
	}
	name := s.cfg.Name
	if name == "" {
		name = s.cfg.OSID
	}
	if name == "" {
		name = "AgentOS"
	}
	s.cfg.Name = name
	version := s.cfg.Version
	if version == "" {
		version = "1.0.0"
	}
	s.cfg.Version = version
	var opts *mcp.ServerOptions
	if s.cfg.Instructions != "" {
		opts = &mcp.ServerOptions{Instructions: s.cfg.Instructions}
	}
	s.mcp = mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, opts)

	s.enabled = enabledTags(&s.cfg, explicit)
	if err := s.registerBuiltins(); err != nil {
		return nil, err
	}
	for _, ct := range s.cfg.Tools {
		if err := s.registerCustom(ct); err != nil {
			return nil, err
		}
	}
	if err := s.registerExposures(); err != nil {
		return nil, err
	}
	s.transport = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcp },
		&mcp.StreamableHTTPOptions{Stateless: s.cfg.Stateless, JSONResponse: s.cfg.JSONResponse})
	return s, nil
}

// MCPServer exposes the underlying go-sdk server (e.g. to serve it over stdio
// with server.Run(ctx, &mcp.StdioTransport{})).
func (s *Server) MCPServer() *mcp.Server { return s.mcp }

// Paths returns the transport paths (primary first).
func (s *Server) Paths() []string { return s.cfg.paths() }

// RootHost returns the configured dedicated host, if any.
func (s *Server) RootHost() string { return s.cfg.RootHost }

// Tools returns the published tools in registration order.
func (s *Server) Tools() []*mcp.Tool {
	out := make([]*mcp.Tool, 0, len(s.order))
	for _, n := range s.order {
		out = append(out, s.tools[n].tool)
	}
	return out
}

// CallTool invokes a published tool directly (no transport), validating its
// arguments exactly as tools/call does. The caller identity comes from ctx
// (see WithIdentity).
func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	d, ok := s.tools[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errToolNotFound, name)
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: name, Arguments: raw}}
	return s.invoke(ctx, d, req)
}

var (
	errToolNotFound   = errors.New("unknown tool")
	errInvalidArgs    = errors.New("invalid arguments")
	errNameCollision  = errors.New("tool name collision")
	errInvalidToolNam = errors.New("invalid tool name")
)

func (s *Server) add(d *toolDef) error {
	name := d.tool.Name
	if _, dup := s.tools[name]; dup {
		return fmt.Errorf("mcp: %w: %q is already registered", errNameCollision, name)
	}
	if d.tool.InputSchema == nil {
		d.tool.InputSchema = &jsonschema.Schema{Type: "object"}
	}
	var schema *jsonschema.Schema
	switch v := d.tool.InputSchema.(type) {
	case *jsonschema.Schema:
		schema = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("mcp: tool %q: input schema: %w", name, err)
		}
		schema = &jsonschema.Schema{}
		if err := json.Unmarshal(b, schema); err != nil {
			return fmt.Errorf("mcp: tool %q: input schema: %w", name, err)
		}
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return fmt.Errorf("mcp: tool %q: input schema: %w", name, err)
	}
	d.resolved = resolved
	s.tools[name] = d
	s.order = append(s.order, name)
	s.mcp.AddTool(d.tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if req.Extra != nil && req.Extra.Header != nil {
			ctx = identityFromHeader(ctx, req.Extra.Header)
		}
		return s.invoke(ctx, d, req)
	})
	return nil
}

func (s *Server) invoke(ctx context.Context, d *toolDef, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := map[string]any{}
	if req.Params != nil && len(req.Params.Arguments) > 0 && string(req.Params.Arguments) != "null" {
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("%w: %v", errInvalidArgs, err)
		}
	}
	// Optional arguments sent as null are treated as omitted.
	for k, v := range args {
		if v == nil {
			delete(args, k)
		}
	}
	if err := d.resolved.ApplyDefaults(&args); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidArgs, err)
	}
	if err := d.resolved.Validate(args); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidArgs, err)
	}
	res, err := d.handler(ctx, req, args)
	if err != nil {
		return errorResult(err), nil
	}
	if res == nil {
		res = &mcp.CallToolResult{}
	}
	if res.Content == nil {
		res.Content = []mcp.Content{}
	}
	return res, nil
}

func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var structured any = json.RawMessage(b)
	// structuredContent must be a JSON object.
	if len(b) == 0 || b[0] != '{' {
		structured = map[string]json.RawMessage{"result": b}
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}, StructuredContent: structured}, nil
}

func (s *Server) registerCustom(ct CustomTool) error {
	if ct.Tool == nil || ct.Handler == nil {
		return fmt.Errorf("mcp: custom tool needs Tool and Handler")
	}
	if err := validateToolName(ct.Tool.Name); err != nil {
		return err
	}
	h := ct.Handler
	t := *ct.Tool
	return s.add(&toolDef{tool: &t, handler: func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, error) {
		// Hand the handler the validated arguments (defaults applied).
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		params := &mcp.CallToolParamsRaw{Name: t.Name, Arguments: raw}
		if req.Params != nil {
			params.Meta = req.Params.Meta
		}
		return h(ctx, &mcp.CallToolRequest{Session: req.Session, Params: params, Extra: req.Extra})
	}})
}

func validateToolName(name string) error {
	if !toolNameRe.MatchString(name) || len(name) > toolNameMaxLength {
		return fmt.Errorf("mcp: %w %q: must start with a letter or underscore, then letters/digits/hyphens/underscores, at most %d chars", errInvalidToolNam, name, toolNameMaxLength)
	}
	return nil
}

func (s *Server) registerExposures() error {
	exposed := map[string]bool{}
	for _, e := range s.cfg.Expose {
		c := e.Component
		if c == nil {
			return fmt.Errorf("mcp: exposure without a component")
		}
		if _, ok := s.reg.Get(c.Kind(), c.ID()); !ok {
			// Exposed components must be part of the roster, so the generic
			// tools and the lifecycle pair can reach them too.
			s.reg.Add(c)
		}
		name := e.Name
		if name == "" {
			name = c.ID()
		}
		if err := validateToolName(name); err != nil {
			return fmt.Errorf("%w (use Exposure.Name to choose a valid tool name for %s %q)", err, c.Kind(), c.ID())
		}
		if _, dup := s.tools[name]; dup {
			return fmt.Errorf("mcp: %w: exposed %s %q collides with tool %q; set Exposure.Name", errNameCollision, c.Kind(), c.ID(), name)
		}
		desc := e.Description
		if desc == "" {
			desc = c.Description()
		}
		if desc == "" {
			desc = fmt.Sprintf("Run the %s %s.", c.Kind(), c.Name())
		}
		comp := c
		if err := s.add(&toolDef{
			tool: &mcp.Tool{
				Name: name, Description: desc, Title: c.Name(),
				InputSchema: s.runSchema(false, ""),
				Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true), OpenWorldHint: ptr(true)},
			},
			handler: func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, error) {
				return s.runComponent(ctx, req, comp, args)
			},
		}); err != nil {
			return err
		}
		exposed[lifecycleKey(c.Kind(), c.ID())] = true
	}
	if s.enabled[TagLifecycle] && !s.enabled[TagCore] {
		s.lifecycleTargets = exposed
	}
	return nil
}

func lifecycleKey(k component.Kind, id string) string { return string(k) + "/" + id }

func ptr[T any](v T) *T { return &v }

func hasTag(enabled map[string]bool, tags ...string) bool {
	for _, t := range tags {
		if enabled[t] {
			return true
		}
	}
	return false
}
