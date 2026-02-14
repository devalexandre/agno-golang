package knowledge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/storage/sqlite"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// Knowledge is the base interface for knowledge bases
type Knowledge interface {
	// Load loads documents into the knowledge base
	Load(ctx context.Context, recreate bool) error

	// LoadDocument loads a specific document
	LoadDocument(ctx context.Context, doc document.Document) error

	// Search searches for documents in the knowledge base
	Search(ctx context.Context, query string, numDocuments int) ([]*SearchResult, error)

	// Drop removes all documents from the base
	Drop(ctx context.Context) error

	// Exists checks if the knowledge base exists
	Exists(ctx context.Context) (bool, error)

	// GetCount returns the number of documents in the base
	GetCount(ctx context.Context) (int64, error)

	// GetInfo returns information about the knowledge base
	GetInfo() KnowledgeInfo

	// GetContentsDB returns the contents database if configured
	GetContentsDB() storage.Storage
}

// VectorDB is an alias for vectordb.VectorDB for native compatibility like in Python Agno
type VectorDB = vectordb.VectorDB

// SearchResult is an alias for vectordb.SearchResult for native compatibility
type SearchResult = vectordb.SearchResult

// KnowledgeInfo contains information about the knowledge base
type KnowledgeInfo struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SearchFilters define filters for search
type SearchFilters struct {
	Include map[string]interface{} `json:"include,omitempty"`
	Exclude map[string]interface{} `json:"exclude,omitempty"`
}

// BaseKnowledge base implementation for knowledge bases
type BaseKnowledge struct {
	Name         string
	VectorDB     VectorDB
	ContentsDB   storage.Storage // Database for storing knowledge content metadata
	Embedder     embedder.Embedder
	NumDocuments int
	Filters      *SearchFilters
	Recreate     bool
	Metadata     map[string]interface{}
}

// NewBaseKnowledge creates a new BaseKnowledge instance
func NewBaseKnowledge(name string, vectorDB VectorDB) *BaseKnowledge {
	kb := &BaseKnowledge{
		Name:         name,
		VectorDB:     vectorDB,
		NumDocuments: 5,
		Metadata:     make(map[string]interface{}),
	}

	// Create a default SQLite ContentsDB if not provided
	// This enables knowledge content management endpoints by default
	// Following Go philosophy of sensible defaults while allowing customization
	dbFile := fmt.Sprintf("%s_knowledge.db", name)
	contentsDB, err := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{
		ID:                "agno-storage", // Default ID expected by frontend
		TableName:         "knowledge_contents",
		DBFile:            &dbFile,
		SchemaVersion:     1,
		AutoUpgradeSchema: true,
	})
	if err == nil {
		// Initialize the database
		if err := contentsDB.Create(); err == nil {
			kb.ContentsDB = contentsDB
		}
	}
	// If there's an error creating the DB, ContentsDB remains nil
	// and knowledge endpoints will return 404 (which is fine)

	return kb
}

// GetInfo returns information about the knowledge base
func (k *BaseKnowledge) GetInfo() KnowledgeInfo {
	return KnowledgeInfo{
		Name:        k.Name,
		Type:        "base",
		Description: fmt.Sprintf("Base knowledge: %s", k.Name),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    k.Metadata,
	}
}

// GetContentsDB returns the contents database if configured
func (k *BaseKnowledge) GetContentsDB() storage.Storage {
	return k.ContentsDB
}

// SaveContent saves content metadata to ContentsDB
func (k *BaseKnowledge) SaveContent(ctx context.Context, content *Content) error {
	if k.ContentsDB == nil {
		return fmt.Errorf("contents database not configured")
	}

	// Check if ContentsDB implements KnowledgeStorage
	knowledgeStorage, ok := k.ContentsDB.(storage.KnowledgeStorage)
	if !ok {
		return fmt.Errorf("contents database does not implement KnowledgeStorage interface")
	}

	// Convert Content to KnowledgeRow
	createdAt := content.CreatedAt.Unix()
	updatedAt := content.UpdatedAt.Unix()

	row := &storage.KnowledgeRow{
		ID:          content.ID,
		Name:        content.Name,
		Description: content.Description,
		Metadata:    content.Metadata,
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
	}

	// Set optional fields
	if content.FileType != "" {
		row.Type = &content.FileType
	}
	if content.Size > 0 {
		size := int(content.Size)
		row.Size = &size
	}
	status := string(content.Status)
	row.Status = &status
	if content.StatusMessage != "" {
		row.StatusMessage = &content.StatusMessage
	}

	_, err := knowledgeStorage.UpsertKnowledgeContent(row)
	return err
}

// PatchContent updates content metadata in ContentsDB
func (k *BaseKnowledge) PatchContent(ctx context.Context, content *Content) error {
	if k.ContentsDB == nil {
		return fmt.Errorf("contents database not configured")
	}

	// Check if ContentsDB implements KnowledgeStorage
	knowledgeStorage, ok := k.ContentsDB.(storage.KnowledgeStorage)
	if !ok {
		return fmt.Errorf("contents database does not implement KnowledgeStorage interface")
	}

	// Get existing content
	existing, err := knowledgeStorage.GetKnowledgeContent(content.ID)
	if err != nil {
		return fmt.Errorf("failed to get content: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("content not found")
	}

	// Merge with updates
	if content.Name != "" {
		existing.Name = content.Name
	}
	if content.Description != "" {
		existing.Description = content.Description
	}
	if content.Status != "" {
		status := string(content.Status)
		existing.Status = &status
	}
	if content.StatusMessage != "" {
		existing.StatusMessage = &content.StatusMessage
	}
	if content.Metadata != nil {
		existing.Metadata = content.Metadata
	}

	// Update timestamp
	now := time.Now().Unix()
	existing.UpdatedAt = &now

	_, err = knowledgeStorage.UpsertKnowledgeContent(existing)
	return err
}

// GetContent retrieves content metadata from ContentsDB
func (k *BaseKnowledge) GetContent(ctx context.Context, contentID string) (*Content, error) {
	if k.ContentsDB == nil {
		return nil, fmt.Errorf("contents database not configured")
	}

	// Check if ContentsDB implements KnowledgeStorage
	knowledgeStorage, ok := k.ContentsDB.(storage.KnowledgeStorage)
	if !ok {
		return nil, fmt.Errorf("contents database does not implement KnowledgeStorage interface")
	}

	row, err := knowledgeStorage.GetKnowledgeContent(contentID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("content not found")
	}

	// Convert KnowledgeRow to Content
	content := &Content{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Metadata:    row.Metadata,
	}

	if row.Type != nil {
		content.FileType = *row.Type
	}
	if row.Size != nil {
		content.Size = int64(*row.Size)
	}
	if row.Status != nil {
		content.Status = ContentStatus(*row.Status)
	}
	if row.StatusMessage != nil {
		content.StatusMessage = *row.StatusMessage
	}
	if row.CreatedAt != nil {
		content.CreatedAt = time.Unix(*row.CreatedAt, 0)
	}
	if row.UpdatedAt != nil {
		content.UpdatedAt = time.Unix(*row.UpdatedAt, 0)
	}

	return content, nil
}

// ListContents retrieves a paginated list of contents from ContentsDB
func (k *BaseKnowledge) ListContents(ctx context.Context, limit, offset int) ([]*Content, int, error) {
	if k.ContentsDB == nil {
		return nil, 0, fmt.Errorf("contents database not configured")
	}

	// Check if ContentsDB implements KnowledgeStorage
	knowledgeStorage, ok := k.ContentsDB.(storage.KnowledgeStorage)
	if !ok {
		return nil, 0, fmt.Errorf("contents database does not implement KnowledgeStorage interface")
	}

	// Calculate page from offset
	page := 1
	if limit > 0 && offset > 0 {
		page = (offset / limit) + 1
	}

	sortBy := "updated_at"
	sortOrder := "desc"

	rows, totalCount, err := knowledgeStorage.GetKnowledgeContents(&limit, &page, &sortBy, &sortOrder)
	if err != nil {
		return nil, 0, err
	}

	// Convert KnowledgeRows to Contents
	var contents []*Content
	for _, row := range rows {
		content := &Content{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Metadata:    row.Metadata,
		}

		if row.Type != nil {
			content.FileType = *row.Type
		}
		if row.Size != nil {
			content.Size = int64(*row.Size)
		}
		if row.Status != nil {
			content.Status = ContentStatus(*row.Status)
		}
		if row.StatusMessage != nil {
			content.StatusMessage = *row.StatusMessage
		}
		if row.CreatedAt != nil {
			content.CreatedAt = time.Unix(*row.CreatedAt, 0)
		}
		if row.UpdatedAt != nil {
			content.UpdatedAt = time.Unix(*row.UpdatedAt, 0)
		}

		contents = append(contents, content)
	}

	return contents, totalCount, nil
}

// DeleteContent removes content metadata from ContentsDB
func (k *BaseKnowledge) DeleteContent(ctx context.Context, contentID string) error {
	if k.ContentsDB == nil {
		return fmt.Errorf("contents database not configured")
	}

	// Check if ContentsDB implements KnowledgeStorage
	knowledgeStorage, ok := k.ContentsDB.(storage.KnowledgeStorage)
	if !ok {
		return fmt.Errorf("contents database does not implement KnowledgeStorage interface")
	}

	return knowledgeStorage.DeleteKnowledgeContent(contentID)
}

// SearchDocuments searches documents with filters
func (k *BaseKnowledge) SearchDocuments(ctx context.Context, query string, numDocuments int, filters map[string]interface{}) ([]document.Document, error) {
	if k.VectorDB == nil {
		return nil, fmt.Errorf("vector database not configured")
	}

	results, err := k.VectorDB.Search(ctx, query, numDocuments, filters)
	if err != nil {
		return nil, err
	}

	// Convert SearchResult to Document slice
	docs := make([]document.Document, len(results))
	for i, result := range results {
		docs[i] = *result.Document
	}

	return docs, nil
}

// LoadDocuments loads documents into the knowledge base
func (k *BaseKnowledge) LoadDocuments(ctx context.Context, docs []document.Document, recreate bool) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	// Check if should recreate
	if recreate {
		if err := k.VectorDB.Drop(ctx); err != nil {
			// Ignore error if doesn't exist
			fmt.Printf("[KNOWLEDGE] Warning: Failed to drop VectorDB (may not exist): %v\n", err)
		}
	}

	// Create table if doesn't exist
	if err := k.VectorDB.Create(ctx); err != nil {
		fmt.Printf("[KNOWLEDGE] Warning: Failed to create VectorDB table (may already exist): %v\n", err)
		// Continue - table might already exist
	}

	// Insert documents
	if len(docs) > 0 {
		fmt.Printf("[KNOWLEDGE] Inserting %d documents into VectorDB...\n", len(docs))

		// Convert []document.Document to []*document.Document
		docPtrs := make([]*document.Document, len(docs))
		for i := range docs {
			docPtrs[i] = &docs[i]
		}

		// Optimize batch size based on dataset size
		batchSize := 100 // Larger batches for better throughput
		numWorkers := 10 // More workers for faster processing

		if len(docPtrs) > 1000 {
			// Very large datasets: use more workers
			numWorkers = 15
			batchSize = 150
		}

		if len(docPtrs) > 500 {
			// Use parallel processing for large datasets
			return k.insertDocumentsParallel(ctx, docPtrs, batchSize, numWorkers)
		}

		// For small datasets, use simple batching with progress
		return k.insertDocumentsSequential(ctx, docPtrs, batchSize)
	}

	fmt.Printf("[KNOWLEDGE] No documents to insert\n")
	return nil
}

// insertDocumentsSequential inserts documents in batches sequentially
func (k *BaseKnowledge) insertDocumentsSequential(ctx context.Context, docPtrs []*document.Document, batchSize int) error {
	totalBatches := (len(docPtrs) + batchSize - 1) / batchSize

	for i := 0; i < len(docPtrs); i += batchSize {
		end := i + batchSize
		if end > len(docPtrs) {
			end = len(docPtrs)
		}

		batch := docPtrs[i:end]
		batchNum := (i / batchSize) + 1

		if err := k.VectorDB.Insert(ctx, batch, nil); err != nil {
			return fmt.Errorf("failed to insert batch %d: %w", batchNum, err)
		}

		// Show progress bar
		k.showProgressBar(batchNum, totalBatches, end, len(docPtrs))
	}

	fmt.Printf("\n[KNOWLEDGE] ✅ Successfully inserted %d documents\n", len(docPtrs))
	return nil
}

// insertDocumentsParallel inserts documents using parallel goroutines
func (k *BaseKnowledge) insertDocumentsParallel(ctx context.Context, docPtrs []*document.Document, batchSize int, numWorkers int) error {
	totalBatches := (len(docPtrs) + batchSize - 1) / batchSize

	// Create batches
	type batch struct {
		docs []*document.Document
		num  int
	}

	batches := make([]batch, 0, totalBatches)
	for i := 0; i < len(docPtrs); i += batchSize {
		end := i + batchSize
		if end > len(docPtrs) {
			end = len(docPtrs)
		}
		batches = append(batches, batch{
			docs: docPtrs[i:end],
			num:  (i / batchSize) + 1,
		})
	}

	// Channels for communication
	batchChan := make(chan batch, numWorkers)
	progressChan := make(chan int, totalBatches)
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	// Start workers
	for range numWorkers {
		wg.Go(func() {
			for b := range batchChan {
				if err := k.VectorDB.Insert(ctx, b.docs, nil); err != nil {
					select {
					case errChan <- fmt.Errorf("batch %d failed: %w", b.num, err):
					default:
					}
					return
				}
				// Report progress
				progressChan <- b.num
			}
		})
	}

	// Progress monitor goroutine
	progressDone := make(chan struct{})
	go func() {
		completed := 0
		for range progressChan {
			completed++
			docsProcessed := completed * batchSize
			if docsProcessed > len(docPtrs) {
				docsProcessed = len(docPtrs)
			}
			k.showProgressBar(completed, totalBatches, docsProcessed, len(docPtrs))
		}
		close(progressDone)
	}()

	// Send batches to workers
	go func() {
		for _, b := range batches {
			batchChan <- b
		}
		close(batchChan)
	}()

	// Wait for workers to complete
	wg.Wait()
	close(progressChan)
	<-progressDone // Wait for progress monitor to finish

	close(errChan)

	// Check for errors
	if err := <-errChan; err != nil {
		fmt.Printf("\n[KNOWLEDGE] ❌ Failed: %v\n", err)
		return err
	}

	fmt.Printf("\n[KNOWLEDGE] ✅ Successfully inserted %d documents (parallel mode)\n", len(docPtrs))
	return nil
}

// showProgressBar displays a progress bar
func (k *BaseKnowledge) showProgressBar(current, total, docsProcessed, totalDocs int) {
	percentage := float64(current) / float64(total) * 100
	barLength := 30
	filledLength := int(percentage / 100.0 * float64(barLength))

	bar := strings.Repeat("█", filledLength) + strings.Repeat("░", barLength-filledLength)

	fmt.Printf("\r[KNOWLEDGE] [%s] %.1f%% (%d/%d batches, %d/%d docs)",
		bar, percentage, current, total, docsProcessed, totalDocs)
}

// Search implementa Knowledge interface
func (k *BaseKnowledge) Search(ctx context.Context, query string, numDocuments int) ([]*SearchResult, error) {
	if numDocuments <= 0 {
		numDocuments = k.NumDocuments
	}

	var filters map[string]interface{}
	if k.Filters != nil {
		filters = k.Filters.Include
	}

	if k.VectorDB == nil {
		return nil, fmt.Errorf("vector database not configured")
	}

	return k.VectorDB.Search(ctx, query, numDocuments, filters)
}

// SearchWithFilters searches the knowledge base with additional per-query filters.
// This is intentionally not part of the Knowledge interface to keep backwards compatibility.
func (k *BaseKnowledge) SearchWithFilters(ctx context.Context, query string, numDocuments int, filters map[string]interface{}) ([]*SearchResult, error) {
	if numDocuments <= 0 {
		numDocuments = k.NumDocuments
	}

	if k.VectorDB == nil {
		return nil, fmt.Errorf("vector database not configured")
	}

	merged := make(map[string]interface{})
	if k.Filters != nil && k.Filters.Include != nil {
		for key, value := range k.Filters.Include {
			merged[key] = value
		}
	}
	for key, value := range filters {
		merged[key] = value
	}
	if len(merged) == 0 {
		merged = nil
	}

	return k.VectorDB.Search(ctx, query, numDocuments, merged)
}

// Add adds documents to the knowledge base
func (k *BaseKnowledge) Add(ctx context.Context, documents []document.Document) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	// Convert []document.Document to []*document.Document
	docPtrs := make([]*document.Document, len(documents))
	for i := range documents {
		docPtrs[i] = &documents[i]
	}

	return k.VectorDB.Insert(ctx, docPtrs, nil)
}

// Upsert upserts documents into the knowledge base.
// This is intentionally not part of the Knowledge interface to keep backwards compatibility.
func (k *BaseKnowledge) Upsert(ctx context.Context, documents []document.Document) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	docPtrs := make([]*document.Document, len(documents))
	for i := range documents {
		docPtrs[i] = &documents[i]
	}

	return k.VectorDB.Upsert(ctx, docPtrs, nil)
}

// UpsertDocument upserts a single document into the knowledge base.
// This is intentionally not part of the Knowledge interface to keep backwards compatibility.
func (k *BaseKnowledge) UpsertDocument(ctx context.Context, doc document.Document) error {
	return k.Upsert(ctx, []document.Document{doc})
}

// Exists checks if the knowledge base exists
func (k *BaseKnowledge) Exists(ctx context.Context) (bool, error) {
	if k.VectorDB == nil {
		return false, fmt.Errorf("vector database not configured")
	}

	return k.VectorDB.Exists(ctx)
}

// Drop removes the knowledge base
func (k *BaseKnowledge) Drop(ctx context.Context) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	return k.VectorDB.Drop(ctx)
}

// Load implements Knowledge interface
func (k *BaseKnowledge) Load(ctx context.Context, recreate bool) error {
	// Empty default implementation - should be overridden by subclasses
	return nil
}

// LoadDocument loads a single document into the knowledge base
func (k *BaseKnowledge) LoadDocument(ctx context.Context, doc document.Document) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	// Load single document
	return k.LoadDocuments(ctx, []document.Document{doc}, false)
}

// GetCount returns the number of documents in the knowledge base
func (k *BaseKnowledge) GetCount(ctx context.Context) (int64, error) {
	if k.VectorDB == nil {
		return 0, fmt.Errorf("vector database not configured")
	}
	return k.VectorDB.GetCount(ctx)
}

// SetEmbedder configures the embedder
func (k *BaseKnowledge) SetEmbedder(emb embedder.Embedder) {
	k.Embedder = emb
	// Note: VectorDB implementation should handle embedder internally
}

// GetEmbedder returns the configured embedder
func (k *BaseKnowledge) GetEmbedder() embedder.Embedder {
	if k.Embedder != nil {
		return k.Embedder
	}
	if k.VectorDB != nil {
		return k.VectorDB.GetEmbedder()
	}
	return nil
}

// ValidateDocuments validates documents before processing
func ValidateDocuments(docs []document.Document) error {
	if len(docs) == 0 {
		return fmt.Errorf("no documents to process")
	}

	for i, doc := range docs {
		if doc.ID == "" {
			return fmt.Errorf("document at index %d has empty ID", i)
		}
		if doc.Content == "" {
			return fmt.Errorf("document at index %d has empty content", i)
		}
	}

	return nil
}

// SanitizeFileName sanitizes filename to use as collection/table name
func SanitizeFileName(filename string) string {
	// Remove extension
	name := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	// Replace special characters with underscore
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")

	// Ensure it starts with a letter
	if len(name) > 0 && !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
		name = "kb_" + name
	}

	// Convert to lowercase
	return strings.ToLower(name)
}
