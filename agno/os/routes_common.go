package os

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// httpError writes an error body. "detail" is the Python AgentOS (FastAPI)
// field; "error" is kept for existing Go clients.
func httpError(c *gin.Context, status int, detail string) {
	c.AbortWithStatusJSON(status, gin.H{"detail": detail, "error": detail})
}

// registerDatabase adds a database to the OS registry.
func (os *AgentOS) registerDatabase(db Database) {
	if db == nil {
		return
	}
	if os.osDBs == nil {
		os.osDBs = map[string]Database{}
	}
	if _, exists := os.osDBs[db.GetID()]; !exists {
		os.osDBOrder = append(os.osDBOrder, db.GetID())
	}
	os.osDBs[db.GetID()] = db
}

// resolveDB mirrors Python agno.os.utils.get_db: the database with the given
// id (404 when unknown), or the only database (400 when several are
// configured and no id was given, or none is configured).
func (os *AgentOS) resolveDB(dbID string) (Database, int, error) {
	if dbID != "" {
		db, ok := os.osDBs[dbID]
		if !ok {
			return nil, http.StatusNotFound, fmt.Errorf("No database found with id '%s'", dbID)
		}
		return db, 0, nil
	}
	if len(os.osDBs) > 1 {
		return nil, http.StatusBadRequest, fmt.Errorf("The db_id query parameter is required when using multiple databases")
	}
	if len(os.osDBs) == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("No database is configured on this AgentOS")
	}
	return os.osDBs[os.osDBOrder[0]], 0, nil
}

// dbFromRequest resolves the database selected by the db_id query parameter,
// writing the error response when it cannot.
func (os *AgentOS) dbFromRequest(c *gin.Context) (Database, bool) {
	db, status, err := os.resolveDB(c.Query("db_id"))
	if err != nil {
		httpError(c, status, err.Error())
		return nil, false
	}
	return db, true
}

// primaryDB is the database runs are recorded in (the first configured one).
func (os *AgentOS) primaryDB() Database {
	if len(os.osDBOrder) == 0 {
		return nil
	}
	return os.osDBs[os.osDBOrder[0]]
}

// queryInt parses an integer query parameter, writing a 422 on bad input.
func queryInt(c *gin.Context, name string, def, min int) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return def, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min {
		httpError(c, http.StatusUnprocessableEntity, fmt.Sprintf("Invalid value for '%s'", name))
		return 0, false
	}
	return v, true
}

// optionalQueryInt parses an optional integer query parameter.
func optionalQueryInt(c *gin.Context, name string) (*int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		httpError(c, http.StatusUnprocessableEntity, fmt.Sprintf("Invalid value for '%s'", name))
		return nil, false
	}
	return &v, true
}

func paginatedResponse[T any](data []T, page, limit, total int) gin.H {
	if data == nil {
		data = []T{}
	}
	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}
	return gin.H{
		"data": data,
		"meta": gin.H{
			"page":           page,
			"limit":          limit,
			"total_pages":    totalPages,
			"total_count":    total,
			"search_time_ms": 0,
		},
	}
}

// unixToTime converts epoch seconds to a UTC time (nil for 0).
func unixToTime(sec int64) *time.Time {
	if sec == 0 {
		return nil
	}
	t := time.Unix(sec, 0).UTC()
	return &t
}

// defaultDBID is the id of the database runs are recorded in.
func (os *AgentOS) defaultDBID() string {
	if db := os.primaryDB(); db != nil {
		return db.GetID()
	}
	return DefaultDBID
}
