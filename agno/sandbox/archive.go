package sandbox

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ArchiveLimits bound checkpoint archives.
type ArchiveLimits struct {
	MaxBytes   int64 // total uncompressed file bytes (default 1 GiB)
	MaxEntries int   // default 100000
}

func (l ArchiveLimits) withDefaults() ArchiveLimits {
	if l.MaxBytes <= 0 {
		l.MaxBytes = 1 << 30
	}
	if l.MaxEntries <= 0 {
		l.MaxEntries = 100_000
	}
	return l
}

// cleanName validates an archive entry name: relative, no "..", no NUL.
func cleanName(name string) (string, error) {
	name = strings.TrimPrefix(name, "./")
	if name == "" || name == "." {
		return "", nil
	}
	if strings.ContainsRune(name, 0) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("unsafe archive entry %q", name)
	}
	c := path.Clean(name)
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", fmt.Errorf("unsafe archive entry %q", name)
	}
	return c, nil
}

// safeLink reports whether a symlink at entry pointing to target stays inside the root.
func safeLink(entry, target string) bool {
	if target == "" || path.IsAbs(target) {
		return false
	}
	j := path.Clean(path.Join(path.Dir(entry), target))
	return j != ".." && !strings.HasPrefix(j, "../")
}

// CaptureDir writes dir as a gzip-compressed tar to w.
func CaptureDir(dir string, w io.Writer, limits ArchiveLimits) error {
	limits = limits.withDefaults()
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	var total int64
	entries := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entries++; entries > limits.MaxEntries {
			return errors.New("checkpoint exceeds its entry limit")
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			if link, err = os.Readlink(p); err != nil {
				return err
			}
			if !safeLink(rel, filepath.ToSlash(link)) {
				return nil // links escaping the workspace are not captured
			}
		} else if !info.Mode().IsRegular() && !info.IsDir() {
			return nil // sockets, devices, fifos
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = rel
		hdr.Uname, hdr.Gname, hdr.Uid, hdr.Gid = "", "", 0, 0
		if info.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			if total += info.Size(); total > limits.MaxBytes {
				return errors.New("checkpoint exceeds its size limit")
			}
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			_, err = io.CopyN(tw, f, info.Size())
			f.Close()
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// RestoreDir replaces dir's contents with the gzip tar from r, rejecting
// unsafe entries (absolute paths, "..", escaping symlinks).
func RestoreDir(r io.Reader, dir string, limits ArchiveLimits) error {
	limits = limits.withDefaults()
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("checkpoint is not a gzip archive: %w", err)
	}
	defer gz.Close()
	// Extract into a sibling staging dir, then swap, so a bad archive leaves the workspace intact.
	stage, err := os.MkdirTemp(filepath.Dir(dir), ".restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := extractTar(tar.NewReader(gz), stage, limits); err != nil {
		return err
	}
	old, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, e := range old {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	staged, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	for _, e := range staged {
		if err := os.Rename(filepath.Join(stage, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func extractTar(tr *tar.Reader, dest string, limits ArchiveLimits) error {
	var total int64
	entries := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if entries++; entries > limits.MaxEntries {
			return errors.New("checkpoint exceeds its entry limit")
		}
		name, err := cleanName(hdr.Name)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		mode := os.FileMode(hdr.Mode).Perm()
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, mode|0o700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA: //nolint:staticcheck
			if total += hdr.Size; total > limits.MaxBytes {
				return errors.New("checkpoint exceeds its size limit")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o600)
			if err != nil {
				return err
			}
			_, err = io.CopyN(f, tr, hdr.Size)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			if !safeLink(name, hdr.Linkname) {
				return fmt.Errorf("unsafe symlink %q -> %q", name, hdr.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			// Hard links, devices and fifos are not restored.
		}
	}
}

// Repack converts a plain tar stream (e.g. from `docker cp container:/dir -`)
// into the canonical gzip checkpoint format, stripping strip leading path
// components and dropping unsafe entries.
func Repack(r io.Reader, w io.Writer, strip int, limits ArchiveLimits) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	if err := copyTar(tar.NewReader(r), tw, strip, limits); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// Unpack converts a canonical gzip checkpoint into a plain tar stream,
// dropping unsafe entries, ready for `docker cp - container:/dir`.
func Unpack(r io.Reader, w io.Writer, limits ArchiveLimits) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("checkpoint is not a gzip archive: %w", err)
	}
	defer gz.Close()
	tw := tar.NewWriter(w)
	if err := copyTar(tar.NewReader(gz), tw, 0, limits); err != nil {
		return err
	}
	return tw.Close()
}

func copyTar(tr *tar.Reader, tw *tar.Writer, strip int, limits ArchiveLimits) error {
	limits = limits.withDefaults()
	var total int64
	entries := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		parts := strings.Split(strings.TrimPrefix(hdr.Name, "./"), "/")
		if len(parts) <= strip {
			continue
		}
		name, err := cleanName(strings.Join(parts[strip:], "/"))
		if err != nil || name == "" {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir, tar.TypeReg, tar.TypeRegA: //nolint:staticcheck
		case tar.TypeSymlink:
			if !safeLink(name, hdr.Linkname) {
				continue
			}
		default:
			continue
		}
		if entries++; entries > limits.MaxEntries {
			return errors.New("checkpoint exceeds its entry limit")
		}
		if total += hdr.Size; total > limits.MaxBytes {
			return errors.New("checkpoint exceeds its size limit")
		}
		out := &tar.Header{Typeflag: hdr.Typeflag, Name: name, Linkname: hdr.Linkname, Size: hdr.Size,
			Mode: hdr.Mode, ModTime: hdr.ModTime, Format: tar.FormatPAX}
		if hdr.Typeflag == tar.TypeDir {
			out.Name += "/"
			out.Size = 0
		}
		if hdr.Typeflag == tar.TypeRegA { //nolint:staticcheck
			out.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(out); err != nil {
			return err
		}
		if out.Typeflag == tar.TypeReg {
			if _, err := io.CopyN(tw, tr, hdr.Size); err != nil {
				return err
			}
		}
	}
}
