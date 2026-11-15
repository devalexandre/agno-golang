package chunking

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// PageMarkdownChunking is a heading-aware markdown chunker that keeps
// newlines and code fences intact. Port of agno-python
// knowledge/chunking/page.py:
//
//   - Splits on headings of level <= SplitLevel (default H1/H2) outside code
//     fences; deeper headings stay with their section.
//   - A section holding only headings is folded into the next one, so no chunk
//     is a bare heading (a page that is only a title yields no chunks).
//   - A section longer than ChunkSize is split on blank lines, then lines,
//     never mid-line; an oversized code fence is closed and re-opened in each
//     piece so every chunk is valid markdown.
//   - Every chunk opens with the page context line ("Title: first paragraph")
//     and a breadcrumb ("Title › Section"). Metadata carries chunk, chunk_size,
//     breadcrumb and, when the document has a "path" (or name), its top-level
//     section and kind (example / reference / guide).
type PageMarkdownChunking struct {
	ChunkSize  int
	SplitLevel int
}

// DefaultPageChunkSize is PageMarkdownChunking's default chunk size.
const DefaultPageChunkSize = 2000

// NewPageMarkdownChunking creates a page chunker (chunkSize default 2000,
// splitLevel default 2).
func NewPageMarkdownChunking(chunkSize, splitLevel int) *PageMarkdownChunking {
	if chunkSize <= 0 {
		chunkSize = DefaultPageChunkSize
	}
	if splitLevel <= 0 {
		splitLevel = 2
	}
	return &PageMarkdownChunking{ChunkSize: chunkSize, SplitLevel: splitLevel}
}

func (c *PageMarkdownChunking) GetChunkSize() int { return c.ChunkSize }
func (c *PageMarkdownChunking) GetOverlap() int   { return 0 }

var (
	pageHeading      = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	pageAnchorSuffix = regexp.MustCompile(`\s*\[#[^\]]*\]\s*$`)
	pageTitleSuffix  = regexp.MustCompile(`\s*\(/[^)\s]*\)\s*$`)
	pageFence        = regexp.MustCompile("^\\s*(`{3,}|~{3,})(.*)$")
)

// fenceState is (delimiter char, opening length, opening line); nil = outside.
type fenceState struct {
	char   byte
	length int
	line   string
}

// advanceCodeFence ports agno.utils.markdown.advance_code_fence.
func advanceCodeFence(line string, opened *fenceState) (*fenceState, bool) {
	m := pageFence.FindStringSubmatch(line)
	if m == nil {
		return opened, false
	}
	delim, suffix := m[1], m[2]
	if opened == nil {
		if delim[0] == '`' && strings.Contains(suffix, "`") {
			return nil, false
		}
		return &fenceState{char: delim[0], length: len(delim), line: line}, true
	}
	if delim[0] == opened.char && len(delim) >= opened.length && strings.TrimSpace(suffix) == "" {
		return nil, true
	}
	return opened, false
}

// CleanHeading strips fumadocs anchor ("[#anchor]") and, for page titles,
// path ("(/docs/path)") suffixes.
func CleanHeading(title string, pageTitle bool) string {
	title = pageAnchorSuffix.ReplaceAllString(title, "")
	if pageTitle {
		title = pageTitleSuffix.ReplaceAllString(title, "")
	}
	return strings.TrimSpace(title)
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func headingOnly(body string) bool {
	for _, l := range splitLines(body) {
		if strings.TrimSpace(l) != "" && !pageHeading.MatchString(l) {
			return false
		}
	}
	return true
}

// PageIntro returns the page's first paragraph of prose, at most limit runes.
func PageIntro(text string, limit int) string {
	var lines []string
	var fence *fenceState
	for _, line := range splitLines(text) {
		stripped := strings.TrimSpace(line)
		fence, _ = advanceCodeFence(line, fence)
		if fence != nil || stripped == "" || strings.HasPrefix(stripped, "#") || strings.HasPrefix(stripped, "```") ||
			strings.HasPrefix(stripped, "~~~") || strings.HasPrefix(stripped, "**Step") {
			if len(lines) > 0 {
				break
			}
			continue
		}
		lines = append(lines, strings.TrimSpace(strings.TrimLeft(stripped, "> ")))
	}
	out := []rune(strings.Join(lines, " "))
	if len(out) > limit {
		out = out[:limit]
	}
	return string(out)
}

type pageSection struct {
	trail []string
	body  string
}

func pageSections(text string, splitLevel int) []pageSection {
	type raw struct {
		trail []string
		lines []string
	}
	var trail []string
	sections := []raw{{}}
	var fence *fenceState
	for _, line := range splitLines(text) {
		fence, _ = advanceCodeFence(line, fence)
		var m []string
		if fence == nil {
			m = pageHeading.FindStringSubmatch(line)
		}
		if m != nil && len(m[1]) <= splitLevel {
			level := len(m[1])
			title := CleanHeading(m[2], level == 1)
			if len(trail) > level-1 {
				trail = trail[:level-1]
			}
			trail = append(append([]string{}, trail...), title)
			sections = append(sections, raw{trail: append([]string{}, trail...), lines: []string{line}})
			continue
		}
		sections[len(sections)-1].lines = append(sections[len(sections)-1].lines, line)
	}
	var folded []pageSection
	pending := ""
	for _, s := range sections {
		body := strings.Trim(strings.Join(s.lines, "\n"), "\n")
		if strings.TrimSpace(body) == "" {
			continue
		}
		if headingOnly(body) {
			if pending != "" {
				pending += "\n\n" + body
			} else {
				pending = body
			}
			continue
		}
		if pending != "" {
			body = pending + "\n\n" + body
		}
		folded = append(folded, pageSection{trail: s.trail, body: body})
		pending = ""
	}
	if pending != "" && len(folded) > 0 {
		folded[len(folded)-1].body += "\n\n" + pending
	}
	return folded
}

// PageKind classifies a docs path: "example", "reference" or "guide".
func PageKind(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if parts[0] == "reference" || parts[0] == "reference-api" {
		return "reference"
	}
	if parts[0] == "examples" {
		return "example"
	}
	for _, p := range parts {
		if p == "usage" {
			return "example"
		}
	}
	return "guide"
}

func splitLong(body string, limit int) []string {
	total := runeLen(body)
	if total <= limit {
		return []string{body}
	}
	var pieces []string
	var current []string
	var fence *fenceState
	size, consumed := 0, 0
	flush := func(reopen bool) {
		if len(current) > 0 {
			text := strings.Trim(strings.Join(current, "\n"), "\n")
			if fence != nil && reopen {
				text += "\n" + strings.Repeat(string(fence.char), fence.length)
			}
			pieces = append(pieces, text)
		}
		current, size = nil, 0
		if fence != nil && reopen {
			current = append(current, fence.line)
			size = runeLen(fence.line) + 1
		}
	}
	for _, line := range splitLines(body) {
		ll := runeLen(line)
		tailLeft := total-consumed > limit/4
		consumed += ll + 1
		wasOpen := fence != nil
		var isDelim bool
		fence, isDelim = advanceCodeFence(line, fence)
		if isDelim && wasOpen && fence == nil {
			current = append(current, line)
			size += ll + 1
			continue
		}
		if fence == nil && strings.TrimSpace(line) == "" && size >= limit/2 && tailLeft {
			flush(false)
			continue
		}
		if size+ll+1 > limit && len(current) > 0 && tailLeft {
			flush(fence != nil && !isDelim)
		}
		current = append(current, line)
		size += ll + 1
	}
	flush(false)
	var out []string
	for _, p := range pieces {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{body}
	}
	return out
}

// PageChunk is one chunk and its breadcrumb.
type PageChunk struct {
	Breadcrumb string
	Text       string
}

// ChunkPairs returns the chunks of text with their breadcrumbs.
func (c *PageMarkdownChunking) ChunkPairs(text string) []PageChunk {
	size := c.ChunkSize
	if size <= 0 {
		size = DefaultPageChunkSize
	}
	level := c.SplitLevel
	if level <= 0 {
		level = 2
	}
	sections := pageSections(text, level)
	pageTitle := ""
	for _, s := range sections {
		if len(s.trail) > 0 {
			pageTitle = s.trail[0]
			break
		}
	}
	intro := PageIntro(text, 300)
	context := ""
	if intro != "" {
		if pageTitle == "" || strings.HasPrefix(intro, pageTitle) {
			context = intro
		} else {
			context = pageTitle + ": " + intro
		}
	}
	var out []PageChunk
	for _, s := range sections {
		crumbs := s.trail
		if pageTitle != "" {
			crumbs = []string{pageTitle}
			for _, t := range s.trail {
				if t != pageTitle {
					crumbs = append(crumbs, t)
				}
			}
		}
		var nonEmpty []string
		for _, cr := range crumbs {
			if cr != "" {
				nonEmpty = append(nonEmpty, cr)
			}
		}
		breadcrumb := strings.Join(nonEmpty, " › ")
		for _, piece := range splitLong(s.body, size) {
			chunk := piece
			if breadcrumb != "" && !strings.HasPrefix(piece, breadcrumb) {
				chunk = breadcrumb + "\n\n" + piece
			}
			if context != "" {
				chunk = context + "\n\n" + chunk
			}
			out = append(out, PageChunk{Breadcrumb: breadcrumb, Text: chunk})
		}
	}
	return out
}

// ChunkTexts returns only the chunk texts.
func (c *PageMarkdownChunking) ChunkTexts(text string) []string {
	pairs := c.ChunkPairs(text)
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.Text
	}
	return out
}

// Chunk implements document.ChunkingStrategy.
func (c *PageMarkdownChunking) Chunk(doc *document.Document) ([]*document.Document, error) {
	path := ""
	if v, ok := doc.Metadata["path"].(string); ok && v != "" {
		path = v
	} else {
		path = doc.Name
	}
	pairs := c.ChunkPairs(doc.Content)
	out := make([]*document.Document, 0, len(pairs))
	for i, p := range pairs {
		index := i + 1
		ch := doc.Clone()
		ch.Content = p.Text
		ch.ChunkIndex = i
		ch.ChunkTotal = len(pairs)
		ch.ParentID = doc.ID
		ch.ID = generateChunkID(doc, index, p.Text)
		if ch.Metadata == nil {
			ch.Metadata = map[string]interface{}{}
		}
		ch.Metadata["chunk"] = index
		ch.Metadata["chunk_size"] = runeLen(p.Text)
		ch.Metadata["breadcrumb"] = p.Breadcrumb
		if path != "" {
			ch.Metadata["section"] = strings.Split(strings.Trim(path, "/"), "/")[0]
			ch.Metadata["kind"] = PageKind(path)
		}
		out = append(out, ch)
	}
	return out, nil
}

// generateChunkID mirrors ChunkingStrategy._generate_chunk_id: "<id>_<n>",
// else "<name>_<n>", else "chunk_<md5[:12]>_<n>".
func generateChunkID(doc *document.Document, n int, content string) string {
	switch {
	case doc.ID != "":
		return fmt.Sprintf("%s_%d", doc.ID, n)
	case doc.Name != "":
		return fmt.Sprintf("%s_%d", doc.Name, n)
	default:
		src := content
		if src == "" {
			src = doc.Content
		}
		sum := md5.Sum([]byte(src))
		return fmt.Sprintf("chunk_%s_%d", hex.EncodeToString(sum[:])[:12], n)
	}
}

var _ document.ChunkingStrategy = (*PageMarkdownChunking)(nil)
