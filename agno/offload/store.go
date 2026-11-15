package offload

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/devalexandre/agno-golang/agno/fs"
)

// Limits (same values as Python).
const (
	MaxResultBytes           = 8_000_000
	MaxSessionNamespaceBytes = 200_000_000
	MaxCallIDAttempts        = 1000
	ReadMaxLines             = 400
	ReadMaxChars             = 16_000
	DefaultThresholdChars    = ReadMaxChars
	SearchMaxChars           = ReadMaxChars
	DefaultPreviewLines      = 20
	DefaultPreviewChars      = 1200
	SearchMaxMatches         = 20
	SearchMaxContextLines    = 20
	SearchLineClip           = 500
	matchHeaderChars         = 48
	tailLines                = 5
	SweepIntervalSeconds     = 300
)

// OffloadInstruction is added to the system prompt when offloading is on.
const OffloadInstruction = "Large tool results are stored as files and shown to you as a short preview with a " +
	"result id. The preview is not the whole result. Use search_result to locate what you " +
	"need and read_result to read that range; do not answer from the preview when the " +
	"preview was truncated. Read a focused excerpt with the match's line, start_char and a " +
	"small max_chars instead of reloading the whole result."

// ErrUnknownResult is returned for an unknown result id.
var ErrUnknownResult = errors.New("unknown result id")

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ResultIDFor derives the deterministic result id for one tool call.
func ResultIDFor(sessionID, runID, toolCallID string) string {
	return "res_" + sha(sessionID + ":" + runID + ":" + toolCallID)[:10]
}

func canonicalArgsHash(args map[string]interface{}) string {
	if args == nil {
		args = map[string]interface{}{}
	}
	data, err := json.Marshal(args) // map keys are sorted by encoding/json
	if err != nil {
		data = []byte(fmt.Sprint(args))
	}
	return sha(string(data))
}

// FormatSize formats bytes human-readably.
func FormatSize(n int) string { return fs.FormatSize(n) }

var namespaceUnsafe = regexp.MustCompile(`[^a-z0-9._@+-]`)

// NamespaceFor is the filesystem namespace holding one session's payloads.
func NamespaceFor(sessionID, scope string) string {
	limit := fs.MaxSegmentChars - 8 - 1
	readable := namespaceUnsafe.ReplaceAllString(strings.ToLower(sessionID), "_")
	if utf8.RuneCountInString(readable) > limit {
		readable = string([]rune(readable)[:limit])
	}
	if readable == "" {
		readable = "_"
	}
	key := sessionID
	if scope != "" {
		key = scope + ":" + sessionID
	}
	return "tool-results/" + readable + "-" + sha(key)[:8]
}

var unsafeSegment = regexp.MustCompile(`[\\/\x00-\x1f]`)

func safeSegment(v string) string {
	c := unsafeSegment.ReplaceAllString(v, "_")
	if c == "" {
		c = "_"
	}
	if r := []rune(c); len(r) > 120 {
		c = string(r[:120])
	}
	return c
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func runeSlice(s string, start, end int) string {
	r := []rune(s)
	if start < 0 {
		start = 0
	}
	if end > len(r) {
		end = len(r)
	}
	if start >= end {
		return ""
	}
	return string(r[start:end])
}

func headPreview(output string, lines, chars int) string {
	parts := strings.Split(output, "\n")
	if len(parts) > lines {
		parts = parts[:lines]
	}
	head := strings.Join(parts, "\n")
	if runeLen(head) > chars {
		head = runeSlice(head, 0, chars)
	}
	return head
}

// PreviewBlock is the preview an envelope shows: the head, plus the tail when cut.
func PreviewBlock(output string, previewLines, previewChars int) string {
	lines := strings.Split(output, "\n")
	head := headPreview(output, previewLines, previewChars)
	linesOmitted := len(lines) - len(strings.Split(head, "\n")) - tailLines
	total := runeLen(output)
	charsOmitted := total - runeLen(head) - min(total, previewChars)
	if linesOmitted <= 0 && charsOmitted <= 0 {
		return head
	}
	var tail, marker string
	if linesOmitted > 0 {
		tail = strings.Join(lines[len(lines)-tailLines:], "\n")
		marker = fmt.Sprintf("[... %d lines omitted ...]", linesOmitted)
	} else {
		tail = runeSlice(output, total-min(total, previewChars), total)
		marker = "[... omitted ...]"
	}
	if n := runeLen(tail); n > previewChars {
		tail = "..." + runeSlice(tail, n-previewChars, n)
	}
	return head + "\n" + marker + "\n" + tail
}

// RenderStoredEnvelope renders the transcript envelope for a stored result.
func RenderStoredEnvelope(ref ResultRef, preview string) string {
	return fmt.Sprintf("<result id=\"%s\" tool=\"%s\" lines=\"%d\" size=\"%s\">\n%s\n</result>\n"+
		"Full result stored; read with read_result(\"%s\") or search_result(\"%s\", pattern).",
		ref.ResultID, ref.ToolName, ref.LineCount, FormatSize(ref.SizeBytes), preview, ref.ResultID, ref.ResultID)
}

// RenderRefusedEnvelope renders the head+tail envelope when a write was refused.
func RenderRefusedEnvelope(toolName, output, reason string, previewLines, previewChars int) string {
	lines := strings.Split(output, "\n")
	head := headPreview(output, previewLines, previewChars)
	parts := []string{
		fmt.Sprintf("<result tool=\"%s\" lines=\"%d\" size=\"%s\" stored=\"false\" reason=\"%s\">", toolName, len(lines), FormatSize(len(output)), reason),
		head,
	}
	if omitted := len(lines) - len(strings.Split(head, "\n")) - tailLines; omitted > 0 {
		parts = append(parts, fmt.Sprintf("[... %d lines omitted ...]", omitted))
		tail := strings.Join(lines[len(lines)-tailLines:], "\n")
		if n := runeLen(tail); n > previewChars {
			tail = "..." + runeSlice(tail, n-previewChars, n)
		}
		parts = append(parts, tail)
	}
	parts = append(parts, "</result>", "Full result was NOT stored. Re-run the tool with a narrower query if you need the rest.")
	return strings.Join(parts, "\n")
}

func looksLikeJSON(output string) bool {
	s := strings.TrimLeft(output, " \t\r\n")
	if s == "" || (s[0] != '[' && s[0] != '{') {
		return false
	}
	return json.Valid([]byte(output))
}

// ResultStore stores oversized tool results as filesystem files with an index.
// It is also the settings object passed as AgentConfig.OffloadToolResults.
type ResultStore struct {
	Backend        fs.Backend
	Index          Index
	ThresholdChars int
	PreviewLines   int
	PreviewChars   int
	TTL            time.Duration // 0 = keep forever
	// Scope separates two stores sharing one backend (Python uses the db schema).
	Scope string

	mu        sync.Mutex
	lastSweep time.Time
}

// StoreOption configures a ResultStore.
type StoreOption func(*ResultStore)

// WithThresholdChars sets the offload threshold (characters).
func WithThresholdChars(n int) StoreOption { return func(s *ResultStore) { s.ThresholdChars = n } }

// WithPreview sets the preview size.
func WithPreview(lines, chars int) StoreOption {
	return func(s *ResultStore) { s.PreviewLines, s.PreviewChars = lines, chars }
}

// WithTTL expires stored results after ttl.
func WithTTL(ttl time.Duration) StoreOption { return func(s *ResultStore) { s.TTL = ttl } }

// WithIndex overrides the index (default: FSIndex over the same backend).
func WithIndex(idx Index) StoreOption { return func(s *ResultStore) { s.Index = idx } }

// NewResultStore creates a store whose payloads live in backend.
func NewResultStore(backend fs.Backend, opts ...StoreOption) (*ResultStore, error) {
	if backend == nil {
		return nil, errors.New("offload: backend is required")
	}
	s := &ResultStore{Backend: backend, ThresholdChars: DefaultThresholdChars,
		PreviewLines: DefaultPreviewLines, PreviewChars: DefaultPreviewChars}
	for _, o := range opts {
		o(s)
	}
	if s.ThresholdChars < 1 {
		return nil, errors.New("offload: ThresholdChars must be at least 1")
	}
	if s.Index == nil {
		idx, err := NewFSIndex(backend)
		if err != nil {
			return nil, err
		}
		s.Index = idx
	}
	return s, nil
}

// NewMemoryResultStore is a fully in-memory store (tests, ephemeral agents).
func NewMemoryResultStore(opts ...StoreOption) *ResultStore {
	s, _ := NewResultStore(fs.NewMemoryBackend(), append([]StoreOption{WithIndex(NewMemoryIndex())}, opts...)...)
	return s
}

func (s *ResultStore) fsFor(namespace string) (*fs.FileSystem, error) {
	return fs.New(s.Backend, fs.WithNamespace(namespace),
		fs.WithMaxFileBytes(MaxResultBytes), fs.WithMaxNamespaceBytes(MaxSessionNamespaceBytes))
}

// ShouldOffload reports whether a tool output crosses the threshold.
func (s *ResultStore) ShouldOffload(toolName, output string) bool {
	for _, n := range NeverOffloadedTools {
		if toolName == n || strings.HasSuffix(toolName, "_"+n) {
			return false
		}
	}
	return runeLen(output) > s.ThresholdChars
}

func (s *ResultStore) freeCallID(sessionID, runID, toolCallID string) string {
	candidate := toolCallID
	for attempt := 2; attempt < MaxCallIDAttempts+2; attempt++ {
		if r, _ := s.Index.Get(ResultIDFor(sessionID, runID, candidate)); r == nil {
			return candidate
		}
		candidate = fmt.Sprintf("%s~%d", toolCallID, attempt)
	}
	return candidate
}

// OffloadRequest describes one payload to store.
type OffloadRequest struct {
	SessionID  string
	RunID      string
	ToolCallID string
	ToolName   string
	ToolArgs   map[string]interface{}
	Output     string
	UserID     string
	Shared     bool
}

// Offload stores one payload and its index row. Returns *fs.QuotaExceededError
// when the store refuses the write.
func (s *ResultStore) Offload(req OffloadRequest) (ResultRef, error) {
	callID := s.freeCallID(req.SessionID, req.RunID, req.ToolCallID)
	resultID := ResultIDFor(req.SessionID, req.RunID, callID)
	contentType, ext := "text", "txt"
	if looksLikeJSON(req.Output) {
		contentType, ext = "json", "json"
	}
	prefix := "results"
	if req.Shared {
		prefix = "shared"
	}
	path := fmt.Sprintf("%s/%s/%s.%s", prefix, safeSegment(req.RunID), resultID, ext)
	ns := NamespaceFor(req.SessionID, s.Scope)
	sfs, err := s.fsFor(ns)
	if err != nil {
		return ResultRef{}, err
	}
	if _, err := sfs.Write(path, req.Output); err != nil {
		return ResultRef{}, err
	}
	now := time.Now().Unix()
	row := Row{
		ResultID: resultID, Namespace: sfs.Namespace(), Path: path, SessionID: req.SessionID, RunID: req.RunID,
		ToolCallID: callID, ToolName: req.ToolName, ArgsHash: canonicalArgsHash(req.ToolArgs), ContentType: contentType,
		SizeBytes: len(req.Output), LineCount: len(strings.Split(req.Output, "\n")),
		Preview: PreviewBlock(req.Output, s.PreviewLines, s.PreviewChars), UserID: req.UserID, CreatedAt: now,
	}
	if s.TTL > 0 {
		row.ExpiresAt = now + int64(s.TTL/time.Second)
	}
	if err := s.Index.Upsert(row); err != nil {
		_, _ = sfs.Delete(path, nil) // a payload without a row is unreachable
		return ResultRef{}, err
	}
	return row.Ref(), nil
}

// OffloadForModel offloads and returns the envelope; on refusal, the
// head+tail envelope. Never fails: the run continues either way.
func (s *ResultStore) OffloadForModel(req OffloadRequest) string {
	s.MaybeSweep()
	ref, err := s.Offload(req)
	if err == nil {
		return RenderStoredEnvelope(ref, PreviewBlock(req.Output, s.PreviewLines, s.PreviewChars))
	}
	var q *fs.QuotaExceededError
	reason := ""
	if errors.As(err, &q) {
		if q.Scope == "namespace" {
			reason = fmt.Sprintf("session storage is full (%d of %d bytes)", q.Current, q.Limit)
		} else {
			reason = fmt.Sprintf("result is too large to store (%d of %d bytes per result)", q.Current, q.Limit)
		}
	} else {
		log.Printf("offload: result offloading failed for %s: %v", req.ToolName, err)
		reason = "the result store refused the write: " + err.Error()
	}
	return RenderRefusedEnvelope(req.ToolName, req.Output, reason, s.PreviewLines, s.PreviewChars)
}

// GetRow returns the index row for a result id (nil when unknown).
func (s *ResultStore) GetRow(resultID string) (*Row, error) { return s.Index.Get(resultID) }

func (s *ResultStore) readPayload(row *Row) (string, error) {
	sfs, err := s.fsFor(row.Namespace)
	if err != nil {
		return "", err
	}
	content, ok, err := sfs.Read(row.Path)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("stored payload for %s is missing", row.ResultID)
	}
	return content, nil
}

func (s *ResultStore) row(resultID string) (*Row, error) {
	row, err := s.Index.Get(resultID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("%w %s", ErrUnknownResult, resultID)
	}
	return row, nil
}

// Payload returns the full stored text (for code, not for a transcript).
func (s *ResultStore) Payload(resultID string) (string, error) {
	row, err := s.row(resultID)
	if err != nil {
		return "", err
	}
	return s.readPayload(row)
}

// Read reads a page of a stored result. Lines are 1-indexed and inclusive
// (endLine 0 = to the end); startChar is the 0-indexed offset into startLine;
// maxChars is clamped to 1..16000 (0 = 16000).
func (s *ResultStore) Read(resultID string, startLine, endLine, startChar, maxChars int) (ResultPage, error) {
	content, err := s.Payload(resultID)
	if err != nil {
		return ResultPage{}, err
	}
	return PageFromContent(content, startLine, endLine, startChar, maxChars), nil
}

// PageFromContent pages content without ever losing a character.
func PageFromContent(content string, startLine, endLine, startChar, maxChars int) ResultPage {
	lines := strings.Split(content, "\n")
	count := len(lines)
	start := min(max(1, startLine), count)
	end := count
	if endLine > 0 {
		end = min(max(endLine, start), count)
	}
	offset := max(0, startChar)
	if maxChars <= 0 {
		maxChars = ReadMaxChars
	}
	limit := max(1, min(maxChars, ReadMaxChars))
	var pieces []string
	chars, last, nextLine, nextChar := 0, start-1, 0, 0
	truncated := false
	for cur := start; cur <= end; cur++ {
		if len(pieces) >= ReadMaxLines {
			truncated, nextLine = true, cur
			break
		}
		skip := 0
		if cur == start {
			skip = offset
		}
		line := []rune(lines[cur-1])
		if skip > len(line) {
			skip = len(line)
		}
		line = line[skip:]
		sep := 0
		if len(pieces) > 0 {
			sep = 1
		}
		room := limit - chars - sep
		if len(line) > room {
			if room > 0 {
				pieces = append(pieces, string(line[:room]))
				chars += room + sep
				last = cur
			}
			truncated, nextLine, nextChar = true, cur, skip+max(room, 0)
			break
		}
		pieces = append(pieces, string(line))
		chars += len(line) + sep
		last = cur
	}
	if nextLine == 0 && end < count {
		nextLine, nextChar = end+1, 0
	}
	return ResultPage{Text: strings.Join(pieces, "\n"), StartLine: start, EndLine: last, LineCount: count,
		Truncated: truncated, NextStartLine: nextLine, NextStartChar: nextChar}
}

// matchWindow returns rune bounds of the excerpt rendered around a match.
func matchWindow(lineLen, off, mlen, clip int) (int, int) {
	if lineLen <= clip {
		return 0, lineLen
	}
	width := clip - 6
	lead := width / 4
	start := max(0, min(off-lead, lineLen-width))
	need := off + min(max(mlen, 1), width)
	if start+width < need {
		start = min(need-width, lineLen-width)
	}
	return start, start + width
}

func clipAround(line []rune, off, mlen int) string {
	s, e := matchWindow(len(line), off, mlen, SearchLineClip)
	head, tail := "", ""
	if s > 0 {
		head = "..."
	}
	if e < len(line) {
		tail = "..."
	}
	return head + string(line[s:e]) + tail
}

type position struct{ line, off, mlen int }

// Search runs a regular expression (Go RE2 syntax, linear time: no
// catastrophic backtracking, so no subprocess deadline is needed) over a
// stored result; at most 20 excerpts, one read page in total.
func (s *ResultStore) Search(resultID, pattern string, contextLines int) ([]ResultMatch, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	content, err := s.Payload(resultID)
	if err != nil {
		return nil, err
	}
	return SearchContent(content, re, contextLines), nil
}

// SearchContent scans content with re and renders bounded excerpts.
func SearchContent(content string, re *regexp.Regexp, contextLines int) []ResultMatch {
	raw := strings.Split(content, "\n")
	lines := make([][]rune, len(raw))
	for i, l := range raw {
		lines[i] = []rune(l)
	}
	var positions []position
	more := false
scan:
	for i, l := range raw {
		covered := -1
		for _, loc := range re.FindAllStringIndex(l, -1) {
			off := utf8.RuneCountInString(l[:loc[0]])
			endR := off + utf8.RuneCountInString(l[loc[0]:loc[1]])
			if endR <= covered {
				continue
			}
			if len(positions) >= SearchMaxMatches {
				more = true
				break scan
			}
			positions = append(positions, position{i + 1, off, endR - off})
			_, covered = matchWindow(len(lines[i]), off, endR-off, SearchLineClip)
			if covered >= len(lines[i]) {
				break
			}
		}
	}
	ctx := max(0, min(contextLines, SearchMaxContextLines))
	var matches []ResultMatch
	budget := SearchMaxChars
	stopped := false
	prev := 0
	for _, p := range positions {
		if len(matches) > 0 && budget <= matchHeaderChars {
			stopped = true
			break
		}
		idx := p.line - 1
		var text string
		switch {
		case ctx > 0 && p.line == prev:
			text = fmt.Sprintf("%d: %s", p.line, clipAround(lines[idx], p.off, p.mlen))
		case ctx > 0:
			start, end := max(0, idx-ctx), min(len(lines), idx+ctx+1)
			rows := make([]string, 0, end-start)
			for k := start; k < end; k++ {
				off := 0
				if k == idx {
					off = p.off
				}
				rows = append(rows, fmt.Sprintf("%d: %s", k+1, clipAround(lines[k], off, p.mlen)))
			}
			text = strings.Join(rows, "\n")
		default:
			text = clipAround(lines[idx], p.off, p.mlen)
		}
		budget -= matchHeaderChars
		if runeLen(text) > budget {
			text = runeSlice(text, 0, max(budget, 0))
			stopped = true
		}
		budget -= runeLen(text) + 1
		prev = p.line
		matches = append(matches, ResultMatch{LineNumber: p.line, Line: text, CharOffset: p.off})
	}
	if len(matches) > 0 && (more || stopped) {
		matches[len(matches)-1].More = true
	}
	return matches
}

// LiveIDs lists the session's stored results, newest first.
func (s *ResultStore) LiveIDs(sessionID string, limit int) ([]ResultRef, error) {
	rows, err := s.Index.ListForSession(sessionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ResultRef, len(rows))
	for i, r := range rows {
		out[i] = r.Ref()
	}
	return out, nil
}

func (s *ResultStore) deleteRows(rows []Row) (int, error) {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if sfs, err := s.fsFor(r.Namespace); err == nil {
			if _, err := sfs.Delete(r.Path, nil); err != nil {
				log.Printf("offload: payload delete failed for %s: %v", r.ResultID, err)
			}
		}
		ids = append(ids, r.ResultID)
	}
	return len(rows), s.Index.Delete(ids)
}

// DeleteForSessions removes every stored result of the given sessions.
func (s *ResultStore) DeleteForSessions(sessionIDs ...string) (int, error) {
	var rows []Row
	for _, id := range sessionIDs {
		r, err := s.Index.ListForSession(id, 0)
		if err != nil {
			return 0, err
		}
		rows = append(rows, r...)
	}
	return s.deleteRows(rows)
}

// SweepExpired deletes results whose expiry has passed.
func (s *ResultStore) SweepExpired(now time.Time) (int, error) {
	rows, err := s.Index.Expired(now.Unix())
	if err != nil {
		return 0, err
	}
	return s.deleteRows(rows)
}

// MaybeSweep runs the TTL sweep at most once per min(TTL, 5 minutes).
func (s *ResultStore) MaybeSweep() {
	if s.TTL <= 0 {
		return
	}
	interval := min(time.Duration(SweepIntervalSeconds)*time.Second, s.TTL)
	s.mu.Lock()
	if time.Since(s.lastSweep) < interval {
		s.mu.Unlock()
		return
	}
	s.lastSweep = time.Now()
	s.mu.Unlock()
	if _, err := s.SweepExpired(time.Now()); err != nil {
		log.Printf("offload: TTL sweep failed: %v", err)
	}
}
