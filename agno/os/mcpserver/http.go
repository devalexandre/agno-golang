package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Internal headers carrying the verified identity from the HTTP layer to the
// tool handlers. Client-supplied values are always stripped first.
const (
	hdrUser   = "X-Agno-Mcp-Verified-User"
	hdrScopes = "X-Agno-Mcp-Verified-Scopes"
	hdrAuthed = "X-Agno-Mcp-Verified"
)

// ServerCardSchema is the $schema of the published Server Card.
const ServerCardSchema = "https://static.modelcontextprotocol.io/schemas/2025-09-29/server.schema.json"

var (
	localhostHosts = []string{"localhost", "127.0.0.1", "[::1]"}
	hostHeaderRe   = regexp.MustCompile(`^(?:\[[0-9A-Fa-f:.]+\]|[A-Za-z0-9._-]+)(?::[0-9]{1,5})?$`)
)

type routeKind int

const (
	routeNone routeKind = iota
	routeTransport
	routeCard
	routeToolRun
)

type route struct {
	kind     routeKind
	tool     string
	endpoint string // public transport path
	card     string // public card path
}

func (s *Server) match(r *http.Request) route {
	path := strings.TrimRight(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	if s.cfg.RootHost != "" && requestHostname(r.Host) == strings.ToLower(s.cfg.RootHost) {
		if rt := matchUnder(path, "", "/", "/server-card"); rt.kind != routeNone {
			return rt
		}
	}
	for _, p := range s.cfg.paths() {
		if rt := matchUnder(path, p, p, p+"/server-card"); rt.kind != routeNone {
			return rt
		}
	}
	return route{}
}

func matchUnder(path, prefix, endpoint, card string) route {
	base := route{endpoint: endpoint, card: card}
	switch {
	case path == endpoint || (prefix == "" && path == "/"):
		base.kind = routeTransport
	case path == prefix+"/server-card":
		base.kind = routeCard
	case strings.HasPrefix(path, prefix+"/server/tools/") && strings.HasSuffix(path, "/run"):
		name := strings.TrimSuffix(strings.TrimPrefix(path, prefix+"/server/tools/"), "/run")
		if name != "" && !strings.Contains(name, "/") {
			base.kind, base.tool = routeToolRun, name
		}
	}
	return base
}

// Handler returns the HTTP handler serving the transport, the server card and
// the tool-run endpoint on every configured path (and the RootHost).
func (s *Server) Handler() http.Handler { return s }

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt := s.match(r)
	if rt.kind == routeNone {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
		return
	}
	if hosts := r.Header.Values("Host"); len(hosts) > 1 || !hostHeaderRe.MatchString(r.Host) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_host"})
		return
	}
	if code, msg := s.checkTransportSecurity(r); code != 0 {
		writeJSON(w, code, map[string]any{"error": msg})
		return
	}
	switch rt.kind {
	case routeCard:
		if s.cfg.DisableServerCard {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
			return
		}
		s.serveCard(w, r, rt)
		return
	case routeTransport:
		if r.Method == http.MethodGet && !s.cfg.DisableServerCard && !acceptsEventStream(r.Header.Get("Accept")) {
			// A browser opening the endpoint gets the card, not a JSON-RPC 405.
			w.Header().Set("Location", rt.card)
			w.WriteHeader(http.StatusFound)
			return
		}
	}

	r, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if rt.kind == routeToolRun {
		s.serveToolRun(w, r, rt.tool)
		return
	}
	s.transport.ServeHTTP(w, r)
}

func (s *Server) isOpen() bool { return s.cfg.Authenticate == nil && !s.cfg.Authenticated }

// authenticate resolves the identity, applies the Authorize gate and rewrites
// the internal identity headers.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	r = r.Clone(r.Context())
	r.Header.Del(hdrUser)
	r.Header.Del(hdrScopes)
	r.Header.Del(hdrAuthed)
	var ident *Identity
	if s.cfg.Authenticate != nil {
		id, err := s.cfg.Authenticate(r)
		if err != nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized", "detail": err.Error()})
			return nil, false
		}
		ident = id
	}
	userID := ""
	if ident != nil {
		userID = ident.UserID
		r.Header.Set(hdrAuthed, "1")
		r.Header.Set(hdrUser, ident.UserID)
		r.Header.Set(hdrScopes, strings.Join(ident.Scopes, " "))
		r = r.WithContext(WithIdentity(r.Context(), ident))
	}
	if s.cfg.Authorize != nil && !s.cfg.Authorize(userID) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized", "detail": "Not authorized for the MCP server."})
		return nil, false
	}
	return r, true
}

func identityFromHeader(ctx context.Context, h http.Header) context.Context {
	if h.Get(hdrAuthed) != "1" {
		return ctx
	}
	id := &Identity{UserID: h.Get(hdrUser)}
	if sc := strings.TrimSpace(h.Get(hdrScopes)); sc != "" {
		id.Scopes = strings.Fields(sc)
	}
	return WithIdentity(ctx, id)
}

// checkTransportSecurity is the DNS-rebinding guard (Python
// _add_transport_security_middleware).
func (s *Server) checkTransportSecurity(r *http.Request) (int, string) {
	allowed := s.cfg.AllowedHosts
	if s.cfg.RootHost != "" {
		allowed = append(append([]string{}, allowed...), s.cfg.RootHost)
	}
	if allowed == nil {
		if !s.isOpen() || s.cfg.DisableHostCheck {
			return 0, ""
		}
		allowed = []string{}
	}
	set := map[string]bool{}
	for _, h := range append(allowed, localhostHosts...) {
		set[requestHostname(h)] = true
	}
	if !hostAllowed(requestHostname(r.Host), set) {
		return http.StatusBadRequest, "invalid_host"
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		for _, o := range s.cfg.AllowedOrigins {
			if o == origin {
				return 0, ""
			}
		}
		if !hostAllowed(originHostname(origin), set) {
			return http.StatusBadRequest, "invalid_origin"
		}
	}
	return 0, ""
}

func requestHostname(host string) string {
	v := strings.ToLower(strings.TrimSpace(host))
	if strings.HasPrefix(v, "[") {
		if end := strings.Index(v, "]"); end != -1 {
			return v[:end+1]
		}
		return v
	}
	if h, _, err := net.SplitHostPort(v); err == nil {
		return h
	}
	return v
}

func originHostname(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	h := strings.ToLower(u.Hostname())
	if strings.Contains(h, ":") {
		return "[" + h + "]"
	}
	return h
}

func hostAllowed(host string, set map[string]bool) bool {
	if set[host] {
		return true
	}
	for p := range set {
		if strings.HasPrefix(p, "*.") && strings.HasSuffix(host, p[1:]) {
			return true
		}
	}
	return false
}

func acceptsEventStream(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		if strings.TrimSpace(strings.SplitN(part, ";", 2)[0]) == "text/event-stream" {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------- server card

var (
	cardSlugRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	cardNSRe   = regexp.MustCompile(`[^a-z0-9.-]+`)
	ipRe       = regexp.MustCompile(`^[\d.]+$`)
)

func truncate(v string, n int) string {
	r := []rune(v)
	if len(r) <= n {
		return v
	}
	return strings.TrimRight(string(r[:n-1]), " ") + "…"
}

// cardName is the reverse-DNS card name (Python _card_name).
func cardName(hostname, serverName string) string {
	ns := ""
	if !strings.HasPrefix(hostname, "[") && !ipRe.MatchString(hostname) {
		parts := strings.Split(strings.ToLower(hostname), ".")
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		ns = strings.Join(parts, ".")
	}
	ns = strings.Trim(cardNSRe.ReplaceAllString(ns, ""), ".-")
	slug := strings.ToLower(strings.Trim(cardSlugRe.ReplaceAllString(serverName, "-"), "-."))
	if ns == "" {
		ns = "localhost"
	}
	if slug == "" {
		slug = "agentos"
	}
	if len(slug) > 100 {
		slug = strings.TrimRight(slug[:100], "-._")
	}
	if max := 200 - len(slug) - 1; len(ns) > max {
		ns = strings.TrimRight(ns[:max], "-.")
	}
	return ns + "/" + slug
}

// ServerCard returns the card for a request.
func (s *Server) ServerCard(r *http.Request, endpoint string) map[string]any {
	host := r.Host
	cardURL := s.cfg.ServerCardURL
	if cardURL != "" {
		if u, err := url.Parse(cardURL); err == nil && u.Host != "" {
			host = u.Host
		}
	} else {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		for _, p := range strings.Split(r.Header.Get("X-Forwarded-Proto"), ",") {
			if strings.TrimSpace(p) == "https" {
				scheme = "https"
			}
		}
		// X-Forwarded-Host is honoured only when it is itself an allowed host.
		if fh := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0]); fh != "" && len(s.cfg.AllowedHosts) > 0 {
			set := map[string]bool{}
			for _, h := range append(append([]string{}, s.cfg.AllowedHosts...), localhostHosts...) {
				set[requestHostname(h)] = true
			}
			if hostAllowed(requestHostname(fh), set) {
				host = fh
			}
		}
		cardURL = scheme + "://" + host + endpoint
	}
	remote := map[string]any{"type": "streamable-http", "url": cardURL}
	if !s.isOpen() {
		remote["headers"] = []map[string]any{{"name": "Authorization", "description": "Bearer token", "isRequired": true, "isSecret": true}}
	}
	desc := s.cfg.Description
	if desc == "" {
		desc = s.cfg.Name + " MCP server"
	}
	tools := make([]map[string]any, 0, len(s.order))
	for _, t := range s.Tools() {
		var m map[string]any
		if b, err := json.Marshal(t); err == nil && json.Unmarshal(b, &m) == nil {
			delete(m, "_meta")
			tools = append(tools, m)
		}
	}
	return map[string]any{
		"$schema":     ServerCardSchema,
		"name":        cardName(requestHostname(host), s.cfg.Name),
		"title":       truncate(s.cfg.Name, 100),
		"description": truncate(desc, 100),
		"version":     truncate(s.cfg.Version, 255),
		"remotes":     []any{remote},
		"tools":       tools,
	}
}

func (s *Server) serveCard(w http.ResponseWriter, r *http.Request, rt route) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	vary := "Origin, Host, X-Forwarded-Host, X-Forwarded-Proto"
	if s.cfg.ServerCardURL != "" {
		vary = "Origin"
	}
	b, _ := json.MarshalIndent(s.ServerCard(r, rt.endpoint), "", "  ")
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "public, max-age=300")
	h.Set("Vary", vary)
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "GET")
	h.Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(b, '\n'))
}

// ---------------------------------------------------------------- tool run

func runFailed(w http.ResponseWriter, code int, kind, msg string, details string, started time.Time) {
	payload := map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": msg}}, "error": kind}
	if details != "" {
		payload["details"] = details
	}
	if !started.IsZero() {
		payload["durationMs"] = time.Since(started).Milliseconds()
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Origin, Authorization")
	writeJSON(w, code, payload)
}

// serveToolRun is POST {path}/server/tools/{name}/run: the plain-HTTP runner
// an operator UI uses to test a published tool (Python _register_tool_run_endpoint).
func (s *Server) serveToolRun(w http.ResponseWriter, r *http.Request, name string) {
	if s.cfg.DisableToolRunEndpoint {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
		return
	}
	if r.Method != http.MethodPost {
		runFailed(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST.", "", time.Time{})
		return
	}
	if !s.isOpen() && IdentityFromContext(r.Context()) == nil && !s.cfg.Authenticated {
		runFailed(w, http.StatusUnauthorized, "unauthorized", "This MCP server requires an authenticated caller.", "", time.Time{})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(s.maxMsg())*4+1<<20))
	if err != nil {
		runFailed(w, http.StatusBadRequest, "invalid_json", "Request body must be a JSON object.", "", time.Time{})
		return
	}
	var payload map[string]any
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			runFailed(w, http.StatusBadRequest, "invalid_json", "Request body must be a JSON object.", "", time.Time{})
			return
		}
	}
	args := map[string]any{}
	if a, present := payload["arguments"]; present && a != nil {
		m, ok := a.(map[string]any)
		if !ok {
			runFailed(w, http.StatusBadRequest, "invalid_arguments", "'arguments' must be a JSON object.", "", time.Time{})
			return
		}
		args = m
	}
	if _, ok := s.tools[name]; !ok {
		runFailed(w, http.StatusNotFound, "tool_not_found", fmt.Sprintf("Unknown tool: %q", name), "", time.Time{})
		return
	}
	timeout := s.cfg.ToolRunTimeout
	if timeout <= 0 {
		timeout = DefaultToolRunTimeout
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	started := time.Now()
	type outcome struct {
		res *mcp.CallToolResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := s.CallTool(ctx, name, args)
		done <- outcome{res, err}
	}()
	select {
	case <-ctx.Done():
		runFailed(w, http.StatusGatewayTimeout, "timeout", fmt.Sprintf("Tool %q did not finish within %s.", name, timeout), "", started)
		return
	case out := <-done:
		if out.err != nil {
			if errors.Is(out.err, errInvalidArgs) {
				runFailed(w, http.StatusBadRequest, "invalid_arguments", fmt.Sprintf("Arguments do not match the input schema of %q.", name), out.err.Error(), started)
				return
			}
			runFailed(w, http.StatusOK, "tool_error", out.err.Error(), "", started)
			return
		}
		var resp map[string]any
		b, _ := json.Marshal(out.res)
		_ = json.Unmarshal(b, &resp)
		resp["isError"] = out.res.IsError
		resp["durationMs"] = time.Since(started).Milliseconds()
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Vary", "Origin, Authorization")
		writeJSON(w, http.StatusOK, resp)
	}
}
