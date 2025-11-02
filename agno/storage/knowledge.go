package storage

import "time"

// KnowledgeRow represents a knowledge content row stored in the database
// Compatible with Python's KnowledgeRow schema
type KnowledgeRow struct {
	ID            string                 `json:"id" db:"id"`
	Name          string                 `json:"name" db:"name"`
	Description   string                 `json:"description" db:"description"`
	Metadata      map[string]interface{} `json:"metadata" db:"metadata"`
	Type          *string                `json:"type" db:"type"`
	Size          *int                   `json:"size" db:"size"`
	LinkedTo      *string                `json:"linked_to" db:"linked_to"`
	AccessCount   *int                   `json:"access_count" db:"access_count"`
	Status        *string                `json:"status" db:"status"`
	StatusMessage *string                `json:"status_message" db:"status_message"`
	CreatedAt     *int64                 `json:"created_at" db:"created_at"`
	UpdatedAt     *int64                 `json:"updated_at" db:"updated_at"`
	ExternalID    *string                `json:"external_id" db:"external_id"`
}

// ToMap converts KnowledgeRow to map for database operations
func (kr *KnowledgeRow) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"id":          kr.ID,
		"name":        kr.Name,
		"description": kr.Description,
		"metadata":    kr.Metadata,
	}

	if kr.Type != nil {
		result["type"] = *kr.Type
	}
	if kr.Size != nil {
		result["size"] = *kr.Size
	}
	if kr.LinkedTo != nil {
		result["linked_to"] = *kr.LinkedTo
	}
	if kr.AccessCount != nil {
		result["access_count"] = *kr.AccessCount
	}
	if kr.Status != nil {
		result["status"] = *kr.Status
	}
	if kr.StatusMessage != nil {
		result["status_message"] = *kr.StatusMessage
	}
	if kr.CreatedAt != nil {
		result["created_at"] = *kr.CreatedAt
	}
	if kr.UpdatedAt != nil {
		result["updated_at"] = *kr.UpdatedAt
	}
	if kr.ExternalID != nil {
		result["external_id"] = *kr.ExternalID
	}

	return result
}

// KnowledgeRowFromMap creates a KnowledgeRow from a map
func KnowledgeRowFromMap(data map[string]interface{}) *KnowledgeRow {
	kr := &KnowledgeRow{
		Metadata: make(map[string]interface{}),
	}

	if v, ok := data["id"].(string); ok {
		kr.ID = v
	}
	if v, ok := data["name"].(string); ok {
		kr.Name = v
	}
	if v, ok := data["description"].(string); ok {
		kr.Description = v
	}
	if v, ok := data["metadata"].(map[string]interface{}); ok {
		kr.Metadata = v
	}

	if v, ok := data["type"].(string); ok {
		kr.Type = &v
	}
	if v, ok := data["size"].(int); ok {
		kr.Size = &v
	} else if v, ok := data["size"].(int64); ok {
		size := int(v)
		kr.Size = &size
	}
	if v, ok := data["linked_to"].(string); ok {
		kr.LinkedTo = &v
	}
	if v, ok := data["access_count"].(int); ok {
		kr.AccessCount = &v
	} else if v, ok := data["access_count"].(int64); ok {
		count := int(v)
		kr.AccessCount = &count
	}
	if v, ok := data["status"].(string); ok {
		kr.Status = &v
	}
	if v, ok := data["status_message"].(string); ok {
		kr.StatusMessage = &v
	}
	if v, ok := data["created_at"].(int64); ok {
		kr.CreatedAt = &v
	} else if v, ok := data["created_at"].(int); ok {
		ts := int64(v)
		kr.CreatedAt = &ts
	}
	if v, ok := data["updated_at"].(int64); ok {
		kr.UpdatedAt = &v
	} else if v, ok := data["updated_at"].(int); ok {
		ts := int64(v)
		kr.UpdatedAt = &ts
	}
	if v, ok := data["external_id"].(string); ok {
		kr.ExternalID = &v
	}

	return kr
}

// KnowledgeStorage defines the interface for knowledge content storage
// Compatible with Python's BaseDb knowledge methods
type KnowledgeStorage interface {
	// Get a single knowledge content by ID
	GetKnowledgeContent(id string) (*KnowledgeRow, error)

	// Get all knowledge contents with pagination
	GetKnowledgeContents(limit, page *int, sortBy, sortOrder *string) ([]*KnowledgeRow, int, error)

	// Upsert (insert or update) a knowledge content
	UpsertKnowledgeContent(row *KnowledgeRow) (*KnowledgeRow, error)

	// Delete a knowledge content
	DeleteKnowledgeContent(id string) error
}

// Helper function to create a new KnowledgeRow
func NewKnowledgeRow(id, name, description string) *KnowledgeRow {
	now := time.Now().Unix()
	zeroInt := 0
	processing := "processing"

	return &KnowledgeRow{
		ID:          id,
		Name:        name,
		Description: description,
		Metadata:    make(map[string]interface{}),
		AccessCount: &zeroInt,
		Status:      &processing,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
}
