package airflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAirflowSaveRead(t *testing.T) {
	dir := t.TempDir()
	a := New(WithDagsDir(dir))
	out, _ := a.SaveDagFile(SaveDagFileParams{Contents: "from airflow import DAG\n", DagFile: "sub/hello.py"})
	path := out.(string)
	if !strings.HasSuffix(path, filepath.Join("sub", "hello.py")) {
		t.Fatalf("bad path: %s", path)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "from airflow import DAG\n" {
		t.Fatalf("file not written: %v", err)
	}
	out, _ = a.ReadDagFile(ReadDagFileParams{DagFile: "sub/hello.py"})
	if out != "from airflow import DAG\n" {
		t.Fatalf("bad read: %v", out)
	}
	if len(a.GetMethods()) != 2 || len(New(WithEnableReadDagFile(false)).GetMethods()) != 1 {
		t.Fatal("bad registration")
	}
}

func TestAirflowPathSafety(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	a := New(WithDagsDir(dir))
	for _, bad := range []string{"../escape.py", "/etc/passwd", "a/../../x.py", "", "bad\x00.py", "CON"} {
		out, _ := a.SaveDagFile(SaveDagFileParams{Contents: "x", DagFile: bad})
		if !strings.HasPrefix(out.(string), "Error saving to file:") {
			t.Fatalf("%q accepted: %v", bad, out)
		}
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err == nil {
		out, _ := a.SaveDagFile(SaveDagFileParams{Contents: "x", DagFile: "link/evil.py"})
		if !strings.HasPrefix(out.(string), "Error saving to file:") {
			t.Fatalf("symlink escape accepted: %v", out)
		}
	}
	out, _ := a.ReadDagFile(ReadDagFileParams{DagFile: "missing.py"})
	if !strings.HasPrefix(out.(string), "Error reading file:") {
		t.Fatalf("bad missing read: %v", out)
	}
}
