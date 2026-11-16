package os

import (
	"regexp"
	"strings"
	"sync"
)

// AgentOS RBAC scopes, ported from Python agno/os/scopes.py.
//
// Scope format:
//   - global resource scopes:  resource:action          (agents:read)
//   - per-resource scopes:     resource:<id>:action     (agents:web-agent:run)
//   - wildcards:               resource:*:action        (agents:*:run)
//   - admin:                   agent_os:admin
//
// Legacy "system:*" scopes are accepted as aliases of "config:*".

// ScopeAdmin grants full access to every endpoint.
const ScopeAdmin = "agent_os:admin"

var legacyResourceAliases = map[string]string{"system": "config"}

func resourceAlias(r string) string {
	if a, ok := legacyResourceAliases[r]; ok {
		return a
	}
	return r
}

// ParsedScope is a parsed scope string.
type ParsedScope struct {
	Raw                string
	ScopeType          string // admin, global, per_resource, unknown
	Resource           string
	ResourceID         string
	Action             string
	IsWildcardResource bool
}

// ParseScope parses a scope string. adminScope defaults to ScopeAdmin.
func ParseScope(scope, adminScope string) ParsedScope {
	if adminScope == "" {
		adminScope = ScopeAdmin
	}
	if scope == adminScope {
		return ParsedScope{Raw: scope, ScopeType: "admin"}
	}
	parts := strings.Split(scope, ":")
	switch len(parts) {
	case 2:
		return ParsedScope{Raw: scope, ScopeType: "global", Resource: resourceAlias(parts[0]), Action: parts[1]}
	case 3:
		return ParsedScope{
			Raw: scope, ScopeType: "per_resource", Resource: resourceAlias(parts[0]),
			ResourceID: parts[1], Action: parts[2], IsWildcardResource: parts[1] == "*",
		}
	}
	return ParsedScope{Raw: scope, ScopeType: "unknown"}
}

// MatchesScope reports whether a user scope satisfies a required scope.
func MatchesScope(user, required ParsedScope, resourceID string) bool {
	if user.ScopeType == "admin" {
		return true
	}
	if user.ScopeType == "unknown" || required.ScopeType == "unknown" {
		return false
	}
	if user.Resource != required.Resource || user.Action != required.Action {
		return false
	}
	if required.ResourceID != "" {
		if user.IsWildcardResource || user.ResourceID == "" {
			return true
		}
		return user.ResourceID == resourceID
	}
	return user.ResourceID == "" || user.IsWildcardResource
}

// HasRequiredScopes reports whether userScopes grant every required scope.
// resourceIDs are the identifiers the accessed resource is known by (the Go
// AgentOS addresses components by generated id or by name).
func HasRequiredScopes(userScopes, requiredScopes []string, resourceType string, resourceIDs []string, adminScope string) bool {
	if len(requiredScopes) == 0 {
		return true
	}
	parsed := make([]ParsedScope, len(userScopes))
	for i, s := range userScopes {
		parsed[i] = ParseScope(s, adminScope)
		if parsed[i].ScopeType == "admin" {
			return true
		}
	}
	resourceID := ""
	if len(resourceIDs) > 0 {
		resourceID = resourceIDs[0]
	}
	for _, req := range requiredScopes {
		var required ParsedScope
		parts := strings.Split(req, ":")
		if len(parts) == 2 && resourceID != "" && resourceType != "" && resourceAlias(parts[0]) == resourceType {
			required = ParseScope(resourceType+":<resource-id>:"+parts[1], adminScope)
		} else {
			required = ParseScope(req, adminScope)
		}
		matched := false
		for _, us := range parsed {
			if len(resourceIDs) == 0 {
				if MatchesScope(us, required, "") {
					matched = true
				}
			}
			for _, id := range resourceIDs {
				if MatchesScope(us, required, id) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// AccessibleResourceIDs returns the resource ids the scopes grant for a
// resource type; {"*"} means all. action "" matches read and run.
func AccessibleResourceIDs(userScopes []string, resourceType, adminScope, action string) map[string]bool {
	allowed := []string{"read", "run"}
	if action != "" {
		allowed = []string{action}
	}
	parsed := make([]ParsedScope, len(userScopes))
	for i, s := range userScopes {
		parsed[i] = ParseScope(s, adminScope)
	}
	for _, s := range parsed {
		if s.ScopeType == "admin" {
			return map[string]bool{"*": true}
		}
		if s.Resource == resourceType && containsString(allowed, s.Action) && (s.ResourceID == "" || s.IsWildcardResource) {
			return map[string]bool{"*": true}
		}
	}
	ids := map[string]bool{}
	for _, s := range parsed {
		if s.Resource == resourceType && s.ResourceID != "" && !s.IsWildcardResource && containsString(allowed, s.Action) {
			ids[s.ResourceID] = true
		}
	}
	return ids
}

// ScopeMapping maps "METHOD /path/pattern" to the scopes the route requires.
type ScopeMapping struct {
	Route  string
	Scopes []string
}

// DefaultScopeMappings returns the default route → scope mappings, in the
// order Python's get_default_scope_mappings declares them (the first matching
// pattern wins).
func DefaultScopeMappings() []ScopeMapping {
	m := []ScopeMapping{
		{"GET /config", []string{"config:read"}},
		{"GET /models", []string{"config:read"}},
		{"GET /diagnostics", []string{"config:read"}},
		{"GET /agents", []string{"agents:read"}},
		{"GET /agents/*", []string{"agents:read"}},
		{"POST /agents", []string{"agents:write"}},
		{"PATCH /agents/*", []string{"agents:write"}},
		{"DELETE /agents/*", []string{"agents:delete"}},
		{"POST /agents/*/runs", []string{"agents:run"}},
		{"POST /agents/*/runs/*/continue", []string{"agents:run"}},
		{"POST /agents/*/runs/*/cancel", []string{"agents:run"}},
		{"POST /agents/*/runs/*/resume", []string{"agents:run"}},
		{"GET /teams", []string{"teams:read"}},
		{"GET /teams/*", []string{"teams:read"}},
		{"POST /teams", []string{"teams:write"}},
		{"PATCH /teams/*", []string{"teams:write"}},
		{"DELETE /teams/*", []string{"teams:delete"}},
		{"POST /teams/*/runs", []string{"teams:run"}},
		{"POST /teams/*/runs/*/continue", []string{"teams:run"}},
		{"POST /teams/*/runs/*/cancel", []string{"teams:run"}},
		{"POST /teams/*/runs/*/resume", []string{"teams:run"}},
		{"GET /workflows", []string{"workflows:read"}},
		{"GET /workflows/*", []string{"workflows:read"}},
		{"POST /workflows", []string{"workflows:write"}},
		{"PATCH /workflows/*", []string{"workflows:write"}},
		{"DELETE /workflows/*", []string{"workflows:delete"}},
		{"POST /workflows/*/runs", []string{"workflows:run"}},
		{"POST /workflows/*/runs/*/continue", []string{"workflows:run"}},
		{"POST /workflows/*/runs/*/cancel", []string{"workflows:run"}},
		{"POST /workflows/*/runs/*/resume", []string{"workflows:run"}},
		{"GET /sessions", []string{"sessions:read"}},
		{"GET /sessions/*", []string{"sessions:read"}},
		{"POST /sessions", []string{"sessions:write"}},
		{"POST /sessions/search", []string{"sessions:read"}},
		{"POST /sessions/*/rename", []string{"sessions:write"}},
		{"PATCH /sessions/*", []string{"sessions:write"}},
		{"DELETE /sessions", []string{"sessions:delete"}},
		{"DELETE /sessions/*", []string{"sessions:delete"}},
		{"GET /memories", []string{"memories:read"}},
		{"GET /memories/*", []string{"memories:read"}},
		{"GET /memory_topics", []string{"memories:read"}},
		{"GET /user_memory_stats", []string{"memories:read"}},
		{"POST /memories", []string{"memories:write"}},
		{"PATCH /memories/*", []string{"memories:write"}},
		{"DELETE /memories", []string{"memories:delete"}},
		{"DELETE /memories/*", []string{"memories:delete"}},
		{"POST /optimize-memories", []string{"memories:write"}},
		{"GET /knowledge/content", []string{"knowledge:read"}},
		{"GET /knowledge/content/*", []string{"knowledge:read"}},
		{"GET /knowledge/config", []string{"knowledge:read"}},
		{"POST /knowledge/content", []string{"knowledge:write"}},
		{"POST /knowledge/content/*/refresh", []string{"knowledge:write"}},
		{"PATCH /knowledge/content/*", []string{"knowledge:write"}},
		{"POST /knowledge/search", []string{"knowledge:read"}},
		{"DELETE /knowledge/content", []string{"knowledge:delete"}},
		{"DELETE /knowledge/content/*", []string{"knowledge:delete"}},
		{"GET /metrics", []string{"metrics:read"}},
		{"POST /metrics/refresh", []string{"metrics:write"}},
		{"GET /metrics/refresh/status", []string{"metrics:read"}},
		{"GET /eval-runs", []string{"evals:read"}},
		{"GET /eval-runs/*", []string{"evals:read"}},
		{"POST /eval-runs", []string{"evals:write"}},
		{"PATCH /eval-runs/*", []string{"evals:write"}},
		{"DELETE /eval-runs", []string{"evals:delete"}},
		{"GET /traces", []string{"traces:read"}},
		{"GET /traces/*", []string{"traces:read"}},
		{"GET /trace_session_stats", []string{"traces:read"}},
		{"POST /traces/search", []string{"traces:read"}},
		{"GET /schedules", []string{"schedules:read"}},
		{"GET /schedules/*", []string{"schedules:read"}},
		{"POST /schedules", []string{"schedules:write"}},
		{"PATCH /schedules/*", []string{"schedules:write"}},
		{"DELETE /schedules/*", []string{"schedules:delete"}},
		{"POST /schedules/*/enable", []string{"schedules:write"}},
		{"POST /schedules/*/disable", []string{"schedules:write"}},
		{"POST /schedules/*/trigger", []string{"schedules:write"}},
		{"GET /schedules/*/runs", []string{"schedules:read"}},
		{"GET /schedules/*/runs/*", []string{"schedules:read"}},
	}
	return m
}

// mergeScopeMappings applies overrides like Python's dict.update: existing
// routes keep their position, new routes are appended.
func mergeScopeMappings(base []ScopeMapping, overrides map[string][]string) []ScopeMapping {
	out := append([]ScopeMapping(nil), base...)
	seen := map[string]int{}
	for i, m := range out {
		seen[m.Route] = i
	}
	// Deterministic order for new keys.
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		if i, ok := seen[k]; ok {
			out[i].Scopes = overrides[k]
			continue
		}
		out = append(out, ScopeMapping{Route: k, Scopes: overrides[k]})
	}
	return out
}

var (
	fnmatchCache   = map[string]*regexp.Regexp{}
	fnmatchCacheMu sync.Mutex
	paramPattern   = regexp.MustCompile(`\{[^}]+\}`)
)

// fnmatch matches like Python's fnmatch.fnmatch: '*' matches any run of
// characters (including '/'), '?' one character.
func fnmatch(name, pattern string) bool {
	fnmatchCacheMu.Lock()
	re, ok := fnmatchCache[pattern]
	if !ok {
		var b strings.Builder
		b.WriteString("^")
		for _, r := range pattern {
			switch r {
			case '*':
				b.WriteString(".*")
			case '?':
				b.WriteString(".")
			default:
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
		}
		b.WriteString("$")
		re = regexp.MustCompile(b.String())
		fnmatchCache[pattern] = re
	}
	fnmatchCacheMu.Unlock()
	return re.MatchString(name)
}

// RequiredScopesForRoute looks up the scopes a request requires. Routes
// without a mapping require none.
func RequiredScopesForRoute(mappings []ScopeMapping, method, path string) []string {
	key := method + " " + path
	for _, m := range mappings {
		if m.Route == key {
			return m.Scopes
		}
	}
	for _, m := range mappings {
		parts := strings.SplitN(m.Route, " ", 2)
		if len(parts) != 2 || parts[0] != method {
			continue
		}
		pattern := paramPattern.ReplaceAllString(parts[1], "*")
		if fnmatch(path, pattern) {
			return m.Scopes
		}
	}
	return nil
}

var resourceTypePattern = regexp.MustCompile(`^/(?:a2a/)?(agents|teams|workflows)(?:/|$)`)

// ResourceContextFromPath extracts (resource_type, resource_id) from a path
// such as /agents/my-agent/runs.
func ResourceContextFromPath(path string) (string, string) {
	m := resourceTypePattern.FindStringSubmatch(path)
	if m == nil {
		return "", ""
	}
	rest := strings.TrimPrefix(path, "/a2a")
	rest = strings.TrimPrefix(rest, "/"+m[1])
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		return m[1], ""
	}
	return m[1], strings.SplitN(rest, "/", 2)[0]
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
