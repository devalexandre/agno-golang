package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LocalProvider runs commands as host processes inside a private workspace
// directory. It is a development convenience, NOT an isolation boundary:
// commands run with the host user's privileges and can reach the network and
// any file that user can.
type LocalProvider struct {
	// Root holds per-session workspaces (default os.TempDir()/agno-sandboxes).
	Root string
	// InheritEnv passes the host environment to commands; otherwise only PATH, HOME and LANG.
	InheritEnv bool
	// KeepWorkspace leaves the directory on Close.
	KeepWorkspace bool
	Limits        ArchiveLimits
}

// Create makes (or reuses) the workspace for session/agent/generation.
func (p *LocalProvider) Create(_ context.Context, sessionID string, opts CreateOptions) (Sandbox, error) {
	if err := ValidateIdentity(sessionID, &opts); err != nil {
		return nil, err
	}
	root := p.Root
	if root == "" {
		root = filepath.Join(os.TempDir(), "agno-sandboxes")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	dir := filepath.Join(root, sandboxName(sessionID, opts.AgentID, opts.Generation))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	return &LocalSandbox{dir: real, env: opts.Env, provider: p}, nil
}

// LocalSandbox is a workspace directory on the host.
type LocalSandbox struct {
	dir      string
	env      map[string]string
	provider *LocalProvider
	mu       sync.Mutex
	closed   bool
}

// Dir returns the workspace path.
func (s *LocalSandbox) Dir() string { return s.dir }

// ID implements Sandbox.
func (s *LocalSandbox) ID() string { return s.dir }

func (s *LocalSandbox) check() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	return nil
}

// resolve maps a workspace-relative path to a host path that cannot escape
// the workspace, including through symlinks.
func (s *LocalSandbox) resolve(p string) (string, error) {
	if strings.ContainsRune(p, 0) {
		return "", errors.New("invalid path")
	}
	full := filepath.Join(s.dir, filepath.Clean("/"+p))
	// Resolve the deepest existing ancestor and require it inside the workspace.
	probe := full
	for {
		if _, err := os.Lstat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		probe = parent
	}
	real, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return "", err
	}
	if real != s.dir && !strings.HasPrefix(real, s.dir+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes the sandbox workspace", p)
	}
	return full, nil
}

func (s *LocalSandbox) environ(extra map[string]string) []string {
	env := map[string]string{}
	if s.provider.InheritEnv {
		for _, kv := range os.Environ() {
			if k, v, ok := strings.Cut(kv, "="); ok {
				env[k] = v
			}
		}
	} else {
		for _, k := range []string{"PATH", "LANG"} {
			if v, ok := os.LookupEnv(k); ok {
				env[k] = v
			}
		}
	}
	env["HOME"] = s.dir
	for k, v := range s.env {
		env[k] = v
	}
	for k, v := range extra {
		env[k] = v
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// Exec implements Sandbox.
func (s *LocalSandbox) Exec(ctx context.Context, argv []string, opts ExecOptions) (*ExecResult, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if len(argv) == 0 || argv[0] == "" {
		return nil, errors.New("sandbox: empty command")
	}
	if err := validateEnv(opts.Env); err != nil {
		return nil, err
	}
	opts = opts.withDefaults()
	dir := s.dir
	if opts.Dir != "" {
		var err error
		if dir, err = s.resolve(opts.Dir); err != nil {
			return nil, err
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, s.environ(opts.Env)
	setProcessGroup(cmd)
	cmd.WaitDelay = 2 * time.Second
	if len(opts.Stdin) > 0 {
		cmd.Stdin = strings.NewReader(string(opts.Stdin))
	}
	stdout := &limitedBuffer{max: opts.MaxOutputBytes}
	stderr := &limitedBuffer{max: opts.MaxOutputBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	res := &ExecResult{Stdout: stdout.buf.String(), Stderr: stderr.buf.String(),
		OutputLimited: stdout.limited || stderr.limited}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
		res.TimedOut, res.ExitCode = true, -1
		return res, nil
	}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		return res, err // command could not start
	}
	return res, nil
}

// PutFile implements Sandbox.
func (s *LocalSandbox) PutFile(_ context.Context, path string, data []byte, mode os.FileMode) error {
	if err := s.check(); err != nil {
		return err
	}
	full, err := s.resolve(path)
	if err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		return err
	}
	if fi, err := os.Lstat(full); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write through symlink %q", path)
	}
	return os.WriteFile(full, data, mode)
}

// GetFile implements Sandbox.
func (s *LocalSandbox) GetFile(_ context.Context, path string) ([]byte, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	full, err := s.resolve(path)
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(full); err != nil {
		return nil, err
	} else if !strings.HasPrefix(real, s.dir+string(os.PathSeparator)) {
		return nil, fmt.Errorf("path %q escapes the sandbox workspace", path)
	}
	return os.ReadFile(full)
}

// Checkpoint implements Sandbox.
func (s *LocalSandbox) Checkpoint(ctx context.Context, store CheckpointStore) (CheckpointRef, error) {
	if err := s.check(); err != nil {
		return CheckpointRef{}, err
	}
	return captureToStore(ctx, s.dir, store, s.provider.Limits)
}

// Restore implements Sandbox.
func (s *LocalSandbox) Restore(ctx context.Context, store CheckpointStore, ref CheckpointRef) error {
	if err := s.check(); err != nil {
		return err
	}
	rc, err := store.Open(ctx, ref)
	if err != nil {
		return err
	}
	defer rc.Close()
	return RestoreDir(rc, s.dir, s.provider.Limits)
}

// Close implements Sandbox.
func (s *LocalSandbox) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.provider.KeepWorkspace {
		return nil
	}
	return os.RemoveAll(s.dir)
}

// sandboxName derives a stable, filesystem/container-safe name.
func sandboxName(sessionID, agentID string, generation int) string {
	clean := func(v string) string {
		var b strings.Builder
		for _, r := range v {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				b.WriteRune(r)
			} else {
				b.WriteRune('-')
			}
			if b.Len() >= 40 {
				break
			}
		}
		return b.String()
	}
	h := shortHash(agentID + "\x00" + sessionID)
	name := "agno-sbx-" + clean(sessionID)
	if agentID != "" {
		name += "-" + clean(agentID)
	}
	return fmt.Sprintf("%s-g%d-%s", name, generation, h)
}
