package fs

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// LocalBackend stores files on disk at root/<namespace-dir>/<path>, where the
// namespace's slashes are percent-encoded into one directory component so a
// child namespace never nests inside its parent. A user partition appends
// "%%<encoded user>" to that component.
type LocalBackend struct {
	Root string
}

// NewLocalBackend creates a disk backend rooted at root (created on demand).
func NewLocalBackend(root string) *LocalBackend {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &LocalBackend{Root: abs}
}

const userIDSafe = "abcdefghijklmnopqrstuvwxyz0123456789-_"

func encodeUserID(u string) string {
	var b strings.Builder
	for _, c := range []byte(u) {
		if c < 0x80 && strings.IndexByte(userIDSafe, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02x", c)
		}
	}
	return b.String()
}

func encodePartition(ns, user string) string {
	enc := strings.ReplaceAll(ns, "/", "%2f")
	if user == "" {
		return enc
	}
	return enc + "%%" + encodeUserID(user)
}

func (l *LocalBackend) safeJoin(rel string) (string, error) {
	if strings.Contains(rel, "\x00") {
		return "", &InvalidPathError{Msg: fmt.Sprintf("invalid path %q: not representable on this backend", rel)}
	}
	target := filepath.Join(l.Root, filepath.FromSlash(rel))
	r, err := filepath.Rel(l.Root, target)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) || filepath.ToSlash(r) != rel {
		return "", &InvalidPathError{Msg: fmt.Sprintf("invalid path %q: not representable on this backend. Use plain relative paths like notes/topic.md.", rel)}
	}
	return target, nil
}

func (l *LocalBackend) target(ns, path, user string) (string, error) {
	return l.safeJoin(encodePartition(ns, user) + "/" + path)
}

func (l *LocalBackend) metaOf(path, user string, info os.FileInfo) FileMeta {
	return FileMeta{Path: path, SizeBytes: int(info.Size()), UpdatedAt: int64Ptr(info.ModTime().Unix()), UserID: user}
}

// Read implements Backend.
func (l *LocalBackend) Read(ns, path, user string) (string, bool, error) {
	t, err := l.target(ns, path, user)
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(t)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || isDirErr(t) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
}

func isDirErr(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func unsupported(op string) error {
	return &UnsupportedOperationError{Operation: op, Backend: "LocalBackend",
		Msg: "LocalBackend does not version files, expected_version is unsupported"}
}

// Write implements Backend (atomic temp-file + rename). CAS is unsupported.
func (l *LocalBackend) Write(ns, path, content, user string, expected *int) (FileMeta, error) {
	if expected != nil {
		return FileMeta{}, unsupported("write")
	}
	t, err := l.target(ns, path, user)
	if err != nil {
		return FileMeta{}, err
	}
	if err := os.MkdirAll(filepath.Dir(t), 0o700); err != nil {
		return FileMeta{}, existsAsFile(err, path)
	}
	tmp, err := os.CreateTemp(filepath.Dir(t), "."+filepath.Base(t)+".*.tmp")
	if err != nil {
		return FileMeta{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return FileMeta{}, err
	}
	if err := tmp.Close(); err != nil {
		return FileMeta{}, err
	}
	if err := os.Rename(tmpName, t); err != nil {
		return FileMeta{}, err
	}
	info, err := os.Stat(t)
	if err != nil {
		return FileMeta{}, err
	}
	return l.metaOf(path, user, info), nil
}

func existsAsFile(err error, path string) error {
	if errors.Is(err, fs.ErrExist) || errors.Is(err, os.ErrExist) || strings.Contains(err.Error(), "not a directory") {
		return fmt.Errorf("%w: a parent path segment of %s is an existing file", ErrExists, path)
	}
	return err
}

// List implements Backend.
func (l *LocalBackend) List(ns, directory, user string) ([]FileMeta, error) {
	root, err := l.safeJoin(encodePartition(ns, user))
	if err != nil {
		return nil, err
	}
	if !isDirErr(root) {
		return nil, nil
	}
	var out []FileMeta
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") && strings.HasSuffix(d.Name(), ".tmp") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !PathInDirectory(rel, directory) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, l.metaOf(rel, user, info))
		return nil
	})
	return out, err
}

// Delete implements Backend.
func (l *LocalBackend) Delete(ns, path, user string, expected *int) (bool, error) {
	if expected != nil {
		return false, unsupported("delete")
	}
	t, err := l.target(ns, path, user)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(t)
	if err != nil || info.IsDir() {
		return false, nil
	}
	return true, os.Remove(t)
}

// Stat implements Stater.
func (l *LocalBackend) Stat(ns, path, user string) (*FileMeta, error) {
	t, err := l.target(ns, path, user)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(t)
	if err != nil || info.IsDir() {
		return nil, nil
	}
	m := l.metaOf(path, user, info)
	return &m, nil
}

// Append implements Appender (O_APPEND, file created 0600).
func (l *LocalBackend) Append(ns, path, chunk, user string, maxFileBytes int) (FileMeta, error) {
	t, err := l.target(ns, path, user)
	if err != nil {
		return FileMeta{}, err
	}
	sep := ""
	size := len(chunk)
	if existing, err := os.ReadFile(t); err == nil {
		if len(existing) > 0 && existing[len(existing)-1] != '\n' {
			sep = "\n"
		}
		size += len(existing) + len(sep)
	}
	if maxFileBytes > 0 && size > maxFileBytes {
		return FileMeta{}, fileQuota(path, size, maxFileBytes)
	}
	if err := os.MkdirAll(filepath.Dir(t), 0o700); err != nil {
		return FileMeta{}, existsAsFile(err, path)
	}
	f, err := os.OpenFile(t, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return FileMeta{}, err
	}
	if _, err := f.WriteString(sep + chunk); err != nil {
		f.Close()
		return FileMeta{}, err
	}
	if err := f.Close(); err != nil {
		return FileMeta{}, err
	}
	info, err := os.Stat(t)
	if err != nil {
		return FileMeta{}, err
	}
	return l.metaOf(path, user, info), nil
}

// Move implements Mover (os.Rename).
func (l *LocalBackend) Move(ns, src, dst, user string, overwrite bool) (FileMeta, error) {
	s, err := l.target(ns, src, user)
	if err != nil {
		return FileMeta{}, err
	}
	d, err := l.target(ns, dst, user)
	if err != nil {
		return FileMeta{}, err
	}
	info, err := os.Stat(s)
	if err != nil || info.IsDir() {
		return FileMeta{}, wrapf(ErrNotFound, src)
	}
	if s == d {
		return l.metaOf(dst, user, info), nil
	}
	if _, err := os.Stat(d); err == nil && !overwrite {
		return FileMeta{}, wrapf(ErrExists, dst)
	}
	if err := os.MkdirAll(filepath.Dir(d), 0o700); err != nil {
		return FileMeta{}, existsAsFile(err, dst)
	}
	if err := os.Rename(s, d); err != nil {
		return FileMeta{}, err
	}
	info, err = os.Stat(d)
	if err != nil {
		return FileMeta{}, err
	}
	return l.metaOf(dst, user, info), nil
}

// Partitions implements Partitioner.
func (l *LocalBackend) Partitions(ns string) ([]string, error) {
	prefix := encodePartition(ns, "") + "%%"
	entries, err := os.ReadDir(l.Root)
	if err != nil {
		return nil, nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) && len(e.Name()) > len(prefix) {
			if u, err := url.PathUnescape(e.Name()[len(prefix):]); err == nil {
				out = append(out, u)
			}
		}
	}
	return out, nil
}
