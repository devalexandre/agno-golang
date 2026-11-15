package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Tools exposes a Sandbox to agents as a toolkit with RunCommand,
// RunShell, WriteFile, ReadFile and (when a store is set) Checkpoint.
//
//	sb, _ := (&sandbox.LocalProvider{}).Create(ctx, "session-1", sandbox.CreateOptions{})
//	agent.Config{Tools: []toolkit.Tool{sandbox.NewTools(sb, nil)}}
type Tools struct {
	toolkit.Toolkit
	Sandbox Sandbox
	Store   CheckpointStore
	// DefaultTimeout per command (default 60s); MaxTimeout caps model-requested timeouts (default 10m).
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration
	// MaxOutputBytes per stream returned to the model (default 64 KiB).
	MaxOutputBytes int
}

// RunCommandParams are RunCommand's arguments.
type RunCommandParams struct {
	Argv           []string `json:"argv" description:"Program and arguments, e.g. [\"python3\", \"main.py\"]" required:"true"`
	Dir            string   `json:"dir,omitempty" description:"Working directory relative to the workspace root"`
	Stdin          string   `json:"stdin,omitempty" description:"Data written to the command's standard input"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" description:"Timeout in seconds"`
}

// RunShellParams are RunShell's arguments.
type RunShellParams struct {
	Command        string `json:"command" description:"A POSIX shell command line, run with sh -c" required:"true"`
	Dir            string `json:"dir,omitempty" description:"Working directory relative to the workspace root"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" description:"Timeout in seconds"`
}

// WriteFileParams are WriteFile's arguments.
type WriteFileParams struct {
	Path    string `json:"path" description:"File path relative to the workspace root" required:"true"`
	Content string `json:"content" description:"Full file content" required:"true"`
}

// ReadFileParams are ReadFile's arguments.
type ReadFileParams struct {
	Path string `json:"path" description:"File path relative to the workspace root" required:"true"`
}

// CheckpointParams has no arguments.
type CheckpointParams struct{}

// NewTools builds the toolkit. store may be nil (no Checkpoint tool).
func NewTools(sb Sandbox, store CheckpointStore) *Tools {
	t := &Tools{Sandbox: sb, Store: store, DefaultTimeout: 60 * time.Second, MaxTimeout: 10 * time.Minute, MaxOutputBytes: 64 << 10}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "SandboxTools"
	t.Toolkit.Description = "Run commands and manage files inside an isolated sandbox workspace."
	t.Toolkit.Register("RunCommand", "Run a program (argv form) inside the sandbox and return exit code, stdout and stderr.", t, t.RunCommand, RunCommandParams{})
	t.Toolkit.Register("RunShell", "Run a shell command line (sh -c) inside the sandbox and return exit code, stdout and stderr.", t, t.RunShell, RunShellParams{})
	t.Toolkit.Register("WriteFile", "Create or overwrite a file in the sandbox workspace.", t, t.WriteFile, WriteFileParams{})
	t.Toolkit.Register("ReadFile", "Read a file from the sandbox workspace.", t, t.ReadFile, ReadFileParams{})
	if store != nil {
		t.Toolkit.Register("Checkpoint", "Save a checkpoint of the whole sandbox workspace and return its reference.", t, t.Checkpoint, CheckpointParams{})
	}
	return t
}

func (t *Tools) timeout(sec int) time.Duration {
	d := t.DefaultTimeout
	if sec > 0 {
		d = time.Duration(sec) * time.Second
	}
	if t.MaxTimeout > 0 && d > t.MaxTimeout {
		d = t.MaxTimeout
	}
	return d
}

func (t *Tools) exec(argv []string, dir, stdin string, timeoutSec int) (interface{}, error) {
	d := t.timeout(timeoutSec)
	ctx, cancel := context.WithTimeout(context.Background(), d+5*time.Second)
	defer cancel()
	res, err := t.Sandbox.Exec(ctx, argv, ExecOptions{Timeout: d, Dir: dir, Stdin: []byte(stdin), MaxOutputBytes: t.MaxOutputBytes})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// RunCommand runs argv.
func (t *Tools) RunCommand(p RunCommandParams) (interface{}, error) {
	if len(p.Argv) == 0 {
		return nil, errors.New("argv is required")
	}
	return t.exec(p.Argv, p.Dir, p.Stdin, p.TimeoutSeconds)
}

// RunShell runs a shell command line.
func (t *Tools) RunShell(p RunShellParams) (interface{}, error) {
	if p.Command == "" {
		return nil, errors.New("command is required")
	}
	return t.exec([]string{"sh", "-c", p.Command}, p.Dir, "", p.TimeoutSeconds)
}

// WriteFile writes a file.
func (t *Tools) WriteFile(p WriteFileParams) (interface{}, error) {
	if err := t.Sandbox.PutFile(context.Background(), p.Path, []byte(p.Content), 0o644); err != nil {
		return nil, err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(p.Content), p.Path), nil
}

// ReadFile reads a file (truncated to MaxOutputBytes).
func (t *Tools) ReadFile(p ReadFileParams) (interface{}, error) {
	b, err := t.Sandbox.GetFile(context.Background(), p.Path)
	if err != nil {
		return nil, err
	}
	if t.MaxOutputBytes > 0 && len(b) > t.MaxOutputBytes {
		return string(b[:t.MaxOutputBytes]) + "\n[truncated]", nil
	}
	return string(b), nil
}

// Checkpoint saves the workspace.
func (t *Tools) Checkpoint(CheckpointParams) (interface{}, error) {
	if t.Store == nil {
		return nil, errors.New("no checkpoint store configured")
	}
	return t.Sandbox.Checkpoint(context.Background(), t.Store)
}

// GetName implements toolkit.Tool.
func (t *Tools) GetName() string { return t.Toolkit.GetName() }

// GetDescription implements toolkit.Tool.
func (t *Tools) GetDescription() string { return t.Toolkit.GetDescription() }

// GetParameterStruct implements toolkit.Tool.
func (t *Tools) GetParameterStruct(methodName string) map[string]interface{} {
	return t.Toolkit.GetParameterStruct(t.Toolkit.GetName() + "_" + methodName)
}

// GetMethods implements toolkit.Tool.
func (t *Tools) GetMethods() map[string]toolkit.Method { return t.Toolkit.GetMethods() }

// GetFunction implements toolkit.Tool.
func (t *Tools) GetFunction(methodName string) interface{} {
	return t.Toolkit.GetFunction(t.Toolkit.GetName() + "_" + methodName)
}

// GetDescriptionOfMethod implements toolkit.Tool.
func (t *Tools) GetDescriptionOfMethod(methodName string) string {
	return t.Toolkit.GetDescriptionOfMethod(t.Toolkit.GetName() + "_" + methodName)
}

// Execute implements toolkit.Tool.
func (t *Tools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(t.Toolkit.GetName()+"_"+methodName, input)
}

var _ toolkit.Tool = (*Tools)(nil)
