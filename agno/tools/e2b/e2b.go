// Package e2b ports Agno's E2BTools: Python execution, shell commands and file
// operations inside an E2B cloud sandbox, using plain HTTP.
//
// Wire protocols (no SDK needed):
//   - Control plane (api.e2b.dev, X-API-Key): create/list/kill sandboxes, set timeout.
//   - Code interpreter (port 49999 in the sandbox): POST /execute, NDJSON stream.
//   - envd (port 49983): GET/POST /files for file transfer, and the Connect
//     protocol (JSON codec) for filesystem.Filesystem/ListDir (unary) and
//     process.Process/Start (server stream) for commands.
//
// Differences from Python: the sandbox is created lazily on first use (not in
// the constructor); background commands are identified by PID (Python passes an
// SDK object); WatchDirectory polls ListDir instead of using the envd watcher
// stream; PNG/chart results are saved to files and returned as paths, since Go
// tools cannot attach media artifacts to the agent.
package e2b

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const (
	// DefaultAPIURL is the E2B control plane.
	DefaultAPIURL = "https://api.e2b.dev"
	// DefaultDomain is the sandbox host domain.
	DefaultDomain = "e2b.app"
	// DefaultTemplate is the code interpreter template.
	DefaultTemplate = "code-interpreter-v1"
	// CodeInterpreterPort is the Jupyter execution port inside the sandbox.
	CodeInterpreterPort = 49999
	// EnvdPort is the sandbox daemon port.
	EnvdPort    = 49983
	defaultUser = "user"
)

// E2BTools executes code in an E2B sandbox.
type E2BTools struct {
	toolkit.Toolkit
	apiKey     string
	apiURL     string
	template   string
	timeout    int
	metadata   map[string]string
	envVars    map[string]string
	httpClient *http.Client
	// hostURL builds the base URL of a sandbox port (overridable for tests).
	hostURL func(port int, sandboxID, domain string) string

	mu           sync.Mutex
	sandboxID    string
	domain       string
	accessToken  string
	lastResults  []execResult
	hasExecution bool
}

// Option configures E2BTools.
type Option func(*E2BTools)

// WithE2BAPIKey sets the API key (default E2B_API_KEY).
func WithE2BAPIKey(k string) Option { return func(e *E2BTools) { e.apiKey = k } }

// WithE2BAPIURL overrides the control plane URL.
func WithE2BAPIURL(u string) Option { return func(e *E2BTools) { e.apiURL = u } }

// WithE2BTemplate sets the sandbox template (default code-interpreter-v1).
func WithE2BTemplate(t string) Option { return func(e *E2BTools) { e.template = t } }

// WithE2BTimeout sets the sandbox timeout in seconds (default 300).
func WithE2BTimeout(seconds int) Option { return func(e *E2BTools) { e.timeout = seconds } }

// WithE2BMetadata sets sandbox metadata.
func WithE2BMetadata(m map[string]string) Option { return func(e *E2BTools) { e.metadata = m } }

// WithE2BEnvVars sets sandbox environment variables.
func WithE2BEnvVars(m map[string]string) Option { return func(e *E2BTools) { e.envVars = m } }

// WithE2BSandboxID attaches to an existing sandbox instead of creating one.
func WithE2BSandboxID(id, domain, accessToken string) Option {
	return func(e *E2BTools) { e.sandboxID, e.domain, e.accessToken = id, domain, accessToken }
}

// WithE2BHTTPClient replaces the HTTP client.
func WithE2BHTTPClient(c *http.Client) Option { return func(e *E2BTools) { e.httpClient = c } }

// WithE2BHostURL overrides how sandbox port URLs are built (tests, proxies).
func WithE2BHostURL(f func(port int, sandboxID, domain string) string) Option {
	return func(e *E2BTools) { e.hostURL = f }
}

// Parameter structs.
type (
	RunPythonCodeParams struct {
		Code string `json:"code" description:"Python code to execute." required:"true"`
	}
	UploadFileParams struct {
		FilePath    string `json:"file_path" description:"Path to the file on the local system." required:"true"`
		SandboxPath string `json:"sandbox_path,omitempty" description:"Destination path in the sandbox. Defaults to the same filename."`
	}
	DownloadPNGParams struct {
		ResultIndex int    `json:"result_index,omitempty" description:"Index of the result to use (default: 0, the first result)."`
		OutputPath  string `json:"output_path,omitempty" description:"Path to save the PNG file. Defaults to a temporary file."`
	}
	DownloadChartParams struct {
		ResultIndex int    `json:"result_index,omitempty" description:"Index of the result to extract data from (default: 0)."`
		OutputPath  string `json:"output_path,omitempty" description:"Path to save the JSON data. Defaults to 'chart-data-{result_index}.json'."`
	}
	DownloadFileParams struct {
		SandboxPath string `json:"sandbox_path" description:"Path to the file in the sandbox." required:"true"`
		LocalPath   string `json:"local_path,omitempty" description:"Destination path on the local system. Defaults to the same filename."`
	}
	ListFilesParams struct {
		DirectoryPath string `json:"directory_path,omitempty" description:"Path to the directory to list (default: root directory)."`
	}
	ReadFileParams struct {
		FilePath string `json:"file_path" description:"Path to the file in the sandbox." required:"true"`
		Encoding string `json:"encoding,omitempty" description:"Encoding to use for text files (default: utf-8)."`
	}
	WriteFileParams struct {
		FilePath string `json:"file_path" description:"Path to the file in the sandbox." required:"true"`
		Content  string `json:"content" description:"Text content to write." required:"true"`
	}
	WatchDirectoryParams struct {
		DirectoryPath   string `json:"directory_path" description:"Path to the directory to watch." required:"true"`
		DurationSeconds int    `json:"duration_seconds,omitempty" description:"How long to watch for changes in seconds (default: 5 seconds)."`
	}
	PortParams struct {
		Port int `json:"port" description:"Port number the service is running on in the sandbox." required:"true"`
	}
	RunServerParams struct {
		Command string `json:"command" description:"Command to start the server." required:"true"`
		Port    int    `json:"port" description:"Port the server will listen on." required:"true"`
	}
	TimeoutParams struct {
		Timeout int `json:"timeout" description:"New timeout in seconds." required:"true"`
	}
	CommandParams struct {
		Command string `json:"command" description:"Shell command to execute." required:"true"`
	}
	KillParams struct {
		PID int `json:"pid" description:"PID returned by run_background_command." required:"true"`
	}
	NoParams struct{}
)

// NewE2BTools creates the E2B toolkit.
func NewE2BTools(opts ...Option) *E2BTools {
	e := &E2BTools{
		apiKey:   os.Getenv("E2B_API_KEY"),
		apiURL:   DefaultAPIURL,
		template: DefaultTemplate,
		timeout:  300,
	}
	e.Toolkit = toolkit.NewToolkit()
	e.Toolkit.Name = "e2b_tools"
	e.Toolkit.Description = "Run Python code, shell commands and file operations in an isolated E2B sandbox."
	for _, o := range opts {
		o(e)
	}
	if e.httpClient == nil {
		e.Toolkit.Timeout = 5 * time.Minute
		e.httpClient = e.Toolkit.HTTPClient()
	}
	if e.hostURL == nil {
		e.hostURL = func(port int, id, domain string) string {
			return fmt.Sprintf("https://%d-%s.%s", port, id, domain)
		}
	}
	r := e.Toolkit.Register
	r("RunPythonCode", "Run Python code in an isolated E2B sandbox environment.", e, e.RunPythonCode, RunPythonCodeParams{})
	r("UploadFile", "Upload a local file to the E2B sandbox.", e, e.UploadFile, UploadFileParams{})
	r("DownloadPngResult", "Save a PNG image result from the last code execution to a local file.", e, e.DownloadPngResult, DownloadPNGParams{})
	r("DownloadChartData", "Extract chart data from an interactive chart in the last execution results and save it as JSON.", e, e.DownloadChartData, DownloadChartParams{})
	r("DownloadFileFromSandbox", "Download a file from the E2B sandbox to the local system.", e, e.DownloadFileFromSandbox, DownloadFileParams{})
	r("ListFiles", "List files and directories in the specified path in the sandbox.", e, e.ListFiles, ListFilesParams{})
	r("ReadFileContent", "Read the content of a file from the sandbox.", e, e.ReadFileContent, ReadFileParams{})
	r("WriteFileContent", "Write text content to a file in the sandbox.", e, e.WriteFileContent, WriteFileParams{})
	r("WatchDirectory", "Watch a directory for changes for a specified duration.", e, e.WatchDirectory, WatchDirectoryParams{})
	r("GetPublicUrl", "Get a public URL for a service running in the sandbox on the specified port.", e, e.GetPublicUrl, PortParams{})
	r("RunServer", "Start a server in the sandbox and return its public URL.", e, e.RunServer, RunServerParams{})
	r("SetSandboxTimeout", "Update the timeout for the sandbox.", e, e.SetSandboxTimeout, TimeoutParams{})
	r("GetSandboxStatus", "Get the current status (ID) of the sandbox.", e, e.GetSandboxStatus, NoParams{})
	e.Toolkit.RegisterWithOptions("ShutdownSandbox", "Shutdown the sandbox immediately.", e, e.ShutdownSandbox, NoParams{}, toolkit.WithConfirmation())
	r("ListRunningSandboxes", "List all running sandboxes.", e, e.ListRunningSandboxes, NoParams{})
	r("RunCommand", "Run a shell command in the sandbox environment.", e, e.RunCommand, CommandParams{})
	r("StreamCommand", "Run a shell command and return its stdout/stderr chunks in order.", e, e.StreamCommand, CommandParams{})
	r("RunBackgroundCommand", "Run a shell command in the background. Returns its PID.", e, e.RunBackgroundCommand, CommandParams{})
	r("KillBackgroundCommand", "Kill a background command by PID.", e, e.KillBackgroundCommand, KillParams{})
	return e
}

func errJSON(format string, args ...interface{}) string {
	b, _ := json.Marshal(map[string]string{"status": "error", "message": fmt.Sprintf(format, args...)})
	return string(b)
}

func (e *E2BTools) api() *restapi.Client {
	return restapi.New(e.apiURL, e.httpClient, map[string]string{"X-API-Key": e.apiKey})
}

// ensureSandbox creates the sandbox on first use.
func (e *E2BTools) ensureSandbox(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sandboxID != "" {
		return nil
	}
	if e.apiKey == "" {
		return errors.New("E2B_API_KEY not set. Please set the E2B_API_KEY environment variable")
	}
	body := map[string]interface{}{"templateID": e.template, "timeout": e.timeout}
	if e.metadata != nil {
		body["metadata"] = e.metadata
	}
	if e.envVars != nil {
		body["envVars"] = e.envVars
	}
	var out struct {
		SandboxID       string  `json:"sandboxID"`
		Domain          *string `json:"domain"`
		EnvdAccessToken *string `json:"envdAccessToken"`
	}
	if err := e.api().JSON(ctx, http.MethodPost, "/sandboxes", nil, body, &out); err != nil {
		return fmt.Errorf("could not create sandbox: %w", err)
	}
	e.sandboxID = out.SandboxID
	if out.Domain != nil {
		e.domain = *out.Domain
	}
	if out.EnvdAccessToken != nil {
		e.accessToken = *out.EnvdAccessToken
	}
	return nil
}

func (e *E2BTools) portURL(port int) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.domain
	if d == "" {
		d = DefaultDomain
	}
	return e.hostURL(port, e.sandboxID, d)
}

func (e *E2BTools) sandboxClient(port int) *restapi.Client {
	e.mu.Lock()
	tok := e.accessToken
	e.mu.Unlock()
	h := map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(defaultUser+":"))}
	if tok != "" {
		h["X-Access-Token"] = tok
	}
	return restapi.New(e.portURL(port), e.httpClient, h)
}

// --- code execution ---

type execResult struct {
	Text  string                 `json:"text"`
	PNG   string                 `json:"png"`
	Chart map[string]interface{} `json:"chart"`
}

// RunPythonCode runs Python code via the code interpreter.
func (e *E2BTools) RunPythonCode(p RunPythonCodeParams) (interface{}, error) {
	ctx := context.Background()
	if err := e.ensureSandbox(ctx); err != nil {
		return errJSON("Error executing code: %v", err), nil
	}
	data, err := e.sandboxClient(CodeInterpreterPort).Do(ctx, http.MethodPost, "/execute", nil, map[string]string{"code": preparePythonCode(p.Code)})
	if err != nil {
		return errJSON("Error executing code: %v", err), nil
	}
	var stdout, stderr []string
	var results []execResult
	var execErr map[string]interface{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var line map[string]interface{}
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		switch line["type"] {
		case "stdout":
			stdout = append(stdout, fmt.Sprint(line["text"]))
		case "stderr":
			stderr = append(stderr, fmt.Sprint(line["text"]))
		case "error":
			execErr = line
		case "result":
			var r execResult
			b, _ := json.Marshal(line)
			_ = json.Unmarshal(b, &r)
			results = append(results, r)
		}
	}
	e.mu.Lock()
	e.lastResults, e.hasExecution = results, true
	e.mu.Unlock()

	if execErr != nil {
		return fmt.Sprintf("Error: %v\n%v\n%v", execErr["name"], execErr["value"], execErr["traceback"]), nil
	}
	var out []string
	if len(stdout) > 0 || len(stderr) > 0 {
		logs, _ := json.Marshal(map[string][]string{"stdout": nonNil(stdout), "stderr": nonNil(stderr)})
		out = append(out, "Logs:\n"+string(logs))
	}
	for i, r := range results {
		switch {
		case r.Text != "":
			out = append(out, fmt.Sprintf("Result %d: %s", i+1, r.Text))
		case r.PNG != "":
			out = append(out, fmt.Sprintf("Result %d: Generated PNG image (use download_png_result to save)", i+1))
		case r.Chart != nil:
			ct, _ := r.Chart["type"].(string)
			if ct == "" {
				ct = "unknown"
			}
			out = append(out, fmt.Sprintf("Result %d: Generated interactive %s chart (use download_chart_data to save)", i+1, ct))
		default:
			out = append(out, fmt.Sprintf("Result %d: Output available", i+1))
		}
	}
	if len(out) == 0 {
		return "Code executed successfully with no output.", nil
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (e *E2BTools) result(idx int) (*execResult, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.hasExecution {
		return nil, "No code has been executed yet"
	}
	if idx < 0 || idx >= len(e.lastResults) {
		return nil, fmt.Sprintf("Result index %d is out of range. Only %d results available.", idx, len(e.lastResults))
	}
	r := e.lastResults[idx]
	return &r, ""
}

// DownloadPngResult saves a PNG result of the last execution to disk.
func (e *E2BTools) DownloadPngResult(p DownloadPNGParams) (interface{}, error) {
	r, msg := e.result(p.ResultIndex)
	if r == nil {
		return msg, nil
	}
	if r.PNG == "" {
		return fmt.Sprintf("Result at index %d is not a PNG image", p.ResultIndex), nil
	}
	data, err := base64.StdEncoding.DecodeString(r.PNG)
	if err != nil {
		return fmt.Sprintf("Error processing PNG: %v", err), nil
	}
	path := p.OutputPath
	if path == "" {
		f, err := os.CreateTemp("", "e2b-*.png")
		if err != nil {
			return fmt.Sprintf("Error processing PNG: %v", err), nil
		}
		path = f.Name()
		f.Close()
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Sprintf("Error processing PNG: %v", err), nil
	}
	return fmt.Sprintf("Image saved to %s", path), nil
}

// DownloadChartData saves chart data of the last execution as JSON.
func (e *E2BTools) DownloadChartData(p DownloadChartParams) (interface{}, error) {
	r, msg := e.result(p.ResultIndex)
	if r == nil {
		return msg, nil
	}
	if r.Chart == nil {
		return fmt.Sprintf("Result at index %d does not contain interactive chart data", p.ResultIndex), nil
	}
	path := p.OutputPath
	if path == "" {
		path = fmt.Sprintf("chart-data-%d.json", p.ResultIndex)
	}
	b, _ := json.MarshalIndent(r.Chart, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Sprintf("Error extracting chart data: %v", err), nil
	}
	ct, _ := r.Chart["type"].(string)
	if ct == "" {
		ct = "unknown"
	}
	summary := fmt.Sprintf("Interactive %s chart data saved to %s\n", ct, path)
	for _, k := range [][2]string{{"title", "Title"}, {"x_label", "X-axis"}, {"y_label", "Y-axis"}} {
		if v, ok := r.Chart[k[0]]; ok {
			summary += fmt.Sprintf("%s: %v\n", k[1], v)
		}
	}
	return summary, nil
}

// --- files ---

func (e *E2BTools) readFile(ctx context.Context, path string) ([]byte, error) {
	if err := e.ensureSandbox(ctx); err != nil {
		return nil, err
	}
	return e.sandboxClient(EnvdPort).Do(ctx, http.MethodGet, "/files", url.Values{"path": {path}, "username": {defaultUser}}, nil)
}

func (e *E2BTools) writeFile(ctx context.Context, path string, data []byte) (string, error) {
	if err := e.ensureSandbox(ctx); err != nil {
		return "", err
	}
	c := e.sandboxClient(EnvdPort)
	c.BaseURL = c.URL("/files", url.Values{"path": {path}, "username": {defaultUser}})
	resp, err := c.Multipart(ctx, http.MethodPost, c.BaseURL, nil, []restapi.FilePart{{Field: "file", Filename: filepath.Base(path), Data: data}})
	if err != nil {
		return "", err
	}
	var infos []struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(resp, &infos) == nil && len(infos) > 0 && infos[0].Path != "" {
		return infos[0].Path, nil
	}
	return path, nil
}

// UploadFile uploads a local file into the sandbox.
func (e *E2BTools) UploadFile(p UploadFileParams) (interface{}, error) {
	dest := p.SandboxPath
	if dest == "" {
		dest = filepath.Base(p.FilePath)
	}
	data, err := os.ReadFile(p.FilePath)
	if err != nil {
		return errJSON("Error uploading file: %v", err), nil
	}
	path, err := e.writeFile(context.Background(), dest, data)
	if err != nil {
		return errJSON("Error uploading file: %v", err), nil
	}
	return path, nil
}

// DownloadFileFromSandbox downloads a sandbox file to disk.
func (e *E2BTools) DownloadFileFromSandbox(p DownloadFileParams) (interface{}, error) {
	local := p.LocalPath
	if local == "" {
		local = filepath.Base(p.SandboxPath)
	}
	data, err := e.readFile(context.Background(), p.SandboxPath)
	if err != nil {
		return errJSON("Error downloading file: %v", err), nil
	}
	if err := os.WriteFile(local, data, 0o644); err != nil {
		return errJSON("Error downloading file: %v", err), nil
	}
	return local, nil
}

// ReadFileContent reads a text file from the sandbox.
func (e *E2BTools) ReadFileContent(p ReadFileParams) (interface{}, error) {
	data, err := e.readFile(context.Background(), p.FilePath)
	if err != nil {
		return errJSON("Error reading file: %v", err), nil
	}
	if !isText(data) {
		return fmt.Sprintf("File read successfully but contains binary data (%d bytes). Use download_file_from_sandbox to save it.", len(data)), nil
	}
	return string(data), nil
}

func isText(b []byte) bool {
	if bytes.IndexByte(b, 0) >= 0 {
		return false
	}
	return strings.ToValidUTF8(string(b), "\uFFFD") == string(b)
}

// WriteFileContent writes text to a sandbox file.
func (e *E2BTools) WriteFileContent(p WriteFileParams) (interface{}, error) {
	path, err := e.writeFile(context.Background(), p.FilePath, []byte(p.Content))
	if err != nil {
		return errJSON("Error writing file: %v", err), nil
	}
	return path, nil
}

type entry struct {
	Name         string       `json:"name"`
	Type         string       `json:"type"`
	Path         string       `json:"path"`
	Size         *json.Number `json:"size"`
	ModifiedTime string       `json:"modifiedTime"`
}

func (e *E2BTools) listDir(ctx context.Context, dir string) ([]entry, error) {
	if err := e.ensureSandbox(ctx); err != nil {
		return nil, err
	}
	c := e.sandboxClient(EnvdPort)
	c.Headers["Connect-Protocol-Version"] = "1"
	var out struct {
		Entries []entry `json:"entries"`
	}
	if err := c.JSON(ctx, http.MethodPost, "/filesystem.Filesystem/ListDir", nil, map[string]interface{}{"path": dir, "depth": 1}, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

// ListFiles lists a sandbox directory.
func (e *E2BTools) ListFiles(p ListFilesParams) (interface{}, error) {
	dir := p.DirectoryPath
	if dir == "" {
		dir = "/"
	}
	entries, err := e.listDir(context.Background(), dir)
	if err != nil {
		return errJSON("Error listing files: %v", err), nil
	}
	if len(entries) == 0 {
		return "No files found in " + dir, nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Contents of %s:\n", dir)
	for _, f := range entries {
		kind := "File"
		if strings.Contains(strings.ToLower(f.Type), "dir") {
			kind = "Directory"
		}
		size := "Unknown size"
		if f.Size != nil {
			size = f.Size.String() + " bytes"
		}
		fmt.Fprintf(&sb, "- %s (%s, %s)\n", f.Name, kind, size)
	}
	return sb.String(), nil
}

// WatchDirectory reports changes in a directory over a period (by polling).
func (e *E2BTools) WatchDirectory(p WatchDirectoryParams) (interface{}, error) {
	dur := p.DurationSeconds
	if dur <= 0 {
		dur = 5
	}
	ctx := context.Background()
	snapshot := func() (map[string]string, error) {
		entries, err := e.listDir(ctx, p.DirectoryPath)
		if err != nil {
			return nil, err
		}
		m := map[string]string{}
		for _, f := range entries {
			sig := f.ModifiedTime
			if f.Size != nil {
				sig += "/" + f.Size.String()
			}
			key := f.Path
			if key == "" {
				key = strings.TrimRight(p.DirectoryPath, "/") + "/" + f.Name
			}
			m[key] = sig
		}
		return m, nil
	}
	before, err := snapshot()
	if err != nil {
		return errJSON("Error watching directory: %v", err), nil
	}
	time.Sleep(time.Duration(dur) * time.Second)
	after, err := snapshot()
	if err != nil {
		return errJSON("Error watching directory: %v", err), nil
	}
	var changes []string
	for k, v := range after {
		if old, ok := before[k]; !ok {
			changes = append(changes, "create - "+k)
		} else if old != v {
			changes = append(changes, "write - "+k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			changes = append(changes, "remove - "+k)
		}
	}
	sort.Strings(changes)
	msg := fmt.Sprintf("No changes detected in %s over %d seconds", p.DirectoryPath, dur)
	if len(changes) > 0 {
		msg = fmt.Sprintf("Changes detected in %s over %d seconds:\n%s", p.DirectoryPath, dur, strings.Join(changes, "\n"))
	}
	b, _ := json.Marshal(map[string]string{"status": "success", "message": msg})
	return string(b), nil
}

// --- commands (Connect server-streaming, JSON codec) ---

type commandResult struct {
	PID      int
	ExitCode int
	Stdout   string
	Stderr   string
	Chunks   []string
}

func envelope(flags byte, payload []byte) []byte {
	b := make([]byte, 5+len(payload))
	b[0] = flags
	binary.BigEndian.PutUint32(b[1:5], uint32(len(payload)))
	copy(b[5:], payload)
	return b
}

// runCommand starts a process and reads its event stream. When untilStart is
// true it returns as soon as the PID is known.
func (e *E2BTools) runCommand(ctx context.Context, command string, untilStart bool) (*commandResult, error) {
	if err := e.ensureSandbox(ctx); err != nil {
		return nil, err
	}
	reqBody, _ := json.Marshal(map[string]interface{}{
		"process": map[string]interface{}{"cmd": "/bin/bash", "args": []string{"-l", "-c", command}, "envs": map[string]string{}},
	})
	c := e.sandboxClient(EnvdPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL("/process.Process/Start", nil), bytes.NewReader(envelope(0, reqBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set("Connect-Protocol-Version", "1")
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return nil, &restapi.APIError{Status: resp.StatusCode, Body: string(b)}
	}
	res := &commandResult{}
	var stdout, stderr strings.Builder
	header := make([]byte, 5)
	for {
		if _, err := io.ReadFull(resp.Body, header); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		n := binary.BigEndian.Uint32(header[1:])
		payload := make([]byte, n)
		if _, err := io.ReadFull(resp.Body, payload); err != nil {
			return nil, err
		}
		if header[0]&0x02 != 0 { // end-of-stream
			var end struct {
				Error *struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(payload, &end) == nil && end.Error != nil {
				return nil, fmt.Errorf("%s: %s", end.Error.Code, end.Error.Message)
			}
			break
		}
		var msg struct {
			Event struct {
				Start *struct {
					PID int `json:"pid"`
				} `json:"start"`
				Data *struct {
					Stdout string `json:"stdout"`
					Stderr string `json:"stderr"`
				} `json:"data"`
				End *struct {
					ExitCode int    `json:"exitCode"`
					Error    string `json:"error"`
				} `json:"end"`
			} `json:"event"`
		}
		if json.Unmarshal(payload, &msg) != nil {
			continue
		}
		ev := msg.Event
		if ev.Start != nil {
			res.PID = ev.Start.PID
			if untilStart {
				return res, nil
			}
		}
		if ev.Data != nil {
			if ev.Data.Stdout != "" {
				s := decodeB64(ev.Data.Stdout)
				stdout.WriteString(s)
				res.Chunks = append(res.Chunks, "STDOUT: "+s)
			}
			if ev.Data.Stderr != "" {
				s := decodeB64(ev.Data.Stderr)
				stderr.WriteString(s)
				res.Chunks = append(res.Chunks, "STDERR: "+s)
			}
		}
		if ev.End != nil {
			res.ExitCode = ev.End.ExitCode
		}
	}
	res.Stdout, res.Stderr = stdout.String(), stderr.String()
	return res, nil
}

func decodeB64(s string) string {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(b)
	}
	return s
}

// RunCommand runs a shell command and returns its output.
func (e *E2BTools) RunCommand(p CommandParams) (interface{}, error) {
	res, err := e.runCommand(context.Background(), p.Command, false)
	if err != nil {
		return errJSON("Error executing command: %v", err), nil
	}
	var out []string
	if res.Stdout != "" {
		out = append(out, "STDOUT:\n"+res.Stdout)
	}
	if res.Stderr != "" {
		out = append(out, "STDERR:\n"+res.Stderr)
	}
	if len(out) == 0 {
		return "Command executed successfully with no output.", nil
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

// StreamCommand runs a command and returns its output chunks in order.
func (e *E2BTools) StreamCommand(p CommandParams) (interface{}, error) {
	res, err := e.runCommand(context.Background(), p.Command, false)
	if err != nil {
		return errJSON("Error streaming command: %v", err), nil
	}
	if len(res.Chunks) == 0 {
		return "Command completed with no output.", nil
	}
	b, _ := json.Marshal(res.Chunks)
	return string(b), nil
}

// RunBackgroundCommand starts a detached command and returns its PID.
func (e *E2BTools) RunBackgroundCommand(p CommandParams) (interface{}, error) {
	wrapped := "nohup bash -c " + shQuote(p.Command) + " >/dev/null 2>&1 & echo $!"
	res, err := e.runCommand(context.Background(), wrapped, false)
	if err != nil {
		return errJSON("Error starting background command: %v", err), nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(res.Stdout))
	if err != nil {
		return errJSON("Error starting background command: unexpected output %q", res.Stdout), nil
	}
	b, _ := json.Marshal(map[string]interface{}{"status": "success", "message": "Command started in background.", "pid": pid})
	return string(b), nil
}

// KillBackgroundCommand kills a background command by PID.
func (e *E2BTools) KillBackgroundCommand(p KillParams) (interface{}, error) {
	if p.PID <= 0 {
		return "Invalid command object. Please provide the PID returned from run_background_command.", nil
	}
	res, err := e.runCommand(context.Background(), fmt.Sprintf("kill -9 %d", p.PID), false)
	if err != nil {
		return errJSON("Error killing background command: %v", err), nil
	}
	if res.ExitCode != 0 {
		return errJSON("Error killing background command: %s", strings.TrimSpace(res.Stderr)), nil
	}
	return "Background command terminated successfully.", nil
}

// --- internet access ---

// GetPublicUrl returns the public URL of a sandbox port.
func (e *E2BTools) GetPublicUrl(p PortParams) (interface{}, error) {
	if err := e.ensureSandbox(context.Background()); err != nil {
		return errJSON("Error getting public URL: %v", err), nil
	}
	return "http://" + e.host(p.Port), nil
}

func (e *E2BTools) host(port int) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.domain
	if d == "" {
		d = DefaultDomain
	}
	return fmt.Sprintf("%d-%s.%s", port, e.sandboxID, d)
}

// RunServer starts a server in the background and returns its public URL.
func (e *E2BTools) RunServer(p RunServerParams) (interface{}, error) {
	out, _ := e.RunBackgroundCommand(CommandParams{Command: p.Command})
	if s, _ := out.(string); strings.Contains(s, `"status":"error"`) {
		return errJSON("Error starting server: %s", s), nil
	}
	time.Sleep(2 * time.Second)
	return "http://" + e.host(p.Port), nil
}

// --- sandbox management ---

// SetSandboxTimeout updates the sandbox timeout.
func (e *E2BTools) SetSandboxTimeout(p TimeoutParams) (interface{}, error) {
	ctx := context.Background()
	if err := e.ensureSandbox(ctx); err != nil {
		return errJSON("Error updating sandbox timeout: %v", err), nil
	}
	if _, err := e.api().Do(ctx, http.MethodPost, "/sandboxes/"+url.PathEscape(e.SandboxID())+"/timeout", nil, map[string]int{"timeout": p.Timeout}); err != nil {
		return errJSON("Error updating sandbox timeout: %v", err), nil
	}
	return strconv.Itoa(p.Timeout), nil
}

// SandboxID returns the current sandbox ID (empty before first use).
func (e *E2BTools) SandboxID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sandboxID
}

// GetSandboxStatus returns the sandbox ID.
func (e *E2BTools) GetSandboxStatus(_ NoParams) (interface{}, error) {
	if err := e.ensureSandbox(context.Background()); err != nil {
		return errJSON("Error getting sandbox status: %v", err), nil
	}
	return e.SandboxID(), nil
}

// ShutdownSandbox kills the sandbox.
func (e *E2BTools) ShutdownSandbox(_ NoParams) (interface{}, error) {
	id := e.SandboxID()
	if id == "" {
		return errJSON("Error shutting down sandbox: no sandbox is running"), nil
	}
	if _, err := e.api().Do(context.Background(), http.MethodDelete, "/sandboxes/"+url.PathEscape(id), nil, nil); err != nil {
		return errJSON("Error shutting down sandbox: %v", err), nil
	}
	e.mu.Lock()
	e.sandboxID, e.accessToken, e.domain = "", "", ""
	e.mu.Unlock()
	b, _ := json.Marshal(map[string]interface{}{"status": "success", "message": "Sandbox shut down successfully", "content": true})
	return string(b), nil
}

// ListRunningSandboxes lists the team's running sandboxes.
func (e *E2BTools) ListRunningSandboxes(_ NoParams) (interface{}, error) {
	if e.apiKey == "" {
		return errJSON("Error listing running sandboxes: E2B_API_KEY not set"), nil
	}
	var list []map[string]interface{}
	if err := e.api().JSON(context.Background(), http.MethodGet, "/sandboxes", nil, nil, &list); err != nil {
		return errJSON("Error listing running sandboxes: %v", err), nil
	}
	if len(list) == 0 {
		b, _ := json.Marshal(map[string]interface{}{"status": "success", "message": "No running sandboxes found", "sandboxes": []interface{}{}})
		return string(b), nil
	}
	infos := make([]map[string]interface{}, 0, len(list))
	for _, s := range list {
		md := s["metadata"]
		if md == nil {
			md = map[string]interface{}{}
		}
		infos = append(infos, map[string]interface{}{
			"sandbox_id": s["sandboxID"], "started_at": s["startedAt"], "template_id": s["templateID"], "metadata": md,
		})
	}
	return restapi.ToJSON(map[string]interface{}{
		"status": "success", "message": fmt.Sprintf("Found %d running sandboxes", len(infos)), "sandboxes": infos,
	}), nil
}

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'" }

// preparePythonCode mirrors agno.utils.code_execution.prepare_python_code.
func preparePythonCode(code string) string {
	for _, kw := range [][2]string{{"true", "True"}, {"false", "False"}, {"none", "None"}} {
		code = replaceWord(code, kw[0], kw[1])
	}
	return code
}

func isWordChar(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func replaceWord(s, word, repl string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], word) && (i == 0 || !isWordChar(s[i-1])) &&
			(i+len(word) == len(s) || !isWordChar(s[i+len(word)])) {
			sb.WriteString(repl)
			i += len(word)
			continue
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}
