// Package localfs ports Agno's LocalFileSystemTools: write and read text
// files inside a target directory. With restrict_to_base_dir (default true)
// absolute paths, ".." traversal and symlink escapes are rejected.
//
// For a richer, read-before-write coding workspace see tools.Workspace.
package localfs

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// LocalFileSystemTools writes/reads files under a base directory.
type LocalFileSystemTools struct {
	toolkit.Toolkit
	targetDirectory   string
	defaultExtension  string
	restrictToBaseDir bool
	enableWrite       bool
	enableRead        bool
}

// Option configures LocalFileSystemTools.
type Option func(*LocalFileSystemTools)

// WithTargetDirectory sets the base directory (default: current directory). Created if missing.
func WithTargetDirectory(d string) Option {
	return func(l *LocalFileSystemTools) { l.targetDirectory = d }
}

// WithDefaultExtension sets the default extension (default "txt").
func WithDefaultExtension(e string) Option {
	return func(l *LocalFileSystemTools) { l.defaultExtension = e }
}

// WithRestrictToBaseDir toggles confinement to the target directory (default true).
func WithRestrictToBaseDir(b bool) Option {
	return func(l *LocalFileSystemTools) { l.restrictToBaseDir = b }
}

// WithWriteFile toggles WriteFile (default true).
func WithWriteFile(b bool) Option { return func(l *LocalFileSystemTools) { l.enableWrite = b } }

// WithReadFile toggles ReadFile (default true).
func WithReadFile(b bool) Option { return func(l *LocalFileSystemTools) { l.enableRead = b } }

// WriteFileParams for WriteFile.
type WriteFileParams struct {
	Content   string `json:"content" description:"Content to write to the file" required:"true"`
	Filename  string `json:"filename,omitempty" description:"Name of the file. Defaults to UUID if not provided"`
	Directory string `json:"directory,omitempty" description:"Directory to write file to. Uses target_directory if not provided"`
	Extension string `json:"extension,omitempty" description:"File extension. Uses default_extension if not provided"`
}

// ReadFileParams for ReadFile.
type ReadFileParams struct {
	Filename  string `json:"filename" description:"Name of the file to read" required:"true"`
	Directory string `json:"directory,omitempty" description:"Directory to read file from. Uses target_directory if not provided"`
}

// NewLocalFileSystemTools creates the toolkit (creating the target directory).
func NewLocalFileSystemTools(opts ...Option) (*LocalFileSystemTools, error) {
	l := &LocalFileSystemTools{defaultExtension: "txt", restrictToBaseDir: true, enableWrite: true, enableRead: true}
	l.Toolkit = toolkit.NewToolkit()
	l.Toolkit.Name = "local_file_system"
	l.Toolkit.Description = "Write and read local text files inside a target directory."
	for _, o := range opts {
		o(l)
	}
	if l.targetDirectory == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		l.targetDirectory = wd
	}
	l.defaultExtension = strings.TrimLeft(l.defaultExtension, ".")
	if err := os.MkdirAll(l.targetDirectory, 0o755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(l.targetDirectory)
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	l.targetDirectory = abs
	if l.enableWrite {
		l.Toolkit.Register("WriteFile", "Write content to a local file. Returns the path to the created file or an error message.", l, l.WriteFile, WriteFileParams{})
	}
	if l.enableRead {
		l.Toolkit.Register("ReadFile", "Read content from a local file. Returns the text content of the file.", l, l.ReadFile, ReadFileParams{})
	}
	return l, nil
}

// TargetDirectory returns the resolved base directory.
func (l *LocalFileSystemTools) TargetDirectory() string { return l.targetDirectory }

// CheckEscape resolves filename (joined with directory) against the base
// directory and reports whether it is safe.
func (l *LocalFileSystemTools) CheckEscape(filename, directory string) (bool, string) {
	rel := filename
	if directory != "" {
		rel = filepath.Join(directory, filename)
	}
	if !l.restrictToBaseDir {
		if filepath.IsAbs(rel) {
			return true, filepath.Clean(rel)
		}
		return true, filepath.Join(l.targetDirectory, rel)
	}
	if filepath.IsAbs(rel) {
		return false, l.targetDirectory
	}
	full := filepath.Join(l.targetDirectory, rel)
	if !within(l.targetDirectory, full) {
		return false, l.targetDirectory
	}
	// Resolve symlinks on the deepest existing ancestor.
	existing := full
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil || !within(l.targetDirectory, resolved) {
		return false, l.targetDirectory
	}
	return true, full
}

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func uuid4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// WriteFile writes content to a file.
func (l *LocalFileSystemTools) WriteFile(p WriteFileParams) (interface{}, error) {
	filename := p.Filename
	if filename == "" {
		filename = uuid4()
	}
	base := filepath.Base(filename)
	suffix := strings.TrimLeft(filepath.Ext(base), ".")
	ext := strings.TrimLeft(p.Extension, ".")
	if ext == "" {
		ext = suffix
	}
	if ext == "" {
		ext = l.defaultExtension
	}
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	fullName := filepath.Join(filepath.Dir(filename), stem+"."+ext)
	safe, path := l.CheckEscape(fullName, p.Directory)
	if !safe {
		return fmt.Sprintf("Error: Path '%s' is outside the allowed base directory", filename), nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Sprintf("Error: Failed to write file: %v", err), nil
	}
	if err := os.WriteFile(path, []byte(p.Content), 0o644); err != nil {
		return fmt.Sprintf("Error: Failed to write file: %v", err), nil
	}
	return fmt.Sprintf("Successfully wrote file to: %s", path), nil
}

// ReadFile reads a file's text content.
func (l *LocalFileSystemTools) ReadFile(p ReadFileParams) (interface{}, error) {
	safe, path := l.CheckEscape(p.Filename, p.Directory)
	if !safe {
		return fmt.Sprintf("Error: Path '%s' is outside the allowed base directory", p.Filename), nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Sprintf("File not found: %s", path), nil
	}
	if err != nil {
		return fmt.Sprintf("Error: Failed to read file: %v", err), nil
	}
	return string(data), nil
}
