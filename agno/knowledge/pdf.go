package knowledge

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/storage/sqlite"
)

// PDFConfig holds configuration for PDF processing
type PDFConfig struct {
	Path     string                 `json:"path"`
	URL      string                 `json:"url,omitempty"`
	Password string                 `json:"password,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PDFKnowledgeBase handles PDF knowledge bases (local files and URLs)
type PDFKnowledgeBase struct {
	*BaseKnowledge
	Paths        []string    `json:"paths,omitempty"`         // Local file paths
	URLs         []string    `json:"urls,omitempty"`          // PDF URLs
	Configs      []PDFConfig `json:"configs,omitempty"`       // Advanced configurations
	ExcludeFiles []string    `json:"exclude_files,omitempty"` // Files to exclude
	Formats      []string    `json:"formats"`                 // Supported formats
	ChunkSize    int         `json:"chunk_size"`              // Text chunk size
	ChunkOverlap int         `json:"chunk_overlap"`           // Overlap between chunks
}

func (p *PDFKnowledgeBase) Search(ctx context.Context, query string, numDocuments int) ([]*SearchResult, error) {
	if p.VectorDB == nil {
		return nil, fmt.Errorf("vector database not configured")
	}

	return p.VectorDB.Search(ctx, query, numDocuments, nil)

}

func (p *PDFKnowledgeBase) GetCount(ctx context.Context) (int64, error) {
	if p.VectorDB == nil {
		return 0, fmt.Errorf("vector database not configured")
	}
	return p.VectorDB.GetCount(ctx)
}

// NewPDFKnowledgeBase creates a new PDF knowledge base
func NewPDFKnowledgeBase(name string, vectorDB VectorDB) *PDFKnowledgeBase {
	base := NewBaseKnowledge(name, vectorDB)
	base.Metadata["description"] = "PDF knowledge base"
	base.Metadata["type"] = "pdf"

	// Create a default SQLite ContentsDB if not provided
	// This enables knowledge content management endpoints by default
	if base.ContentsDB == nil {
		// Create a default SQLite database for storing knowledge content metadata
		dbFile := fmt.Sprintf("%s_knowledge.db", name)
		contentsDB, err := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{
			ID:                generateUUID(), // Default ID expected by frontend
			TableName:         "knowledge_contents",
			DBFile:            &dbFile,
			SchemaVersion:     1,
			AutoUpgradeSchema: true,
		})
		if err == nil {
			// Initialize the database
			if err := contentsDB.Create(); err == nil {
				base.ContentsDB = contentsDB
			}
		}
		// If there's an error creating the DB, ContentsDB remains nil
		// and knowledge endpoints will return 404 (which is fine)
	}

	kb := &PDFKnowledgeBase{
		BaseKnowledge: base,
		Formats:       []string{".pdf"},
		ChunkSize:     500,
		ChunkOverlap:  50,
	}

	return kb
}

// LoadPDFKnowledgeBase loads a PDF knowledge base from config
func LoadPDFKnowledgeBase(name string, vectorDB VectorDB, paths []string, urls []string, configs []PDFConfig) *PDFKnowledgeBase {
	kb := NewPDFKnowledgeBase(name, vectorDB)
	kb.Paths = paths
	kb.URLs = urls
	kb.Configs = configs

	return kb
}

// Load loads all configured PDF documents
func (p *PDFKnowledgeBase) Load(ctx context.Context, recreate bool) error {
	if recreate && p.VectorDB != nil {
		if err := p.VectorDB.Drop(ctx); err != nil {
			return fmt.Errorf("failed to drop existing database: %w", err)
		}
	}

	if p.VectorDB != nil {
		if err := p.VectorDB.Create(ctx); err != nil {
			return fmt.Errorf("failed to create vector database: %w", err)
		}
	}

	var allDocuments []*document.Document
	totalSources := len(p.Paths) + len(p.URLs) + len(p.Configs)
	currentSource := 0

	fmt.Printf("📚 Loading %d PDF source(s)...\n", totalSources)

	// Load from paths
	if len(p.Paths) > 0 {
		for _, path := range p.Paths {
			currentSource++
			p.showLoadProgress(currentSource, totalSources, fmt.Sprintf("Loading: %s", path))

			docs, err := p.loadFromPath(path, nil)
			if err != nil {
				return fmt.Errorf("failed to load from path %s: %w", path, err)
			}
			allDocuments = append(allDocuments, docs...)

			fmt.Printf(" ✅ %d documents extracted\n", len(docs))
		}
	}

	// Load from URLs
	if len(p.URLs) > 0 {
		for _, url := range p.URLs {
			currentSource++
			p.showLoadProgress(currentSource, totalSources, fmt.Sprintf("Downloading: %s", url))

			docs, err := p.loadFromURL(url, nil)
			if err != nil {
				return fmt.Errorf("failed to load from URL %s: %w", url, err)
			}
			allDocuments = append(allDocuments, docs...)

			fmt.Printf(" ✅ %d documents extracted\n", len(docs))
		}
	}

	// Load from configurations
	if len(p.Configs) > 0 {
		for _, config := range p.Configs {
			currentSource++
			source := config.Path
			if source == "" {
				source = config.URL
			}
			p.showLoadProgress(currentSource, totalSources, fmt.Sprintf("Processing config: %s", source))

			var docs []*document.Document
			var err error

			if config.Path != "" {
				docs, err = p.loadFromPath(config.Path, config.Metadata)
			} else if config.URL != "" {
				docs, err = p.loadFromURL(config.URL, config.Metadata)
			} else {
				continue // Skip invalid configs
			}

			if err != nil {
				return fmt.Errorf("failed to load from config: %w", err)
			}
			allDocuments = append(allDocuments, docs...)

			fmt.Printf(" ✅ %d documents extracted\n", len(docs))
		}
	}

	if len(allDocuments) == 0 {
		return fmt.Errorf("no PDF documents found or configured")
	}

	fmt.Printf("\n📊 Total documents loaded: %d\n", len(allDocuments))
	fmt.Println("🔄 Starting vector database insertion...")

	// Convert pointers to values for LoadDocuments
	convertedDocs := ConvertDocumentPointers(allDocuments)

	// Load documents into vector database with progress
	fmt.Println("🚀 Inserting documents into vector database...")
	return p.LoadDocumentsWithProgress(ctx, convertedDocs, false)
}

// showLoadProgress displays a visual progress bar for loading operations
func (p *PDFKnowledgeBase) showLoadProgress(current, total int, message string) {
	percentage := float64(current) / float64(total) * 100
	barLength := 30
	filledLength := int(percentage / 100 * float64(barLength))

	bar := ""
	for i := 0; i < barLength; i++ {
		if i < filledLength {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	fmt.Printf("\r📈 [%s] %.1f%% (%d/%d) %s", bar, percentage, current, total, message)
}

// LoadDocumentsWithProgress loads documents with visual progress tracking
func (p *PDFKnowledgeBase) LoadDocumentsWithProgress(ctx context.Context, docs []document.Document, recreate bool) error {
	if len(docs) == 0 {
		return nil
	}

	fmt.Printf("🚀 Processing %d documents for insertion...\n", len(docs))

	// Process documents in batches for better progress tracking
	batchSize := 10
	totalBatches := (len(docs) + batchSize - 1) / batchSize

	for i := 0; i < len(docs); i += batchSize {

		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}

		batch := docs[i:end]
		batchNum := (i / batchSize) + 1

		fmt.Println("Inserting batch...")
		err := p.LoadDocuments(ctx, batch, false) // Don't recreate for each batch
		if err != nil {
			return fmt.Errorf("failed to load batch %d: %w", batchNum, err)
		}

		// Show progress
		p.showInsertProgress(batchNum, totalBatches, fmt.Sprintf("Batch %d - %d documents", batchNum, len(batch)))

	}

	fmt.Printf("\n✅ Processing complete!\n")
	return nil
}

// showInsertProgress displays a visual progress bar for insertion operations
func (p *PDFKnowledgeBase) showInsertProgress(current, total int, message string) {
	percentage := float64(current) / float64(total) * 100
	barLength := 25
	filledLength := int(percentage / 100 * float64(barLength))

	bar := ""
	for i := 0; i < barLength; i++ {
		if i < filledLength {
			bar += "▓"
		} else {
			bar += "▒"
		}
	}

	fmt.Printf("\r💾 [%s] %.1f%% (%d/%d) %s", bar, percentage, current, total, message)
}

// LoadAsync loads documents asynchronously
func (p *PDFKnowledgeBase) LoadAsync(ctx context.Context, recreate bool) error {
	return p.Load(ctx, recreate)
}

// LoadParallel loads documents with parallel processing using goroutines
func (p *PDFKnowledgeBase) LoadParallel(ctx context.Context, recreate bool, numWorkers int) error {
	if recreate && p.VectorDB != nil {
		if err := p.VectorDB.Drop(ctx); err != nil {
			return fmt.Errorf("failed to drop existing database: %w", err)
		}
	}

	if p.VectorDB != nil {
		if err := p.VectorDB.Create(ctx); err != nil {
			fmt.Println("Collection already exists, skipping creation.")
		}
	}

	var allDocuments []*document.Document
	totalSources := len(p.Paths) + len(p.URLs) + len(p.Configs)
	currentSource := 0

	fmt.Printf("📚 Parallel loading of %d source(s) with %d workers...\n", totalSources, numWorkers)

	// Load documents sequentially first (PDF parsing is I/O bound)
	if len(p.Paths) > 0 {
		for _, path := range p.Paths {
			currentSource++
			p.showLoadProgress(currentSource, totalSources, fmt.Sprintf("Loading: %s", path))

			docs, err := p.loadFromPath(path, nil)
			if err != nil {
				return fmt.Errorf("failed to load from path %s: %w", path, err)
			}
			allDocuments = append(allDocuments, docs...)

			fmt.Printf(" ✅ %d documents\n", len(docs))
		}
	}

	if len(p.URLs) > 0 {
		for _, url := range p.URLs {
			currentSource++
			p.showLoadProgress(currentSource, totalSources, fmt.Sprintf("Downloading: %s", url))

			docs, err := p.loadFromURL(url, nil)
			if err != nil {
				return fmt.Errorf("failed to load from URL %s: %w", url, err)
			}
			allDocuments = append(allDocuments, docs...)

			fmt.Printf(" ✅ %d documents\n", len(docs))
		}
	}

	if len(p.Configs) > 0 {
		for _, config := range p.Configs {
			currentSource++
			source := config.Path
			if source == "" {
				source = config.URL
			}
			p.showLoadProgress(currentSource, totalSources, fmt.Sprintf("Config: %s", source))

			var docs []*document.Document
			var err error

			if config.Path != "" {
				docs, err = p.loadFromPath(config.Path, config.Metadata)
			} else if config.URL != "" {
				docs, err = p.loadFromURL(config.URL, config.Metadata)
			} else {
				continue
			}

			if err != nil {
				return fmt.Errorf("failed to load from config: %w", err)
			}
			allDocuments = append(allDocuments, docs...)

			fmt.Printf(" ✅ %d documents\n", len(docs))
		}
	}

	if len(allDocuments) == 0 {
		return fmt.Errorf("no documents found")
	}

	fmt.Printf("\n📊 Total documents loaded: %d\n", len(allDocuments))
	fmt.Printf("🚀 Starting parallel vector database insertion with %d workers...\n", numWorkers)

	// Convert pointers to values for LoadDocumentsParallel
	convertedDocs := ConvertDocumentPointers(allDocuments)

	// Load documents in parallel with progress tracking
	return p.LoadDocumentsParallel(ctx, convertedDocs, numWorkers)
}

// LoadDocumentsParallel loads documents in parallel using worker goroutines
func (p *PDFKnowledgeBase) LoadDocumentsParallel(ctx context.Context, docs []document.Document, numWorkers int) error {
	if len(docs) == 0 {
		return nil
	}

	if numWorkers <= 0 {
		numWorkers = 2 // Default to 2 workers
	}

	fmt.Printf("⚡ Parallel processing with %d workers for %d documents...\n", numWorkers, len(docs))

	// Create channels for work distribution
	docChan := make(chan document.Document, len(docs))
	errorChan := make(chan error, len(docs))
	doneChan := make(chan bool, numWorkers)

	// Progress tracking
	processedCount := 0
	totalDocs := len(docs)

	// Send all documents to channel
	for _, doc := range docs {
		docChan <- doc
	}
	close(docChan)

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			for doc := range docChan {
				// Process document with retry logic
				err := p.processDocumentWithRetry(ctx, doc, 3)
				if err != nil {
					errorChan <- fmt.Errorf("worker %d failed to process document: %w", workerID, err)
				} else {
					errorChan <- nil
				}

			}
			doneChan <- true
		}(i)
	}

	// Collect results and show progress
	var firstError error
	for i := 0; i < totalDocs; i++ {
		err := <-errorChan
		if err != nil && firstError == nil {
			firstError = err
		}

		processedCount++
		percentage := float64(processedCount) / float64(totalDocs) * 100

		// Show progress with Unicode bars
		barLength := 30
		filledLength := int(percentage / 100 * float64(barLength))

		bar := ""
		for j := 0; j < barLength; j++ {
			if j < filledLength {
				bar += "█"
			} else {
				bar += "░"
			}
		}

		fmt.Printf("\r🔄 [%s] %.1f%% (%d/%d) processados", bar, percentage, processedCount, totalDocs)
	}

	// Wait for all workers to finish
	for i := 0; i < numWorkers; i++ {
		<-doneChan
	}

	fmt.Printf("\n✅ Parallel processing complete!\n")

	return firstError
}

// processDocumentWithRetry processes a document with retry logic
func (p *PDFKnowledgeBase) processDocumentWithRetry(ctx context.Context, doc document.Document, maxRetries int) error {
	var lastErr error

	for retry := 0; retry <= maxRetries; retry++ {
		err := p.LoadDocument(ctx, doc)
		if err == nil {
			return nil
		}

		lastErr = err

		if retry < maxRetries {
			// Exponential backoff
			backoffTime := time.Duration(retry+1) * 200 * time.Millisecond
			time.Sleep(backoffTime)
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries+1, lastErr)
}

// LoadDocumentFromPath loads a document from a file path or URL
func (p *PDFKnowledgeBase) LoadDocumentFromPath(ctx context.Context, pathOrURL string, metadata map[string]interface{}) error {
	var docs []*document.Document
	var err error

	// Determine if it's a URL or local path
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		docs, err = p.loadFromURL(pathOrURL, metadata)
	} else {
		docs, err = p.loadFromPath(pathOrURL, metadata)
	}

	if err != nil {
		return err
	}

	// Convert pointers to values
	convertedDocs := ConvertDocumentPointers(docs)

	// Load documents into vector database
	return p.LoadDocuments(ctx, convertedDocs, false)
}

// LoadDocument loads a single document into the vector database
func (p *PDFKnowledgeBase) LoadDocument(ctx context.Context, doc document.Document) error {
	if p.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	// Chunk the document if it's large
	chunks := p.chunkDocument(doc)

	// Load all chunks using BaseKnowledge method
	return p.BaseKnowledge.LoadDocuments(ctx, chunks, false)
}

// GetInfo returns information about the PDF knowledge base
func (p *PDFKnowledgeBase) GetInfo() KnowledgeInfo {
	info := p.BaseKnowledge.GetInfo()
	info.Type = "pdf"
	info.Description = "PDF knowledge base supporting local files and URLs"

	// Add sources to metadata
	sources := append(p.Paths, p.URLs...)
	for _, config := range p.Configs {
		if config.Path != "" {
			sources = append(sources, config.Path)
		}
		if config.URL != "" {
			sources = append(sources, config.URL)
		}
	}

	info.Metadata["sources"] = sources
	return info
}

// loadFromPath loads documents from a local file path
func (p *PDFKnowledgeBase) loadFromPath(path string, metadata map[string]interface{}) ([]*document.Document, error) {
	// Handle both single files and directories
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("path not found: %w", err)
	}

	var files []string

	if fileInfo.IsDir() {
		// Scan directory for PDF files
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && strings.ToLower(filepath.Ext(filePath)) == ".pdf" {
				// Check if file should be excluded
				for _, exclude := range p.ExcludeFiles {
					if strings.Contains(filePath, exclude) {
						return nil
					}
				}
				files = append(files, filePath)
			}
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to scan directory: %w", err)
		}
	} else {
		// Single file
		if strings.ToLower(filepath.Ext(path)) != ".pdf" {
			return nil, fmt.Errorf("file is not a PDF: %s", path)
		}
		files = []string{path}
	}

	var allDocs []*document.Document

	for _, filePath := range files {
		docs, err := p.extractPDFContent(filePath, "", metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to extract content from %s: %w", filePath, err)
		}
		allDocs = append(allDocs, docs...)
	}

	return allDocs, nil
}

// loadFromURL loads documents from a PDF URL
func (p *PDFKnowledgeBase) loadFromURL(pdfURL string, metadata map[string]interface{}) ([]*document.Document, error) {
	// Download PDF to temporary file
	tempFile, err := p.downloadPDF(pdfURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download PDF: %w", err)
	}
	defer os.Remove(tempFile) // Clean up

	// Extract content
	docs, err := p.extractPDFContent(tempFile, pdfURL, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content: %w", err)
	}

	return docs, nil
}

// downloadPDF downloads a PDF from URL to a temporary file
func (p *PDFKnowledgeBase) downloadPDF(pdfURL string) (string, error) {
	// Validate URL
	_, err := url.Parse(pdfURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp("", "agno_pdf_*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Download PDF
	resp, err := http.Get(pdfURL)
	if err != nil {
		return "", fmt.Errorf("failed to download PDF: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download PDF: status %d", resp.StatusCode)
	}

	// Copy content to temp file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save PDF: %w", err)
	}

	return tempFile.Name(), nil
}

// extractPDFContent extracts content from a PDF file
func (p *PDFKnowledgeBase) extractPDFContent(filePath, sourceURL string, metadata map[string]interface{}) ([]*document.Document, error) {
	if !p.isValidPDF(filePath) {
		return nil, fmt.Errorf("invalid PDF file: %s", filePath)
	}

	// Try extracting with pdftotext first
	content, err := p.extractWithPdftotext(filePath)
	if err != nil {
		// Fallback to basic method
		content, err = p.extractWithBasicMethod(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to extract PDF content: %w", err)
		}
	}

	// Sanitize content
	content = p.sanitizeUTF8(content)

	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("no text content found in PDF")
	}

	// Chunk the content
	chunks := p.chunkText(content)

	var docs []*document.Document

	source := filePath
	if sourceURL != "" {
		source = sourceURL
	}

	for i, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}

		// Prepare document metadata
		docMetadata := map[string]interface{}{
			"source":       source,
			"type":         "pdf",
			"chunk_id":     i,
			"total_chunks": len(chunks),
			"file_path":    filePath,
		}

		// Add custom metadata
		for k, v := range metadata {
			docMetadata[k] = v
		}

		if sourceURL != "" {
			docMetadata["url"] = sourceURL
		}

		// Generate a unique ID for the chunk based on its content
		chunkID := sha1.New()
		chunkID.Write([]byte(chunk))
		id := hex.EncodeToString(chunkID.Sum(nil))
		docMetadata["id"] = id

		doc := &document.Document{
			ID:          id,
			Content:     chunk,
			ContentType: "text/plain",
			Source:      source,
			Metadata:    docMetadata,
		}

		docs = append(docs, doc)
	}

	return docs, nil
}

// extractWithPdftotext extracts text using the pdftotext command-line tool
func (p *PDFKnowledgeBase) extractWithPdftotext(filePath string) (string, error) {
	// Check if pdftotext is available
	_, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("pdftotext not found: %w", err)
	}

	// Create temporary output file
	tempFile, err := os.CreateTemp("", "agno_pdf_text_*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Run pdftotext command
	cmd := exec.Command("pdftotext", "-layout", "-enc", "UTF-8", filePath, tempFile.Name())
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed: %w", err)
	}

	// Read extracted content
	content, err := os.ReadFile(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read extracted content: %w", err)
	}

	return string(content), nil
}

// sanitizeUTF8 ensures the text contains only valid UTF-8 characters
func (p *PDFKnowledgeBase) sanitizeUTF8(text string) string {
	// Replace invalid UTF-8 sequences
	text = strings.ToValidUTF8(text, "")

	// Remove or replace problematic characters
	var builder strings.Builder
	for _, r := range text {
		if utf8.ValidRune(r) && unicode.IsPrint(r) || unicode.IsSpace(r) {
			builder.WriteRune(r)
		}
	}

	// Clean up excessive whitespace
	lines := strings.Split(builder.String(), "\n")
	var cleanLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// extractWithBasicMethod extracts text using basic PDF reading (fallback method)
func (p *PDFKnowledgeBase) extractWithBasicMethod(filePath string) (string, error) {
	// This is a simple fallback that reads the raw PDF file
	// In a real implementation, you'd use a proper PDF library like "github.com/ledongthuc/pdf"

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF file: %w", err)
	}

	// Very basic text extraction (not reliable for complex PDFs)
	text := string(content)

	// Remove binary data and keep only printable text
	var cleanText strings.Builder
	for _, char := range text {
		if unicode.IsPrint(char) || unicode.IsSpace(char) {
			cleanText.WriteRune(char)
		}
	}

	result := cleanText.String()
	if len(result) < 100 {
		return "", fmt.Errorf("insufficient text extracted using basic method")
	}

	return result, nil
}

// chunkText splits text into smaller chunks for processing
func (p *PDFKnowledgeBase) chunkText(text string) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var chunks []string
	var currentChunk strings.Builder
	currentLength := 0

	for _, word := range words {
		wordLength := len(word) + 1 // +1 for space

		if currentLength+wordLength > p.ChunkSize && currentChunk.Len() > 0 {
			// Start new chunk
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))

			// Handle overlap
			overlap := strings.Fields(currentChunk.String())
			overlapWords := len(overlap) - p.ChunkOverlap
			if overlapWords < 0 {
				overlapWords = 0
			}

			currentChunk.Reset()
			currentLength = 0

			// Add overlap words to new chunk
			for i := overlapWords; i < len(overlap); i++ {
				currentChunk.WriteString(overlap[i])
				currentChunk.WriteString(" ")
				currentLength += len(overlap[i]) + 1
			}
		}

		currentChunk.WriteString(word)
		currentChunk.WriteString(" ")
		currentLength += wordLength
	}

	// Add the last chunk
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

// chunkDocument creates smaller documents from a large document
func (p *PDFKnowledgeBase) chunkDocument(doc document.Document) []document.Document {
	content := doc.Content
	if len(content) <= p.ChunkSize {
		return []document.Document{doc}
	}

	chunks := p.chunkText(content)
	var documents []document.Document

	for i, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}

		// Create metadata for chunk
		chunkMetadata := make(map[string]interface{})
		for k, v := range doc.Metadata {
			chunkMetadata[k] = v
		}

		// Add chunk-specific metadata
		chunkMetadata["chunk_index"] = i
		chunkMetadata["total_chunks"] = len(chunks)
		chunkMetadata["chunk_size"] = len(chunk)

		// Gerar um UUID para cada chunk
		id := generateUUID()

		chunkDoc := document.Document{
			ID:          id,
			Content:     chunk,
			ContentType: "text/plain",
			Source:      doc.Source,
			Metadata:    chunkMetadata,
		}

		documents = append(documents, chunkDoc)
	}

	return documents
}

// isValidPDF checks if a file is a valid PDF
func (p *PDFKnowledgeBase) isValidPDF(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Check PDF magic number
	buffer := make([]byte, 4)
	_, err = file.Read(buffer)
	if err != nil {
		return false
	}

	return string(buffer) == "%PDF"
}

// generateUUID generates a new UUID
func generateUUID() string {
	return uuid.New().String()
}
