package fs

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// DefaultNamespace is used when the caller does not name one.
const DefaultNamespace = "default"

// Default quotas.
const (
	DefaultMaxFileBytes      = 1_000_000
	DefaultMaxNamespaceBytes = 20_000_000
)

// DefaultInstructions is the system-prompt text for a writable store.
const DefaultInstructions = `You have your own private, durable filesystem for notes that persist across sessions and runs. Use it to write, maintain and re-read prose that matters later: decisions with their reasoning, running documents on a topic, notes to your future self.

Conventions:
  - Paths are relative, like notes/decisions.md. Group related files in directories.
  - One topic, one file. Add dated entries as things develop; a note is a living document you maintain, not a log you only append to.
  - To correct or update part of a file, read it, then call replace_lines with the line numbers you saw. Never append a contradiction of what a note already says: fix the wording in place, in the same turn you learn it was wrong.
  - To find something, use search_content first: it reports the file and first-match line, which you can pass to read_file as start_line. Then answer from what the note says, not from memory.
  - To retire a note that is no longer current, move_file it into an archive/ directory. Do not blank it and do not overwrite it: its history may still be needed.
  - Store distilled content, not raw fetched payloads.
  - Never store secrets, passwords, or API keys.
  - Files have size limits. If a write is refused, split the topic into smaller files or archive what is finished. Never overwrite a note to make room if you might still need it.`

// ReadOnlyInstructions is the system-prompt text for a read-only store.
const ReadOnlyInstructions = `You have read access to a durable filesystem. The files persist across sessions and runs.

Use it to look up what has been recorded: decisions, running documents, notes. You cannot change these files - you have no tool to write, append, move, or delete.

Conventions:
  - Paths are relative, like notes/decisions.md.
  - Use search_content to find where something is recorded, then read_file to read it.`

// FileSystem is a durable, private filesystem scoped to one namespace.
// Same backend + same namespace = same files. Cheap to construct and to copy
// via Resolve/Partition; the backend owns all state.
type FileSystem struct {
	backend           Backend
	rawNamespace      string
	namespace         string
	placeholders      []string
	maxFileBytes      int
	maxNamespaceBytes int
	userScoped        *bool
	userID            *string
	nsCarriesUser     bool

	// Agent tool options: shape the toolkit, never restrict methods.
	ReadOnly     bool
	AllowDelete  bool
	IncludeTools []string
	instructions string
	toolName     string
}

// Option configures a FileSystem.
type Option func(*FileSystem)

// WithNamespace sets the namespace (may embed {user_id}, {agent_id}, {team_id}).
func WithNamespace(ns string) Option { return func(f *FileSystem) { f.rawNamespace = ns } }

// WithMaxFileBytes sets the per-file cap.
func WithMaxFileBytes(n int) Option { return func(f *FileSystem) { f.maxFileBytes = n } }

// WithMaxNamespaceBytes sets the per-namespace cap.
func WithMaxNamespaceBytes(n int) Option { return func(f *FileSystem) { f.maxNamespaceBytes = n } }

// WithUserScoped partitions files by the run's user (an anonymous run is refused).
func WithUserScoped(scoped bool) Option { return func(f *FileSystem) { f.userScoped = &scoped } }

// WithReadOnly exposes only the read tools to agents.
func WithReadOnly() Option { return func(f *FileSystem) { f.ReadOnly = true } }

// WithAllowDelete adds delete_file to the agent tools.
func WithAllowDelete() Option { return func(f *FileSystem) { f.AllowDelete = true } }

// WithIncludeTools selects an explicit tool whitelist.
func WithIncludeTools(names ...string) Option {
	return func(f *FileSystem) { f.IncludeTools = append([]string(nil), names...) }
}

// WithInstructions overrides the default instructions.
func WithInstructions(text string) Option { return func(f *FileSystem) { f.instructions = text } }

// WithToolName sets the toolkit name (tool names become <name>_<tool>); needed
// to attach several stores to one agent.
func WithToolName(name string) Option { return func(f *FileSystem) { f.toolName = name } }

// New creates a FileSystem over backend.
func New(backend Backend, opts ...Option) (*FileSystem, error) {
	if backend == nil {
		return nil, errors.New("fs: backend is required")
	}
	f := &FileSystem{backend: backend, rawNamespace: DefaultNamespace,
		maxFileBytes: DefaultMaxFileBytes, maxNamespaceBytes: DefaultMaxNamespaceBytes}
	for _, o := range opts {
		o(f)
	}
	if f.ReadOnly && f.AllowDelete {
		return nil, errors.New("fs: AllowDelete contradicts ReadOnly; pick one")
	}
	ns, err := NormalizeNamespace(f.rawNamespace)
	if err != nil {
		return nil, err
	}
	f.namespace = ns
	if f.placeholders, err = ParseNamespaceTemplate(ns); err != nil {
		return nil, err
	}
	for _, p := range f.placeholders {
		if p == "user_id" {
			f.nsCarriesUser = true
		}
	}
	return f, nil
}

// MustNew is New that panics on error.
func MustNew(backend Backend, opts ...Option) *FileSystem {
	f, err := New(backend, opts...)
	if err != nil {
		panic(err)
	}
	return f
}

// Backend returns the storage backend.
func (f *FileSystem) Backend() Backend { return f.backend }

// Namespace returns the normalized namespace (placeholders may remain).
func (f *FileSystem) Namespace() string { return f.namespace }

// MaxFileBytes returns the per-file cap.
func (f *FileSystem) MaxFileBytes() int { return f.maxFileBytes }

// MaxNamespaceBytes returns the per-namespace cap.
func (f *FileSystem) MaxNamespaceBytes() int { return f.maxNamespaceBytes }

// IsTemplated reports whether placeholders remain unresolved.
func (f *FileSystem) IsTemplated() bool { return len(f.placeholders) > 0 }

// UserID returns the bound user partition ("" = shared).
func (f *FileSystem) UserID() string {
	if f.userID == nil {
		return ""
	}
	return *f.userID
}

// Instructions returns the system-prompt text for this store's tools.
func (f *FileSystem) Instructions() string {
	if f.instructions != "" {
		return f.instructions
	}
	if f.ReadOnly {
		return ReadOnlyInstructions
	}
	return DefaultInstructions
}

func (f *FileSystem) clone() *FileSystem {
	c := *f
	c.placeholders = append([]string(nil), f.placeholders...)
	c.IncludeTools = append([]string(nil), f.IncludeTools...)
	return &c
}

// Resolve binds placeholders to concrete values and returns the bound copy.
// A userID also selects that user's partition (unless the namespace already
// embeds {user_id}). Empty values leave placeholders unresolved.
func (f *FileSystem) Resolve(userID, agentID, teamID string) (*FileSystem, error) {
	values := map[string]string{"user_id": userID, "agent_id": agentID, "team_id": teamID}
	c := f.clone()
	name := f.namespace
	var remaining []string
	for _, p := range f.placeholders {
		v := values[p]
		if v == "" {
			remaining = append(remaining, p)
			continue
		}
		enc, err := NormalizeTemplateValue(p, v)
		if err != nil {
			return nil, err
		}
		name = strings.ReplaceAll(name, "{"+p+"}", enc)
	}
	c.namespace = name
	c.placeholders = remaining
	if userID != "" {
		u := userID
		c.userID = &u
	}
	return c, nil
}

// ResolveForRun resolves from run context and enforces user scoping: a
// user-scoped store refuses a run without a user. Fails closed when a
// placeholder has no value.
func (f *FileSystem) ResolveForRun(userID, agentID, teamID string) (*FileSystem, error) {
	userID = strings.TrimSpace(userID)
	scoped := f.userScoped != nil && *f.userScoped
	if scoped && userID == "" {
		return nil, &InvalidPathError{Msg: "this filesystem is partitioned by user and the run has no user_id"}
	}
	bindUser := ""
	if scoped || f.nsCarriesUser {
		bindUser = userID
	}
	r, err := f.Resolve(bindUser, agentID, teamID)
	if err != nil {
		return nil, err
	}
	if _, err := r.requireResolved(); err != nil {
		return nil, err
	}
	return r, nil
}

// Partition binds the store to one user partition ("" = shared, explicit).
func (f *FileSystem) Partition(userID string) (*FileSystem, error) {
	if _, err := f.requireResolved(); err != nil {
		return nil, err
	}
	c := f.clone()
	if userID == "" {
		no := false
		c.userScoped = &no
		c.userID = nil
	} else {
		c.userID = &userID
	}
	return c, nil
}

// Partitions lists users holding files in this namespace (if the backend knows).
func (f *FileSystem) Partitions() ([]string, error) {
	ns, err := f.requireResolved()
	if err != nil {
		return nil, err
	}
	p, ok := f.backend.(Partitioner)
	if !ok {
		return nil, nil
	}
	out, err := p.Partitions(ns)
	sort.Strings(out)
	return out, err
}

func (f *FileSystem) requireResolved() (string, error) {
	if len(f.placeholders) > 0 {
		return "", &InvalidPathError{Msg: fmt.Sprintf("this agent's files require %s for this run and none was provided.", f.placeholders[0])}
	}
	return f.namespace, nil
}

func (f *FileSystem) partition() (string, error) {
	if f.nsCarriesUser {
		return "", nil
	}
	if f.userID != nil {
		return *f.userID, nil
	}
	if f.userScoped != nil && *f.userScoped {
		return "", &InvalidPathError{Msg: "this filesystem is partitioned by user and no user is bound; resolve it with a user_id first"}
	}
	return "", nil
}

func (f *FileSystem) prep(path string) (ns, p, user string, err error) {
	if ns, err = f.requireResolved(); err != nil {
		return
	}
	if user, err = f.partition(); err != nil {
		return
	}
	if path != "" {
		p, err = NormalizePath(path)
	}
	return
}

// Read returns the content and whether the file exists.
func (f *FileSystem) Read(path string) (string, bool, error) {
	ns, p, user, err := f.prep(path)
	if err != nil {
		return "", false, err
	}
	return f.backend.Read(ns, p, user)
}

// WriteOptions control Write.
type WriteOptions struct {
	// NoOverwrite fails with ErrExists when the file exists.
	NoOverwrite bool
	// ExpectedVersion requests a compare-and-swap write.
	ExpectedVersion *int
}

// Write creates or replaces a file (last-writer-wins unless ExpectedVersion).
func (f *FileSystem) Write(path, content string, opts ...WriteOptions) (FileMeta, error) {
	var o WriteOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	ns, p, user, err := f.prep(path)
	if err != nil {
		return FileMeta{}, err
	}
	size := len(content)
	if size > f.maxFileBytes {
		return FileMeta{}, fileQuota(p, size, f.maxFileBytes)
	}
	existing, err := stat(f.backend, ns, p, user)
	if err != nil {
		return FileMeta{}, err
	}
	if existing != nil && o.NoOverwrite {
		return FileMeta{}, wrapf(ErrExists, p)
	}
	delta := size
	if existing != nil {
		delta -= existing.SizeBytes
	}
	if delta > 0 {
		u, err := usage(f.backend, ns, user)
		if err != nil {
			return FileMeta{}, err
		}
		if u.TotalBytes+delta > f.maxNamespaceBytes {
			return FileMeta{}, f.nsQuota(u.TotalBytes)
		}
	}
	return f.backend.Write(ns, p, content, user, o.ExpectedVersion)
}

func (f *FileSystem) nsQuota(current int) error {
	return &QuotaExceededError{Msg: fmt.Sprintf("storage is full (%d of %d bytes)", current, f.maxNamespaceBytes),
		Scope: "namespace", Current: current, Limit: f.maxNamespaceBytes}
}

// Append appends line-oriented content, creating the file if missing. With
// unique, lines already present (or repeated in content) are dropped.
func (f *FileSystem) Append(path, content string, unique bool) (FileMeta, error) {
	ns, p, user, err := f.prep(path)
	if err != nil {
		return FileMeta{}, err
	}
	chunk := BuildChunk(content)
	if chunk != "" && unique {
		existing, _, err := f.backend.Read(ns, p, user)
		if err != nil {
			return FileMeta{}, err
		}
		seen := map[string]bool{}
		for _, l := range strings.Split(existing, "\n") {
			seen[l] = true
		}
		var kept []string
		for _, l := range strings.Split(chunk, "\n") {
			if l == "" || seen[l] {
				continue
			}
			seen[l] = true
			kept = append(kept, l)
		}
		chunk = BuildChunk(strings.Join(kept, "\n"))
	}
	if chunk == "" {
		m, err := stat(f.backend, ns, p, user)
		if err != nil {
			return FileMeta{}, err
		}
		if m != nil {
			return *m, nil
		}
		return FileMeta{Path: p}, nil
	}
	u, err := usage(f.backend, ns, user)
	if err != nil {
		return FileMeta{}, err
	}
	if u.TotalBytes+len(chunk)+1 > f.maxNamespaceBytes {
		return FileMeta{}, f.nsQuota(u.TotalBytes)
	}
	return appendChunk(f.backend, ns, p, chunk, user, f.maxFileBytes)
}

// ReplaceLines replaces lines start..end (1-indexed, inclusive) with content;
// empty content deletes the range. The trailing newline is preserved.
func (f *FileSystem) ReplaceLines(path string, start, end int, content string) (FileMeta, error) {
	ns, p, user, err := f.prep(path)
	if err != nil {
		return FileMeta{}, err
	}
	existing, ok, err := f.backend.Read(ns, p, user)
	if err != nil {
		return FileMeta{}, err
	}
	if !ok {
		return FileMeta{}, wrapf(ErrNotFound, p)
	}
	if start < 1 {
		return FileMeta{}, errors.New("start_line must be 1 or greater")
	}
	if end < start {
		return FileMeta{}, errors.New("end_line must be greater than or equal to start_line")
	}
	lines := strings.Split(existing, "\n")
	trailing := len(lines) > 0 && lines[len(lines)-1] == ""
	if trailing {
		lines = lines[:len(lines)-1]
	}
	if start > len(lines) {
		return FileMeta{}, fmt.Errorf("start_line %d is past the end of %s (%d lines)", start, p, len(lines))
	}
	var repl []string
	if content != "" {
		repl = strings.Split(content, "\n")
		if repl[len(repl)-1] == "" {
			repl = repl[:len(repl)-1]
		}
	}
	if end > len(lines) {
		end = len(lines)
	}
	out := append(append(append([]string{}, lines[:start-1]...), repl...), lines[end:]...)
	nc := strings.Join(out, "\n")
	if nc != "" && trailing {
		nc += "\n"
	}
	return f.Write(p, nc)
}

// Move moves or renames a file.
func (f *FileSystem) Move(src, dst string, overwrite bool) (FileMeta, error) {
	ns, s, user, err := f.prep(src)
	if err != nil {
		return FileMeta{}, err
	}
	d, err := NormalizePath(dst)
	if err != nil {
		return FileMeta{}, err
	}
	return move(f.backend, ns, s, d, user, overwrite)
}

// Delete deletes a file and reports whether it existed.
func (f *FileSystem) Delete(path string, expectedVersion *int) (bool, error) {
	ns, p, user, err := f.prep(path)
	if err != nil {
		return false, err
	}
	return f.backend.Delete(ns, p, user, expectedVersion)
}

// List lists files under directory ("" or "." = root), sorted by path segments.
func (f *FileSystem) List(directory string) ([]FileMeta, error) {
	ns, _, user, err := f.prep("")
	if err != nil {
		return nil, err
	}
	dir, err := NormalizeDirectory(directory)
	if err != nil {
		return nil, err
	}
	metas, err := f.backend.List(ns, dir, user)
	if err != nil {
		return nil, err
	}
	sort.Slice(metas, func(i, j int) bool { return pathLess(metas[i].Path, metas[j].Path) })
	return metas, nil
}

// Search is a case-insensitive substring search returning at most limit files.
func (f *FileSystem) Search(query, directory string, limit int) ([]SearchMatch, error) {
	ns, _, user, err := f.prep("")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	dir, err := NormalizeDirectory(directory)
	if err != nil {
		return nil, err
	}
	return search(f.backend, ns, query, dir, user, limit)
}

// Contains is a batch exact-line membership check (input order preserved).
func (f *FileSystem) Contains(lines []string, directory string) (ContainsResult, error) {
	ns, _, user, err := f.prep("")
	if err != nil {
		return ContainsResult{}, err
	}
	dir, err := NormalizeDirectory(directory)
	if err != nil {
		return ContainsResult{}, err
	}
	normalized, err := NormalizeCheckLines(lines)
	if err != nil {
		return ContainsResult{}, err
	}
	res := ContainsResult{Found: []string{}, Missing: []string{}}
	if len(normalized) == 0 {
		return res, nil
	}
	found, err := contains(f.backend, ns, normalized, dir, user)
	if err != nil {
		return ContainsResult{}, err
	}
	for _, l := range normalized {
		if found[l] {
			res.Found = append(res.Found, l)
		} else {
			res.Missing = append(res.Missing, l)
		}
	}
	return res, nil
}

// Usage returns aggregate file count and bytes for this namespace.
func (f *FileSystem) Usage() (NamespaceUsage, error) {
	ns, _, user, err := f.prep("")
	if err != nil {
		return NamespaceUsage{}, err
	}
	return usage(f.backend, ns, user)
}
