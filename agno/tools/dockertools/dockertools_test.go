package dockertools

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func frame(stream byte, s string) []byte {
	h := make([]byte, 8)
	h[0] = stream
	binary.BigEndian.PutUint32(h[4:], uint32(len(s)))
	return append(h, s...)
}

func newFake(t *testing.T) (*httptest.Server, *map[string]interface{}) {
	var created map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/containers/json":
			_, _ = w.Write([]byte(`[{"Id":"c1","Names":["/web"],"Image":"nginx","State":"running","Created":1,"Ports":[],"Labels":{}}]`))
		case p == "/containers/create":
			_ = json.NewDecoder(r.Body).Decode(&created)
			_, _ = w.Write([]byte(`{"Id":"new1"}`))
		case p == "/containers/new1/start", p == "/containers/c1/start":
			w.WriteHeader(204)
		case p == "/containers/missing/start":
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"message":"No such container: missing"}`))
		case p == "/containers/c1/logs":
			_, _ = w.Write(append(frame(1, "hello\n"), frame(2, "oops\n")...))
		case p == "/containers/c1/exec":
			_, _ = w.Write([]byte(`{"Id":"e1"}`))
		case p == "/exec/e1/start":
			_, _ = w.Write(frame(1, "out"))
		case p == "/exec/e1/json":
			_, _ = w.Write([]byte(`{"ExitCode":2}`))
		case p == "/images/create":
			_, _ = w.Write([]byte("{\"status\":\"Pulling\"}\n{\"status\":\"Done\"}\n"))
		case p == "/build":
			if r.Header.Get("Content-Type") != "application/x-tar" || r.URL.Query().Get("t") != "img:1" {
				w.WriteHeader(400)
				return
			}
			_, _ = w.Write([]byte("{\"stream\":\"Step 1\"}\n"))
		case p == "/images/img:1/json":
			_, _ = w.Write([]byte(`{"Id":"sha256:abc"}`))
		case p == "/networks":
			_, _ = w.Write([]byte(`[{"Id":"n1","Name":"bridge","Driver":"bridge","Scope":"local","Containers":{"c1":{}}}]`))
		case p == "/volumes":
			_, _ = w.Write([]byte(`{"Volumes":[{"Name":"v1","Driver":"local"}]}`))
		default:
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &created
}

func TestDockerTools(t *testing.T) {
	srv, created := newFake(t)
	d := NewDockerTools(WithDockerBaseURL(srv.URL))
	if len(d.GetMethods()) != 24 {
		t.Fatalf("methods %d", len(d.GetMethods()))
	}
	out, _ := d.ListContainers(ListContainersParams{})
	if !strings.Contains(out.(string), `"name": "web"`) {
		t.Fatalf("list: %v", out)
	}
	if out, _ = d.StartContainer(ContainerIDParams{ContainerID: "missing"}); !strings.Contains(out.(string), "No such container") {
		t.Fatalf("start err: %v", out)
	}
	out, _ = d.RunContainer(RunContainerParams{Image: "nginx", Command: `sh -c "echo hi"`, Ports: map[string]interface{}{"80/tcp": float64(8080)}, Environment: map[string]string{"A": "1"}})
	if out != "Container started with ID: new1" {
		t.Fatalf("run: %v", out)
	}
	if !reflect.DeepEqual((*created)["Cmd"], []interface{}{"sh", "-c", "echo hi"}) {
		t.Fatalf("cmd %v", (*created)["Cmd"])
	}
	hc := (*created)["HostConfig"].(map[string]interface{})
	if !strings.Contains(restJSON(hc["PortBindings"]), `"HostPort":"8080"`) {
		t.Fatalf("ports %v", hc)
	}
	if out, _ = d.GetContainerLogs(ContainerLogsParams{ContainerID: "c1"}); out != "hello\noops\n" {
		t.Fatalf("logs %q", out)
	}
	if out, _ = d.ExecInContainer(ExecParams{ContainerID: "c1", Command: "ls"}); out != "Command failed with exit code 2: out" {
		t.Fatalf("exec %v", out)
	}
	if out, _ = d.PullImage(PullImageParams{ImageName: "alpine"}); out != "Image alpine:latest pulled successfully" {
		t.Fatalf("pull %v", out)
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o644)
	if out, _ = d.BuildImage(BuildImageParams{Path: dir, Tag: "img:1"}); out != "Image built successfully with ID: sha256:abc" {
		t.Fatalf("build %v", out)
	}
	if out, _ = d.ListNetworks(NoParams{}); !strings.Contains(out.(string), `"c1"`) {
		t.Fatalf("networks %v", out)
	}
	if out, _ = d.ListVolumes(NoParams{}); !strings.Contains(out.(string), `"v1"`) {
		t.Fatalf("volumes %v", out)
	}
}

func TestDockerUnixSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "d.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip("unix sockets unavailable")
	}
	srv := &httptest.Server{Listener: l, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})}}
	srv.Start()
	defer srv.Close()
	d := NewDockerTools(WithDockerSocket(sock))
	if out, _ := d.ListImages(NoParams{}); out != "[]" {
		t.Fatalf("got %v", out)
	}
}

func TestSplitCommand(t *testing.T) {
	got := splitCommand(`echo 'a b' "c\"d" e\ f`)
	want := []string{"echo", "a b", `c"d`, "e f"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%q", got)
	}
}

func restJSON(v interface{}) string { b, _ := json.Marshal(v); return string(b) }
