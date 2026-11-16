package scheduler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RouterOptions configures RegisterRoutes.
type RouterOptions struct {
	// Scheduler executes manual triggers and cancellations; when nil those
	// routes answer 503 ("Scheduler is not running"), as in Python.
	Scheduler *Scheduler
	// UserID returns the caller's owner scope (nil = unscoped/admin).
	UserID func(c *gin.Context) *string
}

// ScheduleResponse is the public schedule representation.
type ScheduleResponse struct {
	ID                string         `json:"id"`
	UserID            *string        `json:"user_id"`
	Name              string         `json:"name"`
	Description       *string        `json:"description"`
	Method            string         `json:"method"`
	Endpoint          string         `json:"endpoint"`
	Payload           map[string]any `json:"payload"`
	CronExpr          *string        `json:"cron_expr"`
	IntervalSeconds   *int64         `json:"interval_seconds"`
	RunAt             *int64         `json:"run_at"`
	Kind              string         `json:"kind"`
	Timezone          string         `json:"timezone"`
	TimeoutSeconds    int            `json:"timeout_seconds"`
	MaxRetries        int            `json:"max_retries"`
	RetryDelaySeconds int            `json:"retry_delay_seconds"`
	Enabled           bool           `json:"enabled"`
	NextRunAt         *int64         `json:"next_run_at"`
	TargetType        *string        `json:"target_type"`
	TargetID          *string        `json:"target_id"`
	DisabledReason    *string        `json:"disabled_reason"`
	CreatedAt         int64          `json:"created_at"`
	UpdatedAt         *int64         `json:"updated_at"`
}

// ScheduleStateResponse is returned by enable/disable.
type ScheduleStateResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	NextRunAt *int64 `json:"next_run_at"`
	UpdatedAt *int64 `json:"updated_at"`
}

// PaginationInfo mirrors AgentOS pagination metadata.
type PaginationInfo struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"total_pages"`
	TotalCount int `json:"total_count"`
}

// ToResponse converts a schedule to its public form.
func ToResponse(s *Schedule) ScheduleResponse {
	r := ScheduleResponse{
		ID: s.ID, UserID: s.UserID, Name: s.Name, Description: s.Description, Method: s.Method,
		Endpoint: s.Endpoint, Payload: s.Payload, RunAt: s.RunAt, Kind: s.Kind(), Timezone: s.Timezone,
		TimeoutSeconds: s.TimeoutSeconds, MaxRetries: s.MaxRetries, RetryDelaySeconds: s.RetryDelaySeconds,
		Enabled: s.Enabled, NextRunAt: s.NextRunAt, TargetType: s.TargetType, TargetID: s.TargetID,
		DisabledReason: s.DisabledReason, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if !IsOnDemand(s.CronExpr) {
		r.CronExpr = ptr(s.CronExpr)
	}
	if s.IntervalSeconds > 0 {
		r.IntervalSeconds = ptr(s.IntervalSeconds)
	}
	return r
}

func stateResponse(s *Schedule) ScheduleStateResponse {
	return ScheduleStateResponse{ID: s.ID, Name: s.Name, Enabled: s.Enabled, NextRunAt: s.NextRunAt, UpdatedAt: s.UpdatedAt}
}

func pagination(c *gin.Context) (int, int, bool) {
	limit, err1 := strconv.Atoi(c.DefaultQuery("limit", "100"))
	page, err2 := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err1 != nil || err2 != nil || limit < 1 || limit > 1000 || page < 1 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "limit must be 1-1000 and page >= 1"})
		return 0, 0, false
	}
	return limit, page, true
}

func meta(limit, page, total int) PaginationInfo {
	pages := 0
	if total > 0 {
		pages = (total + limit - 1) / limit
	}
	return PaginationInfo{Page: page, Limit: limit, TotalPages: pages, TotalCount: total}
}

func writeErr(c *gin.Context, err error, notFound string) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"detail": notFound})
	case errors.Is(err, ErrDuplicateName):
		c.JSON(http.StatusConflict, gin.H{"detail": "Schedule with this name already exists"})
	case errors.Is(err, ErrDisabled):
		c.JSON(http.StatusConflict, gin.H{"detail": "Schedule is disabled"})
	case IsValidation(err):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
	}
}

// RegisterRoutes mounts the schedule API (mirroring Python's
// os/routers/schedules) onto rg:
//
//	GET    /schedules
//	POST   /schedules
//	GET    /schedules/:schedule_id
//	PATCH  /schedules/:schedule_id
//	DELETE /schedules/:schedule_id
//	POST   /schedules/:schedule_id/enable
//	POST   /schedules/:schedule_id/disable
//	POST   /schedules/:schedule_id/trigger
//	POST   /schedules/:schedule_id/cancel
//	GET    /schedules/:schedule_id/runs
//	GET    /schedules/:schedule_id/runs/:run_id
func RegisterRoutes(rg *gin.RouterGroup, m *Manager, opts RouterOptions) {
	scope := func(c *gin.Context) *string {
		if opts.UserID == nil {
			return nil
		}
		return opts.UserID(c)
	}

	rg.GET("/schedules", func(c *gin.Context) {
		limit, page, ok := pagination(c)
		if !ok {
			return
		}
		f := ListFilter{UserID: scope(c), Limit: limit, Page: page}
		if v := c.Query("enabled"); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "enabled must be a boolean"})
				return
			}
			f.Enabled = &b
		}
		items, total, err := m.List(c.Request.Context(), f)
		if err != nil {
			writeErr(c, err, "")
			return
		}
		data := make([]ScheduleResponse, 0, len(items))
		for _, s := range items {
			data = append(data, ToResponse(s))
		}
		c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta(limit, page, total)})
	})

	rg.POST("/schedules", func(c *gin.Context) {
		var req CreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": err.Error()})
			return
		}
		req.UserID = scope(c)
		s, err := m.Create(c.Request.Context(), req)
		if err != nil {
			writeErr(c, err, "")
			return
		}
		c.JSON(http.StatusCreated, ToResponse(s))
	})

	rg.GET("/schedules/:schedule_id", func(c *gin.Context) {
		s, err := m.Get(c.Request.Context(), c.Param("schedule_id"), scope(c))
		if err != nil {
			writeErr(c, err, "Schedule not found")
			return
		}
		c.JSON(http.StatusOK, ToResponse(s))
	})

	rg.PATCH("/schedules/:schedule_id", func(c *gin.Context) {
		var req UpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": err.Error()})
			return
		}
		s, err := m.Update(c.Request.Context(), c.Param("schedule_id"), scope(c), req)
		if err != nil {
			writeErr(c, err, "Schedule not found")
			return
		}
		c.JSON(http.StatusOK, ToResponse(s))
	})

	rg.DELETE("/schedules/:schedule_id", func(c *gin.Context) {
		if err := m.Delete(c.Request.Context(), c.Param("schedule_id"), scope(c)); err != nil {
			writeErr(c, err, "Schedule not found")
			return
		}
		c.Status(http.StatusNoContent)
	})

	rg.POST("/schedules/:schedule_id/enable", func(c *gin.Context) {
		s, err := m.Enable(c.Request.Context(), c.Param("schedule_id"), scope(c))
		if err != nil {
			writeErr(c, err, "Schedule not found")
			return
		}
		c.JSON(http.StatusOK, stateResponse(s))
	})

	rg.POST("/schedules/:schedule_id/disable", func(c *gin.Context) {
		s, err := m.Disable(c.Request.Context(), c.Param("schedule_id"), scope(c))
		if err != nil {
			writeErr(c, err, "Schedule not found")
			return
		}
		c.JSON(http.StatusOK, stateResponse(s))
	})

	rg.POST("/schedules/:schedule_id/trigger", func(c *gin.Context) {
		if opts.Scheduler == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"detail": "Scheduler is not running"})
			return
		}
		run, err := opts.Scheduler.Trigger(c.Request.Context(), c.Param("schedule_id"), scope(c))
		if err != nil {
			writeErr(c, err, "Schedule not found")
			return
		}
		c.JSON(http.StatusOK, run)
	})

	rg.POST("/schedules/:schedule_id/cancel", func(c *gin.Context) {
		if opts.Scheduler == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"detail": "Scheduler is not running"})
			return
		}
		id := c.Param("schedule_id")
		if _, err := m.Get(c.Request.Context(), id, scope(c)); err != nil {
			writeErr(c, err, "Schedule not found")
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id, "cancelled": opts.Scheduler.Cancel(id)})
	})

	rg.GET("/schedules/:schedule_id/runs", func(c *gin.Context) {
		limit, page, ok := pagination(c)
		if !ok {
			return
		}
		runs, total, err := m.GetRuns(c.Request.Context(), c.Param("schedule_id"), scope(c), limit, page)
		if err != nil {
			writeErr(c, err, "Schedule not found")
			return
		}
		if runs == nil {
			runs = []*ScheduleRun{}
		}
		c.JSON(http.StatusOK, gin.H{"data": runs, "meta": meta(limit, page, total)})
	})

	rg.GET("/schedules/:schedule_id/runs/:run_id", func(c *gin.Context) {
		run, err := m.GetRun(c.Request.Context(), c.Param("schedule_id"), c.Param("run_id"), scope(c))
		if err != nil {
			writeErr(c, err, "Schedule run not found")
			return
		}
		c.JSON(http.StatusOK, run)
	})
}
