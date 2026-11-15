package localfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFileSystem(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLocalFileSystemTools(WithTargetDirectory(dir))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := l.WriteFile(WriteFileParams{Content: "héllo", Filename: "notes", Directory: "sub"})
	if !strings.HasSuffix(out.(string), filepath.Join("sub", "notes.txt")) {
		t.Fatal(out)
	}
	if got, _ := l.ReadFile(ReadFileParams{Filename: "notes.txt", Directory: "sub"}); got != "héllo" {
		t.Fatal(got)
	}
	out, _ = l.WriteFile(WriteFileParams{Content: "x", Filename: "data.md", Extension: ".json"})
	if !strings.HasSuffix(out.(string), "data.json") {
		t.Fatal(out)
	}
	out, _ = l.WriteFile(WriteFileParams{Content: "x"})
	if !strings.HasSuffix(out.(string), ".txt") {
		t.Fatal(out)
	}
	for _, bad := range []string{"../escape.txt", "/etc/passwd", "a/../../x"} {
		if out, _ := l.ReadFile(ReadFileParams{Filename: bad}); !strings.Contains(out.(string), "outside the allowed base directory") {
			t.Fatal(bad, out)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(l.TargetDirectory(), "link")); err == nil {
		if out, _ := l.WriteFile(WriteFileParams{Content: "x", Filename: "link/evil.txt"}); !strings.Contains(out.(string), "outside") {
			t.Fatal(out)
		}
	}
	if out, _ := l.ReadFile(ReadFileParams{Filename: "missing.txt"}); !strings.HasPrefix(out.(string), "File not found") {
		t.Fatal(out)
	}
}
