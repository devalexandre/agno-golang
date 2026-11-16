package os

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Memory routes, ported from Python agno/os/routers/memory/memory.py.

func userMemorySchema(m *UserMemory) gin.H {
	topics := m.Topics
	if topics == nil {
		topics = []string{}
	}
	out := gin.H{
		"memory_id":  m.MemoryID,
		"memory":     m.Memory,
		"topics":     topics,
		"user_id":    m.UserID,
		"updated_at": unixToTime(m.UpdatedAt),
	}
	if m.AgentID != "" {
		out["agent_id"] = m.AgentID
	}
	if m.TeamID != "" {
		out["team_id"] = m.TeamID
	}
	return out
}

// parseTopics accepts ?topics=a&topics=b and ?topics=a,b.
func parseTopics(c *gin.Context) []string {
	var out []string
	for _, raw := range c.QueryArray("topics") {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

type memoryWriteRequest struct {
	Memory string   `json:"memory"`
	UserID string   `json:"user_id"`
	Topics []string `json:"topics"`
}

func (os *AgentOS) bindMemoryWrite(c *gin.Context) (*memoryWriteRequest, bool) {
	var req memoryWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpError(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return nil, false
	}
	if l := len(req.Memory); l < 1 || l > 5000 {
		httpError(c, http.StatusUnprocessableEntity, "memory must be between 1 and 5000 characters")
		return nil, false
	}
	if auth := requestAuthorization(c); auth != nil && auth.Context.UserID != "" {
		req.UserID = auth.Context.UserID
	}
	if req.UserID == "" {
		httpError(c, http.StatusBadRequest, "User ID is required")
		return nil, false
	}
	return &req, true
}

func (os *AgentOS) createMemoryHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	req, ok := os.bindMemoryWrite(c)
	if !ok {
		return
	}
	m := &UserMemory{
		MemoryID:  newUUID(),
		Memory:    req.Memory,
		Topics:    req.Topics,
		UserID:    req.UserID,
		UpdatedAt: time.Now().Unix(),
	}
	if err := db.UpsertMemory(c.Request.Context(), m); err != nil {
		httpError(c, http.StatusInternalServerError, "Failed to create memory")
		return
	}
	c.JSON(http.StatusOK, userMemorySchema(m))
}

func (os *AgentOS) memoryUserID(c *gin.Context) string {
	if scoped := scopedUserID(c); scoped != "" {
		return scoped
	}
	return c.Query("user_id")
}

func (os *AgentOS) getMemoryHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	id := c.Param("memory_id")
	m, err := db.GetMemory(c.Request.Context(), id, os.memoryUserID(c))
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		httpError(c, http.StatusNotFound, fmt.Sprintf("Memory with ID %s not found", id))
		return
	}
	c.JSON(http.StatusOK, userMemorySchema(m))
}

func (os *AgentOS) listMemoriesHandler(c *gin.Context) {
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
	memories, total, err := db.ListMemories(c.Request.Context(), MemoryQuery{
		UserID:        os.memoryUserID(c),
		AgentID:       c.Query("agent_id"),
		TeamID:        c.Query("team_id"),
		Topics:        parseTopics(c),
		SearchContent: c.Query("search_content"),
		SortBy:        c.DefaultQuery("sort_by", "updated_at"),
		SortOrder:     c.DefaultQuery("sort_order", "desc"),
		Limit:         limit,
		Page:          page,
	})
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	data := make([]gin.H, 0, len(memories))
	for _, m := range memories {
		data = append(data, userMemorySchema(m))
	}
	c.JSON(http.StatusOK, paginatedResponse(data, page, limit, total))
}

func (os *AgentOS) updateMemoryHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	req, ok := os.bindMemoryWrite(c)
	if !ok {
		return
	}
	id := c.Param("memory_id")
	existing, err := db.GetMemory(c.Request.Context(), id, req.UserID)
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		httpError(c, http.StatusNotFound, "Memory not found")
		return
	}
	existing.Memory = req.Memory
	if req.Topics != nil {
		existing.Topics = req.Topics
	}
	existing.UpdatedAt = time.Now().Unix()
	if err := db.UpsertMemory(c.Request.Context(), existing); err != nil {
		httpError(c, http.StatusInternalServerError, "Failed to update memory")
		return
	}
	c.JSON(http.StatusOK, userMemorySchema(existing))
}

func (os *AgentOS) deleteMemoryHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	if err := db.DeleteMemories(c.Request.Context(), []string{c.Param("memory_id")}, os.memoryUserID(c)); err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (os *AgentOS) deleteMemoriesHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	var req struct {
		MemoryIDs []string `json:"memory_ids"`
		UserID    string   `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpError(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	if len(req.MemoryIDs) == 0 {
		httpError(c, http.StatusUnprocessableEntity, "memory_ids must contain at least one id")
		return
	}
	if scoped := scopedUserID(c); scoped != "" {
		req.UserID = scoped
	}
	if err := db.DeleteMemories(c.Request.Context(), req.MemoryIDs, req.UserID); err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (os *AgentOS) memoryTopicsHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	memories, _, err := db.ListMemories(c.Request.Context(), MemoryQuery{UserID: os.memoryUserID(c)})
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	seen := map[string]bool{}
	topics := []string{}
	for _, m := range memories {
		for _, t := range m.Topics {
			if !seen[t] {
				seen[t] = true
				topics = append(topics, t)
			}
		}
	}
	sort.Strings(topics)
	c.JSON(http.StatusOK, topics)
}

func (os *AgentOS) userMemoryStatsHandler(c *gin.Context) {
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
	memories, _, err := db.ListMemories(c.Request.Context(), MemoryQuery{UserID: os.memoryUserID(c)})
	if err != nil {
		httpError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get user statistics: %v", err))
		return
	}
	type stat struct {
		userID string
		total  int
		last   int64
	}
	byUser := map[string]*stat{}
	for _, m := range memories {
		s, ok := byUser[m.UserID]
		if !ok {
			s = &stat{userID: m.UserID}
			byUser[m.UserID] = s
		}
		s.total++
		if m.UpdatedAt > s.last {
			s.last = m.UpdatedAt
		}
	}
	stats := make([]*stat, 0, len(byUser))
	for _, s := range byUser {
		stats = append(stats, s)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].last == stats[j].last {
			return stats[i].userID < stats[j].userID
		}
		return stats[i].last > stats[j].last
	})
	total := len(stats)
	data := make([]gin.H, 0)
	for _, s := range paginate(stats, limit, page) {
		data = append(data, gin.H{
			"user_id":                s.userID,
			"total_memories":         s.total,
			"last_memory_updated_at": unixToTime(s.last),
		})
	}
	c.JSON(http.StatusOK, paginatedResponse(data, page, limit, total))
}
