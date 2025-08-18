package knowledge

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
)

// TextKnowledgeBase handles text file knowledge bases
type TextKnowledgeBase struct {
	*BaseKnowledge
	Path    string   `json:"path"`    // Directory or file path
	Formats []string `json:"formats"` // Supported file formats
}

// NewTextKnowledgeBase creates a new text knowledge base
func NewTextKnowledgeBase(name, path string, vectorDB VectorDB) *TextKnowledgeBase {
	base := NewBaseKnowledge(name, vectorDB)
	base.Metadata["description"] = "Text file knowledge base"
	base.Metadata["path"] = path

	return &TextKnowledgeBase{
		BaseKnowledge: base,
		Path:          path,
		Formats:       []string{".txt", ".md", ".text"},
	}
}

// Load loads text documents from the specified path
func (t *TextKnowledgeBase) Load(ctx context.Context, recreate bool) error {
	if recreate && t.VectorDB != nil {
		if err := t.VectorDB.Drop(ctx); err != nil {
			return fmt.Errorf("failed to drop existing database: %w", err)
		}
	}

	if t.VectorDB != nil {
		if err := t.VectorDB.Create(ctx); err != nil {
			return fmt.Errorf("failed to create vector database: %w", err)
		}
	}

	documents, err := t.loadTextDocuments()
	if err != nil {
		return fmt.Errorf("failed to load text documents: %w", err)
	}

	if len(documents) == 0 {
		return fmt.Errorf("no valid text documents found in path: %s", t.Path)
	}

	// Convert and load documents
	convertedDocs := ConvertDocumentPointers(documents)
	return t.LoadDocuments(ctx, convertedDocs, recreate)
}

// LoadAsync loads documents asynchronously
func (t *TextKnowledgeBase) LoadAsync(ctx context.Context, recreate bool) error {
	// For now, implement as synchronous
	return t.Load(ctx, recreate)
}

// GetInfo returns information about the text knowledge base
func (t *TextKnowledgeBase) GetInfo() KnowledgeInfo {
	info := t.BaseKnowledge.GetInfo()
	info.Type = "text"
	info.Metadata["path"] = t.Path
	info.Metadata["formats"] = t.Formats
	return info
}

// loadTextDocuments loads text documents from the path
func (t *TextKnowledgeBase) loadTextDocuments() ([]*document.Document, error) {
	var documents []*document.Document

	// Check if path is a file or directory
	fileInfo, err := os.Stat(t.Path)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %s", t.Path)
	}

	if fileInfo.IsDir() {
		// Load all text files from directory
		err = filepath.Walk(t.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && IsValidFileFormat(path, t.Formats) {
				doc, err := t.loadTextFile(path)
				if err != nil {
					return fmt.Errorf("failed to load file %s: %w", path, err)
				}
				documents = append(documents, doc)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		// Load single file
		if !IsValidFileFormat(t.Path, t.Formats) {
			return nil, fmt.Errorf("unsupported file format: %s", t.Path)
		}
		doc, err := t.loadTextFile(t.Path)
		if err != nil {
			return nil, err
		}
		documents = append(documents, doc)
	}

	return documents, nil
}

// loadTextFile loads a single text file
func (t *TextKnowledgeBase) loadTextFile(filePath string) (*document.Document, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	doc := document.NewDocument(string(content))
	doc.Name = filepath.Base(filePath)
	doc.Source = filePath
	doc.ContentType = "text/plain"

	// Add file metadata
	doc.AddMetadata("file_path", filePath)
	doc.AddMetadata("file_extension", GetFileExtension(filePath))
	doc.AddMetadata("file_size", len(content))

	return doc, nil
}

// JSONKnowledgeBase handles JSON file knowledge bases
type JSONKnowledgeBase struct {
	*BaseKnowledge
	Path    string   `json:"path"`    // Directory or file path
	Formats []string `json:"formats"` // Supported file formats
}

// NewJSONKnowledgeBase creates a new JSON knowledge base
func NewJSONKnowledgeBase(name, path string, vectorDB VectorDB) *JSONKnowledgeBase {
	base := NewBaseKnowledge(name, vectorDB)
	base.Metadata["description"] = "JSON file knowledge base"
	base.Metadata["path"] = path

	return &JSONKnowledgeBase{
		BaseKnowledge: base,
		Path:          path,
		Formats:       []string{".json"},
	}
}

// Load loads JSON documents from the specified path
func (j *JSONKnowledgeBase) Load(ctx context.Context, recreate bool) error {
	if recreate && j.VectorDB != nil {
		if err := j.VectorDB.Drop(ctx); err != nil {
			return fmt.Errorf("failed to drop existing database: %w", err)
		}
	}

	if j.VectorDB != nil {
		if err := j.VectorDB.Create(ctx); err != nil {
			return fmt.Errorf("failed to create vector database: %w", err)
		}
	}

	documents, err := j.loadJSONDocuments()
	if err != nil {
		return fmt.Errorf("failed to load JSON documents: %w", err)
	}

	if len(documents) == 0 {
		return fmt.Errorf("no valid JSON documents found in path: %s", j.Path)
	}

	// Convert and load documents
	convertedDocs := ConvertDocumentPointers(documents)
	return j.LoadDocuments(ctx, convertedDocs, recreate)
}

// LoadAsync loads documents asynchronously
func (j *JSONKnowledgeBase) LoadAsync(ctx context.Context, recreate bool) error {
	return j.Load(ctx, recreate)
}

// GetInfo returns information about the JSON knowledge base
func (j *JSONKnowledgeBase) GetInfo() KnowledgeInfo {
	info := j.BaseKnowledge.GetInfo()
	info.Type = "json"
	info.Metadata["path"] = j.Path
	info.Metadata["formats"] = j.Formats
	return info
}

// loadJSONDocuments loads JSON documents from the path
func (j *JSONKnowledgeBase) loadJSONDocuments() ([]*document.Document, error) {
	var documents []*document.Document

	// Check if path is a file or directory
	fileInfo, err := os.Stat(j.Path)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %s", j.Path)
	}

	if fileInfo.IsDir() {
		// Load all JSON files from directory
		err = filepath.Walk(j.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && IsValidFileFormat(path, j.Formats) {
				docs, err := j.loadJSONFile(path)
				if err != nil {
					return fmt.Errorf("failed to load file %s: %w", path, err)
				}
				documents = append(documents, docs...)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		// Load single file
		if !IsValidFileFormat(j.Path, j.Formats) {
			return nil, fmt.Errorf("unsupported file format: %s", j.Path)
		}
		docs, err := j.loadJSONFile(j.Path)
		if err != nil {
			return nil, err
		}
		documents = append(documents, docs...)
	}

	return documents, nil
}

// loadJSONFile loads JSON documents from a file
func (j *JSONKnowledgeBase) loadJSONFile(filePath string) ([]*document.Document, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON and extract text content
	// For now, treat entire JSON as text content
	doc := document.NewDocument(string(content))
	doc.Name = filepath.Base(filePath)
	doc.Source = filePath
	doc.ContentType = "application/json"

	// Add file metadata
	doc.AddMetadata("file_path", filePath)
	doc.AddMetadata("file_extension", GetFileExtension(filePath))
	doc.AddMetadata("file_size", len(content))

	return []*document.Document{doc}, nil
}

// DocumentKnowledgeBase handles direct document input
type DocumentKnowledgeBase struct {
	*BaseKnowledge
	Documents []*document.Document `json:"documents"`
}

// NewDocumentKnowledgeBase creates a new document knowledge base
func NewDocumentKnowledgeBase(name string, vectorDB VectorDB) *DocumentKnowledgeBase {
	base := NewBaseKnowledge(name, vectorDB)
	base.Metadata["description"] = "Direct document knowledge base"

	return &DocumentKnowledgeBase{
		BaseKnowledge: base,
		Documents:     []*document.Document{},
	}
}

// Load loads the provided documents
func (d *DocumentKnowledgeBase) Load(ctx context.Context, recreate bool) error {
	if recreate && d.VectorDB != nil {
		if err := d.VectorDB.Drop(ctx); err != nil {
			return fmt.Errorf("failed to drop existing database: %w", err)
		}
	}

	if d.VectorDB != nil {
		if err := d.VectorDB.Create(ctx); err != nil {
			return fmt.Errorf("failed to create vector database: %w", err)
		}
	}

	if len(d.Documents) == 0 {
		return fmt.Errorf("no documents provided")
	}

	// Convert and load documents
	convertedDocs := ConvertDocumentPointers(d.Documents)
	return d.LoadDocuments(ctx, convertedDocs, recreate)
}

// LoadAsync loads documents asynchronously
func (d *DocumentKnowledgeBase) LoadAsync(ctx context.Context, recreate bool) error {
	return d.Load(ctx, recreate)
}

// GetInfo returns information about the document knowledge base
func (d *DocumentKnowledgeBase) GetInfo() KnowledgeInfo {
	info := d.BaseKnowledge.GetInfo()
	info.Type = "document"
	info.Metadata["document_count"] = len(d.Documents)
	return info
}

// AddDocument adds a document to the knowledge base
func (d *DocumentKnowledgeBase) AddDocument(doc *document.Document) {
	d.Documents = append(d.Documents, doc)
	d.Metadata["updated_at"] = time.Now()
}

// AddDocuments adds multiple documents to the knowledge base
func (d *DocumentKnowledgeBase) AddDocuments(docs []*document.Document) {
	d.Documents = append(d.Documents, docs...)
	d.Metadata["updated_at"] = time.Now()
}
