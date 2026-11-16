package offload

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/uuid"
)

// Scope is the framework-injected run context a store acts in.
type Scope struct {
	SessionID string
	RunID     string
	UserID    string
}

// ScopeFunc returns the current run's scope.
type ScopeFunc func() Scope

// StaticScope returns a ScopeFunc for a fixed scope.
func StaticScope(s Scope) ScopeFunc { return func() Scope { return s } }

// Tools exposes read_result and search_result.
type Tools struct {
	toolkit.Toolkit
	store *ResultStore
	scope ScopeFunc
}

// ReadResultParams are the read_result arguments.
type ReadResultParams struct {
	ResultID  string `json:"result_id" description:"The id from the result envelope, e.g. \"res_a91c4f20b3\"." required:"true"`
	StartLine int    `json:"start_line,omitempty" description:"First line to read (1-indexed, default 1)."`
	EndLine   int    `json:"end_line,omitempty" description:"Last line to read (inclusive). Defaults to the end."`
	StartChar int    `json:"start_char,omitempty" description:"Character offset into start_line to begin at (0-indexed, default 0)."`
	MaxChars  int    `json:"max_chars,omitempty" description:"Maximum characters to read, clamped to 1-16000. Use a small value for focused excerpts."`
}

// SearchResultParams are the search_result arguments.
type SearchResultParams struct {
	ResultID     string `json:"result_id" description:"The id from the result envelope, e.g. \"res_a91c4f20b3\"." required:"true"`
	Pattern      string `json:"pattern" description:"A regular expression (RE2 syntax)." required:"true"`
	ContextLines int    `json:"context_lines,omitempty" description:"Lines of context to include around each match (at most 20)."`
}

// NewTools builds the read-back toolkit (tool names read_result, search_result).
func NewTools(store *ResultStore, scope ScopeFunc) *Tools {
	if scope == nil {
		scope = StaticScope(Scope{})
	}
	t := &Tools{store: store, scope: scope}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "offload"
	t.Toolkit.Description = "Read back large tool results that were stored instead of shown in full."
	t.Register("read_result", "Read a stored tool result. Lines are 1-indexed and inclusive. Output is capped at 400 lines or 16000 characters; the reply names where to continue when there is more. A line longer than one page is read in pieces: continue with the start_char the reply names.", t, t.ReadResult, ReadResultParams{})
	t.Register("search_result", "Search a stored tool result with a regular expression. Returns at most 20 excerpts, each clipped to 500 characters. A long line is shown as a window around the match with its character offset; read the rest with read_result(start_line, start_char=offset).", t, t.SearchResult, SearchResultParams{})
	return t
}

func (t *Tools) accessError(id string) string {
	row, err := t.store.GetRow(id)
	if err != nil {
		return "Error: " + err.Error()
	}
	if row == nil {
		return "Error: unknown result id " + id
	}
	sc := t.scope()
	if row.SessionID != sc.SessionID {
		return fmt.Sprintf("Error: result %s belongs to a different session", id)
	}
	if row.UserID != "" && sc.UserID != "" && row.UserID != sc.UserID {
		return fmt.Sprintf("Error: result %s belongs to a different user", id)
	}
	return ""
}

// ReadResult implements read_result.
func (t *Tools) ReadResult(p ReadResultParams) (interface{}, error) {
	if e := t.accessError(p.ResultID); e != "" {
		return e, nil
	}
	page, err := t.store.Read(p.ResultID, p.StartLine, p.EndLine, p.StartChar, p.MaxChars)
	if err != nil {
		return "Error reading result: " + err.Error(), nil
	}
	header := fmt.Sprintf("Result %s, lines %d-%d of %d.", p.ResultID, page.StartLine, page.EndLine, page.LineCount)
	if page.NextStartLine > 0 {
		if page.NextStartChar > 0 {
			header += fmt.Sprintf(" More follows: line %d continues; read from line %d with start_char=%d.", page.NextStartLine, page.NextStartLine, page.NextStartChar)
		} else {
			header += fmt.Sprintf(" More follows: read from line %d.", page.NextStartLine)
		}
	}
	return header + "\n" + page.Text, nil
}

// SearchResult implements search_result.
func (t *Tools) SearchResult(p SearchResultParams) (interface{}, error) {
	if e := t.accessError(p.ResultID); e != "" {
		return e, nil
	}
	if _, err := regexp.Compile(p.Pattern); err != nil {
		return "Error: invalid regular expression: " + err.Error(), nil
	}
	matches, err := t.store.Search(p.ResultID, p.Pattern, p.ContextLines)
	if err != nil {
		return "Error searching result: " + err.Error(), nil
	}
	if len(matches) == 0 {
		return fmt.Sprintf("No matches in %s.", p.ResultID), nil
	}
	var lines []string
	if matches[len(matches)-1].More || len(matches) >= SearchMaxMatches {
		lines = append(lines, fmt.Sprintf("First %d matches in %s (more follow; narrow the pattern):", len(matches), p.ResultID))
	} else {
		lines = append(lines, fmt.Sprintf("%d match(es) in %s:", len(matches), p.ResultID))
	}
	for _, m := range matches {
		switch {
		case strings.Contains(m.Line, "\n"):
			lines = append(lines, fmt.Sprintf("match at line %d (char %d):\n%s", m.LineNumber, m.CharOffset, m.Line))
		case strings.HasPrefix(m.Line, "...") || strings.HasSuffix(m.Line, "..."):
			lines = append(lines, fmt.Sprintf("%d (char %d): %s", m.LineNumber, m.CharOffset, m.Line))
		default:
			lines = append(lines, fmt.Sprintf("%d: %s", m.LineNumber, m.Line))
		}
	}
	return strings.Join(lines, "\n"), nil
}

// OffloadingTool wraps a tool so results over the store's threshold are
// replaced by an envelope pointing at the stored payload.
type OffloadingTool struct {
	toolkit.Tool
	store *ResultStore
	scope ScopeFunc
}

// Wrap returns tool wrapped with result offloading.
func Wrap(tool toolkit.Tool, store *ResultStore, scope ScopeFunc) toolkit.Tool {
	if scope == nil {
		scope = StaticScope(Scope{})
	}
	return &OffloadingTool{Tool: tool, store: store, scope: scope}
}

// Unwrap returns the wrapped tool.
func (o *OffloadingTool) Unwrap() toolkit.Tool { return o.Tool }

var runFallback sync.Once
var fallbackRunID string

// Execute runs the tool and offloads an oversized result.
func (o *OffloadingTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	result, err := o.Tool.Execute(methodName, input)
	if err != nil || result == nil {
		return result, err
	}
	var text string
	switch v := result.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	default:
		data, mErr := json.Marshal(v)
		if mErr != nil {
			return result, nil
		}
		text = string(data)
	}
	if !o.store.ShouldOffload(methodName, text) {
		return result, nil
	}
	sc := o.scope()
	if sc.RunID == "" {
		runFallback.Do(func() { fallbackRunID = "run-" + uuid.NewString()[:8] })
		sc.RunID = fallbackRunID
	}
	var args map[string]interface{}
	_ = json.Unmarshal(input, &args)
	return o.store.OffloadForModel(OffloadRequest{
		SessionID: sc.SessionID, RunID: sc.RunID, ToolCallID: uuid.NewString(),
		ToolName: methodName, ToolArgs: args, Output: text, UserID: sc.UserID,
	}), nil
}

// WrapAll wraps every tool except the store's own read-back toolkit.
func WrapAll(tools []toolkit.Tool, store *ResultStore, scope ScopeFunc) []toolkit.Tool {
	out := make([]toolkit.Tool, len(tools))
	for i, t := range tools {
		if _, ok := t.(*Tools); ok {
			out[i] = t
			continue
		}
		out[i] = Wrap(t, store, scope)
	}
	return out
}
