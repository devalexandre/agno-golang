// Package sandbox ports Agno's sandbox contract (agno.sandbox): a per-session
// compute workspace that can execute commands, move files in and out, and be
// checkpointed to (and restored from) a content-addressed CheckpointStore.
//
// Backends: Local (host process in a private directory; convenience, NOT an
// isolation boundary) and Docker (one container per session, via the docker
// CLI). Checkpoint stores: filesystem, SQL (Postgres/SQLite) and
// S3-compatible object storage. Tools exposes a sandbox to agents.
package sandbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ErrClosed is returned by operations on a closed sandbox.
var ErrClosed = errors.New("sandbox: closed")

// ExecResult is the outcome of one command.
type ExecResult struct {
	ExitCode      int    `json:"exit_code"`
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	TimedOut      bool   `json:"timed_out"`
	OutputLimited bool   `json:"output_limited"`
}

// ExecOptions tunes Exec.
type ExecOptions struct {
	// Timeout for the command (default 60s).
	Timeout time.Duration
	// MaxOutputBytes caps stdout and stderr each (default 1 MiB).
	MaxOutputBytes int
	Stdin          []byte
	// Env adds variables for this command.
	Env map[string]string
	// Dir is the working directory relative to the workspace root.
	Dir string
}

func (o ExecOptions) withDefaults() ExecOptions {
	if o.Timeout <= 0 {
		o.Timeout = 60 * time.Second
	}
	if o.MaxOutputBytes <= 0 {
		o.MaxOutputBytes = 1 << 20
	}
	return o
}

// Sandbox is one session's workspace. Implementations must not retry
// commands whose outcome is uncertain.
type Sandbox interface {
	// ID is the provider reference (directory, container name).
	ID() string
	// Exec runs argv inside the workspace. A non-zero exit is not an error.
	Exec(ctx context.Context, argv []string, opts ExecOptions) (*ExecResult, error)
	// PutFile writes data at path (relative to the workspace root).
	PutFile(ctx context.Context, path string, data []byte, mode os.FileMode) error
	// GetFile reads the file at path (relative to the workspace root).
	GetFile(ctx context.Context, path string) ([]byte, error)
	// Checkpoint archives the workspace into store.
	Checkpoint(ctx context.Context, store CheckpointStore) (CheckpointRef, error)
	// Restore replaces the workspace contents with a checkpoint.
	Restore(ctx context.Context, store CheckpointStore, ref CheckpointRef) error
	// Close releases the sandbox (destroying its compute and workspace).
	Close(ctx context.Context) error
}

// CreateOptions configures a new sandbox.
type CreateOptions struct {
	// AgentID scopes the sandbox name together with the session id.
	AgentID string
	// Generation distinguishes recreated sandboxes for the same session (default 1).
	Generation int
	// Env applies to every command.
	Env map[string]string
}

// Provider creates sandboxes.
type Provider interface {
	Create(ctx context.Context, sessionID string, opts CreateOptions) (Sandbox, error)
}

// ValidateIdentity mirrors Python's create-argument validation.
func ValidateIdentity(sessionID string, opts *CreateOptions) error {
	if sessionID == "" || strings.ContainsRune(sessionID, 0) {
		return errors.New("sandbox session id must be a nonempty string")
	}
	if strings.ContainsRune(opts.AgentID, 0) {
		return errors.New("sandbox agent id must not contain NUL")
	}
	if opts.Generation == 0 {
		opts.Generation = 1
	}
	if opts.Generation < 1 {
		return errors.New("sandbox generation must be a positive integer")
	}
	return validateEnv(opts.Env)
}

func validateEnv(env map[string]string) error {
	for k, v := range env {
		if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
			return fmt.Errorf("sandbox environment must contain valid names and string values (%q)", k)
		}
	}
	return nil
}

// limitedBuffer keeps at most max bytes and records truncation.
type limitedBuffer struct {
	buf     bytes.Buffer
	max     int
	limited bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	room := l.max - l.buf.Len()
	if room <= 0 {
		l.limited = l.limited || len(p) > 0
		return len(p), nil
	}
	if len(p) > room {
		l.buf.Write(p[:room])
		l.limited = true
		return len(p), nil
	}
	return l.buf.Write(p)
}

var _ io.Writer = (*limitedBuffer)(nil)

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:12]
}
