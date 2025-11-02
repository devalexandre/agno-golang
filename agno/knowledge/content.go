package knowledge

import (
	"time"
)

// ContentStatus represents the status of content processing
type ContentStatus string

const (
	ContentStatusProcessing ContentStatus = "processing"
	ContentStatusCompleted  ContentStatus = "completed"
	ContentStatusFailed     ContentStatus = "failed"
)

// Content represents a knowledge content item stored in ContentsDB
type Content struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	URL           string                 `json:"url,omitempty"`
	FileType      string                 `json:"file_type,omitempty"`
	FileName      string                 `json:"file_name,omitempty"`
	Size          int64                  `json:"size,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Status        ContentStatus          `json:"status"`
	StatusMessage string                 `json:"status_message,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// ToMap converts Content to map for database storage
func (c *Content) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":             c.ID,
		"name":           c.Name,
		"description":    c.Description,
		"url":            c.URL,
		"file_type":      c.FileType,
		"file_name":      c.FileName,
		"size":           c.Size,
		"metadata":       c.Metadata,
		"status":         string(c.Status),
		"status_message": c.StatusMessage,
		"created_at":     c.CreatedAt,
		"updated_at":     c.UpdatedAt,
	}
}

// ContentFromMap creates Content from database map
func ContentFromMap(m map[string]interface{}) *Content {
	content := &Content{
		Metadata: make(map[string]interface{}),
	}

	if id, ok := m["id"].(string); ok {
		content.ID = id
	}
	if name, ok := m["name"].(string); ok {
		content.Name = name
	}
	if desc, ok := m["description"].(string); ok {
		content.Description = desc
	}
	if url, ok := m["url"].(string); ok {
		content.URL = url
	}
	if fileType, ok := m["file_type"].(string); ok {
		content.FileType = fileType
	}
	if fileName, ok := m["file_name"].(string); ok {
		content.FileName = fileName
	}
	if size, ok := m["size"].(int64); ok {
		content.Size = size
	} else if size, ok := m["size"].(float64); ok {
		content.Size = int64(size)
	}
	if metadata, ok := m["metadata"].(map[string]interface{}); ok {
		content.Metadata = metadata
	}
	if status, ok := m["status"].(string); ok {
		content.Status = ContentStatus(status)
	}
	if statusMsg, ok := m["status_message"].(string); ok {
		content.StatusMessage = statusMsg
	}
	if createdAt, ok := m["created_at"].(time.Time); ok {
		content.CreatedAt = createdAt
	}
	if updatedAt, ok := m["updated_at"].(time.Time); ok {
		content.UpdatedAt = updatedAt
	}

	return content
}
