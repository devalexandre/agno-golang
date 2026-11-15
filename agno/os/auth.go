package os

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWT authentication and RBAC, ported from Python agno/os/middleware/jwt.py
// and agno/run/authorization.py.

// Token sources (Python TokenSource).
const (
	TokenSourceHeader = "header"
	TokenSourceCookie = "cookie"
	TokenSourceBoth   = "both"
)

// JWTConfig configures the JWT middleware. When set on AgentOSOptions it
// replaces the static security-key check (Python authorization_enabled).
type JWTConfig struct {
	// VerificationKeys are the shared secrets (HS*) or PEM public keys
	// (RS*/ES*/PS*/EdDSA) a token may be signed with. Each is tried in turn.
	VerificationKeys []string
	// Algorithm is the expected signing algorithm. Default RS256.
	Algorithm string
	// TokenSource is header (default), cookie or both.
	TokenSource string
	// CookieName holds the token when TokenSource includes cookies. Default access_token.
	CookieName string
	// Claim names. Defaults: scopes, sub, session_id, aud.
	ScopesClaim    string
	UserIDClaim    string
	SessionIDClaim string
	AudienceClaim  string
	// VerifyAudience checks the audience claim against Audience (default: the OS id).
	VerifyAudience bool
	Audience       []string
	// Authorization enables RBAC scope checks per route. Without it the token
	// is only authenticated and its claims exposed to handlers.
	Authorization bool
	// AdminScope grants access to everything. Default agent_os:admin.
	AdminScope string
	// ScopeMappings override or extend DefaultScopeMappings ("METHOD /path": scopes).
	ScopeMappings map[string][]string
	// ExcludedRoutePaths skip authentication (fnmatch patterns). Default:
	// /, /health, /info, /docs, /redoc, /openapi.json, /docs/oauth2-redirect.
	ExcludedRoutePaths []string
	// UserIsolation scopes sessions/memories to the JWT user for non-admins.
	UserIsolation bool
}

func (c *JWTConfig) withDefaults() *JWTConfig {
	out := *c
	if out.Algorithm == "" {
		out.Algorithm = "RS256"
	}
	if out.TokenSource == "" {
		out.TokenSource = TokenSourceHeader
	}
	if out.CookieName == "" {
		out.CookieName = "access_token"
	}
	if out.ScopesClaim == "" {
		out.ScopesClaim = "scopes"
	}
	if out.UserIDClaim == "" {
		out.UserIDClaim = "sub"
	}
	if out.SessionIDClaim == "" {
		out.SessionIDClaim = "session_id"
	}
	if out.AudienceClaim == "" {
		out.AudienceClaim = "aud"
	}
	if out.AdminScope == "" {
		out.AdminScope = ScopeAdmin
	}
	if out.ExcludedRoutePaths == nil {
		out.ExcludedRoutePaths = []string{"/", "/health", "/info", "/docs", "/redoc", "/openapi.json", "/docs/oauth2-redirect"}
	}
	return &out
}

// AuthorizationContext is the identity a request runs with. It is placed in
// the request's context.Context and in every run started by the request.
type AuthorizationContext struct {
	UserID     string                 `json:"user_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Scopes     []string               `json:"scopes,omitempty"`
	Audience   interface{}            `json:"audience,omitempty"`
	Claims     map[string]interface{} `json:"claims,omitempty"`
	AdminScope string                 `json:"admin_scope,omitempty"`
}

// RunAuthorization is the caller authority available to tools during a run
// (Python agno.run.authorization.RunAuthorization). Tools read it with
// RunAuthorizationFromContext and call Require.
type RunAuthorization struct {
	Context AuthorizationContext
	// EnforceTools activates operation checks for this run.
	EnforceTools bool
	// Scheduled marks a run fired by a schedule with its owner's authority.
	Scheduled bool
}

// IsAdmin reports whether the authority holds the admin scope.
func (a *RunAuthorization) IsAdmin() bool {
	adminScope := a.Context.AdminScope
	if adminScope == "" {
		adminScope = ScopeAdmin
	}
	return containsString(a.Context.Scopes, adminScope)
}

// Require checks that the authority grants resource:action (optionally on a
// resource id), mirroring RunAuthorization.require.
func (a *RunAuthorization) Require(resource, action, resourceID string) error {
	if a == nil {
		return nil
	}
	var required string
	var ids []string
	resourceType := ""
	switch {
	case resource == "agent_os" && action == "admin":
		required = a.Context.AdminScope
		if required == "" {
			required = ScopeAdmin
		}
	default:
		required = resource + ":" + action
		if resource == "agents" || resource == "teams" || resource == "workflows" {
			resourceType = resource
			if resourceID != "" {
				ids = []string{resourceID}
			}
		}
	}
	if HasRequiredScopes(a.Context.Scopes, []string{required}, resourceType, ids, a.Context.AdminScope) {
		return nil
	}
	permission := required
	if resourceID != "" {
		permission = fmt.Sprintf("%s:%s:%s", resource, resourceID, action)
	}
	return fmt.Errorf("Permission required: %s", permission)
}

type runAuthorizationKey struct{}

// WithRunAuthorization returns a context carrying the run authority.
func WithRunAuthorization(ctx context.Context, auth *RunAuthorization) context.Context {
	return context.WithValue(ctx, runAuthorizationKey{}, auth)
}

// RunAuthorizationFromContext returns the authority of the current request
// or run, or nil when the call is unauthenticated.
func RunAuthorizationFromContext(ctx context.Context) *RunAuthorization {
	if ctx == nil {
		return nil
	}
	a, _ := ctx.Value(runAuthorizationKey{}).(*RunAuthorization)
	return a
}

// RequirePermission enforces resource:action when the context carries an
// authority with tool enforcement; it is a no-op otherwise.
func RequirePermission(ctx context.Context, resource, action, resourceID string) error {
	a := RunAuthorizationFromContext(ctx)
	if a == nil || !a.EnforceTools {
		return nil
	}
	return a.Require(resource, action, resourceID)
}

const (
	ginKeyAuth          = "agno.authorization"
	ginKeyAccessibleIDs = "agno.accessible_resource_ids"
)

func requestAuthorization(c *gin.Context) *RunAuthorization {
	if v, ok := c.Get(ginKeyAuth); ok {
		return v.(*RunAuthorization)
	}
	return nil
}

// scopedUserID mirrors get_scoped_user_id: the JWT user when user isolation
// is enabled and the caller is not an admin, else "".
func scopedUserID(c *gin.Context) string {
	auth := requestAuthorization(c)
	v, ok := c.Get("agno.os")
	if auth == nil || !ok {
		return ""
	}
	os := v.(*AgentOS)
	if os.jwt == nil || !os.jwt.UserIsolation || auth.IsAdmin() {
		return ""
	}
	return auth.Context.UserID
}

// accessibleResourceIDs returns the listing filter set by the RBAC check
// (nil = no filtering).
func accessibleResourceIDs(c *gin.Context) map[string]bool {
	if v, ok := c.Get(ginKeyAccessibleIDs); ok {
		return v.(map[string]bool)
	}
	return nil
}

func isRouteExcluded(path string, excluded []string) bool {
	for _, p := range excluded {
		if fnmatch(path, p) || fnmatch(strings.TrimRight(path, "/"), p) {
			return true
		}
	}
	return false
}

func (os *AgentOS) extractToken(c *gin.Context, cfg *JWTConfig) string {
	fromHeader := func() string {
		h := c.GetHeader("Authorization")
		if strings.HasPrefix(h, "Bearer ") {
			return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		}
		return ""
	}
	fromCookie := func() string {
		v, _ := c.Cookie(cfg.CookieName)
		return v
	}
	switch cfg.TokenSource {
	case TokenSourceCookie:
		return fromCookie()
	case TokenSourceBoth:
		if t := fromHeader(); t != "" {
			return t
		}
		return fromCookie()
	}
	return fromHeader()
}

// parseVerificationKey converts a configured key to the type jwt expects.
func parseVerificationKey(alg, key string) (interface{}, error) {
	switch {
	case strings.HasPrefix(alg, "HS"):
		return []byte(key), nil
	case strings.HasPrefix(alg, "RS"), strings.HasPrefix(alg, "PS"):
		return jwt.ParseRSAPublicKeyFromPEM([]byte(key))
	case strings.HasPrefix(alg, "ES"):
		return jwt.ParseECPublicKeyFromPEM([]byte(key))
	case alg == "EdDSA":
		return jwt.ParseEdPublicKeyFromPEM([]byte(key))
	}
	block, _ := pem.Decode([]byte(key))
	if block == nil {
		return nil, fmt.Errorf("unsupported algorithm %s", alg)
	}
	return x509.ParsePKIXPublicKey(block.Bytes)
}

// validateToken verifies signature, expiry and (optionally) audience.
func (os *AgentOS) validateToken(token string, cfg *JWTConfig) (jwt.MapClaims, error) {
	if len(cfg.VerificationKeys) == 0 {
		return nil, errors.New("no JWT verification key configured")
	}
	var lastErr error
	for _, raw := range cfg.VerificationKeys {
		key, err := parseVerificationKey(cfg.Algorithm, raw)
		if err != nil {
			lastErr = err
			continue
		}
		claims := jwt.MapClaims{}
		_, err = jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) { return key, nil },
			jwt.WithValidMethods([]string{cfg.Algorithm}))
		if err != nil {
			lastErr = err
			if errors.Is(err, jwt.ErrTokenExpired) {
				return nil, err
			}
			continue
		}
		if cfg.VerifyAudience {
			expected := cfg.Audience
			if len(expected) == 0 {
				expected = []string{os.osID}
			}
			var tokenAud []string
			switch v := claims[cfg.AudienceClaim].(type) {
			case string:
				tokenAud = []string{v}
			case []interface{}:
				for _, a := range v {
					if s, ok := a.(string); ok {
						tokenAud = append(tokenAud, s)
					}
				}
			case nil:
				return nil, fmt.Errorf("Token is missing the \"%s\" claim", cfg.AudienceClaim)
			}
			ok := false
			for _, a := range tokenAud {
				if containsString(expected, a) {
					ok = true
				}
			}
			if !ok {
				return nil, fmt.Errorf("Invalid audience. Expected one of: %v, got: %v", expected, tokenAud)
			}
		}
		return claims, nil
	}
	return nil, lastErr
}

func claimScopes(v interface{}) []string {
	switch s := v.(type) {
	case string:
		return strings.Fields(s)
	case []interface{}:
		out := make([]string, 0, len(s))
		for _, x := range s {
			if str, ok := x.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

func claimString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

// authenticationMiddleware applies JWT (with optional RBAC) when configured,
// else the static security key, to every non-excluded route.
func (os *AgentOS) authenticationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("agno.os", os)
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if os.jwt == nil {
			if os.settings == nil || os.settings.SecurityKey == "" || isRouteExcluded(path, securityKeyExcludedRoutes) {
				c.Next()
				return
			}
			h := c.GetHeader("Authorization")
			if h == "" {
				httpError(c, http.StatusUnauthorized, "Authorization header required")
				return
			}
			if !strings.HasPrefix(h, "Bearer ") {
				httpError(c, http.StatusUnauthorized, "Invalid authorization format")
				return
			}
			if strings.TrimPrefix(h, "Bearer ") != os.settings.SecurityKey {
				httpError(c, http.StatusUnauthorized, "Invalid authentication token")
				return
			}
			c.Next()
			return
		}

		cfg := os.jwt
		if isRouteExcluded(path, cfg.ExcludedRoutePaths) {
			c.Next()
			return
		}
		token := os.extractToken(c, cfg)
		if token == "" {
			msg := "Authorization header missing"
			if cfg.TokenSource == TokenSourceCookie {
				msg = fmt.Sprintf("JWT cookie '%s' missing", cfg.CookieName)
			} else if cfg.TokenSource == TokenSourceBoth {
				msg = fmt.Sprintf("JWT token missing from both Authorization header and '%s' cookie", cfg.CookieName)
			}
			httpError(c, http.StatusUnauthorized, msg)
			return
		}
		claims, err := os.validateToken(token, cfg)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				httpError(c, http.StatusUnauthorized, "Token has expired")
				return
			}
			httpError(c, http.StatusUnauthorized, fmt.Sprintf("Invalid token: %v", err))
			return
		}
		authCtx := AuthorizationContext{
			UserID:     claimString(claims[cfg.UserIDClaim]),
			SessionID:  claimString(claims[cfg.SessionIDClaim]),
			Scopes:     claimScopes(claims[cfg.ScopesClaim]),
			Audience:   claims[cfg.AudienceClaim],
			Claims:     map[string]interface{}(claims),
			AdminScope: cfg.AdminScope,
		}
		auth := &RunAuthorization{Context: authCtx, EnforceTools: cfg.Authorization}

		if cfg.Authorization {
			required := RequiredScopesForRoute(os.scopeMappings, c.Request.Method, path)
			if len(required) > 0 {
				resourceType, resourceID := ResourceContextFromPath(path)
				var ids []string
				if resourceID != "" {
					ids = os.resourceAliases(resourceType, resourceID)
				}
				allowed := HasRequiredScopes(authCtx.Scopes, required, resourceType, ids, cfg.AdminScope)
				family := strings.SplitN(required[0], ":", 2)[0]
				if !allowed && c.Request.Method == http.MethodGet && resourceID == "" && resourceType != "" && family == resourceType {
					action := ""
					if i := strings.LastIndex(required[0], ":"); i >= 0 {
						action = required[0][i+1:]
					}
					c.Set(ginKeyAccessibleIDs, AccessibleResourceIDs(authCtx.Scopes, resourceType, cfg.AdminScope, action))
					allowed = true
				}
				if !allowed {
					detail := "Insufficient permissions"
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"detail":          detail,
						"error":           detail,
						"required_scopes": required,
					})
					return
				}
			}
		}
		c.Set(ginKeyAuth, auth)
		c.Request = c.Request.WithContext(WithRunAuthorization(c.Request.Context(), auth))
		c.Next()
	}
}

// securityKeyExcludedRoutes are public when only a security key is set.
var securityKeyExcludedRoutes = []string{"/", "/health", "/info", "/ping", "/status", "/ws", "/workflows/ws", "/docs", "/openapi.json"}

// resourceAliases returns every identifier a component is known by (the
// path value plus the component's generated id and name).
func (os *AgentOS) resourceAliases(resourceType, id string) []string {
	ids := []string{id}
	add := func(v string) {
		if v != "" && !containsString(ids, v) {
			ids = append(ids, v)
		}
	}
	switch resourceType {
	case "agents":
		if a := os.findAgent(id); a != nil {
			add(generateDeterministicID("agent", a.GetName()))
			add(a.GetName())
			add(a.GetID())
		}
	case "teams":
		if t := os.findTeam(id); t != nil {
			add(generateDeterministicID("team", t.GetName()))
			add(t.GetName())
		}
	case "workflows":
		if w := os.findWorkflow(id); w != nil {
			add(w.WorkflowID)
			add(w.Name)
			add(generateDeterministicID("workflow", w.Name))
		}
	}
	return ids
}

// resourceVisible applies a listing filter to a component known by ids.
func resourceVisible(filter map[string]bool, ids ...string) bool {
	if filter == nil || filter["*"] {
		return true
	}
	for _, id := range ids {
		if filter[id] {
			return true
		}
	}
	return false
}
