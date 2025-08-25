package tools

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// FileTool provides file system operations
type FileTool struct {
	toolkit.Toolkit
	WriteEnabled bool // Controls whether write operations are allowed
}

// FileOperationResult represents the result of file operations
type FileOperationResult struct {
	Path      string `json:"path"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Content   string `json:"content,omitempty"`
	Size      int64  `json:"size,omitempty"`
	IsDir     bool   `json:"is_dir,omitempty"`
	Exists    bool   `json:"exists,omitempty"`
	Operation string `json:"operation"`
}

// ReadFileParams represents parameters for reading files
type ReadFileParams struct {
	Path     string `json:"path" description:"The file path to read" required:"true"`
	MaxBytes int    `json:"max_bytes,omitempty" description:"Maximum bytes to read. Default: 10000"`
	Encoding string `json:"encoding,omitempty" description:"File encoding. Default: utf-8"`
}

// WriteFileParams represents parameters for writing files
type WriteFileParams struct {
	Path    string `json:"path" description:"The file path to write" required:"true"`
	Content string `json:"content" description:"Content to write to the file" required:"true"`
	Append  bool   `json:"append,omitempty" description:"Whether to append to file instead of overwriting. Default: false"`
	Mode    string `json:"mode,omitempty" description:"File permissions (e.g., '0644'). Default: 0644"`
}

type StringParams struct {
	Content string `json:"content" description:"Content to encode or decode"`
}

// FileInfoParams represents parameters for file info operations
type FileInfoParams struct {
	Path string `json:"path" description:"The file or directory path" required:"true"`
}

// ListDirParams represents parameters for listing directories
type ListDirParams struct {
	Path      string `json:"path" description:"The directory path to list" required:"true"`
	Recursive bool   `json:"recursive,omitempty" description:"Whether to list recursively. Default: false"`
	Pattern   string `json:"pattern,omitempty" description:"File pattern to match (e.g., '*.txt')"`
}

// SearchFileParams represents parameters for searching files
type SearchFileParams struct {
	Path    string `json:"path" description:"The directory path to search in" required:"true"`
	Pattern string `json:"pattern" description:"File name pattern to search for" required:"true"`
	Content string `json:"content,omitempty" description:"Content to search within files"`
}

// NewFileTool creates a new FileTool instance
func NewFileTool(write bool) *FileTool {
	tk := toolkit.NewToolkit()
	tk.Name = "FileTool"
	tk.Description = "A comprehensive file system tool for reading, writing, listing, and managing files and directories. Supports text files, binary operations, directory traversal, and file search functionality. Write operations are disabled by default for security."

	ft := &FileTool{
		Toolkit:      tk,
		WriteEnabled: write,
	}

	// Register methods
	ft.Toolkit.Register("ReadFile", ft, ft.ReadFile, ReadFileParams{})
	ft.Toolkit.Register("WriteFile", ft, ft.WriteFile, WriteFileParams{})
	ft.Toolkit.Register("GetFileInfo", ft, ft.GetFileInfo, FileInfoParams{})
	ft.Toolkit.Register("ListDirectory", ft, ft.ListDirectory, ListDirParams{})
	ft.Toolkit.Register("SearchFiles", ft, ft.SearchFiles, SearchFileParams{})
	ft.Toolkit.Register("CreateDirectory", ft, ft.CreateDirectory, FileInfoParams{})
	ft.Toolkit.Register("DeleteFile", ft, ft.DeleteFile, FileInfoParams{})
	// ft.Toolkit.Register("Base64Encode", ft, ft.base64encode, StringParams{})
	// ft.Toolkit.Register("Base64Decode", ft, ft.base64decode, StringParams{})

	return ft
}

// NewFileToolWithWrite creates a new FileTool instance with write operations enabled
// Use this carefully as it allows file modifications
func NewFileToolWithWrite() *FileTool {
	ft := NewFileTool(true)
	ft.EnableWrite()
	return ft
}

// EnableWrite enables write operations for this FileTool instance
// This is a security measure to prevent accidental file modifications
func (ft *FileTool) EnableWrite() {
	ft.WriteEnabled = true
}

// DisableWrite disables write operations for this FileTool instance
func (ft *FileTool) DisableWrite() {
	ft.WriteEnabled = false
}

// IsWriteEnabled returns whether write operations are enabled
func (ft *FileTool) IsWriteEnabled() bool {
	return ft.WriteEnabled
}

// ReadFile reads content from a file
func (ft *FileTool) ReadFile(params ReadFileParams) (interface{}, error) {
	if params.Path == "" {
		return nil, fmt.Errorf("file path is required")
	}

	// Set default max bytes
	if params.MaxBytes <= 0 {
		params.MaxBytes = 10000
	}

	// Check if file exists
	info, err := os.Stat(params.Path)
	if err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("file not found: %v", err),
			Operation: "ReadFile",
		}, nil
	}

	if info.IsDir() {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     "path is a directory, not a file",
			Operation: "ReadFile",
		}, nil
	}

	// Open file
	file, err := os.Open(params.Path)
	if err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("failed to open file: %v", err),
			Operation: "ReadFile",
		}, nil
	}
	defer file.Close()

	// Read file content (limited by MaxBytes)
	buffer := make([]byte, params.MaxBytes)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("failed to read file: %v", err),
			Operation: "ReadFile",
		}, nil
	}

	content := string(buffer[:n])

	// Add truncation notice if file was larger than MaxBytes
	if int64(n) == int64(params.MaxBytes) && info.Size() > int64(params.MaxBytes) {
		content += fmt.Sprintf("\n\n[... file truncated, showing first %d bytes of %d total bytes]", params.MaxBytes, info.Size())
	}

	return FileOperationResult{
		Path:      params.Path,
		Success:   true,
		Content:   content,
		Size:      info.Size(),
		Operation: "ReadFile",
	}, nil
}

// WriteFile writes content to a file
func (ft *FileTool) WriteFile(params WriteFileParams) (interface{}, error) {
	// Updated to accept []byte for content
	// Check if write operations are enabled
	if !ft.WriteEnabled {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     "write operations are disabled for security. Use EnableWrite() to enable them",
			Operation: "WriteFile",
		}, nil
	}

	if params.Path == "" {
		return nil, fmt.Errorf("file path is required")
	}

	if len(params.Content) == 0 {
		return nil, fmt.Errorf("content is required")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(params.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("failed to create directory: %v", err),
			Operation: "WriteFile",
		}, nil
	}

	// Determine file mode
	var flags int
	if params.Append {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	} else {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	// Open file for writing
	file, err := os.OpenFile(params.Path, flags, 0644)
	if err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("failed to open file for writing: %v", err),
			Operation: "WriteFile",
		}, nil
	}
	defer file.Close()

	// Decode base64 content if needed

	chunkSize := 10000
	contentBytes := []byte(params.Content)
	totalBytes := len(contentBytes)
	bytesWritten := 0

	for start := 0; start < totalBytes; start += chunkSize {
		end := start + chunkSize
		if end > totalBytes {
			end = totalBytes
		}
		_, err := file.Write(contentBytes[start:end])
		if err != nil {
			return FileOperationResult{
				Path:      params.Path,
				Success:   false,
				Error:     fmt.Sprintf("failed to write chunk to file: %v", err),
				Operation: "WriteFile",
			}, nil
		}
		bytesWritten += end - start
	}

	return FileOperationResult{
		Path:      params.Path,
		Success:   true,
		Size:      int64(bytesWritten),
		Operation: "WriteFile",
	}, nil
}

// GetFileInfo gets information about a file or directory
func (ft *FileTool) GetFileInfo(params FileInfoParams) (interface{}, error) {
	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	info, err := os.Stat(params.Path)
	if err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Exists:    false,
			Error:     fmt.Sprintf("path not found: %v", err),
			Operation: "GetFileInfo",
		}, nil
	}

	return map[string]interface{}{
		"path":      params.Path,
		"exists":    true,
		"is_dir":    info.IsDir(),
		"size":      info.Size(),
		"mode":      info.Mode().String(),
		"mod_time":  info.ModTime().Format("2006-01-02 15:04:05"),
		"operation": "GetFileInfo",
	}, nil
}

// ListDirectory lists contents of a directory
func (ft *FileTool) ListDirectory(params ListDirParams) (interface{}, error) {
	if params.Path == "" {
		return nil, fmt.Errorf("directory path is required")
	}

	// Check if path is a directory
	info, err := os.Stat(params.Path)
	if err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("path not found: %v", err),
			Operation: "ListDirectory",
		}, nil
	}

	if !info.IsDir() {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     "path is not a directory",
			Operation: "ListDirectory",
		}, nil
	}

	var files []map[string]interface{}

	if params.Recursive {
		err = filepath.Walk(params.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Continue walking even if there's an error
			}

			// Apply pattern filter if specified
			if params.Pattern != "" {
				matched, _ := filepath.Match(params.Pattern, info.Name())
				if !matched {
					return nil
				}
			}

			relPath, _ := filepath.Rel(params.Path, path)
			files = append(files, map[string]interface{}{
				"name":     info.Name(),
				"path":     path,
				"rel_path": relPath,
				"is_dir":   info.IsDir(),
				"size":     info.Size(),
				"mod_time": info.ModTime().Format("2006-01-02 15:04:05"),
			})
			return nil
		})
	} else {
		entries, err := os.ReadDir(params.Path)
		if err != nil {
			return FileOperationResult{
				Path:      params.Path,
				Success:   false,
				Error:     fmt.Sprintf("failed to read directory: %v", err),
				Operation: "ListDirectory",
			}, nil
		}

		for _, entry := range entries {
			// Apply pattern filter if specified
			if params.Pattern != "" {
				matched, _ := filepath.Match(params.Pattern, entry.Name())
				if !matched {
					continue
				}
			}

			info, _ := entry.Info()
			fullPath := filepath.Join(params.Path, entry.Name())
			files = append(files, map[string]interface{}{
				"name":     entry.Name(),
				"path":     fullPath,
				"is_dir":   entry.IsDir(),
				"size":     info.Size(),
				"mod_time": info.ModTime().Format("2006-01-02 15:04:05"),
			})
		}
	}

	return map[string]interface{}{
		"path":      params.Path,
		"files":     files,
		"count":     len(files),
		"recursive": params.Recursive,
		"pattern":   params.Pattern,
		"operation": "ListDirectory",
	}, nil
}

// SearchFiles searches for files matching patterns
func (ft *FileTool) SearchFiles(params SearchFileParams) (interface{}, error) {
	if params.Path == "" {
		return nil, fmt.Errorf("search path is required")
	}

	if params.Pattern == "" {
		return nil, fmt.Errorf("search pattern is required")
	}

	var matches []map[string]interface{}

	err := filepath.Walk(params.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking
		}

		// Check filename pattern
		matched, _ := filepath.Match(params.Pattern, info.Name())
		if !matched {
			return nil
		}

		matchInfo := map[string]interface{}{
			"name":     info.Name(),
			"path":     path,
			"is_dir":   info.IsDir(),
			"size":     info.Size(),
			"mod_time": info.ModTime().Format("2006-01-02 15:04:05"),
		}

		// If content search is specified and it's a file
		if params.Content != "" && !info.IsDir() {
			content, err := os.ReadFile(path)
			if err == nil && strings.Contains(string(content), params.Content) {
				matchInfo["content_match"] = true
			} else {
				return nil // Skip if content doesn't match
			}
		}

		matches = append(matches, matchInfo)
		return nil
	})

	if err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("search failed: %v", err),
			Operation: "SearchFiles",
		}, nil
	}

	return map[string]interface{}{
		"search_path": params.Path,
		"pattern":     params.Pattern,
		"content":     params.Content,
		"matches":     matches,
		"count":       len(matches),
		"operation":   "SearchFiles",
	}, nil
}

// CreateDirectory creates a directory
func (ft *FileTool) CreateDirectory(params FileInfoParams) (interface{}, error) {
	// Check if write operations are enabled
	if !ft.WriteEnabled {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     "write operations are disabled for security. Use EnableWrite() to enable them",
			Operation: "CreateDirectory",
		}, nil
	}

	if params.Path == "" {
		return nil, fmt.Errorf("directory path is required")
	}

	err := os.MkdirAll(params.Path, 0755)
	if err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("failed to create directory: %v", err),
			Operation: "CreateDirectory",
		}, nil
	}

	return FileOperationResult{
		Path:      params.Path,
		Success:   true,
		Operation: "CreateDirectory",
	}, nil
}

// DeleteFile deletes a file or directory
func (ft *FileTool) DeleteFile(params FileInfoParams) (interface{}, error) {
	// Check if write operations are enabled
	if !ft.WriteEnabled {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     "write operations are disabled for security. Use EnableWrite() to enable them",
			Operation: "DeleteFile",
		}, nil
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Check if path exists
	info, err := os.Stat(params.Path)
	if err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("path not found: %v", err),
			Operation: "DeleteFile",
		}, nil
	}

	// Use RemoveAll for directories, Remove for files
	if info.IsDir() {
		err = os.RemoveAll(params.Path)
	} else {
		err = os.Remove(params.Path)
	}

	if err != nil {
		return FileOperationResult{
			Path:      params.Path,
			Success:   false,
			Error:     fmt.Sprintf("failed to delete: %v", err),
			Operation: "DeleteFile",
		}, nil
	}

	return FileOperationResult{
		Path:      params.Path,
		Success:   true,
		IsDir:     info.IsDir(),
		Operation: "DeleteFile",
	}, nil
}

func (ft *FileTool) base64encode(content string) string {
	// Encode content to base64
	return base64.StdEncoding.EncodeToString([]byte(content))
}

func (ft *FileTool) base64decode(encoded string) (string, error) {
	// Decode base64 content
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 content: %v", err)
	}
	return string(decoded), nil
}
