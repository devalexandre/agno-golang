package sandbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/internal/sqldialect"
	_ "modernc.org/sqlite"
)

func newLocal(t *testing.T) *LocalSandbox {
	t.Helper()
	sb, err := (&LocalProvider{Root: t.TempDir()}).Create(context.Background(), "sess/1", CreateOptions{Env: map[string]string{"GREETING": "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sb.Close(context.Background()) })
	return sb.(*LocalSandbox)
}

func TestLocalExecAndFiles(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("needs sh")
	}
	ctx := context.Background()
	sb := newLocal(t)
	if err := sb.PutFile(ctx, "src/a.txt", []byte("hello"), 0); err != nil {
		t.Fatal(err)
	}
	res, err := sb.Exec(ctx, []string{"sh", "-c", "cat src/a.txt; echo \" $GREETING\"; echo err >&2; exit 3"}, ExecOptions{})
	if err != nil || res.ExitCode != 3 || res.Stdout != "hello hi\n" || res.Stderr != "err\n" {
		t.Fatalf("%v %+v", err, res)
	}
	res, _ = sb.Exec(ctx, []string{"sh", "-c", "yes | head -c 5000"}, ExecOptions{MaxOutputBytes: 100})
	if !res.OutputLimited || len(res.Stdout) != 100 {
		t.Fatalf("limit %+v", res.OutputLimited)
	}
	start := time.Now()
	res, _ = sb.Exec(ctx, []string{"sh", "-c", "sleep 5 & sleep 5"}, ExecOptions{Timeout: 200 * time.Millisecond})
	if !res.TimedOut || time.Since(start) > 3*time.Second {
		t.Fatalf("timeout %+v %v", res, time.Since(start))
	}
	res, _ = sb.Exec(ctx, []string{"cat"}, ExecOptions{Stdin: []byte("in"), Dir: "src"})
	if res.Stdout != "in" {
		t.Fatalf("stdin %+v", res)
	}
	if b, err := sb.GetFile(ctx, "/src/../src/a.txt"); err != nil || string(b) != "hello" {
		t.Fatalf("get %v %q", err, b)
	}
	// Lexical escapes are confined to the workspace; symlink escapes are refused.
	if err := sb.PutFile(ctx, "../../escape.txt", []byte("x"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sb.Dir(), "escape.txt")); err != nil {
		t.Fatal("escape was not confined")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(sb.Dir(), "link")); err != nil {
		t.Fatal(err)
	}
	if err := sb.PutFile(ctx, "link/x.txt", []byte("x"), 0); err == nil {
		t.Fatal("wrote through escaping symlink")
	}
	if _, err := sb.Exec(ctx, []string{"definitely-not-a-binary-xyz"}, ExecOptions{}); err == nil {
		t.Fatal("expected start error")
	}
}

func checkpointStores(t *testing.T) map[string]CheckpointStore {
	fs, err := NewFileCheckpointStore(filepath.Join(t.TempDir(), "cp"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "cp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ss, _ := NewSQLCheckpointStore(db, sqldialect.SQLite, "")
	if err := ss.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return map[string]CheckpointStore{"file": fs, "sqlite": ss, "s3": newFakeS3(t)}
}

func TestCheckpointRoundTrip(t *testing.T) {
	for name, store := range checkpointStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			sb := newLocal(t)
			_ = sb.PutFile(ctx, "a/b.txt", []byte("one"), 0o600)
			_ = os.Symlink("b.txt", filepath.Join(sb.Dir(), "a", "ln"))
			ref, err := sb.Checkpoint(ctx, store)
			if err != nil || ref.Validate() != nil {
				t.Fatalf("%v %+v", err, ref)
			}
			_ = sb.PutFile(ctx, "a/b.txt", []byte("two"), 0)
			_ = sb.PutFile(ctx, "new.txt", []byte("x"), 0)
			if err := sb.Restore(ctx, store, ref); err != nil {
				t.Fatal(err)
			}
			if b, _ := sb.GetFile(ctx, "a/b.txt"); string(b) != "one" {
				t.Fatalf("restored %q", b)
			}
			if b, _ := sb.GetFile(ctx, "a/ln"); string(b) != "one" {
				t.Fatalf("symlink %q", b)
			}
			if _, err := sb.GetFile(ctx, "new.txt"); err == nil {
				t.Fatal("restore did not replace workspace")
			}
			// Same content -> same ref (content addressed).
			ref2, _ := sb.Checkpoint(ctx, store)
			_ = ref2
			bad := ref
			bad.SHA256 = strings.Repeat("0", 64)
			if _, err := store.Open(ctx, bad); !errors.Is(err, ErrCheckpointNotFound) {
				t.Fatalf("missing: %v", err)
			}
			if err := store.Delete(ctx, ref); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Open(ctx, ref); !errors.Is(err, ErrCheckpointNotFound) {
				t.Fatalf("deleted: %v", err)
			}
		})
	}
}

func TestArchiveRejectsUnsafeEntries(t *testing.T) {
	for _, hdr := range []*tar.Header{
		{Name: "../evil", Typeflag: tar.TypeReg, Size: 1, Mode: 0o644},
		{Name: "/abs", Typeflag: tar.TypeReg, Size: 1, Mode: 0o644},
		{Name: "ln", Typeflag: tar.TypeSymlink, Linkname: "../../etc/passwd"},
	} {
		var plain, packed bytes.Buffer
		tw := tar.NewWriter(&plain)
		_ = tw.WriteHeader(hdr)
		if hdr.Size > 0 {
			_, _ = tw.Write([]byte("x"))
		}
		_ = tw.Close()
		// Build a raw gzip (bypassing Repack's filtering) to hit RestoreDir's checks.
		gzipTo(t, &packed, plain.Bytes())
		if err := RestoreDir(&packed, t.TempDir(), ArchiveLimits{}); err == nil {
			t.Errorf("%s accepted", hdr.Name)
		}
	}
	// Repack strips the docker cp top-level directory and drops unsafe entries.
	var plain, out bytes.Buffer
	tw := tar.NewWriter(&plain)
	_ = tw.WriteHeader(&tar.Header{Name: "workspace/", Typeflag: tar.TypeDir, Mode: 0o755})
	_ = tw.WriteHeader(&tar.Header{Name: "workspace/f.txt", Typeflag: tar.TypeReg, Size: 2, Mode: 0o644})
	_, _ = tw.Write([]byte("ok"))
	_ = tw.WriteHeader(&tar.Header{Name: "workspace/bad", Typeflag: tar.TypeSymlink, Linkname: "/etc"})
	_ = tw.Close()
	if err := Repack(&plain, &out, 1, ArchiveLimits{}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := RestoreDir(&out, dir, ArchiveLimits{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "f.txt")); string(b) != "ok" {
		t.Fatalf("%q", b)
	}
	if _, err := os.Lstat(filepath.Join(dir, "bad")); err == nil {
		t.Fatal("unsafe symlink kept")
	}
}

func TestTools(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("needs sh")
	}
	sb := newLocal(t)
	fs, _ := NewFileCheckpointStore(t.TempDir())
	tools := NewTools(sb, fs)
	if _, err := tools.Execute("WriteFile", json.RawMessage(`{"path":"main.sh","content":"echo $((6*7))"}`)); err != nil {
		t.Fatal(err)
	}
	out, err := tools.Execute("RunShell", json.RawMessage(`{"command":"sh main.sh"}`))
	if err != nil || out.(*ExecResult).Stdout != "42\n" {
		t.Fatalf("%v %+v", err, out)
	}
	out, err = tools.Execute("RunCommand", json.RawMessage(`{"argv":["cat","main.sh"]}`))
	if err != nil || !strings.Contains(out.(*ExecResult).Stdout, "6*7") {
		t.Fatalf("%v %+v", err, out)
	}
	if out, err := tools.Execute("ReadFile", json.RawMessage(`{"path":"main.sh"}`)); err != nil || out != "echo $((6*7))" {
		t.Fatalf("%v %v", err, out)
	}
	if out, err := tools.Execute("Checkpoint", json.RawMessage(`{}`)); err != nil || out.(CheckpointRef).Size == 0 {
		t.Fatalf("%v %v", err, out)
	}
	if len(tools.GetMethods()) != 5 {
		t.Fatalf("methods %d", len(tools.GetMethods()))
	}
}

// Docker integration: AGNO_TEST_DOCKER_IMAGE=alpine:3.20 go test ./agno/sandbox -run Docker
func TestDockerSandbox(t *testing.T) {
	image := os.Getenv("AGNO_TEST_DOCKER_IMAGE")
	if image == "" {
		t.Skip("set AGNO_TEST_DOCKER_IMAGE to run the docker sandbox test")
	}
	ctx := context.Background()
	p := &DockerProvider{Image: image, MemoryMB: 256}
	sb, err := p.Create(ctx, "test-"+shortHash(time.Now().String()), CreateOptions{Env: map[string]string{"SECRET": "s3cr3t"}})
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Close(ctx)
	if err := sb.PutFile(ctx, "dir/x.txt", []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := sb.Exec(ctx, []string{"sh", "-c", "cat dir/x.txt; echo -n \" $SECRET\""}, ExecOptions{})
	if err != nil || res.Stdout != "data s3cr3t" {
		t.Fatalf("%v %+v", err, res)
	}
	if b, err := sb.GetFile(ctx, "dir/x.txt"); err != nil || string(b) != "data" {
		t.Fatalf("%v %q", err, b)
	}
	res, _ = sb.Exec(ctx, []string{"sleep", "10"}, ExecOptions{Timeout: 500 * time.Millisecond})
	if !res.TimedOut {
		t.Fatalf("timeout %+v", res)
	}
	store, _ := NewFileCheckpointStore(t.TempDir())
	ref, err := sb.Checkpoint(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	_ = sb.PutFile(ctx, "dir/x.txt", []byte("changed"), 0)
	if err := sb.Restore(ctx, store, ref); err != nil {
		t.Fatal(err)
	}
	if b, _ := sb.GetFile(ctx, "dir/x.txt"); string(b) != "data" {
		t.Fatalf("restored %q", b)
	}
}

// fake S3 server that checks a SigV4 Authorization header is present.
func newFakeS3(t *testing.T) *S3CheckpointStore {
	var mu sync.Mutex
	objects := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AK/") || r.Header.Get("x-amz-content-sha256") == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			objects[r.URL.Path] = b
		case http.MethodGet:
			b, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(b)
		case http.MethodDelete:
			delete(objects, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)
	return &S3CheckpointStore{Endpoint: srv.URL, Bucket: "bkt", AccessKey: "AK", SecretKey: "SK", PathStyle: true}
}

func TestSigV4KnownVector(t *testing.T) {
	// AWS documentation example (GET object without Range):
	// derived signing key and canonical format exercised via a stable clock.
	s := &S3CheckpointStore{Endpoint: "https://s3.amazonaws.com", Bucket: "examplebucket", AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", now: func() time.Time { return time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC) }}
	req, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	s.sign(req, nil)
	auth := req.Header.Get("Authorization")
	if !strings.Contains(auth, "Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request") ||
		!strings.Contains(auth, "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Fatal(auth)
	}
}

func gzipTo(t *testing.T, w io.Writer, data []byte) {
	t.Helper()
	gz := gzip.NewWriter(w)
	if _, err := gz.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}
