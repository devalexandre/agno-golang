package os

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Session routes, ported from Python agno/os/routers/session/session.py.

// sessionFilterField describes one advanced-search field (SESSION_FILTER_FIELDS).
type sessionFilterField struct {
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	Type      string   `json:"type"`
	Operators []string `json:"operators"`
	Values    []string `json:"values,omitempty"`
}

var sessionStringOperators = []string{"EQ", "NEQ", "IN", "CONTAINS", "STARTSWITH"}

// sessionFilterFields is ordered like the Python dict.
var sessionFilterFields = []sessionFilterField{
	{Key: "session_id", Label: "Session ID", Type: "string", Operators: sessionStringOperators},
	{Key: "session_name", Label: "Saved Session Name", Type: "string", Operators: sessionStringOperators},
	{Key: "user_id", Label: "User ID", Type: "string", Operators: sessionStringOperators},
	{Key: "agent_id", Label: "Agent ID", Type: "string", Operators: sessionStringOperators},
	{Key: "team_id", Label: "Team ID", Type: "string", Operators: sessionStringOperators},
	{Key: "workflow_id", Label: "Workflow ID", Type: "string", Operators: sessionStringOperators},
	{Key: "session_type", Label: "Session Type", Type: "enum", Operators: []string{"EQ", "NEQ", "IN"}, Values: []string{"agent", "team", "workflow"}},
	{Key: "created_at", Label: "Created At", Type: "datetime", Operators: []string{"EQ", "NEQ", "GT", "GTE", "LT", "LTE"}},
	{Key: "updated_at", Label: "Updated At", Type: "datetime", Operators: []string{"EQ", "NEQ", "GT", "GTE", "LT", "LTE"}},
}

const maxSessionFilterDepth = 10

func lookupSessionFilterField(key string) (sessionFilterField, bool) {
	for _, f := range sessionFilterFields {
		if f.Key == key {
			return f, true
		}
	}
	return sessionFilterField{}, false
}

// normalizeSessionFilter validates a filter expression and converts datetimes
// to epoch seconds (Python agno.db.session_filters.normalize_session_filter).
func normalizeSessionFilter(expr interface{}, depth int) (map[string]interface{}, error) {
	if expr == nil && depth == 0 {
		return nil, nil
	}
	if depth > maxSessionFilterDepth {
		return nil, fmt.Errorf("Session filter exceeds maximum nesting depth")
	}
	m, ok := expr.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Each filter must be an object")
	}
	op, _ := m["op"].(string)
	switch op {
	case "AND", "OR":
		conds, ok := m["conditions"].([]interface{})
		if !ok || len(conds) == 0 || len(conds) > 100 {
			return nil, fmt.Errorf("AND/OR require between 1 and 100 conditions")
		}
		out := make([]interface{}, 0, len(conds))
		for _, c := range conds {
			n, err := normalizeSessionFilter(c, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, n)
		}
		return map[string]interface{}{"op": op, "conditions": out}, nil
	case "NOT":
		n, err := normalizeSessionFilter(m["condition"], depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"op": op, "condition": n}, nil
	}
	key, _ := m["key"].(string)
	field, ok := lookupSessionFilterField(key)
	if !ok {
		return nil, fmt.Errorf("Unknown session filter field: %v", m["key"])
	}
	if !containsString(field.Operators, op) {
		return nil, fmt.Errorf("Unsupported operator for %s: %s", key, op)
	}
	var values []interface{}
	if op == "IN" {
		values, _ = m["values"].([]interface{})
	} else {
		values = []interface{}{m["value"]}
	}
	if len(values) == 0 || len(values) > 100 {
		return nil, fmt.Errorf("IN requires between 1 and 100 values")
	}
	normalized := make([]interface{}, 0, len(values))
	for _, v := range values {
		switch field.Type {
		case "datetime":
			ts, err := filterTimestamp(v)
			if err != nil {
				return nil, fmt.Errorf("Invalid timestamp for %s", key)
			}
			normalized = append(normalized, ts)
			continue
		default:
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%s requires string values", key)
			}
			if field.Type == "enum" && !containsString(field.Values, s) {
				return nil, fmt.Errorf("Invalid value for %s: %s", key, s)
			}
			normalized = append(normalized, s)
		}
	}
	out := map[string]interface{}{"op": op, "key": key}
	if op == "IN" {
		out["values"] = normalized
	} else {
		out["value"] = normalized[0]
	}
	return out, nil
}

func filterTimestamp(v interface{}) (float64, error) {
	switch t := v.(type) {
	case bool:
		return 0, fmt.Errorf("bool")
	case float64:
		if math.IsInf(t, 0) || math.IsNaN(t) {
			return 0, fmt.Errorf("not finite")
		}
		return t, nil
	case string:
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return float64(parsed.UTC().UnixNano()) / 1e9, nil
			}
		}
	}
	return 0, fmt.Errorf("invalid timestamp")
}

// sessionFieldValue returns the value of a filter field; ok=false means NULL.
func sessionFieldValue(s *SessionRecord, key string) (interface{}, bool) {
	var v string
	switch key {
	case "session_id":
		v = s.SessionID
	case "session_name":
		v = s.SessionName
	case "user_id":
		v = s.UserID
	case "agent_id":
		v = s.AgentID
	case "team_id":
		v = s.TeamID
	case "workflow_id":
		v = s.WorkflowID
	case "session_type":
		v = s.SessionType
	case "created_at":
		return float64(s.CreatedAt), true
	case "updated_at":
		return float64(s.UpdatedAt), true
	}
	if v == "" {
		return nil, false
	}
	return v, true
}

// matchSessionFilter evaluates a normalized filter with SQL NULL semantics:
// NOT is pushed to the leaves and a NULL value never matches a leaf nor its
// negation (as in Python's session_filter_to_mongo).
func matchSessionFilter(expr map[string]interface{}, s *SessionRecord, negate bool) bool {
	op, _ := expr["op"].(string)
	switch op {
	case "AND", "OR":
		conds, _ := expr["conditions"].([]interface{})
		all := (op == "AND") != negate
		for _, c := range conds {
			m := matchSessionFilter(c.(map[string]interface{}), s, negate)
			if all && !m {
				return false
			}
			if !all && m {
				return true
			}
		}
		return all
	case "NOT":
		return matchSessionFilter(expr["condition"].(map[string]interface{}), s, !negate)
	}
	key, _ := expr["key"].(string)
	val, ok := sessionFieldValue(s, key)
	if !ok {
		return false
	}
	var result bool
	switch op {
	case "IN":
		values, _ := expr["values"].([]interface{})
		for _, want := range values {
			if want == val {
				result = true
				break
			}
		}
	case "CONTAINS":
		result = strings.Contains(strings.ToLower(val.(string)), strings.ToLower(expr["value"].(string)))
	case "STARTSWITH":
		result = strings.HasPrefix(strings.ToLower(val.(string)), strings.ToLower(expr["value"].(string)))
	default:
		result = compareFilterValues(op, val, expr["value"])
	}
	if negate {
		return !result
	}
	return result
}

func compareFilterValues(op string, have, want interface{}) bool {
	if hf, ok := have.(float64); ok {
		wf, _ := want.(float64)
		switch op {
		case "EQ":
			return hf == wf
		case "NEQ":
			return hf != wf
		case "GT":
			return hf > wf
		case "GTE":
			return hf >= wf
		case "LT":
			return hf < wf
		case "LTE":
			return hf <= wf
		}
		return false
	}
	switch op {
	case "EQ":
		return have == want
	case "NEQ":
		return have != want
	}
	return false
}

// sessionName mirrors Python get_session_name: the saved name, else the
// first user input of the session.
func sessionName(s *SessionRecord) string {
	if s.SessionName != "" {
		return s.SessionName
	}
	for _, r := range s.Runs {
		if s.SessionType == SessionTypeTeam && r.AgentID != "" {
			continue
		}
		if r.ParentRunID == "" && r.RunInput != "" {
			return r.RunInput
		}
	}
	return ""
}

func sessionTotals(s *SessionRecord) (map[string]interface{}, int) {
	totals := map[string]interface{}{}
	tokens := 0
	for _, r := range s.Runs {
		for k, v := range r.Metrics {
			if f, ok := toFloat(v); ok {
				prev, _ := toFloat(totals[k])
				totals[k] = prev + f
			}
		}
	}
	if t, ok := toFloat(totals["total_tokens"]); ok {
		tokens = int(t)
	}
	return totals, tokens
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	}
	return 0, false
}

// sessionSchema mirrors Python SessionSchema (list entries).
func sessionSchema(s *SessionRecord) gin.H {
	metrics, tokens := sessionTotals(s)
	out := gin.H{
		"session_id":   s.SessionID,
		"session_name": sessionName(s),
		"session_type": s.SessionType,
		"created_at":   unixToTime(s.CreatedAt),
		"updated_at":   unixToTime(s.UpdatedAt),
	}
	setIf := func(k string, v string) {
		if v != "" {
			out[k] = v
		}
	}
	setIf("user_id", s.UserID)
	setIf("agent_id", s.AgentID)
	setIf("team_id", s.TeamID)
	setIf("workflow_id", s.WorkflowID)
	if s.SessionState != nil {
		out["session_state"] = s.SessionState
	}
	if s.Summary != nil {
		out["session_summary"] = s.Summary
	}
	if s.Metadata != nil {
		out["metadata"] = s.Metadata
	}
	if len(metrics) > 0 {
		out["metrics"] = metrics
		out["total_tokens"] = tokens
	}
	return out
}

// sessionDetailSchema mirrors the Agent/Team/WorkflowSessionDetailSchema.
func sessionDetailSchema(s *SessionRecord) gin.H {
	out := sessionSchema(s)
	switch s.SessionType {
	case SessionTypeTeam:
		out["team_session_id"] = s.SessionID
	case SessionTypeWorkflow:
		out["workflow_session_id"] = s.SessionID
	default:
		out["agent_session_id"] = s.SessionID
	}
	history := make([]gin.H, 0, len(s.Runs)*2)
	for _, r := range s.Runs {
		if r.ParentRunID != "" {
			continue
		}
		if r.RunInput != "" {
			history = append(history, gin.H{"role": "user", "content": r.RunInput, "created_at": r.CreatedAt})
		}
		if r.Content != nil && r.Content != "" {
			history = append(history, gin.H{"role": "assistant", "content": r.Content, "created_at": r.UpdatedAt})
		}
	}
	out["chat_history"] = history
	return out
}

// runSchema mirrors Python RunSchema / TeamRunSchema / WorkflowRunSchema.
func runSchema(r *RunRecord) gin.H {
	out := gin.H{
		"run_id":     r.RunID,
		"run_index":  r.RunIndex,
		"session_id": r.SessionID,
		"created_at": unixToTime(r.CreatedAt),
	}
	setIf := func(k string, v string) {
		if v != "" {
			out[k] = v
		}
	}
	setIf("parent_run_id", r.ParentRunID)
	setIf("agent_id", r.AgentID)
	setIf("team_id", r.TeamID)
	setIf("workflow_id", r.WorkflowID)
	setIf("user_id", r.UserID)
	setIf("status", r.Status)
	setIf("cancellation_stage", r.CancellationStage)
	setIf("run_input", r.RunInput)
	if r.Content != nil {
		out["content"] = r.Content
		out["run_response_format"] = "text"
		if _, ok := r.Content.(string); !ok {
			out["run_response_format"] = "json"
		}
	}
	if r.Metrics != nil {
		out["metrics"] = r.Metrics
	}
	if len(r.Events) > 0 {
		out["events"] = r.Events
	}
	return out
}

func (os *AgentOS) listSessionsHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	limit, ok := queryInt(c, "limit", 20, 1)
	if !ok {
		return
	}
	page, ok := queryInt(c, "page", 1, 0)
	if !ok {
		return
	}
	sessionType := c.Query("type")
	if sessionType != "" && !containsString([]string{SessionTypeAgent, SessionTypeTeam, SessionTypeWorkflow}, sessionType) {
		httpError(c, http.StatusUnprocessableEntity, "Invalid session type")
		return
	}
	userID := c.Query("user_id")
	if scoped := scopedUserID(c); scoped != "" {
		userID = scoped
	}
	sessions, total, err := db.ListSessions(c.Request.Context(), SessionQuery{
		SessionType: sessionType,
		ComponentID: c.Query("component_id"),
		UserID:      userID,
		SessionName: c.Query("session_name"),
		SortBy:      c.DefaultQuery("sort_by", "created_at"),
		SortOrder:   c.DefaultQuery("sort_order", "desc"),
		Limit:       limit,
		Page:        page,
	})
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	data := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		data = append(data, sessionSchema(s))
	}
	c.JSON(http.StatusOK, paginatedResponse(data, page, limit, total))
}

func (os *AgentOS) sessionsFilterSchemaHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"fields":            sessionFilterFields,
		"logical_operators": []string{"AND", "OR", "NOT"},
	})
}

func (os *AgentOS) searchSessionsHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	var req struct {
		Filter      interface{} `json:"filter"`
		SessionType string      `json:"session_type"`
		Page        *int        `json:"page"`
		Limit       *int        `json:"limit"`
		SortBy      string      `json:"sort_by"`
		SortOrder   string      `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpError(c, http.StatusBadRequest, fmt.Sprintf("Invalid session search: %v", err))
		return
	}
	page, limit := 1, 20
	if req.Page != nil {
		page = *req.Page
	}
	if req.Limit != nil {
		limit = *req.Limit
	}
	if page < 1 || limit < 1 || limit > 100 {
		httpError(c, http.StatusUnprocessableEntity, "page must be >= 1 and limit between 1 and 100")
		return
	}
	if req.SortBy == "" {
		req.SortBy = "created_at"
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}
	if req.SortBy != "created_at" && req.SortBy != "updated_at" {
		httpError(c, http.StatusUnprocessableEntity, "sort_by must be created_at or updated_at")
		return
	}
	if req.SortOrder != "asc" && req.SortOrder != "desc" {
		httpError(c, http.StatusUnprocessableEntity, "sort_order must be asc or desc")
		return
	}
	expr, err := normalizeSessionFilter(req.Filter, 0)
	if err != nil {
		httpError(c, http.StatusBadRequest, fmt.Sprintf("Invalid session search: %v", err))
		return
	}
	q := SessionQuery{
		SessionType: req.SessionType,
		UserID:      scopedUserID(c),
		SortBy:      req.SortBy,
		SortOrder:   req.SortOrder,
		Limit:       limit,
		Page:        page,
	}
	if expr != nil {
		q.Filter = func(s *SessionRecord) bool { return matchSessionFilter(expr, s, false) }
	}
	sessions, total, err := db.ListSessions(c.Request.Context(), q)
	if err != nil {
		httpError(c, http.StatusInternalServerError, fmt.Sprintf("Error searching sessions: %v", err))
		return
	}
	data := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		data = append(data, sessionSchema(s))
	}
	c.JSON(http.StatusOK, paginatedResponse(data, page, limit, total))
}

func (os *AgentOS) createSessionHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	sessionType := c.DefaultQuery("type", SessionTypeAgent)
	if !containsString([]string{SessionTypeAgent, SessionTypeTeam, SessionTypeWorkflow}, sessionType) {
		httpError(c, http.StatusUnprocessableEntity, "Invalid session type")
		return
	}
	var req struct {
		SessionID    string                 `json:"session_id"`
		SessionName  string                 `json:"session_name"`
		SessionState map[string]interface{} `json:"session_state"`
		Metadata     map[string]interface{} `json:"metadata"`
		UserID       string                 `json:"user_id"`
		AgentID      string                 `json:"agent_id"`
		TeamID       string                 `json:"team_id"`
		WorkflowID   string                 `json:"workflow_id"`
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			httpError(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
			return
		}
	}
	if scoped := scopedUserID(c); scoped != "" {
		req.UserID = scoped
	}
	if req.SessionID == "" {
		req.SessionID = newUUID()
	}
	if existing, _ := db.GetSession(c.Request.Context(), req.SessionID); existing != nil {
		httpError(c, http.StatusConflict, fmt.Sprintf("Session with id '%s' already exists", req.SessionID))
		return
	}
	now := time.Now().Unix()
	s := &SessionRecord{
		SessionID:    req.SessionID,
		SessionType:  sessionType,
		AgentID:      req.AgentID,
		TeamID:       req.TeamID,
		WorkflowID:   req.WorkflowID,
		UserID:       req.UserID,
		SessionName:  req.SessionName,
		SessionState: req.SessionState,
		Metadata:     req.Metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.UpsertSession(c.Request.Context(), s); err != nil {
		httpError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to create session: %v", err))
		return
	}
	c.JSON(http.StatusCreated, sessionDetailSchema(s))
}

// loadSession fetches a session, enforcing type and user scoping.
func (os *AgentOS) loadSession(c *gin.Context, db Database) (*SessionRecord, bool) {
	sessionID := c.Param("session_id")
	s, err := db.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	userID := c.Query("user_id")
	if scoped := scopedUserID(c); scoped != "" {
		userID = scoped
	}
	if s == nil || (c.Query("type") != "" && s.SessionType != c.Query("type")) || (userID != "" && s.UserID != userID) {
		httpError(c, http.StatusNotFound, fmt.Sprintf("Session with id '%s' not found", sessionID))
		return nil, false
	}
	return s, true
}

func (os *AgentOS) getSessionHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	s, ok := os.loadSession(c, db)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, sessionDetailSchema(s))
}

func (os *AgentOS) getSessionRunsHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	s, ok := os.loadSession(c, db)
	if !ok {
		return
	}
	createdAfter, ok := optionalQueryInt(c, "created_after")
	if !ok {
		return
	}
	createdBefore, ok := optionalQueryInt(c, "created_before")
	if !ok {
		return
	}
	limit, ok := optionalQueryInt(c, "limit")
	if !ok {
		return
	}
	before, ok := optionalQueryInt(c, "before_run_index")
	if !ok {
		return
	}
	after, ok := optionalQueryInt(c, "after_run_index")
	if !ok {
		return
	}
	if limit == nil && (before != nil || after != nil) {
		httpError(c, http.StatusUnprocessableEntity, "before_run_index and after_run_index require limit")
		return
	}
	if limit != nil && *limit < 1 {
		httpError(c, http.StatusUnprocessableEntity, "limit must be >= 1")
		return
	}
	statuses := map[string]bool{}
	for _, st := range c.QueryArray("status") {
		statuses[strings.ToUpper(st)] = true
	}

	// Filters select top-level runs; member/step runs follow their parent.
	top := make([]*RunRecord, 0, len(s.Runs))
	for _, r := range s.Runs {
		if r.ParentRunID != "" {
			continue
		}
		if createdAfter != nil && r.CreatedAt < int64(*createdAfter) {
			continue
		}
		if createdBefore != nil && r.CreatedAt > int64(*createdBefore) {
			continue
		}
		if len(statuses) > 0 && !statuses[strings.ToUpper(r.Status)] {
			continue
		}
		top = append(top, r)
	}
	withChildren := func(parents []*RunRecord) []gin.H {
		ids := map[string]bool{}
		for _, p := range parents {
			ids[p.RunID] = true
		}
		out := make([]gin.H, 0, len(parents))
		for _, r := range s.Runs {
			if ids[r.RunID] || (r.ParentRunID != "" && ids[r.ParentRunID]) {
				out = append(out, runSchema(r))
			}
		}
		return out
	}

	if limit == nil {
		c.JSON(http.StatusOK, withChildren(top))
		return
	}

	// Cursor pagination over run_index.
	window := make([]*RunRecord, 0, len(top))
	for _, r := range top {
		if before != nil && r.RunIndex >= *before {
			continue
		}
		if after != nil && r.RunIndex <= *after {
			continue
		}
		window = append(window, r)
	}
	hasMore := false
	if len(window) > *limit {
		hasMore = true
		if after != nil {
			window = window[:*limit] // oldest runs above the cursor
		} else {
			window = window[len(window)-*limit:] // newest runs
		}
	}
	meta := gin.H{"limit": *limit, "total_count": len(top), "has_more": hasMore}
	if len(window) > 0 {
		meta["oldest_run_index"] = window[0].RunIndex
		meta["newest_run_index"] = window[len(window)-1].RunIndex
	}
	c.JSON(http.StatusOK, gin.H{"data": withChildren(window), "meta": meta})
}

func (os *AgentOS) getSessionRunHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	s, ok := os.loadSession(c, db)
	if !ok {
		return
	}
	runID := c.Param("run_id")
	for _, r := range s.Runs {
		if r.RunID == runID {
			c.JSON(http.StatusOK, runSchema(r))
			return
		}
	}
	httpError(c, http.StatusNotFound, fmt.Sprintf("Run with id '%s' not found in session '%s'", runID, s.SessionID))
}

func (os *AgentOS) deleteSessionHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	userID := c.Query("user_id")
	if scoped := scopedUserID(c); scoped != "" {
		userID = scoped
	}
	if err := db.DeleteSessions(c.Request.Context(), []string{c.Param("session_id")}, userID); err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (os *AgentOS) deleteSessionsHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	var req struct {
		SessionIDs   []string `json:"session_ids"`
		SessionTypes []string `json:"session_types"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpError(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	if len(req.SessionIDs) == 0 || len(req.SessionTypes) == 0 {
		httpError(c, http.StatusUnprocessableEntity, "session_ids and session_types are required")
		return
	}
	if len(req.SessionIDs) != len(req.SessionTypes) {
		httpError(c, http.StatusBadRequest, "Session IDs and session types must have the same length")
		return
	}
	userID := c.Query("user_id")
	if scoped := scopedUserID(c); scoped != "" {
		userID = scoped
	}
	if err := db.DeleteSessions(c.Request.Context(), req.SessionIDs, userID); err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (os *AgentOS) renameSessionHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	var req struct {
		SessionName string `json:"session_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SessionName) == "" {
		httpError(c, http.StatusBadRequest, "session_name is required")
		return
	}
	s, ok := os.loadSession(c, db)
	if !ok {
		return
	}
	s.SessionName = req.SessionName
	s.UpdatedAt = time.Now().Unix()
	if err := db.UpsertSession(c.Request.Context(), s); err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, sessionDetailSchema(s))
}

func (os *AgentOS) updateSessionHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	var req struct {
		SessionName  *string                `json:"session_name"`
		SessionState map[string]interface{} `json:"session_state"`
		Metadata     map[string]interface{} `json:"metadata"`
		Summary      map[string]interface{} `json:"summary"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpError(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	s, ok := os.loadSession(c, db)
	if !ok {
		return
	}
	if req.SessionName != nil {
		s.SessionName = *req.SessionName
	}
	if req.SessionState != nil {
		s.SessionState = req.SessionState
	}
	if req.Metadata != nil {
		s.Metadata = req.Metadata
	}
	if req.Summary != nil {
		s.Summary = req.Summary
	}
	s.UpdatedAt = time.Now().Unix()
	if err := db.UpsertSession(c.Request.Context(), s); err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, sessionDetailSchema(s))
}

// jsonClone deep-copies a JSON-compatible value.
func jsonClone(v interface{}) interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out interface{}
	if json.Unmarshal(b, &out) != nil {
		return v
	}
	return out
}
