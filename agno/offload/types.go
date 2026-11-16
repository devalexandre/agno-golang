// Package offload ports agno/offload: big tool results become files in an
// agent filesystem (agno/fs), not messages.
//
// When a tool result crosses the threshold, the full payload is written to a
// per-session namespace and the transcript gets a short envelope: a head (and
// tail) preview, the total size and a result id. The model reads the payload
// back through two bounded tools, read_result (paged) and search_result
// (regex, focused passages). Writing is lossless and free (no model call);
// every read back is capped. A refused write yields a head+tail envelope that
// says so, and the run continues.
package offload

// NeverOffloadedTools are the read-back tools whose capped output is never offloaded.
var NeverOffloadedTools = []string{"read_result", "search_result"}

// ResultRef is a stored tool result: the pointer the transcript holds.
type ResultRef struct {
	ResultID    string `json:"result_id"`
	Path        string `json:"path"`
	ToolName    string `json:"tool_name"`
	SizeBytes   int    `json:"size_bytes"`
	LineCount   int    `json:"line_count"`
	ContentType string `json:"content_type"`
	CreatedAt   int64  `json:"created_at"`
}

// ResultPage is one bounded page of a stored result. When one line is longer
// than a page, the page ends inside it and NextStartChar says where in
// NextStartLine the next page begins.
type ResultPage struct {
	Text          string
	StartLine     int
	EndLine       int
	LineCount     int
	Truncated     bool
	NextStartLine int // 0 when nothing follows
	NextStartChar int
}

// ResultMatch is one search hit inside a stored result.
type ResultMatch struct {
	LineNumber int
	Line       string
	CharOffset int
	More       bool
}

// Row is one index row describing a stored payload.
type Row struct {
	ResultID    string `json:"result_id"`
	Namespace   string `json:"namespace"`
	Path        string `json:"path"`
	SessionID   string `json:"session_id"`
	RunID       string `json:"run_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	ArgsHash    string `json:"args_hash"`
	ContentType string `json:"content_type"`
	SizeBytes   int    `json:"size_bytes"`
	LineCount   int    `json:"line_count"`
	Preview     string `json:"preview"`
	UserID      string `json:"user_id,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at,omitempty"` // 0 = never
}

// Ref converts a row to its ResultRef.
func (r Row) Ref() ResultRef {
	return ResultRef{ResultID: r.ResultID, Path: r.Path, ToolName: r.ToolName, SizeBytes: r.SizeBytes,
		LineCount: r.LineCount, ContentType: r.ContentType, CreatedAt: r.CreatedAt}
}
