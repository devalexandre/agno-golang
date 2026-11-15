package daytona

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

// fakeDaytona emulates the sandbox API and runs toolbox commands with the local
// shell, with "/home/daytona" mapped to a temp dir.
func fakeDaytona(t *testing.T) (*httptest.Server, *int) {
	root := t.TempDir()
	creates := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/sandbox":
			creates++
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["labels"].(map[string]interface{})["created_by"] != "agno_daytona_toolkit" {
				w.WriteHeader(400)
				return
			}
			_, _ = w.Write([]byte(`{"id":"sb1","state":"started"}`))
		case r.URL.Path == "/toolbox/sb1/toolbox/process/execute":
			var req struct {
				Command string `json:"command"`
				Cwd     string `json:"cwd"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			// sh -c 'echo <b64> | base64 -d | sh'
			b64 := strings.TrimSuffix(strings.TrimPrefix(req.Command, "sh -c 'echo "), " | base64 -d | sh'")
			raw, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				w.WriteHeader(400)
				return
			}
			script := strings.ReplaceAll(string(raw), "/home/daytona", root)
			c := exec.Command("sh", "-c", script)
			c.Dir = root
			if req.Cwd != "" && req.Cwd != "/" {
				c.Dir = strings.ReplaceAll(req.Cwd, "/home/daytona", root)
			}
			out, err := c.CombinedOutput()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
			res := strings.ReplaceAll(string(out), root, "/home/daytona")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"exitCode": code, "result": res})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &creates
}

func TestDaytonaFileAndShell(t *testing.T) {
	if _, err := exec.LookPath("base64"); err != nil {
		t.Skip("base64 not available")
	}
	srv, creates := fakeDaytona(t)
	d := NewDaytonaTools(WithDaytonaAPIKey("k"), WithDaytonaAPIURL(srv.URL))

	out, _ := d.CreateFile(CreateFileParams{FilePath: "src/a.txt", Content: "it's \"quoted\"\n$HOME"})
	if out != "File created/updated: /home/daytona/src/a.txt" {
		t.Fatalf("create: %v", out)
	}
	if out, _ = d.ReadFile(FilePathParams{FilePath: "src/a.txt"}); out != "it's \"quoted\"\n$HOME" {
		t.Fatalf("read: %q", out)
	}
	if out, _ = d.ChangeDirectory(ChangeDirectoryParams{Directory: "src"}); out != "Changed directory to: /home/daytona/src" {
		t.Fatalf("cd: %v", out)
	}
	if out, _ = d.RunShellCommand(RunShellCommandParams{Command: "cat a.txt | wc -l"}); strings.TrimSpace(out.(string)) != "1" {
		t.Fatalf("shell: %q", out)
	}
	if out, _ = d.RunShellCommand(RunShellCommandParams{Command: "cd nope"}); out != "Error: Directory /home/daytona/src/nope not found" {
		t.Fatalf("cd missing: %v", out)
	}
	if out, _ = d.ListFiles(ListFilesParams{}); !strings.Contains(out.(string), "a.txt") {
		t.Fatalf("ls: %v", out)
	}
	if out, _ = d.DeleteFile(FilePathParams{FilePath: "a.txt"}); out != "Deleted: /home/daytona/src/a.txt" {
		t.Fatalf("delete: %v", out)
	}
	if *creates != 1 || d.SandboxID() != "sb1" {
		t.Fatalf("sandbox created %d times", *creates)
	}
	if _, err := exec.LookPath("python3"); err == nil {
		if out, _ = d.RunCode(RunCodeParams{Code: "x = true\nprint('ok', x)"}); strings.TrimSpace(out.(string)) != "ok True" {
			t.Fatalf("code: %q", out)
		}
	}
}

func TestDaytonaMissingKey(t *testing.T) {
	t.Setenv("DAYTONA_API_KEY", "")
	d := NewDaytonaTools()
	out, _ := d.ReadFile(FilePathParams{FilePath: "x"})
	if !strings.Contains(out.(string), "DAYTONA_API_KEY not set") {
		t.Fatalf("%v", out)
	}
}

func TestPreparePythonCode(t *testing.T) {
	if got := preparePythonCode("a = true or none; b = 'trueish'"); got != "a = True or None; b = 'trueish'" {
		t.Fatal(got)
	}
}
