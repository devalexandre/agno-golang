package fs

import (
	"sort"
	"strings"
)

// Backend stores text files keyed by (namespace, userID, path). userID ""
// is the shared partition. Paths and namespaces arrive normalized.
//
// Only the four core methods are required; the optional interfaces below let
// a backend provide native versions, otherwise FileSystem emulates them.
type Backend interface {
	// Read returns the content and true, or "", false when missing.
	Read(namespace, path, userID string) (string, bool, error)
	// Write creates or replaces a file. expectedVersion requests an atomic CAS;
	// backends without versioning return *UnsupportedOperationError for it.
	Write(namespace, path, content, userID string, expectedVersion *int) (FileMeta, error)
	// List returns metadata for every file under directory (order unspecified).
	List(namespace, directory, userID string) ([]FileMeta, error)
	// Delete removes a file and reports whether it existed.
	Delete(namespace, path, userID string, expectedVersion *int) (bool, error)
}

// Appender is a native line-oriented append.
type Appender interface {
	Append(namespace, path, chunk, userID string, maxFileBytes int) (FileMeta, error)
}

// Mover is a native move/rename.
type Mover interface {
	Move(namespace, src, dst, userID string, overwrite bool) (FileMeta, error)
}

// Searcher is a native case-insensitive substring search.
type Searcher interface {
	Search(namespace, query, directory, userID string, limit int) ([]SearchMatch, error)
}

// UsageReporter reports namespace usage natively.
type UsageReporter interface {
	Usage(namespace, userID string) (NamespaceUsage, error)
}

// Partitioner lists the user partitions holding files in a namespace.
type Partitioner interface {
	Partitions(namespace string) ([]string, error)
}

// Stater returns one file's metadata without content.
type Stater interface {
	Stat(namespace, path, userID string) (*FileMeta, error)
}

// buildMatch builds a SearchMatch for the first case-insensitive occurrence.
func buildMatch(path string, size int, content, query string) *SearchMatch {
	lc, lq := strings.ToLower(content), strings.ToLower(query)
	idx := strings.Index(lc, lq)
	if idx < 0 {
		return nil
	}
	if len(lc) != len(content) {
		// Lowercasing changed byte lengths (rare scripts): locate in the
		// original text case-sensitively, else anchor the snippet at the start.
		idx = strings.Index(content, query)
		if idx < 0 {
			idx = 0
		}
	}
	const ctx = 200
	start := idx - ctx
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + ctx
	if end > len(content) {
		end = len(content)
	}
	for start > 0 && !utf8Start(content[start]) {
		start--
	}
	for end < len(content) && !utf8Start(content[end]) {
		end++
	}
	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet += "..."
	}
	return &SearchMatch{
		Path: path, SizeBytes: size, Snippet: snippet,
		Line:       strings.Count(content[:min(idx, len(content))], "\n") + 1,
		MatchCount: strings.Count(lc, lq),
	}
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

func stat(b Backend, namespace, path, userID string) (*FileMeta, error) {
	if s, ok := b.(Stater); ok {
		return s.Stat(namespace, path, userID)
	}
	parent := ""
	if i := strings.LastIndex(path, "/"); i >= 0 {
		parent = path[:i]
	}
	metas, err := b.List(namespace, parent, userID)
	if err != nil {
		return nil, err
	}
	for _, m := range metas {
		if m.Path == path {
			mm := m
			return &mm, nil
		}
	}
	return nil, nil
}

func usage(b Backend, namespace, userID string) (NamespaceUsage, error) {
	if u, ok := b.(UsageReporter); ok {
		return u.Usage(namespace, userID)
	}
	metas, err := b.List(namespace, "", userID)
	if err != nil {
		return NamespaceUsage{}, err
	}
	u := NamespaceUsage{FileCount: len(metas)}
	for _, m := range metas {
		u.TotalBytes += m.SizeBytes
	}
	return u, nil
}

func appendChunk(b Backend, namespace, path, chunk, userID string, maxFileBytes int) (FileMeta, error) {
	if a, ok := b.(Appender); ok {
		return a.Append(namespace, path, chunk, userID, maxFileBytes)
	}
	existing, ok, err := b.Read(namespace, path, userID)
	if err != nil {
		return FileMeta{}, err
	}
	newContent := chunk
	if ok {
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			newContent = existing + "\n" + chunk
		} else {
			newContent = existing + chunk
		}
	}
	if maxFileBytes > 0 && len(newContent) > maxFileBytes {
		return FileMeta{}, fileQuota(path, len(newContent), maxFileBytes)
	}
	return b.Write(namespace, path, newContent, userID, nil)
}

func fileQuota(path string, size, limit int) error {
	return &QuotaExceededError{Msg: sprintf("%s would be %d bytes (limit %d per file)", path, size, limit), Scope: "file", Current: size, Limit: limit}
}

func move(b Backend, namespace, src, dst, userID string, overwrite bool) (FileMeta, error) {
	if m, ok := b.(Mover); ok {
		return m.Move(namespace, src, dst, userID, overwrite)
	}
	content, ok, err := b.Read(namespace, src, userID)
	if err != nil {
		return FileMeta{}, err
	}
	if !ok {
		return FileMeta{}, wrapf(ErrNotFound, src)
	}
	if src == dst {
		return b.Write(namespace, dst, content, userID, nil)
	}
	if !overwrite {
		if _, exists, err := b.Read(namespace, dst, userID); err != nil {
			return FileMeta{}, err
		} else if exists {
			return FileMeta{}, wrapf(ErrExists, dst)
		}
	}
	meta, err := b.Write(namespace, dst, content, userID, nil)
	if err != nil {
		return FileMeta{}, err
	}
	_, err = b.Delete(namespace, src, userID, nil)
	return meta, err
}

func search(b Backend, namespace, query, directory, userID string, limit int) ([]SearchMatch, error) {
	if query == "" {
		return nil, nil
	}
	if s, ok := b.(Searcher); ok {
		return s.Search(namespace, query, directory, userID, limit)
	}
	metas, err := b.List(namespace, directory, userID)
	if err != nil {
		return nil, err
	}
	sort.Slice(metas, func(i, j int) bool { return pathLess(metas[i].Path, metas[j].Path) })
	var out []SearchMatch
	for _, m := range metas {
		if len(out) >= limit {
			break
		}
		content, ok, err := b.Read(namespace, m.Path, userID)
		if err != nil || !ok {
			continue
		}
		if sm := buildMatch(m.Path, m.SizeBytes, content, query); sm != nil {
			out = append(out, *sm)
		}
	}
	return out, nil
}

func contains(b Backend, namespace string, lines []string, directory, userID string) (map[string]bool, error) {
	remaining := map[string]bool{}
	for _, l := range lines {
		remaining[l] = true
	}
	found := map[string]bool{}
	metas, err := b.List(namespace, directory, userID)
	if err != nil {
		return nil, err
	}
	for _, m := range metas {
		if len(remaining) == 0 {
			break
		}
		content, ok, err := b.Read(namespace, m.Path, userID)
		if err != nil || !ok {
			continue
		}
		for _, l := range strings.Split(content, "\n") {
			if remaining[l] {
				found[l] = true
				delete(remaining, l)
			}
		}
	}
	return found, nil
}
