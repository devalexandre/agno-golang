package os

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// KnowledgeDocument represents a document in the knowledge base
type KnowledgeDocument struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	URL         string      `json:"url"`
	FileName    string      `json:"file_name"`
	FileSize    int64       `json:"file_size"`
	FileType    string      `json:"file_type"`
	Content     string      `json:"content"`
	Metadata    interface{} `json:"metadata"`
	DBID        string      `json:"db_id"`
	Status      string      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
}

// ContentResponseSchema represents the response schema for content operations
type ContentResponseSchema struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ContentStatus represents the status of content processing
type ContentStatus string

const (
	ContentStatusProcessing ContentStatus = "processing"
	ContentStatusComplete   ContentStatus = "complete"
	ContentStatusCompleted  ContentStatus = "completed"
	ContentStatusFailed     ContentStatus = "failed"
)

// Simple handlers for Python compatibility testing

// uploadContentHandler handles POST /knowledge/content
func (os *AgentOS) uploadContentHandler(c *gin.Context) {
	contentID := generateID("content")
	c.JSON(http.StatusAccepted, ContentResponseSchema{
		ID:      contentID,
		Status:  string(ContentStatusProcessing),
		Message: "Content upload started",
	})
}

// getContentStatusHandler handles GET /knowledge/content/{content_id}/status
func (os *AgentOS) getContentStatusHandler(c *gin.Context) {
	contentID := c.Param("content_id")
	c.JSON(http.StatusOK, gin.H{
		"id":     contentID,
		"status": string(ContentStatusComplete),
	})
}

// deleteContentHandler handles DELETE /knowledge/content/{content_id}
func (os *AgentOS) deleteContentHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Content deleted"})
}

// vectorSearchHandler handles POST /knowledge/vector-search
func (os *AgentOS) vectorSearchHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

// getContentHandler handles GET /knowledge/content/{content_id} - DEPRECATED, kept for compatibility
// Use getKnowledgeContentHandler from additional_handlers.go instead
func (os *AgentOS) getContentHandler(c *gin.Context) {
	contentID := c.Param("content_id")
	c.JSON(http.StatusOK, gin.H{
		"id":      contentID,
		"status":  string(ContentStatusComplete),
		"message": "Content retrieved",
	})
}

// processContentBackground simulates async content processing
func (os *AgentOS) processContentBackground(kb interface{}, doc *KnowledgeDocument, readerID, chunker string) {
	// Simulate processing
	time.Sleep(100 * time.Millisecond)
}
