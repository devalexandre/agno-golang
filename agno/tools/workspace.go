package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Workspace operation aliases. They name the operations that can be enabled
// (Allowed) or gated behind a user confirmation (Confirm).
const (
	OpRead    = "read"
	OpList    = "list"
	OpSearch  = "search"
	OpWrite   = "write"
	OpEdit    = "edit"
	OpMove    = "move"
	OpDelete  = "delete"
	OpCommand = "shell"
)

// WorkspaceReadOps are the non-destructive operations.
var WorkspaceReadOps = []string{OpRead, OpList, OpSearch}

// WorkspaceWriteOps are the destructive operations.
var WorkspaceWriteOps = []string{OpWrite, OpEdit, OpMove, OpDelete, OpCommand}

// DefaultWorkspaceExcludes are the directories skipped by list_files and
// search_content. They are the usual noise in a source tree.
var DefaultWorkspaceExcludes = []string{
	".git",
	".svn",
	".hg",
	".idea",
	".vscode",
	"node_modules",
	"vendor",
	"bin",
	"dist",
	"build",
	"target",
	"__pycache__",
	".venv",
	".mypy_cache",
	".pytest_cache",
}

const (
	// DefaultWorkspaceMaxFileSize is the largest file read_file will return.
	DefaultWorkspaceMaxFileSize int64 = 10 * 1024 * 1024
	// DefaultWorkspaceMaxFileLines caps the number of lines returned by read_file.
	DefaultWorkspaceMaxFileLines = 2000
	// DefaultWorkspaceCommandTimeout is the wall-clock limit for run_command.
	DefaultWorkspaceCommandTimeout = 120 * time.Second
	// workspaceSnippetContext is how many characters of context search_content
	// keeps around a match.
	workspaceSnippetContext = 120
)

// WorkspaceConfig configures a Workspace toolkit.
type WorkspaceConfig struct {
	// Root is the directory every operation is scoped to. Defaults to the
	// process working directory.
	Root string
	// Allowed lists the operation aliases to register. Nil means all of them.
	// An operation that is not registered is invisible to the model.
	Allowed []string
	// Confirm lists the operations that require user confirmation before they
	// run. Nil means the destructive ones (WorkspaceWriteOps).
	Confirm []string
	// RequireReadBeforeWrite blocks write/edit/move/delete on an existing file
	// until it has been read in this session. Enabled by default; set
	// AllowBlindWrites to turn it off.
	AllowBlindWrites bool
	// ExcludePatterns overrides DefaultWorkspaceExcludes. Patterns are matched
	// against each path segment with filepath.Match. Use an empty (non-nil)
	// slice to disable exclusions.
	ExcludePatterns []string
	// MaxFileSize is the largest file that can be read or written, in bytes.
	// Defaults to DefaultWorkspaceMaxFileSize.
	MaxFileSize int64
	// MaxFileLines caps how many lines read_file returns in a single call.
	// Defaults to DefaultWorkspaceMaxFileLines.
	MaxFileLines int
	// CommandTimeout is the default wall-clock limit for run_command.
	CommandTimeout time.Duration
}

// Workspace is a toolkit for reading, writing, editing, searching and running
// commands inside a single directory tree.
//
// Every path is resolved against the workspace root and rejected if it escapes
// it — through "..", an absolute path, or a symlink pointing outside. This is a
// path-scoping boundary, not a process sandbox: run_command still runs with the
// privileges of the host process, so untrusted input belongs in a container.
type Workspace struct {
	toolkit.Toolkit

	root            string
	excludePatterns []string
	maxFileSize     int64
	maxFileLines    int
	commandTimeout  time.Duration
	readBeforeWrite bool

	mu        sync.Mutex
	readFiles map[string]bool
}

// --- Parameters ---

// WorkspaceReadFileParams are the parameters of read_file.
type WorkspaceReadFileParams struct {
	Path      string `json:"path" description:"Path of the file to read, relative to the workspace root" required:"true"`
	StartLine int    `json:"start_line,omitempty" description:"1-indexed line to start reading from. Default: 1"`
	Limit     int    `json:"limit,omitempty" description:"Maximum number of lines to return"`
}

// WorkspaceListFilesParams are the parameters of list_files.
type WorkspaceListFilesParams struct {
	Directory string `json:"directory,omitempty" description:"Directory to list, relative to the workspace root. Default: the root itself"`
	Recursive bool   `json:"recursive,omitempty" description:"List nested directories too. Default: false"`
	Pattern   string `json:"pattern,omitempty" description:"Glob matched against the file name (e.g. '*.go')"`
}

// WorkspaceSearchContentParams are the parameters of search_content.
type WorkspaceSearchContentParams struct {
	Query     string `json:"query" description:"Text to search for, case-insensitive" required:"true"`
	Directory string `json:"directory,omitempty" description:"Directory to search in, relative to the workspace root. Default: the root itself"`
	Limit     int    `json:"limit,omitempty" description:"Maximum number of matching files to return. Default: 10"`
}

// WorkspaceWriteFileParams are the parameters of write_file.
type WorkspaceWriteFileParams struct {
	Path      string `json:"path" description:"Path of the file to write, relative to the workspace root" required:"true"`
	Content   string `json:"content" description:"Full content of the file" required:"true"`
	Overwrite bool   `json:"overwrite,omitempty" description:"Allow overwriting an existing file. Default: false"`
}

// WorkspaceEditFileParams are the parameters of edit_file.
type WorkspaceEditFileParams struct {
	Path       string `json:"path" description:"Path of the file to edit, relative to the workspace root" required:"true"`
	OldString  string `json:"old_string" description:"Exact text to replace. Must appear in the file" required:"true"`
	NewString  string `json:"new_string" description:"Text to replace it with" required:"true"`
	ReplaceAll bool   `json:"replace_all,omitempty" description:"Replace every occurrence instead of requiring a unique match. Default: false"`
}

// WorkspaceMoveFileParams are the parameters of move_file.
type WorkspaceMoveFileParams struct {
	Src       string `json:"src" description:"Path of the file to move, relative to the workspace root" required:"true"`
	Dst       string `json:"dst" description:"Destination path, relative to the workspace root" required:"true"`
	Overwrite bool   `json:"overwrite,omitempty" description:"Allow overwriting the destination. Default: false"`
}

// WorkspaceDeleteFileParams are the parameters of delete_file.
type WorkspaceDeleteFileParams struct {
	Path string `json:"path" description:"Path of the file to delete, relative to the workspace root" required:"true"`
}

// WorkspaceRunCommandParams are the parameters of run_command.
type WorkspaceRunCommandParams struct {
	Command string   `json:"command" description:"Executable to run. It runs with the workspace root as working directory" required:"true"`
	Args    []string `json:"args,omitempty" description:"Arguments passed to the command"`
	Timeout int      `json:"timeout,omitempty" description:"Timeout in seconds. Default: 120"`
	Tail    int      `json:"tail,omitempty" description:"Keep only the last N lines of the output. Default: 100"`
}

// --- Results ---

// WorkspaceFileContent is returned by read_file.
type WorkspaceFileContent struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	TotalLines int    `json:"total_lines"`
	Truncated  bool   `json:"truncated"`
	Size       int64  `json:"size"`
}

// WorkspaceEntry describes one entry returned by list_files.
type WorkspaceEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

// WorkspaceListing is returned by list_files.
type WorkspaceListing struct {
	Directory string           `json:"directory"`
	Entries   []WorkspaceEntry `json:"entries"`
	Count     int              `json:"count"`
}

// WorkspaceMatch is a single search_content hit.
type WorkspaceMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

// WorkspaceSearchResult is returned by search_content.
type WorkspaceSearchResult struct {
	Query   string           `json:"query"`
	Matches []WorkspaceMatch `json:"matches"`
	Count   int              `json:"count"`
}

// WorkspaceWriteResult is returned by write_file, edit_file, move_file and delete_file.
type WorkspaceWriteResult struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
	Replaced  int    `json:"replaced,omitempty"`
}

// WorkspaceCommandResult is returned by run_command.
type WorkspaceCommandResult struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Success  bool   `json:"success"`
	Duration string `json:"duration"`
	TimedOut bool   `json:"timed_out,omitempty"`
}

// NewWorkspace creates a Workspace scoped to root with the default settings:
// every operation registered, destructive ones behind a confirmation, and
// read-before-write enforced.
func NewWorkspace(root string) (*Workspace, error) {
	return NewWorkspaceWithConfig(WorkspaceConfig{Root: root})
}

// NewWorkspaceWithConfig creates a Workspace from an explicit configuration.
func NewWorkspaceWithConfig(cfg WorkspaceConfig) (*Workspace, error) {
	root := cfg.Root
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("workspace: resolve working directory: %w", err)
		}
		root = wd
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("workspace: resolve root %q: %w", root, err)
	}
	// Resolve symlinks once so containment checks compare canonical paths.
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("workspace: root %q is not accessible: %w", root, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("workspace: root %q is not accessible: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace: root %q is not a directory", root)
	}

	excludes := DefaultWorkspaceExcludes
	if cfg.ExcludePatterns != nil {
		excludes = cfg.ExcludePatterns
	}
	maxSize := cfg.MaxFileSize
	if maxSize <= 0 {
		maxSize = DefaultWorkspaceMaxFileSize
	}
	maxLines := cfg.MaxFileLines
	if maxLines <= 0 {
		maxLines = DefaultWorkspaceMaxFileLines
	}
	timeout := cfg.CommandTimeout
	if timeout <= 0 {
		timeout = DefaultWorkspaceCommandTimeout
	}

	allowed, err := workspaceOpSet(cfg.Allowed, append(append([]string{}, WorkspaceReadOps...), WorkspaceWriteOps...))
	if err != nil {
		return nil, err
	}
	confirm, err := workspaceOpSet(cfg.Confirm, WorkspaceWriteOps)
	if err != nil {
		return nil, err
	}

	w := &Workspace{
		root:            resolved,
		excludePatterns: excludes,
		maxFileSize:     maxSize,
		maxFileLines:    maxLines,
		commandTimeout:  timeout,
		readBeforeWrite: !cfg.AllowBlindWrites,
		readFiles:       make(map[string]bool),
	}

	w.Toolkit = toolkit.NewToolkit()
	w.Toolkit.Name = "Workspace"
	w.Toolkit.Description = fmt.Sprintf(
		"Read, write, edit, search and run commands inside the working directory %q. "+
			"All paths are relative to that directory; paths escaping it are rejected.", resolved)

	register := func(op, name, description string, fn interface{}, params interface{}) {
		if !allowed[op] {
			return
		}
		if confirm[op] {
			w.Toolkit.RegisterWithOptions(name, description, w, fn, params, toolkit.WithConfirmation())
			return
		}
		w.Toolkit.Register(name, description, w, fn, params)
	}

	register(OpRead, "read_file", "Read a file from the workspace. The content is returned with line numbers", w.ReadFile, WorkspaceReadFileParams{})
	register(OpList, "list_files", "List the files and directories of a workspace directory", w.ListFiles, WorkspaceListFilesParams{})
	register(OpSearch, "search_content", "Search the workspace files for a text query and return matching snippets", w.SearchContent, WorkspaceSearchContentParams{})
	register(OpWrite, "write_file", "Create a file, or overwrite one that was read first", w.WriteFile, WorkspaceWriteFileParams{})
	register(OpEdit, "edit_file", "Replace a piece of text inside a file that was read first", w.EditFile, WorkspaceEditFileParams{})
	register(OpMove, "move_file", "Move or rename a file inside the workspace", w.MoveFile, WorkspaceMoveFileParams{})
	register(OpDelete, "delete_file", "Delete a file from the workspace", w.DeleteFile, WorkspaceDeleteFileParams{})
	register(OpCommand, "run_command", "Run a command with the workspace root as working directory", w.RunCommand, WorkspaceRunCommandParams{})

	return w, nil
}

// Root returns the resolved workspace root.
func (w *Workspace) Root() string { return w.root }

// --- Path safety ---

// resolvePath maps a caller-supplied path to an absolute path inside the root,
// rejecting anything that escapes it. Traversal ("../etc/passwd"), absolute
// paths outside the root, and symlinks pointing outside are all refused. Paths
// that do not exist yet are checked through their closest existing ancestor, so
// a write cannot be smuggled through a symlinked directory either.
func (w *Workspace) resolvePath(path string) (string, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" || cleaned == "." {
		return w.root, nil
	}

	var candidate string
	if filepath.IsAbs(cleaned) {
		candidate = filepath.Clean(cleaned)
	} else {
		candidate = filepath.Clean(filepath.Join(w.root, cleaned))
	}

	if !isWithin(w.root, candidate) {
		return "", fmt.Errorf("workspace: path %q resolves outside the workspace root %q", path, w.root)
	}

	// Follow symlinks on the deepest existing ancestor, then re-check.
	existing := candidate
	var rest []string
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", fmt.Errorf("workspace: path %q resolves outside the workspace root %q", path, w.root)
		}
		rest = append([]string{filepath.Base(existing)}, rest...)
		existing = parent
	}

	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", fmt.Errorf("workspace: resolve path %q: %w", path, err)
	}
	if !isWithin(w.root, resolved) {
		return "", fmt.Errorf("workspace: path %q escapes the workspace root %q through a symlink", path, w.root)
	}

	final := resolved
	if len(rest) > 0 {
		final = filepath.Join(append([]string{resolved}, rest...)...)
		if !isWithin(w.root, final) {
			return "", fmt.Errorf("workspace: path %q resolves outside the workspace root %q", path, w.root)
		}
	}

	return final, nil
}

// isWithin reports whether path is root itself or lives under it.
func isWithin(root, path string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(os.PathSeparator))
}

// relative renders an absolute workspace path as a root-relative one.
func (w *Workspace) relative(abs string) string {
	rel, err := filepath.Rel(w.root, abs)
	if err != nil {
		return abs
	}
	return rel
}

// isExcluded reports whether any segment of the path matches an exclude pattern.
func (w *Workspace) isExcluded(abs string) bool {
	rel := w.relative(abs)
	if rel == "." {
		return false
	}
	for _, segment := range strings.Split(rel, string(os.PathSeparator)) {
		for _, pattern := range w.excludePatterns {
			if ok, err := filepath.Match(pattern, segment); err == nil && ok {
				return true
			}
		}
	}
	return false
}

// markRead records that a file was read in this session.
func (w *Workspace) markRead(abs string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.readFiles[abs] = true
}

// wasRead reports whether a file was read in this session.
func (w *Workspace) wasRead(abs string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.readFiles[abs]
}

// forgetRead drops a file from the read set (after it is moved or deleted).
func (w *Workspace) forgetRead(abs string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.readFiles, abs)
}

// checkReadBeforeWrite refuses to touch an existing file the model never read.
// It is what stops a model from overwriting a file whose content it only guessed.
func (w *Workspace) checkReadBeforeWrite(abs, operation string) error {
	if !w.readBeforeWrite {
		return nil
	}
	if _, err := os.Stat(abs); err != nil {
		// The file does not exist yet: nothing to clobber.
		return nil
	}
	if w.wasRead(abs) {
		return nil
	}
	return fmt.Errorf(
		"workspace: refusing to %s %q because it has not been read in this session. Call read_file on it first",
		operation, w.relative(abs))
}

// --- Operations ---

// ReadFile reads a workspace file and returns it with line numbers.
func (w *Workspace) ReadFile(params WorkspaceReadFileParams) (interface{}, error) {
	abs, err := w.resolvePath(params.Path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("workspace: read %q: %w", params.Path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("workspace: %q is a directory, use list_files", params.Path)
	}
	if info.Size() > w.maxFileSize {
		return nil, fmt.Errorf("workspace: %q is %d bytes, over the %d byte limit", params.Path, info.Size(), w.maxFileSize)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("workspace: read %q: %w", params.Path, err)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("workspace: %q looks like a binary file", params.Path)
	}

	lines := splitLines(string(data))
	total := len(lines)

	start := params.StartLine
	if start <= 0 {
		start = 1
	}
	if start > total {
		start = total + 1
	}
	limit := params.Limit
	if limit <= 0 || limit > w.maxFileLines {
		limit = w.maxFileLines
	}

	end := start + limit - 1
	if end > total {
		end = total
	}

	var chunk []string
	if start <= total {
		chunk = lines[start-1 : end]
	}

	// Reading the whole file is what unlocks a later write or edit.
	if start == 1 && end == total {
		w.markRead(abs)
	}

	return &WorkspaceFileContent{
		Path:       w.relative(abs),
		Content:    formatWithLineNumbers(chunk, start),
		StartLine:  start,
		EndLine:    end,
		TotalLines: total,
		Truncated:  end < total,
		Size:       info.Size(),
	}, nil
}

// ListFiles lists a workspace directory, skipping the excluded patterns.
func (w *Workspace) ListFiles(params WorkspaceListFilesParams) (interface{}, error) {
	abs, err := w.resolvePath(params.Directory)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("workspace: list %q: %w", params.Directory, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace: %q is not a directory", params.Directory)
	}

	entries := []WorkspaceEntry{}
	appendEntry := func(path string, fi os.FileInfo) {
		entry := WorkspaceEntry{Path: w.relative(path), IsDir: fi.IsDir()}
		if !fi.IsDir() {
			entry.Size = fi.Size()
		}
		entries = append(entries, entry)
	}

	matches := func(name string, isDir bool) bool {
		if params.Pattern == "" || isDir {
			return true
		}
		ok, err := filepath.Match(params.Pattern, name)
		return err == nil && ok
	}

	if params.Recursive {
		err = filepath.Walk(abs, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip what cannot be stat'ed instead of failing the listing.
			}
			if path == abs {
				return nil
			}
			if w.isExcluded(path) {
				if fi.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if matches(fi.Name(), fi.IsDir()) {
				appendEntry(path, fi)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("workspace: list %q: %w", params.Directory, err)
		}
	} else {
		dirEntries, err := os.ReadDir(abs)
		if err != nil {
			return nil, fmt.Errorf("workspace: list %q: %w", params.Directory, err)
		}
		for _, de := range dirEntries {
			path := filepath.Join(abs, de.Name())
			if w.isExcluded(path) {
				continue
			}
			fi, err := de.Info()
			if err != nil {
				continue
			}
			if matches(de.Name(), de.IsDir()) {
				appendEntry(path, fi)
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	return &WorkspaceListing{
		Directory: w.relative(abs),
		Entries:   entries,
		Count:     len(entries),
	}, nil
}

// SearchContent grep-searches the workspace for a case-insensitive query and
// returns a snippet around each first match.
func (w *Workspace) SearchContent(params WorkspaceSearchContentParams) (interface{}, error) {
	if strings.TrimSpace(params.Query) == "" {
		return nil, fmt.Errorf("workspace: search query cannot be empty")
	}

	abs, err := w.resolvePath(params.Directory)
	if err != nil {
		return nil, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}

	matches := []WorkspaceMatch{}
	err = filepath.Walk(abs, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if len(matches) >= limit {
			return filepath.SkipAll
		}
		if w.isExcluded(path) {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if fi.IsDir() || !fi.Mode().IsRegular() || fi.Size() > w.maxFileSize {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}

		content := string(data)
		idx := strings.Index(strings.ToLower(content), strings.ToLower(params.Query))
		if idx < 0 {
			return nil
		}

		matches = append(matches, WorkspaceMatch{
			Path:    w.relative(path),
			Line:    strings.Count(content[:idx], "\n") + 1,
			Snippet: extractSnippet(content, idx, len(params.Query)),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("workspace: search %q: %w", params.Query, err)
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].Path < matches[j].Path })

	return &WorkspaceSearchResult{
		Query:   params.Query,
		Matches: matches,
		Count:   len(matches),
	}, nil
}

// WriteFile creates a file, or overwrites one that was read first.
func (w *Workspace) WriteFile(params WorkspaceWriteFileParams) (interface{}, error) {
	abs, err := w.resolvePath(params.Path)
	if err != nil {
		return nil, err
	}
	if int64(len(params.Content)) > w.maxFileSize {
		return nil, fmt.Errorf("workspace: content is %d bytes, over the %d byte limit", len(params.Content), w.maxFileSize)
	}

	info, statErr := os.Stat(abs)
	if statErr == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("workspace: %q is a directory", params.Path)
		}
		if !params.Overwrite {
			return nil, fmt.Errorf("workspace: %q already exists, pass overwrite=true to replace it", params.Path)
		}
		if err := w.checkReadBeforeWrite(abs, "overwrite"); err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("workspace: create parent directory of %q: %w", params.Path, err)
	}
	if err := writeFileAtomic(abs, []byte(params.Content)); err != nil {
		return nil, fmt.Errorf("workspace: write %q: %w", params.Path, err)
	}

	// The content is now known: further edits do not need another read.
	w.markRead(abs)

	return &WorkspaceWriteResult{
		Path:      w.relative(abs),
		Operation: "write_file",
		Success:   true,
		Bytes:     len(params.Content),
		Message:   fmt.Sprintf("wrote %d bytes", len(params.Content)),
	}, nil
}

// EditFile replaces a piece of text inside a file that was read first.
func (w *Workspace) EditFile(params WorkspaceEditFileParams) (interface{}, error) {
	abs, err := w.resolvePath(params.Path)
	if err != nil {
		return nil, err
	}
	if params.OldString == "" {
		return nil, fmt.Errorf("workspace: old_string cannot be empty, use write_file to create content")
	}
	if params.OldString == params.NewString {
		return nil, fmt.Errorf("workspace: old_string and new_string are identical, nothing to do")
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("workspace: edit %q: %w", params.Path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("workspace: %q is a directory", params.Path)
	}
	if err := w.checkReadBeforeWrite(abs, "edit"); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("workspace: edit %q: %w", params.Path, err)
	}

	content := string(data)
	occurrences := strings.Count(content, params.OldString)
	switch {
	case occurrences == 0:
		return nil, fmt.Errorf("workspace: old_string was not found in %q", params.Path)
	case occurrences > 1 && !params.ReplaceAll:
		return nil, fmt.Errorf(
			"workspace: old_string appears %d times in %q. Make it unique or pass replace_all=true",
			occurrences, params.Path)
	}

	replaced := occurrences
	updated := content
	if params.ReplaceAll {
		updated = strings.ReplaceAll(content, params.OldString, params.NewString)
	} else {
		updated = strings.Replace(content, params.OldString, params.NewString, 1)
		replaced = 1
	}

	if int64(len(updated)) > w.maxFileSize {
		return nil, fmt.Errorf("workspace: the edit would grow %q past the %d byte limit", params.Path, w.maxFileSize)
	}
	if err := writeFileAtomic(abs, []byte(updated)); err != nil {
		return nil, fmt.Errorf("workspace: edit %q: %w", params.Path, err)
	}

	return &WorkspaceWriteResult{
		Path:      w.relative(abs),
		Operation: "edit_file",
		Success:   true,
		Bytes:     len(updated),
		Replaced:  replaced,
		Message:   fmt.Sprintf("replaced %d occurrence(s)", replaced),
	}, nil
}

// MoveFile moves or renames a file inside the workspace.
func (w *Workspace) MoveFile(params WorkspaceMoveFileParams) (interface{}, error) {
	src, err := w.resolvePath(params.Src)
	if err != nil {
		return nil, err
	}
	dst, err := w.resolvePath(params.Dst)
	if err != nil {
		return nil, err
	}
	if src == w.root || dst == w.root {
		return nil, fmt.Errorf("workspace: refusing to move the workspace root")
	}

	if _, err := os.Stat(src); err != nil {
		return nil, fmt.Errorf("workspace: move %q: %w", params.Src, err)
	}
	if err := w.checkReadBeforeWrite(src, "move"); err != nil {
		return nil, err
	}

	if _, err := os.Stat(dst); err == nil {
		if !params.Overwrite {
			return nil, fmt.Errorf("workspace: destination %q already exists, pass overwrite=true to replace it", params.Dst)
		}
		if err := w.checkReadBeforeWrite(dst, "overwrite"); err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, fmt.Errorf("workspace: create parent directory of %q: %w", params.Dst, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return nil, fmt.Errorf("workspace: move %q to %q: %w", params.Src, params.Dst, err)
	}

	w.forgetRead(src)
	w.markRead(dst)

	return &WorkspaceWriteResult{
		Path:      w.relative(dst),
		Operation: "move_file",
		Success:   true,
		Message:   fmt.Sprintf("moved %s to %s", w.relative(src), w.relative(dst)),
	}, nil
}

// DeleteFile deletes a workspace file.
func (w *Workspace) DeleteFile(params WorkspaceDeleteFileParams) (interface{}, error) {
	abs, err := w.resolvePath(params.Path)
	if err != nil {
		return nil, err
	}
	if abs == w.root {
		return nil, fmt.Errorf("workspace: refusing to delete the workspace root")
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("workspace: delete %q: %w", params.Path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("workspace: %q is a directory, delete_file only removes files", params.Path)
	}
	if err := w.checkReadBeforeWrite(abs, "delete"); err != nil {
		return nil, err
	}

	if err := os.Remove(abs); err != nil {
		return nil, fmt.Errorf("workspace: delete %q: %w", params.Path, err)
	}
	w.forgetRead(abs)

	return &WorkspaceWriteResult{
		Path:      w.relative(abs),
		Operation: "delete_file",
		Success:   true,
		Message:   "deleted",
	}, nil
}

// RunCommand runs a command with the workspace root as its working directory.
// The command is executed directly, without a shell, so nothing in the arguments
// is interpreted as an operator or a glob.
func (w *Workspace) RunCommand(params WorkspaceRunCommandParams) (interface{}, error) {
	command := strings.TrimSpace(params.Command)
	if command == "" {
		return nil, fmt.Errorf("workspace: command cannot be empty")
	}

	timeout := w.commandTimeout
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Second
	}
	tail := params.Tail
	if tail <= 0 {
		tail = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, params.Args...)
	cmd.Dir = w.root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	runErr := cmd.Run()
	duration := time.Since(started)

	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		// The command could not be started at all (not found, not executable).
		return nil, fmt.Errorf("workspace: run %q: %w", command, runErr)
	}

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	return &WorkspaceCommandResult{
		Command:  strings.TrimSpace(command + " " + strings.Join(params.Args, " ")),
		ExitCode: exitCode,
		Stdout:   tailLines(stdout.String(), tail),
		Stderr:   tailLines(stderr.String(), tail),
		Success:  runErr == nil,
		Duration: duration.String(),
		TimedOut: ctx.Err() == context.DeadlineExceeded,
	}, nil
}

// --- Helpers ---

// workspaceOpSet validates the operation aliases and turns them into a set.
// A nil list means "use the default".
func workspaceOpSet(ops []string, fallback []string) (map[string]bool, error) {
	if ops == nil {
		ops = fallback
	}
	set := make(map[string]bool, len(ops))
	for _, op := range ops {
		normalized := strings.ToLower(strings.TrimSpace(op))
		if !isWorkspaceOp(normalized) {
			return nil, fmt.Errorf(
				"workspace: unknown operation %q. Supported operations: %s",
				op, strings.Join(append(append([]string{}, WorkspaceReadOps...), WorkspaceWriteOps...), ", "))
		}
		set[normalized] = true
	}
	return set, nil
}

func isWorkspaceOp(op string) bool {
	switch op {
	case OpRead, OpList, OpSearch, OpWrite, OpEdit, OpMove, OpDelete, OpCommand:
		return true
	default:
		return false
	}
}

// splitLines splits text into lines, dropping the empty element a trailing
// newline produces.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// formatWithLineNumbers prefixes each line with its 1-indexed number, cat -n style.
func formatWithLineNumbers(lines []string, startLine int) string {
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%6d\t%s", startLine+i, line)
	}
	return b.String()
}

// extractSnippet returns the text around a match, with ellipses when it is cut.
func extractSnippet(content string, idx, matchLen int) string {
	start := idx - workspaceSnippetContext
	if start < 0 {
		start = 0
	}
	end := idx + matchLen + workspaceSnippetContext
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet += "..."
	}
	return snippet
}

// tailLines keeps only the last n lines of the output.
func tailLines(output string, n int) string {
	lines := splitLines(output)
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// writeFileAtomic writes to a temporary file in the same directory and renames
// it over the target, so a failed write never leaves a half-written file.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".workspace-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
