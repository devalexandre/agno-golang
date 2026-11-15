package fs

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func backends(t *testing.T) map[string]Backend {
	return map[string]Backend{"memory": NewMemoryBackend(), "local": NewLocalBackend(t.TempDir())}
}

func TestNormalizePath(t *testing.T) {
	good := map[string]string{"/a//b.md": "a/b.md", "notes/x.md": "notes/x.md"}
	for in, want := range good {
		got, err := NormalizePath(in)
		if err != nil || got != want {
			t.Fatalf("%q -> %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "a/../b", "a\\b", "dir/", "a\x01b", strings.Repeat("x/", 20) + "f"} {
		if _, err := NormalizePath(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
		var ipe *InvalidPathError
		if _, err := NormalizePath(bad); !errors.As(err, &ipe) || !errors.Is(err, ErrFileSystem) {
			t.Fatalf("wrong error type for %q", bad)
		}
	}
}

func TestNamespaces(t *testing.T) {
	ns, err := NormalizeNamespace("Research/Team A")
	if err != nil || ns != "research/team%20a" {
		t.Fatalf("got %q %v", ns, err)
	}
	if _, err := NormalizeNamespace("x/{user_id}{team_id}"); err == nil {
		t.Fatal("adjacent placeholders must fail")
	}
	if _, err := NormalizeNamespace("x/{bogus}"); err == nil {
		t.Fatal("unknown placeholder must fail")
	}
	tpl, _ := NormalizeNamespace("Radar/{user_id}")
	if tpl != "radar/{user_id}" {
		t.Fatalf("got %q", tpl)
	}
}

func TestFileSystemOps(t *testing.T) {
	for name, b := range backends(t) {
		t.Run(name, func(t *testing.T) {
			f := MustNew(b, WithNamespace("notes"))
			if _, err := f.Write("a/one.md", "line1\nline2\nline3\n"); err != nil {
				t.Fatal(err)
			}
			got, ok, _ := f.Read("/a/one.md")
			if !ok || got != "line1\nline2\nline3\n" {
				t.Fatalf("read %q %v", got, ok)
			}
			if _, err := f.Write("a/one.md", "x", WriteOptions{NoOverwrite: true}); !errors.Is(err, ErrExists) {
				t.Fatalf("expected ErrExists, got %v", err)
			}
			if _, err := f.ReplaceLines("a/one.md", 2, 2, "LINE2"); err != nil {
				t.Fatal(err)
			}
			got, _, _ = f.Read("a/one.md")
			if got != "line1\nLINE2\nline3\n" {
				t.Fatalf("replace got %q", got)
			}
			if _, err := f.Append("seen.txt", "u1\nu2\n", false); err != nil {
				t.Fatal(err)
			}
			if _, err := f.Append("seen.txt", "u2\nu3\nu3", true); err != nil {
				t.Fatal(err)
			}
			got, _, _ = f.Read("seen.txt")
			if got != "u1\nu2\nu3\n" {
				t.Fatalf("append got %q", got)
			}
			res, err := f.Contains([]string{"u3", "nope", "u1\n"}, "")
			if err != nil || strings.Join(res.Found, ",") != "u3,u1" || strings.Join(res.Missing, ",") != "nope" {
				t.Fatalf("contains %+v %v", res, err)
			}
			matches, _ := f.Search("line2", "", 10)
			if len(matches) != 1 || matches[0].Line != 2 || matches[0].Path != "a/one.md" {
				t.Fatalf("search %+v", matches)
			}
			if _, err := f.Move("a/one.md", "archive/one.md", false); err != nil {
				t.Fatal(err)
			}
			if _, err := f.Move("missing.md", "x.md", false); !errors.Is(err, ErrNotFound) {
				t.Fatalf("expected ErrNotFound, got %v", err)
			}
			list, _ := f.List("")
			if len(list) != 2 || list[0].Path != "archive/one.md" {
				t.Fatalf("list %+v", list)
			}
			ok, _ = f.Delete("seen.txt", nil)
			if !ok {
				t.Fatal("delete")
			}
			// Isolation by namespace.
			other := MustNew(b, WithNamespace("other"))
			if l, _ := other.List(""); len(l) != 0 {
				t.Fatal("namespaces must be isolated")
			}
		})
	}
}

func TestQuotas(t *testing.T) {
	f := MustNew(NewMemoryBackend(), WithMaxFileBytes(10), WithMaxNamespaceBytes(15))
	var q *QuotaExceededError
	if _, err := f.Write("a", strings.Repeat("x", 11)); !errors.As(err, &q) || q.Scope != "file" {
		t.Fatalf("expected file quota, got %v", err)
	}
	if _, err := f.Write("a", strings.Repeat("x", 10)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write("b", strings.Repeat("x", 10)); !errors.As(err, &q) || q.Scope != "namespace" {
		t.Fatalf("expected namespace quota, got %v", err)
	}
}

func TestTemplatesAndPartitions(t *testing.T) {
	b := NewMemoryBackend()
	f := MustNew(b, WithNamespace("radar/{user_id}"))
	if _, err := f.Write("x.md", "hi"); err == nil {
		t.Fatal("unresolved template must fail closed")
	}
	alice, err := f.ResolveForRun("Alice", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if alice.Namespace() != "radar/%41lice" && alice.Namespace() != "radar/Alice" {
		t.Logf("namespace %s", alice.Namespace())
	}
	alice.Write("x.md", "hi")
	bob, _ := f.ResolveForRun("bob", "", "")
	if _, ok, _ := bob.Read("x.md"); ok {
		t.Fatal("users must be isolated")
	}

	scoped := MustNew(b, WithNamespace("shared"), WithUserScoped(true))
	if _, err := scoped.ResolveForRun("", "", ""); err == nil {
		t.Fatal("user scoped store must refuse anonymous run")
	}
	u1, _ := scoped.ResolveForRun("u1", "", "")
	u1.Write("n.md", "u1 data")
	u2, _ := scoped.ResolveForRun("u2", "", "")
	if _, ok, _ := u2.Read("n.md"); ok {
		t.Fatal("partitions must be isolated")
	}
	parts, _ := scoped.Partitions()
	if len(parts) != 1 || parts[0] != "u1" {
		t.Fatalf("partitions %v", parts)
	}
}

func TestLocalRejectsEscape(t *testing.T) {
	l := NewLocalBackend(t.TempDir())
	if _, err := l.Write("ns", "../../etc/passwd", "x", "", nil); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
	if _, err := l.Write("ns", "a.md", "x", "", intPtr(1)); err == nil {
		t.Fatal("local backend must refuse CAS")
	}
}

func TestMemoryCAS(t *testing.T) {
	f := MustNew(NewMemoryBackend())
	m, _ := f.Write("a", "1")
	if _, err := f.Write("a", "2", WriteOptions{ExpectedVersion: intPtr(*m.Version + 5)}); err == nil {
		t.Fatal("expected version conflict")
	}
	if _, err := f.Write("a", "2", WriteOptions{ExpectedVersion: m.Version}); err != nil {
		t.Fatal(err)
	}
}

func exec(t *testing.T, tools *Tools, name string, args interface{}) string {
	t.Helper()
	data, _ := json.Marshal(args)
	out, err := tools.Execute(tools.GetName()+"_"+name, data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out.(string)
}

func TestToolkit(t *testing.T) {
	f := MustNew(NewMemoryBackend(), WithAllowDelete())
	tools := f.Tools()
	if _, ok := tools.GetMethods()["filesystem_delete_file"]; !ok {
		t.Fatal("delete_file should be registered with AllowDelete")
	}
	if !strings.HasPrefix(exec(t, tools, "write_file", map[string]interface{}{"path": "notes/a.md", "content": "alpha\nbeta\n"}), "Wrote 11 bytes") {
		t.Fatal("write")
	}
	if out := exec(t, tools, "write_file", map[string]interface{}{"path": "notes/a.md", "content": "x", "overwrite": false}); !strings.Contains(out, "overwrite=False") {
		t.Fatalf("no-overwrite: %s", out)
	}
	if out := exec(t, tools, "read_file", map[string]interface{}{"path": "notes/a.md", "start_line": 2}); out != "     2\tbeta" {
		t.Fatalf("read %q", out)
	}
	if out := exec(t, tools, "read_file", map[string]interface{}{"path": "notes/a.md", "start_line": 9}); !strings.Contains(out, "past the end") {
		t.Fatalf("range %q", out)
	}
	out := exec(t, tools, "list_files", map[string]interface{}{"recursive": true})
	if !strings.Contains(out, `"notes/a.md"`) || !strings.Contains(out, `"type": "dir"`) {
		t.Fatalf("list %s", out)
	}
	if out := exec(t, tools, "search_content", map[string]interface{}{"query": "BETA"}); !strings.Contains(out, `"line": 2`) {
		t.Fatalf("search %s", out)
	}
	if out := exec(t, tools, "append_file", map[string]interface{}{"path": "seen", "content": "a\nb", "unique": true}); !strings.HasPrefix(out, "Appended") {
		t.Fatalf("append %s", out)
	}
	if out := exec(t, tools, "delete_file", map[string]interface{}{"path": "nope"}); !strings.HasPrefix(out, "Error") {
		t.Fatalf("delete %s", out)
	}

	ro := MustNew(f.Backend(), WithReadOnly()).Tools()
	if len(ro.GetMethods()) != 3 || ro.CanWrite() {
		t.Fatalf("read only toolkit should have 3 read tools, got %d", len(ro.GetMethods()))
	}
	if out := exec(t, ro, "read_file", map[string]interface{}{"path": "notes/a.md"}); !strings.Contains(out, "alpha") {
		t.Fatalf("ro read %s", out)
	}
}
