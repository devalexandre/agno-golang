// Package fs ports agno/fs: a durable, private filesystem for agents.
//
// To the agent it looks like a normal file toolkit (read_file, write_file,
// append_file, replace_lines, list_files, search_content, check_lines,
// move_file, delete_file); underneath it is a pluggable Backend keyed by
// (namespace, user partition, path). Namespaces isolate stores, may embed the
// {user_id}/{agent_id}/{team_id} placeholders, and are size-capped per file
// and per namespace. A FileSystem can be exposed read-only.
//
//	store := fs.New(fs.NewLocalBackend("./agent-files"), fs.WithNamespace("research"))
//	agent.NewAgent(agent.AgentConfig{..., FileSystems: []*fs.FileSystem{store}})
package fs

// FileMeta is the metadata for one stored file.
type FileMeta struct {
	Path      string `json:"path"`
	SizeBytes int    `json:"size_bytes"`
	Version   *int   `json:"version,omitempty"`    // nil on backends without versioning
	UpdatedAt *int64 `json:"updated_at,omitempty"` // epoch seconds
	UserID    string `json:"user_id,omitempty"`
}

// FileData is one consistent read of a file's content and metadata.
type FileData struct {
	Content  string
	Metadata FileMeta
}

// SearchMatch is one file matching a content search.
type SearchMatch struct {
	Path       string `json:"path"`
	SizeBytes  int    `json:"size_bytes"`
	Snippet    string `json:"snippet"`
	Line       int    `json:"line"`
	MatchCount int    `json:"match_count"`
}

// ContainsResult is the result of a batch exact-line membership check.
type ContainsResult struct {
	Found   []string `json:"found"`
	Missing []string `json:"missing"`
}

// NamespaceUsage aggregates usage of a namespace.
type NamespaceUsage struct {
	FileCount  int `json:"file_count"`
	TotalBytes int `json:"total_bytes"`
}
