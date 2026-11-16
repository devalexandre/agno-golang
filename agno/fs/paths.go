package fs

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Path grammar limits.
const (
	MaxPathChars    = 512
	MaxSegments     = 16
	MaxSegmentChars = 128
	MaxCheckLines   = 200
)

// TemplatePlaceholders are the placeholders a namespace may embed.
var TemplatePlaceholders = []string{"user_id", "agent_id", "team_id"}

var placeholderRe = regexp.MustCompile(`\{([^{}]*)\}`)

// namespaceSafe characters stay literal in a namespace; the rest are percent-encoded.
const namespaceSafe = "abcdefghijklmnopqrstuvwxyz0123456789.-_/@+"

// NormalizePath validates and canonicalizes a file path (no leading slash).
func NormalizePath(path string) (string, error) {
	original := path
	value := norm.NFC.String(path)
	for _, r := range value {
		if unicode.Is(unicode.Cc, r) {
			return "", invalidPath(original, "control characters are not allowed")
		}
	}
	if strings.Contains(value, `\`) {
		return "", invalidPath(original, "backslashes are not allowed, use forward slashes")
	}
	for strings.Contains(value, "//") {
		value = strings.ReplaceAll(value, "//", "/")
	}
	value = strings.TrimPrefix(value, "/")
	if strings.HasSuffix(value, "/") {
		return "", invalidPath(original, "trailing slashes are not allowed, paths address files")
	}
	if value == "" {
		return "", invalidPath(original, "path is empty")
	}
	if utf8.RuneCountInString(value) > MaxPathChars {
		return "", invalidPath(original, fmt.Sprintf("path is longer than %d characters", MaxPathChars))
	}
	segments := strings.Split(value, "/")
	if len(segments) > MaxSegments {
		return "", invalidPath(original, fmt.Sprintf("path has more than %d segments", MaxSegments))
	}
	for _, s := range segments {
		if s == "." || s == ".." {
			return "", invalidPath(original, "segments '.' and '..' are not allowed")
		}
		if utf8.RuneCountInString(s) > MaxSegmentChars {
			return "", invalidPath(original, fmt.Sprintf("a segment is longer than %d characters", MaxSegmentChars))
		}
	}
	return value, nil
}

// ParseNamespaceTemplate returns the placeholders embedded in a namespace name.
func ParseNamespaceTemplate(name string) ([]string, error) {
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(name, -1) {
		ok := false
		for _, p := range TemplatePlaceholders {
			if m[1] == p {
				ok = true
			}
		}
		if !ok {
			return nil, invalidPath(name, fmt.Sprintf("unknown placeholder {%s}, valid placeholders are {user_id}, {agent_id}, {team_id}", m[1]))
		}
		out = append(out, m[1])
	}
	rest := placeholderRe.ReplaceAllString(name, "")
	if strings.ContainsAny(rest, "{}") {
		return nil, invalidPath(name, "braces are reserved in namespace names")
	}
	if strings.Contains(name, "}{") {
		return nil, invalidPath(name, "placeholders must be separated (e.g. '{user_id}/{team_id}'), because adjacent placeholders cannot be resolved back to distinct values")
	}
	return out, nil
}

func quoteNamespaceValue(value string) string {
	var b strings.Builder
	for _, c := range []byte(value) {
		if c < 0x80 && strings.IndexByte(namespaceSafe, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02x", c)
		}
	}
	return b.String()
}

// SanitizeNamespaceSegment lowercases and percent-encodes a namespace to URL-safe ASCII.
func SanitizeNamespaceSegment(value string) string {
	return quoteNamespaceValue(strings.ToLower(norm.NFC.String(value)))
}

// NormalizeNamespace validates and canonicalizes a namespace (placeholders kept).
func NormalizeNamespace(name string) (string, error) {
	value := norm.NFC.String(name)
	placeholders, err := ParseNamespaceTemplate(value)
	if err != nil {
		return "", err
	}
	if len(placeholders) == 0 {
		p, err := NormalizePath(value)
		if err != nil {
			return "", err
		}
		return SanitizeNamespaceSegment(p), nil
	}
	standIn := placeholderRe.ReplaceAllString(value, "x")
	if _, err := NormalizePath(standIn); err != nil {
		return "", err
	}
	var b strings.Builder
	cursor := 0
	for _, loc := range placeholderRe.FindAllStringIndex(value, -1) {
		b.WriteString(SanitizeNamespaceSegment(value[cursor:loc[0]]))
		b.WriteString(value[loc[0]:loc[1]])
		cursor = loc[1]
	}
	b.WriteString(SanitizeNamespaceSegment(value[cursor:]))
	return b.String(), nil
}

// NormalizeTemplateValue validates one template value and encodes it case-preservingly.
func NormalizeTemplateValue(placeholder, value string) (string, error) {
	if value == "" {
		return "", &InvalidPathError{Msg: fmt.Sprintf("invalid %s value %q: must be a non-empty string", placeholder, value)}
	}
	n := norm.NFC.String(value)
	if strings.Contains(n, "/") || strings.ContainsAny(n, "{}") {
		return "", &InvalidPathError{Msg: fmt.Sprintf("invalid %s value %q: must be a single path segment without braces", placeholder, value)}
	}
	p, err := NormalizePath(n)
	if err != nil || p != n {
		return "", &InvalidPathError{Msg: fmt.Sprintf("invalid %s value %q: must be a single valid path segment", placeholder, value)}
	}
	return quoteNamespaceValue(n), nil
}

// NormalizeDirectory validates a directory parameter; "" and "." mean the root.
func NormalizeDirectory(directory string) (string, error) {
	if directory == "" || directory == "." {
		return "", nil
	}
	return NormalizePath(directory)
}

// NormalizeLine strips trailing line terminators; interior newlines are rejected.
func NormalizeLine(line string) (string, error) {
	v := strings.TrimRight(line, "\r\n")
	if strings.ContainsAny(v, "\r\n") {
		return "", &InvalidPathError{Msg: fmt.Sprintf("invalid record %q: records must be single lines with no newlines.", line)}
	}
	return v, nil
}

// NormalizeCheckLines normalizes a check_lines batch: cap 200, strip, drop empties.
func NormalizeCheckLines(lines []string) ([]string, error) {
	if len(lines) > MaxCheckLines {
		return nil, &InvalidPathError{Msg: fmt.Sprintf("too many records (%d > %d). Check them in batches of %d or fewer.", len(lines), MaxCheckLines, MaxCheckLines)}
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		n, err := NormalizeLine(l)
		if err != nil {
			return nil, err
		}
		if n != "" {
			out = append(out, n)
		}
	}
	return out, nil
}

// BuildChunk builds the canonical append chunk: one record per line, no
// empty lines, trailing newline. Returns "" when nothing remains.
func BuildChunk(content string) string {
	var kept []string
	for _, piece := range strings.Split(content, "\n") {
		if l := strings.TrimRight(piece, "\r\n"); l != "" {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "\n") + "\n"
}

// PathInDirectory applies directory semantics at segment boundaries.
func PathInDirectory(path, directory string) bool {
	return directory == "" || path == directory || strings.HasPrefix(path, directory+"/")
}

// pathLess compares paths segment by segment.
func pathLess(a, b string) bool {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] != bs[i] {
			return as[i] < bs[i]
		}
	}
	return len(as) < len(bs)
}
