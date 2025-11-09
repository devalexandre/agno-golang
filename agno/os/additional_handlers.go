package os

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	stdos "os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/gin-gonic/gin"
)

// KnowledgeWithContent is a helper interface for knowledge bases that support content management
type KnowledgeWithContent interface {
	knowledge.Knowledge
	SaveContent(ctx context.Context, content *knowledge.Content) error
	PatchContent(ctx context.Context, content *knowledge.Content) error
	GetContent(ctx context.Context, contentID string) (*knowledge.Content, error)
	ListContents(ctx context.Context, limit, offset int) ([]*knowledge.Content, int, error)
	DeleteContent(ctx context.Context, contentID string) error
}

// PaginationMeta represents pagination metadata
type PaginationMeta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"total_pages"`
	TotalCount int `json:"total_count"`
}

// ContentResponse represents a knowledge content item
type ContentResponse struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name,omitempty"`
	Description   string                 `json:"description,omitempty"`
	FileType      string                 `json:"file_type,omitempty"`
	Size          int64                  `json:"size,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Status        string                 `json:"status,omitempty"`
	StatusMessage string                 `json:"status_message,omitempty"`
	CreatedAt     *time.Time             `json:"created_at,omitempty"`
	UpdatedAt     *time.Time             `json:"updated_at,omitempty"`
}

// Helper function to build paginated response
func BuildPaginatedResponse(data []interface{}, page, limit, totalCount int) gin.H {
	totalPages := 0
	if limit > 0 {
		totalPages = (totalCount + limit - 1) / limit
	}

	return gin.H{
		"data": data,
		"meta": gin.H{
			"page":        page,
			"limit":       limit,
			"total_pages": totalPages,
			"total_count": totalCount,
		},
	}
}

// Helper function to convert any slice to []interface{}
func convertToInterfaceSlice(slice interface{}) []interface{} {
	if slice == nil {
		return nil
	}

	// Use reflection to convert any slice type to []interface{}
	v := reflect.ValueOf(slice)
	if v.Kind() != reflect.Slice {
		return nil
	}

	result := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		result[i] = v.Index(i).Interface()
	}
	return result
}

// Knowledge handlers - compatible with Python API

// listKnowledgeContentHandler handles GET /knowledge/content
// Supports query params: limit, page, sort_by, sort_order, db_id
func (os *AgentOS) listKnowledgeContentHandler(c *gin.Context) {
	// Get query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	_ = c.DefaultQuery("sort_by", "created_at") // TODO: implement sorting
	_ = c.DefaultQuery("sort_order", "desc")    // TODO: implement sorting
	dbID := c.Query("db_id")

	// Validate parameters
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}

	// Get knowledge instance by db_id
	instance, err := getKnowledgeInstanceByDBID(os.knowledgeInstances, dbID)
	if err != nil {
		// Determine status code based on error
		statusCode := http.StatusNotFound
		if dbID == "" && len(os.knowledgeInstances) > 1 {
			statusCode = http.StatusBadRequest
		} else if len(os.knowledgeInstances) == 0 {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	// Verify instance has ContentsDB
	if instance.ContentsDB == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent does not have a database configured for knowledge"})
		return
	}

	// Calculate offset for pagination
	offset := (page - 1) * limit

	// Get contents from ContentsDB
	ctx := context.Background()
	var contents []*knowledge.Content
	var totalCount int
	var listErr error

	switch kb := instance.Knowledge.(type) {
	case *knowledge.PDFKnowledgeBase:
		contents, totalCount, listErr = kb.BaseKnowledge.ListContents(ctx, limit, offset)
	case *knowledge.BaseKnowledge:
		contents, totalCount, listErr = kb.ListContents(ctx, limit, offset)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "knowledge type does not support content listing"})
		return
	}

	if listErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list contents: %v", listErr)})
		return
	}

	// Convert to response format
	var responseData []ContentResponse
	for _, content := range contents {
		responseData = append(responseData, ContentResponse{
			ID:            content.ID,
			Name:          content.Name,
			Description:   content.Description,
			FileType:      content.FileType,
			Size:          content.Size,
			Metadata:      content.Metadata,
			Status:        string(content.Status),
			StatusMessage: content.StatusMessage,
			CreatedAt:     &content.CreatedAt,
			UpdatedAt:     &content.UpdatedAt,
		})
	}

	// Return paginated response using the new helper function
	// Add cache-control to ensure fresh data
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.JSON(http.StatusOK, BuildPaginatedResponse(
		convertToInterfaceSlice(responseData),
		page,
		limit,
		totalCount,
	))
}

// createKnowledgeContentHandler handles POST /knowledge/content
func (os *AgentOS) createKnowledgeContentHandler(c *gin.Context) {
	// Get form data
	name := c.PostForm("name")
	description := c.PostForm("description")
	url := c.PostForm("url")
	metadataStr := c.PostForm("metadata")
	textContent := c.PostForm("text_content")
	readerID := c.PostForm("reader_id")
	chunker := c.PostForm("chunker")
	dbID := c.Query("db_id")

	// Get knowledge instance by db_id
	instance, err := getKnowledgeInstanceByDBID(os.knowledgeInstances, dbID)
	if err != nil {
		// Determine status code based on error
		statusCode := http.StatusNotFound
		if dbID == "" && len(os.knowledgeInstances) > 1 {
			statusCode = http.StatusBadRequest
		} else if len(os.knowledgeInstances) == 0 {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	// Verify instance has ContentsDB
	if instance.ContentsDB == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent does not have a database configured for knowledge"})
		return
	}

	// Get uploaded file if present
	file, err := c.FormFile("file")
	var fileName string
	var fileSize int64
	var fileType string
	if err == nil && file != nil {
		fileName = file.Filename
		fileSize = file.Size
		fileType = file.Header.Get("Content-Type")
	}

	// Generate content ID
	contentID := generateID("content")

	// If no name provided, use filename or URL
	if name == "" {
		if fileName != "" {
			name = fileName
		} else if url != "" {
			name = url
		} else {
			name = "Untitled Content"
		}
	}

	// Parse metadata
	var metadata map[string]interface{}
	if metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			metadata = map[string]interface{}{"value": metadataStr}
		}
	}

	// Create content record
	now := time.Now()
	content := &knowledge.Content{
		ID:          contentID,
		Name:        name,
		Description: description,
		URL:         url,
		FileType:    fileType,
		FileName:    fileName,
		Size:        fileSize,
		Metadata:    metadata,
		Status:      knowledge.ContentStatusProcessing,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Save to ContentsDB immediately
	ctx := context.Background()

	// Try to save content using the knowledge instance
	var saveErr error
	switch kb := instance.Knowledge.(type) {
	case *knowledge.PDFKnowledgeBase:
		saveErr = kb.BaseKnowledge.SaveContent(ctx, content)
	case *knowledge.BaseKnowledge:
		saveErr = kb.SaveContent(ctx, content)
	default:
		saveErr = fmt.Errorf("knowledge type does not support content management")
	}

	if saveErr != nil {
		errMsg := fmt.Sprintf("failed to save content: %v", saveErr)
		fmt.Printf("ERROR: %s\n", errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	// Return response immediately with "processing" status (like Python background_tasks)
	response := ContentResponse{
		ID:            content.ID,
		Name:          content.Name,
		Description:   content.Description,
		FileType:      content.FileType,
		Size:          content.Size,
		Metadata:      content.Metadata,
		Status:        string(knowledge.ContentStatusProcessing),
		StatusMessage: "",
		CreatedAt:     &content.CreatedAt,
		UpdatedAt:     &content.UpdatedAt,
	}

	// Process content asynchronously in background goroutine (like Python background_tasks)
	go func() {
		var processingErr error

		log.Printf("[KNOWLEDGE] [START] Processing content ID: %s, Name: %s, Type: %s, HasFile: %v, HasURL: %v, HasText: %v",
			contentID, name, fileType, file != nil, url != "", textContent != "")

		// If file uploaded, save and process it
		if file != nil {
			// Create a dedicated temp directory for uploads
			uploadDir := filepath.Join(stdos.TempDir(), "agno-uploads")
			log.Printf("[KNOWLEDGE] [INFO] Creating upload directory: %s", uploadDir)

			if err := stdos.MkdirAll(uploadDir, 0755); err != nil {
				processingErr = fmt.Errorf("failed to create upload directory: %w", err)
				log.Printf("[KNOWLEDGE] [ERROR] Failed to create upload directory: %v", processingErr)
			} else {
				safeName := fmt.Sprintf("%s_%s", contentID, filepath.Base(file.Filename))
				dst := filepath.Join(uploadDir, safeName)
				log.Printf("[KNOWLEDGE] [INFO] Saving file to: %s", dst)

				if saveErr := c.SaveUploadedFile(file, dst); saveErr != nil {
					processingErr = fmt.Errorf("failed to save uploaded file: %w", saveErr)
					log.Printf("[KNOWLEDGE] [ERROR] Failed to save file: %v", processingErr)
				} else {
					log.Printf("[KNOWLEDGE] [INFO] File saved successfully")
					ext := strings.ToLower(filepath.Ext(dst))
					if ext == ".pdf" {
						if pdfKB, ok := instance.Knowledge.(*knowledge.PDFKnowledgeBase); ok {
							log.Printf("[KNOWLEDGE] [INFO] Loading PDF from path: %s", dst)
							if err := pdfKB.LoadDocumentFromPath(ctx, dst, metadata); err != nil {
								processingErr = fmt.Errorf("failed to load PDF: %w", err)
								log.Printf("[KNOWLEDGE] [ERROR] LoadDocumentFromPath failed: %v", processingErr)
							} else {
								log.Printf("[KNOWLEDGE] [SUCCESS] PDF loaded into VectorDB successfully")
							}
						} else {
							processingErr = fmt.Errorf("PDF processing not supported")
							log.Printf("[KNOWLEDGE] [ERROR] %v", processingErr)
						}
					} else {
						// Generic document processing
						log.Printf("[KNOWLEDGE] [INFO] Processing generic document type: %s", ext)
						doc := document.Document{
							ID:          contentID,
							Name:        name,
							Content:     "",
							ContentType: fileType,
							Source:      dst,
							Metadata:    metadata,
						}
						if err := instance.Knowledge.LoadDocument(ctx, doc); err != nil {
							processingErr = fmt.Errorf("failed to load document: %w", err)
							log.Printf("[KNOWLEDGE] [ERROR] LoadDocument failed: %v", processingErr)
						} else {
							log.Printf("[KNOWLEDGE] [SUCCESS] Document loaded into VectorDB successfully")
						}
					}
				}
			}
		} else if url != "" {
			// Handle URL
			log.Printf("[KNOWLEDGE] [INFO] Processing URL: %s", url)
			if strings.HasSuffix(strings.ToLower(url), ".pdf") {
				if pdfKB, ok := instance.Knowledge.(*knowledge.PDFKnowledgeBase); ok {
					log.Printf("[KNOWLEDGE] [INFO] Loading PDF from URL")
					if err := pdfKB.LoadDocumentFromPath(ctx, url, metadata); err != nil {
						processingErr = fmt.Errorf("failed to load PDF from URL: %w", err)
						log.Printf("[KNOWLEDGE] [ERROR] LoadDocumentFromPath failed: %v", processingErr)
					} else {
						log.Printf("[KNOWLEDGE] [SUCCESS] PDF from URL loaded into VectorDB successfully")
					}
				} else {
					processingErr = fmt.Errorf("PDF processing not supported")
					log.Printf("[KNOWLEDGE] [ERROR] %v", processingErr)
				}
			} else {
				processingErr = fmt.Errorf("unsupported URL type")
				log.Printf("[KNOWLEDGE] [ERROR] %v", processingErr)
			}
		} else if textContent != "" {
			// Handle text content
			log.Printf("[KNOWLEDGE] [INFO] Processing text content (%d bytes)", len(textContent))
			doc := document.Document{
				ID:          contentID,
				Name:        name,
				Content:     textContent,
				ContentType: "text/plain",
				Source:      "uploaded_text",
				Metadata:    metadata,
			}
			if err := instance.Knowledge.LoadDocument(ctx, doc); err != nil {
				processingErr = fmt.Errorf("failed to load text content: %w", err)
				log.Printf("[KNOWLEDGE] [ERROR] LoadDocument failed: %v", processingErr)
			} else {
				log.Printf("[KNOWLEDGE] [SUCCESS] Text content loaded into VectorDB successfully")
			}
		}

		// Update content status
		if processingErr != nil {
			content.Status = knowledge.ContentStatusFailed
			content.StatusMessage = processingErr.Error()
			log.Printf("[KNOWLEDGE] [FAILED] Content processing failed: %v", processingErr)
		} else {
			content.Status = knowledge.ContentStatusCompleted
			content.StatusMessage = ""
			log.Printf("[KNOWLEDGE] [COMPLETED] Content processing completed successfully")
		}
		content.UpdatedAt = time.Now()

		// Update in database
		log.Printf("[KNOWLEDGE] [INFO] Updating content status to: %s", content.Status)
		var patchErr error
		switch kb := instance.Knowledge.(type) {
		case *knowledge.PDFKnowledgeBase:
			patchErr = kb.BaseKnowledge.PatchContent(ctx, content)
		case *knowledge.BaseKnowledge:
			patchErr = kb.PatchContent(ctx, content)
		}

		if patchErr != nil {
			log.Printf("[KNOWLEDGE] [ERROR] Failed to update content status: %v", patchErr)
		} else {
			log.Printf("[KNOWLEDGE] [SUCCESS] Content status updated in database")
		}

		// Avoid unused variables
		_ = readerID
		_ = chunker
	}()

	// Return immediately with "processing" status (like Python)
	// Add headers to help UI invalidate cache
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.JSON(http.StatusAccepted, response)
}

// getKnowledgeContentHandler handles GET /knowledge/content/{content_id}
func (os *AgentOS) getKnowledgeContentHandler(c *gin.Context) {
	contentID := c.Param("content_id")
	if contentID == "" {
		contentID = c.Param("document_id") // Support legacy route
	}
	dbID := c.Query("db_id")

	// Get knowledge instance by db_id
	instance, err := getKnowledgeInstanceByDBID(os.knowledgeInstances, dbID)
	if err != nil {
		// Determine status code based on error
		statusCode := http.StatusNotFound
		if dbID == "" && len(os.knowledgeInstances) > 1 {
			statusCode = http.StatusBadRequest
		} else if len(os.knowledgeInstances) == 0 {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	// Verify instance has ContentsDB
	if instance.ContentsDB == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent does not have a database configured for knowledge"})
		return
	}

	// Get content from ContentsDB
	ctx := context.Background()
	var content *knowledge.Content
	var getErr error

	switch kb := instance.Knowledge.(type) {
	case *knowledge.PDFKnowledgeBase:
		content, getErr = kb.BaseKnowledge.GetContent(ctx, contentID)
	case *knowledge.BaseKnowledge:
		content, getErr = kb.GetContent(ctx, contentID)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "knowledge type does not support content retrieval"})
		return
	}

	if getErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get content: %v", getErr)})
		return
	}

	if content == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("content with id '%s' not found", contentID)})
		return
	}

	// Return content
	response := ContentResponse{
		ID:            content.ID,
		Name:          content.Name,
		Description:   content.Description,
		FileType:      content.FileType,
		Size:          content.Size,
		Metadata:      content.Metadata,
		Status:        string(content.Status),
		StatusMessage: content.StatusMessage,
		CreatedAt:     &content.CreatedAt,
		UpdatedAt:     &content.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// getKnowledgeContentStatusHandler handles GET /knowledge/content/{content_id}/status
func (os *AgentOS) getKnowledgeContentStatusHandler(c *gin.Context) {
	contentID := c.Param("content_id")
	if contentID == "" {
		contentID = c.Param("document_id") // Support legacy route
	}
	dbID := c.Query("db_id")

	// Get knowledge instance by db_id
	instance, err := getKnowledgeInstanceByDBID(os.knowledgeInstances, dbID)
	if err != nil {
		// Determine status code based on error
		statusCode := http.StatusNotFound
		if dbID == "" && len(os.knowledgeInstances) > 1 {
			statusCode = http.StatusBadRequest
		} else if len(os.knowledgeInstances) == 0 {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	// Verify instance has ContentsDB
	if instance.ContentsDB == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent does not have a database configured for knowledge"})
		return
	}

	// Get content from ContentsDB
	ctx := context.Background()
	var content *knowledge.Content
	var getErr error

	switch kb := instance.Knowledge.(type) {
	case *knowledge.PDFKnowledgeBase:
		content, getErr = kb.BaseKnowledge.GetContent(ctx, contentID)
	case *knowledge.BaseKnowledge:
		content, getErr = kb.GetContent(ctx, contentID)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "knowledge type does not support content retrieval"})
		return
	}

	// Handle the case where content is not found (like Python)
	// Return 200 with status="failed" instead of 404
	if getErr != nil || content == nil {
		statusMessage := "Content not found"
		if getErr != nil {
			statusMessage = getErr.Error()
		}
		c.JSON(http.StatusOK, gin.H{
			"status":         string(knowledge.ContentStatusFailed),
			"status_message": statusMessage,
		})
		return
	}

	// Return only status information
	c.JSON(http.StatusOK, gin.H{
		"status":         string(content.Status),
		"status_message": content.StatusMessage,
	})
}

// updateKnowledgeContentHandler handles PATCH /knowledge/content/{content_id}
func (os *AgentOS) updateKnowledgeContentHandler(c *gin.Context) {
	contentID := c.Param("content_id")
	if contentID == "" {
		contentID = c.Param("document_id") // Support legacy route
	}

	// Get form data
	_ = c.PostForm("name")
	_ = c.PostForm("description")
	metadata := c.PostForm("metadata")
	readerID := c.PostForm("reader_id")
	dbID := c.Query("db_id")

	// Get knowledge instance by db_id
	instance, err := getKnowledgeInstanceByDBID(os.knowledgeInstances, dbID)
	if err != nil {
		// Determine status code based on error
		statusCode := http.StatusNotFound
		if dbID == "" && len(os.knowledgeInstances) > 1 {
			statusCode = http.StatusBadRequest
		} else if len(os.knowledgeInstances) == 0 {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	// Verify instance has ContentsDB
	if instance.ContentsDB == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent does not have a database configured for knowledge"})
		return
	}

	// TODO: Update actual content in instance.ContentsDB
	_ = metadata
	_ = readerID

	// For now, return 404 as content doesn't exist
	c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("content with id '%s' not found", contentID)})
}

// deleteKnowledgeContentHandler handles DELETE /knowledge/content/{content_id}
func (os *AgentOS) deleteKnowledgeContentHandler(c *gin.Context) {
	contentID := c.Param("content_id")
	if contentID == "" {
		contentID = c.Param("document_id") // Support legacy route
	}
	dbID := c.Query("db_id")

	// Get knowledge instance by db_id
	instance, err := getKnowledgeInstanceByDBID(os.knowledgeInstances, dbID)
	if err != nil {
		// Determine status code based on error
		statusCode := http.StatusNotFound
		if dbID == "" && len(os.knowledgeInstances) > 1 {
			statusCode = http.StatusBadRequest
		} else if len(os.knowledgeInstances) == 0 {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	// Verify instance has ContentsDB
	if instance.ContentsDB == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent does not have a database configured for knowledge"})
		return
	}

	// Delete content from ContentsDB
	ctx := context.Background()
	var deleteErr error

	switch kb := instance.Knowledge.(type) {
	case *knowledge.PDFKnowledgeBase:
		deleteErr = kb.BaseKnowledge.DeleteContent(ctx, contentID)
	case *knowledge.BaseKnowledge:
		deleteErr = kb.DeleteContent(ctx, contentID)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "knowledge type does not support content deletion"})
		return
	}

	if deleteErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete content: %v", deleteErr)})
		return
	}

	// Return success with content ID (like Python - status 200, not 204)
	// Add headers to help UI invalidate cache
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.JSON(http.StatusOK, ContentResponse{
		ID: contentID,
	})
}

// deleteAllKnowledgeContentHandler handles DELETE /knowledge/content
func (os *AgentOS) deleteAllKnowledgeContentHandler(c *gin.Context) {
	dbID := c.Query("db_id")

	// Get knowledge instance by db_id
	instance, err := getKnowledgeInstanceByDBID(os.knowledgeInstances, dbID)
	if err != nil {
		// Determine status code based on error
		statusCode := http.StatusNotFound
		if dbID == "" && len(os.knowledgeInstances) > 1 {
			statusCode = http.StatusBadRequest
		} else if len(os.knowledgeInstances) == 0 {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	// Verify instance has ContentsDB
	if instance.ContentsDB == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent does not have a database configured for knowledge"})
		return
	}

	// TODO: Delete all content from instance.ContentsDB
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// searchKnowledgeHandler handles POST /knowledge/search
func (os *AgentOS) searchKnowledgeHandler(c *gin.Context) {
	var req struct {
		Query       string                 `json:"query" binding:"required"`
		MaxResults  int                    `json:"max_results"`
		Filters     map[string]interface{} `json:"filters"`
		SearchType  string                 `json:"search_type"`
		VectorDBIDs []string               `json:"vector_db_ids"`
		DBID        string                 `json:"db_id"`
		Meta        *struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
		} `json:"meta"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// Set defaults
	page := 1
	limit := 20
	if req.Meta != nil {
		if req.Meta.Page > 0 {
			page = req.Meta.Page
		}
		if req.Meta.Limit > 0 {
			limit = req.Meta.Limit
		}
	}
	if req.MaxResults == 0 {
		req.MaxResults = 100
	}

	// TODO: Perform actual vector search
	_ = req.Filters
	_ = req.SearchType
	_ = req.VectorDBIDs
	_ = req.DBID

	// Mock search results
	results := []map[string]interface{}{
		{
			"id":              "doc_1",
			"content":         fmt.Sprintf("Sample result for query: %s", req.Query),
			"name":            "Sample Document",
			"meta_data":       map[string]interface{}{"page": 1},
			"reranking_score": 0.95,
			"content_id":      "content_1",
		},
	}

	totalCount := len(results)
	totalPages := (totalCount + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"data": results,
		"meta": gin.H{
			"page":        page,
			"limit":       limit,
			"total_pages": totalPages,
			"total_count": totalCount,
		},
	})
}

// getKnowledgeConfigHandler handles GET /knowledge/config (moved from knowledge_handlers.go)
func (os *AgentOS) getKnowledgeConfigHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"readers": gin.H{
			"pdf": gin.H{
				"id":          "pdf",
				"name":        "PdfReader",
				"description": "Reads PDF files",
				"chunkers":    []string{"DocumentChunker", "FixedSizeChunker"},
			},
			"text": gin.H{
				"id":          "text",
				"name":        "TextReader",
				"description": "Reads text files",
				"chunkers":    []string{"FixedSizeChunker", "RecursiveChunker"},
			},
		},
		"readersForType": gin.H{
			".pdf": []string{"pdf"},
			".txt": []string{"text"},
		},
		"chunkers": gin.H{
			"DocumentChunker": gin.H{
				"id":          "DocumentChunker",
				"name":        "DocumentChunker",
				"description": "Chunks documents by structure",
			},
			"FixedSizeChunker": gin.H{
				"id":          "FixedSizeChunker",
				"name":        "FixedSizeChunker",
				"description": "Chunks documents by fixed size",
			},
		},
		"vector_dbs": []gin.H{},
	})
}

// getKnowledgeConversationsHandler handles GET /knowledge/conversations
func (os *AgentOS) getKnowledgeConversationsHandler(c *gin.Context) {
	// TODO: Implement actual knowledge conversations retrieval
	conversations := []map[string]interface{}{
		{
			"id":         "conv_1",
			"title":      "Sample Conversation",
			"messages":   []interface{}{},
			"created_at": time.Now().Unix(),
		},
	}
	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

// Memory handlers - compatible with Python API

// addMemoryHandler handles POST /memory/add
func (os *AgentOS) addMemoryHandler(c *gin.Context) {
	var req struct {
		Content  string                 `json:"content" binding:"required"`
		UserID   string                 `json:"user_id"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement actual memory addition
	memory := map[string]interface{}{
		"id":         generateID("mem"),
		"content":    req.Content,
		"user_id":    req.UserID,
		"metadata":   req.Metadata,
		"created_at": time.Now().Unix(),
	}

	c.JSON(http.StatusCreated, memory)
}

// deleteMemoryHandler handles DELETE /memory/{memory_id}
func (os *AgentOS) deleteMemoryHandler(c *gin.Context) {
	memoryID := c.Param("memory_id")

	// TODO: Implement actual memory deletion
	c.JSON(http.StatusOK, gin.H{"message": "Memory deleted", "id": memoryID})
}

// deleteAllMemoriesHandler handles DELETE /memory
func (os *AgentOS) deleteAllMemoriesHandler(c *gin.Context) {
	// TODO: Implement actual memory clearing
	c.JSON(http.StatusOK, gin.H{"message": "All memories deleted"})
}

// getMemoriesHandler handles GET /memory
func (os *AgentOS) getMemoriesHandler(c *gin.Context) {
	// TODO: Implement actual memory retrieval
	memories := []map[string]interface{}{
		{
			"id":         "mem_1",
			"content":    "Sample memory",
			"user_id":    "user_1",
			"metadata":   map[string]interface{}{},
			"created_at": time.Now().Unix(),
		},
	}
	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// getMemoryEntitiesHandler handles GET /memory/entities
func (os *AgentOS) getMemoryEntitiesHandler(c *gin.Context) {
	// TODO: Implement actual memory entities retrieval
	entities := []map[string]interface{}{
		{
			"id":         "entity_1",
			"name":       "Sample Entity",
			"type":       "person",
			"attributes": map[string]interface{}{},
		},
	}
	c.JSON(http.StatusOK, gin.H{"entities": entities})
}

// getMemoryConversationsHandler handles GET /memory/conversations
func (os *AgentOS) getMemoryConversationsHandler(c *gin.Context) {
	// TODO: Implement actual memory conversations retrieval
	conversations := []map[string]interface{}{
		{
			"id":         "conv_1",
			"title":      "Sample Conversation",
			"messages":   []interface{}{},
			"created_at": time.Now().Unix(),
		},
	}
	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

// updateMemoryConversationHandler handles PATCH /memory/conversations/{conversation_id}
func (os *AgentOS) updateMemoryConversationHandler(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	var req struct {
		Title    string                 `json:"title"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement actual conversation update
	conversation := map[string]interface{}{
		"id":         conversationID,
		"title":      req.Title,
		"metadata":   req.Metadata,
		"updated_at": time.Now().Unix(),
	}

	c.JSON(http.StatusOK, conversation)
}

// getMemoriesByRunIDHandler handles GET /memory/run_ids/{run_id}
func (os *AgentOS) getMemoriesByRunIDHandler(c *gin.Context) {
	runID := c.Param("run_id")

	// TODO: Implement actual memory retrieval by run ID
	memories := []map[string]interface{}{
		{
			"id":         "mem_1",
			"content":    "Memory from run",
			"run_id":     runID,
			"metadata":   map[string]interface{}{},
			"created_at": time.Now().Unix(),
		},
	}
	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// Metrics handlers - compatible with Python API

// getMetricsHandler handles GET /metrics
func (os *AgentOS) getMetricsHandler(c *gin.Context) {
	// TODO: Implement actual metrics retrieval
	metrics := map[string]interface{}{
		"agent_runs_count":        10,
		"agent_sessions_count":    5,
		"team_runs_count":         3,
		"team_sessions_count":     2,
		"workflow_runs_count":     1,
		"workflow_sessions_count": 1,
		"users_count":             1,
		"total_cost":              0.0,
		"avg_session_length":      120.5,
		"created_at":              time.Now().Unix(),
	}

	c.JSON(http.StatusOK, gin.H{"metrics": []interface{}{metrics}})
}

// createMetricsHandler handles POST /metrics
func (os *AgentOS) createMetricsHandler(c *gin.Context) {
	var req map[string]interface{}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement actual metrics creation
	req["id"] = generateID("metric")
	req["created_at"] = time.Now().Unix()

	c.JSON(http.StatusCreated, req)
}

// Evals handlers - compatible with Python API

// listEvalsHandler handles GET /evals
func (os *AgentOS) listEvalsHandler(c *gin.Context) {
	// TODO: Implement actual evals listing
	evals := []map[string]interface{}{
		{
			"id":          "eval_1",
			"name":        "Sample Evaluation",
			"description": "Sample evaluation description",
			"status":      "completed",
			"score":       0.85,
			"created_at":  time.Now().Unix(),
		},
	}
	c.JSON(http.StatusOK, gin.H{"evals": evals})
}

// createEvalHandler handles POST /evals
func (os *AgentOS) createEvalHandler(c *gin.Context) {
	var req struct {
		Name        string                 `json:"name" binding:"required"`
		Description string                 `json:"description"`
		Config      map[string]interface{} `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement actual eval creation
	eval := map[string]interface{}{
		"id":          generateID("eval"),
		"name":        req.Name,
		"description": req.Description,
		"config":      req.Config,
		"status":      "created",
		"created_at":  time.Now().Unix(),
	}

	c.JSON(http.StatusCreated, eval)
}

// getEvalHandler handles GET /evals/{eval_id}
func (os *AgentOS) getEvalHandler(c *gin.Context) {
	evalID := c.Param("eval_id")

	// TODO: Implement actual eval retrieval
	eval := map[string]interface{}{
		"id":          evalID,
		"name":        "Sample Evaluation",
		"description": "Sample evaluation description",
		"status":      "completed",
		"score":       0.85,
		"created_at":  time.Now().Unix(),
	}

	c.JSON(http.StatusOK, eval)
}

// runEvalHandler handles POST /evals/{eval_id}/run
func (os *AgentOS) runEvalHandler(c *gin.Context) {
	evalID := c.Param("eval_id")

	var req map[string]interface{}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement actual eval execution
	result := map[string]interface{}{
		"eval_id":    evalID,
		"run_id":     generateID("eval_run"),
		"status":     "running",
		"started_at": time.Now().Unix(),
	}

	c.JSON(http.StatusOK, result)
}
