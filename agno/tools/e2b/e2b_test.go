package e2b

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func connectFrame(flags byte, v interface{}) []byte {
	b, _ := json.Marshal(v)
	h := make([]byte, 5)
	h[0] = flags
	binary.BigEndian.PutUint32(h[1:], uint32(len(b)))
	return append(h, b...)
}

func fakeE2B(t *testing.T) (*httptest.Server, map[string]string) {
	files := map[string]string{}
	var gotCmd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/port") && r.Header.Get("X-Access-Token") != "tok" {
			w.WriteHeader(401)
			return
		}
		switch {
		case p == "/sandboxes" && r.Method == "POST":
			if r.Header.Get("X-API-Key") != "k" {
				w.WriteHeader(401)
				return
			}
			_, _ = w.Write([]byte(`{"sandboxID":"sb1","domain":"e2b.app","envdAccessToken":"tok"}`))
		case p == "/sandboxes" && r.Method == "GET":
			_, _ = w.Write([]byte(`[{"sandboxID":"sb1","templateID":"code-interpreter-v1","startedAt":"now"}]`))
		case p == "/sandboxes/sb1/timeout", p == "/sandboxes/sb1":
			w.WriteHeader(204)
		case p == "/port49999/execute":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["code"] != "print(True)" {
				_, _ = w.Write([]byte(`{"type":"error","name":"NameError","value":"bad","traceback":"tb"}` + "\n"))
				return
			}
			png := base64.StdEncoding.EncodeToString([]byte("PNGDATA"))
			fmt.Fprintf(w, "{\"type\":\"stdout\",\"text\":\"True\\n\"}\n{\"type\":\"result\",\"png\":%q}\n{\"type\":\"end_of_execution\"}\n", png)
		case p == "/port49983/files" && r.Method == "GET":
			c, ok := files[r.URL.Query().Get("path")]
			if !ok {
				w.WriteHeader(404)
				return
			}
			_, _ = w.Write([]byte(c))
		case p == "/port49983/files" && r.Method == "POST":
			f, _, err := r.FormFile("file")
			if err != nil {
				w.WriteHeader(400)
				return
			}
			b, _ := io.ReadAll(f)
			path := r.URL.Query().Get("path")
			files[path] = string(b)
			_, _ = w.Write([]byte(fmt.Sprintf(`[{"path":%q,"name":"x","type":"file"}]`, path)))
		case p == "/port49983/filesystem.Filesystem/ListDir":
			_, _ = w.Write([]byte(`{"entries":[{"name":"a.txt","type":"FILE_TYPE_FILE","path":"/a.txt","size":"3"},{"name":"d","type":"FILE_TYPE_DIRECTORY","path":"/d"}]}`))
		case p == "/port49983/process.Process/Start":
			if r.Header.Get("Content-Type") != "application/connect+json" {
				w.WriteHeader(415)
				return
			}
			hdr := make([]byte, 5)
			_, _ = io.ReadFull(r.Body, hdr)
			payload := make([]byte, binary.BigEndian.Uint32(hdr[1:]))
			_, _ = io.ReadFull(r.Body, payload)
			var req struct {
				Process struct {
					Args []string `json:"args"`
				} `json:"process"`
			}
			_ = json.Unmarshal(payload, &req)
			gotCmd = req.Process.Args[2]
			w.Header().Set("Content-Type", "application/connect+json")
			_, _ = w.Write(connectFrame(0, map[string]interface{}{"event": map[string]interface{}{"start": map[string]int{"pid": 7}}}))
			out := "hello\n"
			if strings.Contains(gotCmd, "echo $!") {
				out = "4242\n"
			}
			_, _ = w.Write(connectFrame(0, map[string]interface{}{"event": map[string]interface{}{"data": map[string]string{"stdout": base64.StdEncoding.EncodeToString([]byte(out))}}}))
			_, _ = w.Write(connectFrame(0, map[string]interface{}{"event": map[string]interface{}{"data": map[string]string{"stderr": base64.StdEncoding.EncodeToString([]byte("warn"))}}}))
			_, _ = w.Write(connectFrame(0, map[string]interface{}{"event": map[string]interface{}{"end": map[string]interface{}{"exited": true}}}))
			_, _ = w.Write(connectFrame(2, map[string]interface{}{}))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, files
}

func newTools(srv *httptest.Server) *E2BTools {
	return NewE2BTools(WithE2BAPIKey("k"), WithE2BAPIURL(srv.URL), WithE2BHostURL(func(port int, id, domain string) string {
		return fmt.Sprintf("%s/port%d", srv.URL, port)
	}))
}

func TestE2BCodeAndResults(t *testing.T) {
	srv, _ := fakeE2B(t)
	e := newTools(srv)
	if out, _ := e.DownloadPngResult(DownloadPNGParams{}); out != "No code has been executed yet" {
		t.Fatalf("%v", out)
	}
	out, _ := e.RunPythonCode(RunPythonCodeParams{Code: "print(true)"})
	s := out.(string)
	if !strings.Contains(s, `True\\n`) || !strings.Contains(s, "Result 1: Generated PNG image") {
		t.Fatalf("run: %s", s)
	}
	dest := filepath.Join(t.TempDir(), "o.png")
	if out, _ = e.DownloadPngResult(DownloadPNGParams{OutputPath: dest}); out != "Image saved to "+dest {
		t.Fatalf("png: %v", out)
	}
	if b, _ := os.ReadFile(dest); string(b) != "PNGDATA" {
		t.Fatalf("png content %q", b)
	}
	if out, _ = e.RunPythonCode(RunPythonCodeParams{Code: "x"}); out != "Error: NameError\nbad\ntb" {
		t.Fatalf("err: %v", out)
	}
	if out, _ = e.GetSandboxStatus(NoParams{}); out != "sb1" {
		t.Fatalf("status %v", out)
	}
	if out, _ = e.GetPublicUrl(PortParams{Port: 3000}); out != "http://3000-sb1.e2b.app" {
		t.Fatalf("url %v", out)
	}
}

func TestE2BFilesAndCommands(t *testing.T) {
	srv, files := fakeE2B(t)
	e := newTools(srv)
	if out, _ := e.WriteFileContent(WriteFileParams{FilePath: "/home/user/a.txt", Content: "abc"}); out != "/home/user/a.txt" {
		t.Fatalf("write %v", out)
	}
	if files["/home/user/a.txt"] != "abc" {
		t.Fatalf("files %v", files)
	}
	if out, _ := e.ReadFileContent(ReadFileParams{FilePath: "/home/user/a.txt"}); out != "abc" {
		t.Fatalf("read %v", out)
	}
	if out, _ := e.ListFiles(ListFilesParams{}); out != "Contents of /:\n- a.txt (File, 3 bytes)\n- d (Directory, Unknown size)\n" {
		t.Fatalf("list %q", out)
	}
	if out, _ := e.RunCommand(CommandParams{Command: "echo hello"}); out != `["STDOUT:\nhello\n","STDERR:\nwarn"]` {
		t.Fatalf("cmd %v", out)
	}
	if out, _ := e.StreamCommand(CommandParams{Command: "echo hello"}); out != `["STDOUT: hello\n","STDERR: warn"]` {
		t.Fatalf("stream %v", out)
	}
	out, _ := e.RunBackgroundCommand(CommandParams{Command: "sleep 100"})
	if !strings.Contains(out.(string), `"pid":4242`) {
		t.Fatalf("bg %v", out)
	}
	if out, _ := e.KillBackgroundCommand(KillParams{PID: 4242}); out != "Background command terminated successfully." {
		t.Fatalf("kill %v", out)
	}
	if out, _ := e.ListRunningSandboxes(NoParams{}); !strings.Contains(out.(string), "Found 1 running sandboxes") {
		t.Fatalf("list sb %v", out)
	}
	if out, _ := e.SetSandboxTimeout(TimeoutParams{Timeout: 60}); out != "60" {
		t.Fatalf("timeout %v", out)
	}
	if out, _ := e.ShutdownSandbox(NoParams{}); !strings.Contains(out.(string), "success") {
		t.Fatalf("shutdown %v", out)
	}
}

func TestE2BMissingKey(t *testing.T) {
	t.Setenv("E2B_API_KEY", "")
	e := NewE2BTools()
	if out, _ := e.RunCommand(CommandParams{Command: "ls"}); !strings.Contains(out.(string), "E2B_API_KEY not set") {
		t.Fatalf("%v", out)
	}
}
