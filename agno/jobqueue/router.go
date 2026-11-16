package jobqueue

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the operator queue API (Python os/routers/job_queue)
// onto rg. Mount it on an admin-only group, e.g. router.Group("/queue"):
//
//	GET  /jobs                 ?status=..&session_id=..&user_id=..&limit=&page=&sort_by=&sort_order=
//	GET  /jobs/:job_id
//	POST /jobs/:job_id/requeue
//	POST /jobs/:job_id/cancel  (extension: cancels queued/paused or locally running jobs)
//	GET  /stats
//
// worker may be nil; then cancel only tombstones queued/paused tickets.
func RegisterRoutes(rg *gin.RouterGroup, store Store, worker *Worker) {
	rg.GET("/jobs", func(c *gin.Context) {
		limit, err1 := strconv.Atoi(c.DefaultQuery("limit", "20"))
		page, err2 := strconv.Atoi(c.DefaultQuery("page", "1"))
		if err1 != nil || err2 != nil || limit < 1 || limit > 1000 || page < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "limit must be 1-1000 and page >= 1"})
			return
		}
		f := ListFilter{Statuses: c.QueryArray("status"), SessionIDs: c.QueryArray("session_id"), Limit: limit, Page: page,
			SortBy: c.DefaultQuery("sort_by", "created_at"), SortAsc: c.Query("sort_order") == "asc"}
		for _, s := range f.Statuses {
			switch s {
			case StatusQueued, StatusRunning, StatusPaused, StatusCompleted, StatusFailed, StatusCancelled:
			default:
				c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid status " + strconv.Quote(s)})
				return
			}
		}
		if u, ok := c.GetQuery("user_id"); ok {
			if u == "" {
				c.JSON(http.StatusBadRequest, gin.H{"detail": "session_id and user_id must not be empty"})
				return
			}
			f.UserID = &u
		}
		jobs, total, err := store.List(c.Request.Context(), f)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}
		if jobs == nil {
			jobs = []*Job{}
		}
		pages := 0
		if total > 0 {
			pages = (total + limit - 1) / limit
		}
		c.JSON(http.StatusOK, gin.H{"data": jobs, "meta": gin.H{"page": page, "limit": limit, "total_pages": pages, "total_count": total}})
	})

	rg.GET("/jobs/:job_id", func(c *gin.Context) {
		j, err := store.Get(c.Request.Context(), c.Param("job_id"))
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"detail": "Job queue job " + c.Param("job_id") + " not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}
		c.JSON(http.StatusOK, j)
	})

	rg.POST("/jobs/:job_id/requeue", func(c *gin.Context) {
		id := c.Param("job_id")
		ok, err := store.Requeue(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Job " + id + " not found or not in a requeueable state"})
			return
		}
		if worker != nil {
			worker.Wake()
		}
		j, _ := store.Get(c.Request.Context(), id)
		c.JSON(http.StatusOK, j)
	})

	rg.POST("/jobs/:job_id/cancel", func(c *gin.Context) {
		id := c.Param("job_id")
		var ok bool
		var err error
		if worker != nil {
			ok, err = worker.Cancel(c.Request.Context(), id)
		} else {
			ok, err = store.Cancel(c.Request.Context(), id)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusConflict, gin.H{"detail": "Job " + id + " not found or not cancellable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id, "cancelled": true})
	})

	rg.GET("/stats", func(c *gin.Context) {
		st, err := store.Stats(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}
		c.JSON(http.StatusOK, st)
	})
}
