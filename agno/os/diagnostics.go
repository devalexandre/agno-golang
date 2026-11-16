package os

import (
	"context"
	"fmt"
	"net/http"
	stdos "os"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

// Model-free diagnostics, ported from Python agno/os/diagnostics.py.
// Each check reports whether it inspected configuration or made a live
// database request. Reports never include secret values: credential checks
// report presence only and errors carry the error type, not the message.

// Diagnostic statuses (worst wins for the report).
const (
	DiagnosticPass = "PASS"
	DiagnosticWarn = "WARN"
	DiagnosticFail = "FAIL"
	DiagnosticSkip = "SKIP"
)

var diagnosticSeverity = map[string]int{DiagnosticFail: 3, DiagnosticWarn: 2, DiagnosticPass: 1, DiagnosticSkip: 0}

// CheckResult is one diagnostic check.
type CheckResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
	Evidence string `json:"evidence"` // configuration or live
}

// DiagnosticReport is the result of RunDiagnostics.
type DiagnosticReport struct {
	RunID     string        `json:"run_id"`
	CreatedAt int64         `json:"created_at"`
	Status    string        `json:"status"`
	Checks    []CheckResult `json:"checks"`
}

// RunDiagnostics checks settings, database access and component wiring
// without calling any model. requiredEnv lists environment variables whose
// presence is required.
func (os *AgentOS) RunDiagnostics(ctx context.Context, requiredEnv ...string) *DiagnosticReport {
	checks := []CheckResult{
		os.checkDatabase(ctx),
		os.checkAuthorization(),
		os.checkComponents(),
		os.checkKnowledge(),
		{Name: "interfaces", Status: DiagnosticPass, Detail: fmt.Sprintf("%d interfaces configured; delivery not tested", len(os.interfaces)), Evidence: "configuration"},
	}
	env := append([]string{}, requiredEnv...)
	sort.Strings(env)
	for _, key := range env {
		ok := stdos.Getenv(key) != ""
		r := CheckResult{Name: "env:" + key, Status: DiagnosticFail, Detail: "Not set", Evidence: "configuration"}
		if ok {
			r.Status, r.Detail = DiagnosticPass, "Set"
		}
		checks = append(checks, r)
	}
	status := DiagnosticSkip
	for _, c := range checks {
		if diagnosticSeverity[c.Status] > diagnosticSeverity[status] {
			status = c.Status
		}
	}
	return &DiagnosticReport{RunID: newUUID(), CreatedAt: time.Now().Unix(), Status: status, Checks: checks}
}

func (os *AgentOS) checkDatabase(ctx context.Context) (res CheckResult) {
	res = CheckResult{Name: "database", Evidence: "configuration"}
	if len(os.osDBOrder) == 0 {
		res.Status, res.Detail = DiagnosticSkip, "No OS database configured"
		return
	}
	defer func() {
		if r := recover(); r != nil {
			res.Status, res.Detail = DiagnosticFail, fmt.Sprintf("Check failed (%T)", r)
		}
	}()
	for _, id := range os.osDBOrder {
		if _, _, err := os.osDBs[id].ListSessions(ctx, SessionQuery{Limit: 1, Page: 1}); err != nil {
			res.Status, res.Evidence = DiagnosticFail, "live"
			res.Detail = fmt.Sprintf("Session read failed on database '%s' (%T)", id, err)
			return
		}
	}
	res.Status, res.Evidence = DiagnosticPass, "live"
	res.Detail = fmt.Sprintf("Session read succeeded on %d database(s)", len(os.osDBOrder))
	return
}

func (os *AgentOS) checkAuthorization() CheckResult {
	r := CheckResult{Name: "authorization", Evidence: "configuration"}
	switch {
	case os.jwt != nil && os.jwt.Authorization:
		r.Status, r.Detail = DiagnosticPass, "Authorization enabled"
		if len(os.jwt.VerificationKeys) == 0 {
			r.Status, r.Detail = DiagnosticFail, "JWT authorization enabled without verification keys"
		}
	case os.jwt != nil:
		r.Status, r.Detail = DiagnosticWarn, "JWT authentication enabled; RBAC scope checks disabled"
	case os.settings != nil && os.settings.SecurityKey != "":
		r.Status, r.Detail = DiagnosticWarn, "Security key set; every authenticated caller has full access"
	default:
		r.Status, r.Detail = DiagnosticWarn, "Authorization disabled; every caller is anonymous"
	}
	return r
}

func (os *AgentOS) checkComponents() CheckResult {
	r := CheckResult{Name: "components", Evidence: "configuration"}
	dup := func(ids []string) bool {
		seen := map[string]bool{}
		for _, id := range ids {
			if id == "" || seen[id] {
				return true
			}
			seen[id] = true
		}
		return false
	}
	var agents, teams, workflows []string
	for _, a := range os.agents {
		agents = append(agents, a.GetName())
	}
	for _, t := range os.teams {
		teams = append(teams, t.GetName())
	}
	for _, w := range os.workflows {
		workflows = append(workflows, w.WorkflowID)
	}
	if dup(agents) || dup(teams) || dup(workflows) {
		r.Status, r.Detail = DiagnosticFail, "A component has no id or two components of one type share an id"
		return r
	}
	n := len(agents) + len(teams) + len(workflows)
	r.Status, r.Detail = DiagnosticPass, fmt.Sprintf("%d components registered with unique ids within each type", n)
	return r
}

func (os *AgentOS) checkKnowledge() CheckResult {
	r := CheckResult{Name: "knowledge", Evidence: "configuration"}
	if len(os.knowledgeInstances) == 0 {
		r.Status, r.Detail = DiagnosticSkip, "No knowledge base with a contents database"
		return r
	}
	r.Status, r.Detail = DiagnosticPass, fmt.Sprintf("%d knowledge base(s) with a contents database", len(os.knowledgeInstances))
	return r
}

// diagnosticsHandler serves GET /diagnostics (?required_env=A,B).
func (os *AgentOS) diagnosticsHandler(c *gin.Context) {
	var env []string
	for _, v := range c.QueryArray("required_env") {
		for _, k := range splitComma(v) {
			env = append(env, k)
		}
	}
	c.JSON(http.StatusOK, os.RunDiagnostics(c.Request.Context(), env...))
}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if part := trimSpace(s[start:i]); part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
