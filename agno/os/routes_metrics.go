package os

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

// Metrics routes, ported from Python agno/os/routers/metrics/metrics.py
// (GET /metrics, POST /metrics/refresh, GET /metrics/refresh/status).

func parseDateParam(c *gin.Context, name string) (*time.Time, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		httpError(c, http.StatusUnprocessableEntity, fmt.Sprintf("Invalid date for '%s' (expected YYYY-MM-DD)", name))
		return nil, false
	}
	return &t, true
}

// CalculateMetrics aggregates the sessions of db into one DayMetrics per day
// (runs are attributed to the day they were created) and stores them.
func CalculateMetrics(ctx context.Context, db Database) ([]*DayMetrics, error) {
	sessions, _, err := db.ListSessions(ctx, SessionQuery{})
	if err != nil {
		return nil, err
	}
	type acc struct {
		m        *DayMetrics
		sessions map[string]map[string]bool // type -> session ids
		users    map[string]bool
		models   map[string]map[string]interface{}
	}
	days := map[string]*acc{}
	now := time.Now().UTC()
	get := func(ts int64) *acc {
		day := time.Unix(ts, 0).UTC().Truncate(24 * time.Hour)
		key := day.Format("2006-01-02")
		a, ok := days[key]
		if !ok {
			a = &acc{
				m: &DayMetrics{
					ID: newUUID(), Date: day, CreatedAt: now, UpdatedAt: now,
					TokenMetrics: map[string]interface{}{}, ModelMetrics: []map[string]interface{}{},
				},
				sessions: map[string]map[string]bool{},
				users:    map[string]bool{},
				models:   map[string]map[string]interface{}{},
			}
			days[key] = a
		}
		return a
	}
	for _, s := range sessions {
		for _, r := range s.Runs {
			a := get(r.CreatedAt)
			if a.sessions[s.SessionType] == nil {
				a.sessions[s.SessionType] = map[string]bool{}
			}
			a.sessions[s.SessionType][s.SessionID] = true
			if r.ParentRunID == "" {
				switch s.SessionType {
				case SessionTypeTeam:
					a.m.TeamRunsCount++
				case SessionTypeWorkflow:
					a.m.WorkflowRunsCount++
				default:
					a.m.AgentRunsCount++
				}
			}
			if s.UserID != "" {
				a.users[s.UserID] = true
			}
			for _, k := range []string{"input_tokens", "output_tokens", "total_tokens"} {
				if v, ok := toFloat(r.Metrics[k]); ok {
					prev, _ := toFloat(a.m.TokenMetrics[k])
					a.m.TokenMetrics[k] = prev + v
				}
			}
		}
		if len(s.Runs) == 0 {
			a := get(s.CreatedAt)
			if a.sessions[s.SessionType] == nil {
				a.sessions[s.SessionType] = map[string]bool{}
			}
			a.sessions[s.SessionType][s.SessionID] = true
		}
	}
	out := make([]*DayMetrics, 0, len(days))
	for _, a := range days {
		a.m.AgentSessionsCount = len(a.sessions[SessionTypeAgent])
		a.m.TeamSessionsCount = len(a.sessions[SessionTypeTeam])
		a.m.WorkflowSessionsCount = len(a.sessions[SessionTypeWorkflow])
		a.m.UsersCount = len(a.users)
		out = append(out, a.m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	if err := db.UpsertMetrics(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (os *AgentOS) getMetricsHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	start, ok := parseDateParam(c, "starting_date")
	if !ok {
		return
	}
	end, ok := parseDateParam(c, "ending_date")
	if !ok {
		return
	}
	if start != nil && end != nil && start.After(*end) {
		httpError(c, http.StatusUnprocessableEntity, "starting_date must be on or before ending_date")
		return
	}
	metrics, err := db.ListMetrics(c.Request.Context(), start, end)
	if err != nil {
		httpError(c, http.StatusInternalServerError, fmt.Sprintf("Error getting metrics: %v", err))
		return
	}
	if metrics == nil {
		metrics = []*DayMetrics{}
	}
	var updatedAt *time.Time
	for _, m := range metrics {
		if updatedAt == nil || m.UpdatedAt.After(*updatedAt) {
			t := m.UpdatedAt
			updatedAt = &t
		}
	}
	c.JSON(http.StatusOK, gin.H{"metrics": metrics, "updated_at": updatedAt, "refresh": os.metricsRefreshStatus()})
}

func (os *AgentOS) metricsRefreshStatus() gin.H {
	os.metricsMu.Lock()
	defer os.metricsMu.Unlock()
	if os.lastMetricsRefresh.IsZero() {
		return nil
	}
	return gin.H{"status": "COMPLETED", "completed_at": os.lastMetricsRefresh}
}

func (os *AgentOS) refreshMetricsHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	metrics, err := CalculateMetrics(c.Request.Context(), db)
	if err != nil {
		httpError(c, http.StatusInternalServerError, fmt.Sprintf("Error refreshing metrics: %v", err))
		return
	}
	os.metricsMu.Lock()
	os.lastMetricsRefresh = time.Now().UTC()
	os.metricsMu.Unlock()
	c.JSON(http.StatusOK, metrics)
}

func (os *AgentOS) metricsRefreshStatusHandler(c *gin.Context) {
	if _, ok := os.dbFromRequest(c); !ok {
		return
	}
	status := os.metricsRefreshStatus()
	if status == nil {
		status = gin.H{"status": "NEVER_RUN"}
	}
	c.JSON(http.StatusOK, status)
}
