// Package daytona ports Agno's DaytonaTools: code and shell execution in a
// persistent Daytona sandbox through the Daytona REST API and its toolbox
// (process execution) endpoints.
//
// Differences from Python: the sandbox ID and working directory are kept on
// the toolkit (Go tools receive no agent session state), and TLS
// verification stays enabled by default (WithDaytonaVerifySSL(false) opts out).
package daytona

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultAPIURL is the Daytona API base URL.
const DefaultAPIURL = "https://app.daytona.io/api"

// DefaultInstructions mirrors Python's DEFAULT_INSTRUCTIONS.
const DefaultInstructions = `You have access to a persistent Daytona sandbox for code execution. The sandbox maintains state across interactions.
Available tools:
- run_code: Execute code in the sandbox
- run_shell_command: Execute shell commands (bash)
- create_file: Create or update files
- read_file: Read file contents
- list_files: List directory contents
- delete_file: Delete files or directories
- change_directory: Change the working directory
MANDATORY: When users ask for code (Python, JavaScript, TypeScript, etc.), you MUST:
1. Write the code
2. Execute it using run_code tool
3. Show the actual output/results
4. Never just provide code without executing it
CRITICAL WORKFLOW:
1. Before running Python scripts, check if required packages are installed
2. Install missing packages with: run_shell_command("pip install package1 package2")
3. When running scripts, capture both output AND errors
4. If a script produces no output, check for errors or add print statements
Remember: Your job is to provide working, executed code examples, not just code snippets!`

const defaultWorkDir = "/home/daytona"

// DaytonaTools runs code in a Daytona sandbox.
type DaytonaTools struct {
	toolkit.Toolkit
	apiKey           string
	apiURL           string
	organizationID   string
	sandboxID        string
	language         string
	target           string
	autoStopInterval int
	osUser           string
	envVars          map[string]string
	labels           map[string]string
	public           *bool
	timeout          time.Duration
	verifySSL        bool
	instructions     string
	httpClient       *http.Client

	mu       sync.Mutex
	sandbox  map[string]interface{}
	workDir  string
	lastCode string
}

// Option configures DaytonaTools.
type Option func(*DaytonaTools)

// WithDaytonaAPIKey sets the API key (default DAYTONA_API_KEY).
func WithDaytonaAPIKey(k string) Option { return func(d *DaytonaTools) { d.apiKey = k } }

// WithDaytonaAPIURL sets the API URL (default DAYTONA_API_URL, else DefaultAPIURL).
func WithDaytonaAPIURL(u string) Option { return func(d *DaytonaTools) { d.apiURL = u } }

// WithDaytonaOrganizationID sets the organisation ID header.
func WithDaytonaOrganizationID(id string) Option {
	return func(d *DaytonaTools) { d.organizationID = id }
}

// WithDaytonaSandboxID reuses an existing sandbox.
func WithDaytonaSandboxID(id string) Option { return func(d *DaytonaTools) { d.sandboxID = id } }

// WithDaytonaLanguage sets the sandbox language: python (default), javascript, typescript.
func WithDaytonaLanguage(l string) Option { return func(d *DaytonaTools) { d.language = l } }

// WithDaytonaTarget sets the sandbox target region.
func WithDaytonaTarget(t string) Option { return func(d *DaytonaTools) { d.target = t } }

// WithDaytonaAutoStopInterval sets the auto-stop interval in minutes (default 60).
func WithDaytonaAutoStopInterval(m int) Option {
	return func(d *DaytonaTools) { d.autoStopInterval = m }
}

// WithDaytonaOSUser sets the sandbox OS user.
func WithDaytonaOSUser(u string) Option { return func(d *DaytonaTools) { d.osUser = u } }

// WithDaytonaEnvVars sets sandbox environment variables.
func WithDaytonaEnvVars(env map[string]string) Option {
	return func(d *DaytonaTools) { d.envVars = env }
}

// WithDaytonaLabels sets sandbox labels.
func WithDaytonaLabels(l map[string]string) Option { return func(d *DaytonaTools) { d.labels = l } }

// WithDaytonaPublic sets whether the sandbox is public.
func WithDaytonaPublic(b bool) Option { return func(d *DaytonaTools) { d.public = &b } }

// WithDaytonaTimeout sets the per-request timeout (default 300s).
func WithDaytonaTimeout(t time.Duration) Option { return func(d *DaytonaTools) { d.timeout = t } }

// WithDaytonaVerifySSL toggles TLS certificate verification (default true).
func WithDaytonaVerifySSL(b bool) Option { return func(d *DaytonaTools) { d.verifySSL = b } }

// WithDaytonaInstructions overrides the toolkit instructions.
func WithDaytonaInstructions(s string) Option { return func(d *DaytonaTools) { d.instructions = s } }

// WithDaytonaHTTPClient replaces the HTTP client.
func WithDaytonaHTTPClient(c *http.Client) Option { return func(d *DaytonaTools) { d.httpClient = c } }

// Parameter structs.
type (
	RunCodeParams struct {
		Code string `json:"code" description:"Code to execute." required:"true"`
	}
	RunShellCommandParams struct {
		Command string `json:"command" description:"Shell command to execute." required:"true"`
	}
	CreateFileParams struct {
		FilePath string `json:"file_path" description:"Path to the file (relative to current directory or absolute)." required:"true"`
		Content  string `json:"content" description:"Content to write to the file." required:"true"`
	}
	FilePathParams struct {
		FilePath string `json:"file_path" description:"Path to the file (relative to current directory or absolute)." required:"true"`
	}
	ListFilesParams struct {
		Directory string `json:"directory,omitempty" description:"Directory to list (defaults to current working directory)."`
	}
	ChangeDirectoryParams struct {
		Directory string `json:"directory" description:"Directory to change to (relative to current directory or absolute)." required:"true"`
	}
)

// NewDaytonaTools creates the Daytona toolkit.
func NewDaytonaTools(opts ...Option) *DaytonaTools {
	d := &DaytonaTools{
		apiKey:           os.Getenv("DAYTONA_API_KEY"),
		apiURL:           os.Getenv("DAYTONA_API_URL"),
		language:         "python",
		autoStopInterval: 60,
		timeout:          300 * time.Second,
		verifySSL:        true,
		workDir:          defaultWorkDir,
	}
	d.Toolkit = toolkit.NewToolkit()
	d.Toolkit.Name = "daytona_tools"
	d.Toolkit.Description = "Execute code and shell commands in a persistent Daytona sandbox."
	for _, o := range opts {
		o(d)
	}
	if d.apiURL == "" {
		d.apiURL = DefaultAPIURL
	}
	if d.instructions == "" {
		d.instructions = DefaultInstructions
	}
	if d.httpClient == nil {
		d.Toolkit.Timeout = d.timeout
		d.httpClient = d.Toolkit.HTTPClient()
		if !d.verifySSL {
			d.httpClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // explicit opt-out
		}
	}
	d.Toolkit.Register("RunCode", "Execute code in the Daytona sandbox (Python by default).", d, d.RunCode, RunCodeParams{})
	d.Toolkit.Register("RunShellCommand", "Execute a shell command in the sandbox. 'cd <dir>' changes the persistent working directory.", d, d.RunShellCommand, RunShellCommandParams{})
	d.Toolkit.Register("CreateFile", "Create or update a file in the sandbox.", d, d.CreateFile, CreateFileParams{})
	d.Toolkit.Register("ReadFile", "Read a file from the sandbox.", d, d.ReadFile, FilePathParams{})
	d.Toolkit.Register("ListFiles", "List files in a directory.", d, d.ListFiles, ListFilesParams{})
	d.Toolkit.RegisterWithOptions("DeleteFile", "Delete a file or directory from the sandbox.", d, d.DeleteFile, FilePathParams{}, toolkit.WithConfirmation())
	d.Toolkit.Register("ChangeDirectory", "Change the current working directory.", d, d.ChangeDirectory, ChangeDirectoryParams{})
	return d
}

// Instructions returns the toolkit instructions.
func (d *DaytonaTools) Instructions() string { return d.instructions }

// SandboxID returns the ID of the sandbox in use (empty before first use).
func (d *DaytonaTools) SandboxID() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.sandboxID
}

// WorkingDirectory returns the persistent working directory.
func (d *DaytonaTools) WorkingDirectory() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.workDir
}

func (d *DaytonaTools) client() *restapi.Client {
	h := map[string]string{"Authorization": "Bearer " + d.apiKey}
	if d.organizationID != "" {
		h["X-Daytona-Organization-ID"] = d.organizationID
	}
	return restapi.New(d.apiURL, d.httpClient, h)
}

// ensureSandbox returns the sandbox (fetching/creating/starting as needed).
func (d *DaytonaTools) ensureSandbox(ctx context.Context) (map[string]interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.apiKey == "" {
		return nil, fmt.Errorf("DAYTONA_API_KEY not set")
	}
	if d.sandbox != nil {
		return d.sandbox, nil
	}
	c := d.client()
	var sb map[string]interface{}
	if d.sandboxID != "" {
		if err := c.JSON(ctx, http.MethodGet, "/sandbox/"+url.PathEscape(d.sandboxID), nil, nil, &sb); err != nil {
			sb = nil
		}
	}
	if sb == nil {
		labels := map[string]string{}
		for k, v := range d.labels {
			labels[k] = v
		}
		setDefault(labels, "created_by", "agno_daytona_toolkit")
		setDefault(labels, "language", d.language)
		setDefault(labels, "persistent", "true")
		setDefault(labels, "code-toolbox-language", d.language)
		body := map[string]interface{}{"labels": labels, "autoStopInterval": d.autoStopInterval}
		if d.osUser != "" {
			body["user"] = d.osUser
		}
		if d.envVars != nil {
			body["env"] = d.envVars
		}
		if d.public != nil {
			body["public"] = *d.public
		}
		if d.target != "" {
			body["target"] = d.target
		}
		if err := c.JSON(ctx, http.MethodPost, "/sandbox", nil, body, &sb); err != nil {
			return nil, fmt.Errorf("error creating Daytona sandbox: %w", err)
		}
	}
	id, _ := sb["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("Daytona API returned a sandbox without id")
	}
	if state, _ := sb["state"].(string); state != "" && state != "started" {
		if _, err := c.Do(ctx, http.MethodPost, "/sandbox/"+url.PathEscape(id)+"/start", nil, nil); err != nil {
			return nil, fmt.Errorf("error starting sandbox %s: %w", id, err)
		}
	}
	d.sandboxID = id
	d.sandbox = sb
	return sb, nil
}

func setDefault(m map[string]string, k, v string) {
	if _, ok := m[k]; !ok {
		m[k] = v
	}
}

type execResult struct {
	ExitCode int    `json:"exitCode"`
	Result   string `json:"result"`
}

// exec runs a command through the sandbox toolbox.
func (d *DaytonaTools) exec(ctx context.Context, command, cwd string) (*execResult, error) {
	sb, err := d.ensureSandbox(ctx)
	if err != nil {
		return nil, err
	}
	id := sb["id"].(string)
	endpoint := "/toolbox/" + url.PathEscape(id) + "/toolbox/process/execute"
	if proxy, _ := sb["toolboxProxyUrl"].(string); proxy != "" {
		endpoint = strings.TrimRight(proxy, "/") + "/" + url.PathEscape(id) + "/process/execute"
	}
	// Like the Daytona SDK, ship the command base64-encoded and pipe it to sh
	// so quoting, pipes and redirects behave as in an interactive shell.
	wrapped := "sh -c " + shQuote("echo "+base64.StdEncoding.EncodeToString([]byte(command))+" | base64 -d | sh")
	body := map[string]interface{}{"command": wrapped, "timeout": int(d.timeout.Seconds())}
	if cwd != "" {
		body["cwd"] = cwd
	}
	var res execResult
	if err := d.client().JSON(ctx, http.MethodPost, endpoint, nil, body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func errJSON(msg string) string {
	b, _ := json.Marshal(map[string]string{"status": "error", "message": msg})
	return string(b)
}

func (d *DaytonaTools) resolve(p string) string {
	if p == "" {
		return d.WorkingDirectory()
	}
	if !path.IsAbs(p) {
		p = path.Join(d.WorkingDirectory(), p)
	}
	return path.Clean(p)
}

// RunCode executes code in the sandbox's language.
func (d *DaytonaTools) RunCode(p RunCodeParams) (interface{}, error) {
	code := p.Code
	var interp string
	switch strings.ToLower(d.language) {
	case "javascript", "js":
		interp = "node -"
	case "typescript", "ts":
		interp = "npx --yes ts-node -"
	default:
		code = preparePythonCode(code)
		interp = "python3 -u -"
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(code))
	cmd := fmt.Sprintf("echo %s | base64 -d | %s", b64, interp)
	res, err := d.exec(context.Background(), cmd, d.WorkingDirectory())
	if err != nil {
		return errJSON(fmt.Sprintf("Error executing code: %v", err)), nil
	}
	d.mu.Lock()
	d.lastCode = res.Result
	d.mu.Unlock()
	return res.Result, nil
}

// RunShellCommand executes a shell command; "cd" updates the working directory.
func (d *DaytonaTools) RunShellCommand(p RunShellCommandParams) (interface{}, error) {
	ctx := context.Background()
	cmd := strings.TrimSpace(p.Command)
	if strings.HasPrefix(cmd, "cd ") {
		target := strings.TrimSpace(cmd[3:])
		newPath := target
		if !path.IsAbs(target) {
			res, err := d.exec(ctx, "cd "+shQuote(d.WorkingDirectory())+" && pwd", "/")
			if err != nil {
				return errJSON(fmt.Sprintf("Error executing command: %v", err)), nil
			}
			newPath = path.Join(strings.TrimSpace(res.Result), target)
		}
		newPath = path.Clean(newPath)
		res, err := d.exec(ctx, "test -d "+shQuote(newPath)+" && echo 'exists' || echo 'not found'", "/")
		if err != nil {
			return errJSON(fmt.Sprintf("Error executing command: %v", err)), nil
		}
		if strings.Contains(res.Result, "exists") {
			d.mu.Lock()
			d.workDir = newPath
			d.mu.Unlock()
			return "Changed directory to: " + newPath, nil
		}
		return fmt.Sprintf("Error: Directory %s not found", newPath), nil
	}
	res, err := d.exec(ctx, p.Command, d.WorkingDirectory())
	if err != nil {
		return errJSON(fmt.Sprintf("Error executing command: %v", err)), nil
	}
	return res.Result, nil
}

// CreateFile writes a file in the sandbox.
func (d *DaytonaTools) CreateFile(p CreateFileParams) (interface{}, error) {
	ctx := context.Background()
	full := d.resolve(p.FilePath)
	if dir := path.Dir(full); dir != "" && dir != "/" {
		res, err := d.exec(ctx, "mkdir -p "+shQuote(dir), "")
		if err != nil {
			return errJSON(fmt.Sprintf("Error creating file: %v", err)), nil
		}
		if res.ExitCode != 0 {
			return errJSON("Failed to create directory: " + res.Result), nil
		}
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(p.Content))
	res, err := d.exec(ctx, "echo "+b64+" | base64 -d > "+shQuote(full), "")
	if err != nil {
		return errJSON(fmt.Sprintf("Error creating file: %v", err)), nil
	}
	if res.ExitCode != 0 {
		return errJSON("Failed to create file: " + res.Result), nil
	}
	return "File created/updated: " + full, nil
}

// ReadFile returns a file's content.
func (d *DaytonaTools) ReadFile(p FilePathParams) (interface{}, error) {
	full := d.resolve(p.FilePath)
	res, err := d.exec(context.Background(), "cat "+shQuote(full), "")
	if err != nil {
		return errJSON(fmt.Sprintf("Error reading file: %v", err)), nil
	}
	if res.ExitCode != 0 {
		return errJSON("Error reading file: " + res.Result), nil
	}
	return res.Result, nil
}

// ListFiles lists a directory with ls -la.
func (d *DaytonaTools) ListFiles(p ListFilesParams) (interface{}, error) {
	full := d.resolve(p.Directory)
	res, err := d.exec(context.Background(), "ls -la "+shQuote(full), "")
	if err != nil {
		return errJSON(fmt.Sprintf("Error listing files: %v", err)), nil
	}
	if res.ExitCode != 0 {
		return errJSON("Error listing directory: " + res.Result), nil
	}
	return fmt.Sprintf("Contents of %s:\n%s", full, res.Result), nil
}

// DeleteFile removes a file or directory.
func (d *DaytonaTools) DeleteFile(p FilePathParams) (interface{}, error) {
	full := d.resolve(p.FilePath)
	res, err := d.exec(context.Background(), "rm -rf "+shQuote(full), "")
	if err != nil {
		return errJSON(fmt.Sprintf("Error deleting file: %v", err)), nil
	}
	if res.ExitCode != 0 {
		return errJSON("Failed to delete: " + res.Result), nil
	}
	return "Deleted: " + full, nil
}

// ChangeDirectory changes the persistent working directory.
func (d *DaytonaTools) ChangeDirectory(p ChangeDirectoryParams) (interface{}, error) {
	return d.RunShellCommand(RunShellCommandParams{Command: "cd " + p.Directory})
}

// shQuote single-quotes s for POSIX shells.
func shQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

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
