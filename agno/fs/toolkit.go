package fs

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Tool sets.
var (
	FullTools          = []string{"read_file", "write_file", "append_file", "replace_lines", "list_files", "search_content", "check_lines", "move_file", "delete_file"}
	DefaultTools       = []string{"read_file", "write_file", "append_file", "replace_lines", "list_files", "search_content", "move_file"}
	ReadOnlyTools      = []string{"read_file", "list_files", "search_content"}
	readCapableTools   = []string{"read_file", "list_files", "search_content", "check_lines"}
	maxDirEntries      = 200
	maxReadChars       = 100_000
	defaultToolkitName = "filesystem"
)

// RunScope carries the framework-injected identities used to resolve a
// templated namespace and the user partition. Never model-supplied.
type RunScope struct {
	UserID  string
	AgentID string
	TeamID  string
}

// Tools is the agent-facing toolkit over one FileSystem. Tool errors are
// returned as "Error: ..." strings, never as Go errors, so the model can
// correct itself.
type Tools struct {
	toolkit.Toolkit
	fs       *FileSystem
	scopeMu  sync.RWMutex
	scope    RunScope
	scopeFn  func() RunScope
	locks    sync.Map // path -> *sync.Mutex
	readOnly bool
}

// ToolsOption configures the toolkit.
type ToolsOption func(*toolsCfg)

type toolsCfg struct {
	scope   RunScope
	scopeFn func() RunScope
	name    string
}

// WithRunScope sets the identities used to resolve the namespace.
func WithRunScope(scope RunScope) ToolsOption { return func(c *toolsCfg) { c.scope = scope } }

// WithScopeFunc resolves the identities at each tool call (e.g. from the
// agent's current run), taking precedence over WithRunScope.
func WithScopeFunc(fn func() RunScope) ToolsOption { return func(c *toolsCfg) { c.scopeFn = fn } }

// WithToolkitName overrides the toolkit name (tool names become <name>_<tool>).
func WithToolkitName(name string) ToolsOption { return func(c *toolsCfg) { c.name = name } }

// NewTools builds the toolkit using the FileSystem's tool options
// (ReadOnly, AllowDelete, IncludeTools, tool name).
func NewTools(f *FileSystem, opts ...ToolsOption) *Tools {
	cfg := toolsCfg{}
	for _, o := range opts {
		o(&cfg)
	}
	t := &Tools{fs: f, scope: cfg.scope, scopeFn: cfg.scopeFn, readOnly: f.ReadOnly}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = f.toolName
	if cfg.name != "" {
		t.Toolkit.Name = cfg.name
	}
	if t.Toolkit.Name == "" {
		t.Toolkit.Name = defaultToolkitName
	}
	t.Toolkit.Description = "A durable, private filesystem for the agent's notes."
	for _, name := range t.selected() {
		t.register(name)
	}
	return t
}

// ToolName returns the configured toolkit name ("" = default "filesystem").
func (f *FileSystem) ToolName() string { return f.toolName }

// Tools returns the toolkit for this FileSystem (convenience for NewTools).
func (f *FileSystem) Tools(opts ...ToolsOption) *Tools { return NewTools(f, opts...) }

func (t *Tools) selected() []string {
	if t.fs.IncludeTools != nil {
		allowed := FullTools
		if t.readOnly {
			allowed = readCapableTools
		}
		var out []string
		for _, n := range t.fs.IncludeTools {
			for _, a := range allowed {
				if n == a {
					out = append(out, n)
				}
			}
		}
		return out
	}
	if t.readOnly {
		return ReadOnlyTools
	}
	out := append([]string{}, DefaultTools...)
	if t.fs.AllowDelete {
		out = append(out, "delete_file")
	}
	return out
}

// SetRunScope updates the identities used to resolve the namespace.
func (t *Tools) SetRunScope(scope RunScope) {
	t.scopeMu.Lock()
	t.scope = scope
	t.scopeMu.Unlock()
}

// CanWrite reports whether any registered tool can change files.
func (t *Tools) CanWrite() bool {
	for _, n := range t.selected() {
		w := true
		for _, r := range readCapableTools {
			if n == r {
				w = false
			}
		}
		if w {
			return true
		}
	}
	return false
}

// Instructions returns the instructions matching this toolkit.
func (t *Tools) Instructions() string { return t.fs.Instructions() }

func (t *Tools) resolved() (*FileSystem, error) {
	t.scopeMu.RLock()
	s, fn := t.scope, t.scopeFn
	t.scopeMu.RUnlock()
	if fn != nil {
		s = fn()
	}
	return t.fs.ResolveForRun(s.UserID, s.AgentID, s.TeamID)
}

// Param structs.
type (
	ReadFileParams struct {
		Path      string `json:"path" description:"File path, e.g. \"notes/decisions.md\"." required:"true"`
		StartLine int    `json:"start_line,omitempty" description:"Optional 1-indexed first line to return."`
		EndLine   int    `json:"end_line,omitempty" description:"Optional 1-indexed last line to return (inclusive)."`
	}
	ListFilesParams struct {
		Directory string `json:"directory,omitempty" description:"Directory to list (default \".\" = top level)."`
		Pattern   string `json:"pattern,omitempty" description:"Optional glob to filter names, e.g. \"*.md\"."`
		Recursive bool   `json:"recursive,omitempty" description:"Walk the directory tree up to max_depth levels deep."`
		MaxDepth  int    `json:"max_depth,omitempty" description:"Depth limit when recursive (default 3)."`
	}
	SearchContentParams struct {
		Query     string `json:"query" description:"Substring to search for (case-insensitive)." required:"true"`
		Directory string `json:"directory,omitempty" description:"Directory to scope the search (default everything)."`
		Limit     int    `json:"limit,omitempty" description:"Maximum matching files to return (default 10)."`
	}
	CheckLinesParams struct {
		Lines     []string `json:"lines" description:"Records to check (max 200), exactly as stored." required:"true"`
		Directory string   `json:"directory,omitempty" description:"Directory to scope the check."`
	}
	WriteFileParams struct {
		Path      string `json:"path" description:"File path, e.g. \"state/last-run.md\". Parent folders are implicit." required:"true"`
		Content   string `json:"content" description:"The complete new file content." required:"true"`
		Overwrite *bool  `json:"overwrite,omitempty" description:"If false, fail when the file already exists (default true)."`
	}
	AppendFileParams struct {
		Path    string `json:"path" description:"File path, e.g. \"seen/2026-07-24.md\"." required:"true"`
		Content string `json:"content" description:"One or more lines to append." required:"true"`
		Unique  bool   `json:"unique,omitempty" description:"Skip lines the file already contains."`
	}
	ReplaceLinesParams struct {
		Path      string `json:"path" description:"File path." required:"true"`
		StartLine int    `json:"start_line" description:"First line to replace, 1-indexed." required:"true"`
		EndLine   int    `json:"end_line" description:"Last line to replace, inclusive." required:"true"`
		Content   string `json:"content,omitempty" description:"Replacement lines. Empty deletes the range. Never include line-number prefixes."`
	}
	MoveFileParams struct {
		Src       string `json:"src" description:"Source file path." required:"true"`
		Dst       string `json:"dst" description:"Destination path." required:"true"`
		Overwrite bool   `json:"overwrite,omitempty" description:"Replace dst if it exists (default false)."`
	}
	DeleteFileParams struct {
		Path string `json:"path" description:"File path to delete." required:"true"`
	}
)

func (t *Tools) register(name string) {
	switch name {
	case "read_file":
		t.Register(name, "Read a file from your files, returning cat -n style line-numbered output. Line numbers are display only: never include them in content you write.", t, t.ReadFile, ReadFileParams{})
	case "list_files":
		t.Register(name, "List your files with size and last-updated time (UTC), plus total usage against your storage limit.", t, t.ListFiles, ListFilesParams{})
	case "search_content":
		t.Register(name, "Search your files for text (case-insensitive substring). Each result gives the first-match line to pass to read_file.", t, t.SearchContent, SearchContentParams{})
	case "check_lines":
		t.Register(name, "Check which of these exact lines already exist in your files (whole-line match). Use before acting on items so you never repeat work.", t, t.CheckLines, CheckLinesParams{})
	case "write_file":
		t.Register(name, "Create or overwrite a file. For adding records to a log-style file, use append_file instead.", t, t.WriteFile, WriteFileParams{})
		// Overwrite is *bool (default true); the schema generator maps pointers to string.
		if m, ok := t.GetMethods()[t.Name+"_"+name]; ok {
			if props, ok := m.Schema["properties"].(map[string]interface{}); ok {
				if prop, ok := props["overwrite"].(map[string]interface{}); ok {
					prop["type"] = "boolean"
				}
			}
		}
	case "append_file":
		t.Register(name, "Append lines to a file, creating it if needed. Keep one record per line.", t, t.AppendFile, AppendFileParams{})
	case "replace_lines":
		t.Register(name, "Replace lines start_line through end_line (1-indexed, inclusive) with new content, or delete them.", t, t.ReplaceLines, ReplaceLinesParams{})
	case "move_file":
		t.Register(name, "Move or rename a file.", t, t.MoveFile, MoveFileParams{})
	case "delete_file":
		t.Register(name, "Delete a file. This cannot be undone.", t, t.DeleteFile, DeleteFileParams{})
	}
}

func errString(err error) string { return "Error: " + err.Error() }

func quotaMessage(e *QuotaExceededError, p string) string {
	if e.Scope == "file" {
		return fmt.Sprintf("Error: %s would be %d bytes (limit %d per file). Split the topic into smaller files (or partition by date) and retry.", p, e.Current, e.Limit)
	}
	return fmt.Sprintf("Error: storage is full (%d of %d bytes). Free space only if you have a tool for it and only from files you are certain are obsolete (see list_files). Never overwrite or discard records you might still need to make room; if nothing is safely disposable, stop and report that storage is full.", e.Current, e.Limit)
}

func toolError(err error, p string) string {
	var q *QuotaExceededError
	if errors.As(err, &q) {
		return quotaMessage(q, p)
	}
	return errString(err)
}

func lineCount(s string) int {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return len(lines)
}

func withLineNumbers(text string, start int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%6d\t%s", i+start, l)
	}
	return b.String()
}

// FormatSize formats bytes human-readably (B, KB, MB, GB).
func FormatSize(size int) string {
	f := float64(size)
	for _, unit := range []string{"B", "KB", "MB"} {
		if f < 1024 {
			if unit == "B" {
				return fmt.Sprintf("%d%s", int(f), unit)
			}
			return fmt.Sprintf("%.1f%s", f, unit)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.1fGB", f)
}

func formatTime(ts *int64) interface{} {
	if ts == nil {
		return nil
	}
	return time.Unix(*ts, 0).UTC().Format("2006-01-02T15:04:05Z")
}

func (t *Tools) lock(paths ...string) func() {
	keys := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" {
			keys = append(keys, t.fs.namespace+":"+p)
		}
	}
	sort.Strings(keys)
	var held []*sync.Mutex
	for i, k := range keys {
		if i > 0 && keys[i-1] == k {
			continue
		}
		m, _ := t.locks.LoadOrStore(k, &sync.Mutex{})
		mu := m.(*sync.Mutex)
		mu.Lock()
		held = append(held, mu)
	}
	return func() {
		for i := len(held) - 1; i >= 0; i-- {
			held[i].Unlock()
		}
	}
}

func jsonString(v interface{}) string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}

// ReadFile implements read_file.
func (t *Tools) ReadFile(p ReadFileParams) (interface{}, error) {
	f, err := t.resolved()
	if err != nil {
		return errString(err), nil
	}
	contents, ok, err := f.Read(p.Path)
	if err != nil {
		return errString(err), nil
	}
	if !ok {
		return "Error: file not found: " + p.Path, nil
	}
	total := lineCount(contents)
	if total == 0 {
		return fmt.Sprintf("(%s is empty)", p.Path), nil
	}
	if p.StartLine == 0 && p.EndLine == 0 {
		if n := len([]rune(contents)); n > maxReadChars {
			return fmt.Sprintf("Error: file too long to read whole (%d chars, %d lines; limit %d chars). Read a range with start_line/end_line, or use search_content first: it reports each matching file's first-match line.", n, total, maxReadChars), nil
		}
		return withLineNumbers(contents, 1), nil
	}
	start, end := p.StartLine, p.EndLine
	if start == 0 {
		start = 1
	}
	if end == 0 {
		end = total
	}
	switch {
	case start < 1:
		return fmt.Sprintf("Error: start_line must be 1 or greater (got %d).", start), nil
	case end < 1:
		return fmt.Sprintf("Error: end_line must be 1 or greater (got %d).", end), nil
	case start > total:
		return fmt.Sprintf("Error: start_line %d is past the end of %s, which has %d lines.", start, p.Path, total), nil
	case end < start:
		return fmt.Sprintf("Error: end_line %d is before start_line %d.", end, start), nil
	}
	lines := strings.Split(contents, "\n")
	return withLineNumbers(strings.Join(lines[start-1:min(end, total)], "\n"), start), nil
}

// ListFiles implements list_files.
func (t *Tools) ListFiles(p ListFilesParams) (interface{}, error) {
	f, err := t.resolved()
	if err != nil {
		return errString(err), nil
	}
	directory := p.Directory
	if directory == "" {
		directory = "."
	}
	metas, err := f.List(directory)
	if err != nil {
		return errString(err), nil
	}
	nd, _ := NormalizeDirectory(directory)
	prefixLen := 0
	if nd != "" {
		prefixLen = len(strings.Split(nd, "/"))
	}
	maxDepth := p.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	depthCap := 1
	if p.Recursive {
		depthCap = maxDepth + 1
	}
	match := func(name string) bool {
		if p.Pattern == "" {
			return true
		}
		ok, _ := path.Match(p.Pattern, name)
		return ok
	}
	type entry = map[string]interface{}
	var files []entry
	dirs := map[string]bool{}
	for _, m := range metas {
		segs := strings.Split(m.Path, "/")
		rel := segs[prefixLen:]
		if len(rel) <= depthCap && match(segs[len(segs)-1]) {
			files = append(files, entry{"path": m.Path, "type": "file", "size": FormatSize(m.SizeBytes), "updated": formatTime(m.UpdatedAt)})
		}
		for k := 1; k <= min(len(rel)-1, depthCap); k++ {
			dp := strings.Join(segs[:prefixLen+k], "/")
			if match(segs[prefixLen+k-1]) {
				dirs[dp] = true
			}
		}
	}
	dirList := make([]string, 0, len(dirs))
	for d := range dirs {
		dirList = append(dirList, d)
	}
	sort.Slice(dirList, func(i, j int) bool { return pathLess(dirList[i], dirList[j]) })
	truncDirs, truncFiles := 0, 0
	if len(dirList) > maxDirEntries {
		truncDirs = len(dirList) - maxDirEntries
		dirList = dirList[:maxDirEntries]
	}
	if len(files) > maxDirEntries {
		truncFiles = len(files) - maxDirEntries
		files = files[:maxDirEntries]
	}
	entries := files
	for _, d := range dirList {
		entries = append(entries, entry{"path": d, "type": "dir", "size": nil, "updated": nil})
	}
	sort.SliceStable(entries, func(i, j int) bool { return pathLess(entries[i]["path"].(string), entries[j]["path"].(string)) })
	if truncFiles > 0 {
		entries = append(entries, entry{"path": fmt.Sprintf("...and %d more", truncFiles), "type": "file", "size": nil, "updated": nil})
	}
	if truncDirs > 0 {
		entries = append(entries, entry{"path": fmt.Sprintf("...and %d more", truncDirs), "type": "dir", "size": nil, "updated": nil})
	}
	if entries == nil {
		entries = []entry{}
	}
	u, err := f.Usage()
	if err != nil {
		return errString(err), nil
	}
	var pattern interface{}
	if p.Pattern != "" {
		pattern = p.Pattern
	}
	return jsonString(map[string]interface{}{
		"directory": directory, "pattern": pattern, "recursive": p.Recursive, "files": entries,
		"usage": map[string]interface{}{"files": u.FileCount, "bytes_used": u.TotalBytes, "bytes_limit": f.maxNamespaceBytes},
	}), nil
}

// SearchContent implements search_content.
func (t *Tools) SearchContent(p SearchContentParams) (interface{}, error) {
	if strings.TrimSpace(p.Query) == "" {
		return "Error: query cannot be empty", nil
	}
	f, err := t.resolved()
	if err != nil {
		return errString(err), nil
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 10
	}
	matches, err := f.Search(p.Query, p.Directory, limit)
	if err != nil {
		return errString(err), nil
	}
	files := make([]map[string]interface{}, 0, len(matches))
	for _, m := range matches {
		files = append(files, map[string]interface{}{"file": m.Path, "line": m.Line, "matches": m.MatchCount, "size": FormatSize(m.SizeBytes), "snippet": m.Snippet})
	}
	return jsonString(map[string]interface{}{"query": p.Query, "matches_found": len(files), "files": files}), nil
}

// CheckLines implements check_lines.
func (t *Tools) CheckLines(p CheckLinesParams) (interface{}, error) {
	f, err := t.resolved()
	if err != nil {
		return errString(err), nil
	}
	res, err := f.Contains(p.Lines, p.Directory)
	if err != nil {
		return errString(err), nil
	}
	return jsonString(map[string]interface{}{"found": res.Found, "missing": res.Missing}), nil
}

// WriteFile implements write_file.
func (t *Tools) WriteFile(p WriteFileParams) (interface{}, error) {
	defer t.lock(p.Path)()
	f, err := t.resolved()
	if err != nil {
		return errString(err), nil
	}
	overwrite := p.Overwrite == nil || *p.Overwrite
	meta, err := f.Write(p.Path, p.Content, WriteOptions{NoOverwrite: !overwrite})
	if err != nil {
		if errors.Is(err, ErrExists) {
			if !overwrite {
				return "Error: file exists and overwrite=False: " + p.Path, nil
			}
			return fmt.Sprintf("Error: cannot write %s: a parent path segment is an existing file.", p.Path), nil
		}
		return toolError(err, p.Path), nil
	}
	return fmt.Sprintf("Wrote %d bytes to %s", meta.SizeBytes, p.Path), nil
}

// AppendFile implements append_file.
func (t *Tools) AppendFile(p AppendFileParams) (interface{}, error) {
	defer t.lock(p.Path)()
	f, err := t.resolved()
	if err != nil {
		return errString(err), nil
	}
	if p.Unique {
		prev, _, err := f.Read(p.Path)
		if err != nil {
			return errString(err), nil
		}
		meta, err := f.Append(p.Path, p.Content, true)
		if err != nil {
			return toolError(err, p.Path), nil
		}
		added := meta.SizeBytes - len(prev)
		if added <= 0 {
			return fmt.Sprintf("Appended nothing to %s: every line was already present (still %d bytes)", p.Path, meta.SizeBytes), nil
		}
		return fmt.Sprintf("Appended %d bytes to %s, skipping lines already present (now %d bytes)", added, p.Path, meta.SizeBytes), nil
	}
	meta, err := f.Append(p.Path, p.Content, false)
	if err != nil {
		return toolError(err, p.Path), nil
	}
	return fmt.Sprintf("Appended %d bytes to %s (now %d bytes)", len(BuildChunk(p.Content)), p.Path, meta.SizeBytes), nil
}

// ReplaceLines implements replace_lines.
func (t *Tools) ReplaceLines(p ReplaceLinesParams) (interface{}, error) {
	defer t.lock(p.Path)()
	f, err := t.resolved()
	if err != nil {
		return errString(err), nil
	}
	meta, err := f.ReplaceLines(p.Path, p.StartLine, p.EndLine, p.Content)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "Error: file not found: " + p.Path, nil
		}
		return toolError(err, p.Path), nil
	}
	action := "Replaced"
	if p.Content == "" {
		action = "Deleted"
	}
	return fmt.Sprintf("%s lines %d-%d in %s (now %d bytes)", action, p.StartLine, p.EndLine, p.Path, meta.SizeBytes), nil
}

// MoveFile implements move_file.
func (t *Tools) MoveFile(p MoveFileParams) (interface{}, error) {
	defer t.lock(p.Src, p.Dst)()
	f, err := t.resolved()
	if err != nil {
		return errString(err), nil
	}
	if _, err := f.Move(p.Src, p.Dst, p.Overwrite); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return "Error: file not found: " + p.Src, nil
		case errors.Is(err, ErrExists) && !p.Overwrite:
			return "Error: dst exists and overwrite=False: " + p.Dst, nil
		case errors.Is(err, ErrExists):
			return fmt.Sprintf("Error: could not move to %s; it may have changed concurrently. Retry or use another name.", p.Dst), nil
		}
		return errString(err), nil
	}
	return fmt.Sprintf("Moved %s -> %s", p.Src, p.Dst), nil
}

// DeleteFile implements delete_file.
func (t *Tools) DeleteFile(p DeleteFileParams) (interface{}, error) {
	defer t.lock(p.Path)()
	f, err := t.resolved()
	if err != nil {
		return errString(err), nil
	}
	ok, err := f.Delete(p.Path, nil)
	if err != nil {
		return errString(err), nil
	}
	if !ok {
		return "Error: file not found: " + p.Path, nil
	}
	return "Deleted " + p.Path, nil
}
