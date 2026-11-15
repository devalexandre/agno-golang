package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DockerProvider runs one container per session through the docker CLI (no
// SDK dependency). Defaults mirror Python's DockerSandbox hardening: no
// network, dropped capabilities, no-new-privileges, memory/CPU/pids limits.
// The Docker daemon is trusted infrastructure; its socket is never mounted.
type DockerProvider struct {
	// Image to run (required), e.g. "python:3.12-slim" or "alpine:3.20".
	Image string
	// Binary is the docker CLI (default "docker").
	Binary string
	// Network (default "none"); "host" and "container:*" are rejected.
	Network string
	// MemoryMB (default 2048), CPUs (default 2), PidsLimit (default 256).
	MemoryMB  int
	CPUs      float64
	PidsLimit int
	// User to run as inside the container (default image user).
	User string
	// Workdir inside the container (default "/workspace").
	Workdir string
	// Command keeps the container alive (default: a POSIX sh sleep loop).
	Command []string
	// Namespace labels and names containers (default "default").
	Namespace string
	Limits    ArchiveLimits
}

const dockerLabel = "ai.agno.sandbox."

func (p *DockerProvider) defaults() error {
	if p.Image == "" || strings.HasPrefix(p.Image, "-") {
		return errors.New("docker sandbox requires a non-option image")
	}
	if p.Binary == "" {
		p.Binary = "docker"
	}
	if p.Network == "" {
		p.Network = "none"
	}
	if p.Network == "host" || strings.HasPrefix(p.Network, "container:") {
		return errors.New("shared host/container networking is not supported")
	}
	if p.MemoryMB <= 0 {
		p.MemoryMB = 2048
	}
	if p.CPUs <= 0 {
		p.CPUs = 2
	}
	if p.PidsLimit <= 0 {
		p.PidsLimit = 256
	}
	if p.Workdir == "" {
		p.Workdir = "/workspace"
	}
	if !path.IsAbs(p.Workdir) {
		return errors.New("docker sandbox workdir must be absolute")
	}
	if len(p.Command) == 0 {
		p.Command = []string{"sh", "-c", "trap 'exit 0' TERM INT; while :; do sleep 3600 & wait $!; done"}
	}
	if p.Namespace == "" {
		p.Namespace = "default"
	}
	return nil
}

// docker runs the CLI with optional stdin/stdout; stderr is returned in errors.
func (p *DockerProvider) docker(ctx context.Context, stdin io.Reader, stdout io.Writer, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, p.Binary, args...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = stdout, &stderr
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Create starts (or reuses) the session's container.
func (p *DockerProvider) Create(ctx context.Context, sessionID string, opts CreateOptions) (Sandbox, error) {
	if err := p.defaults(); err != nil {
		return nil, err
	}
	if err := ValidateIdentity(sessionID, &opts); err != nil {
		return nil, err
	}
	name := sandboxName(p.Namespace+"-"+sessionID, opts.AgentID, opts.Generation)
	sb := &DockerSandbox{name: name, provider: p, env: opts.Env}

	var state bytes.Buffer
	if err := p.docker(ctx, nil, &state, nil, "container", "inspect", "--format", "{{.State.Running}}", name); err == nil {
		if strings.TrimSpace(state.String()) != "true" {
			if err := p.docker(ctx, nil, io.Discard, nil, "container", "start", name); err != nil {
				return nil, err
			}
		}
		return sb, nil
	}

	args := []string{"container", "run", "-d", "--init", "--name", name,
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--network", p.Network,
		"--memory", strconv.Itoa(p.MemoryMB) + "m", "--memory-swap", strconv.Itoa(p.MemoryMB) + "m",
		"--cpus", strconv.FormatFloat(p.CPUs, 'f', -1, 64), "--pids-limit", strconv.Itoa(p.PidsLimit),
		"--workdir", p.Workdir,
		"--label", dockerLabel + "namespace=" + p.Namespace,
		"--label", dockerLabel + "session=" + sessionID,
		"--label", dockerLabel + "generation=" + strconv.Itoa(opts.Generation),
	}
	if opts.AgentID != "" {
		args = append(args, "--label", dockerLabel+"agent="+opts.AgentID)
	}
	if p.User != "" {
		args = append(args, "--user", p.User)
	}
	// Env values go through the CLI's own environment (-e NAME), so they never
	// appear in the host process list.
	keys, env := envArgs(opts.Env)
	args = append(args, keys...)
	args = append(args, "--entrypoint", p.Command[0], p.Image)
	args = append(args, p.Command[1:]...)
	if err := p.docker(ctx, nil, io.Discard, env, args...); err != nil {
		return nil, err
	}
	return sb, nil
}

func envArgs(m map[string]string) ([]string, []string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var args, env []string
	for _, k := range keys {
		args = append(args, "-e", k)
		env = append(env, k+"="+m[k])
	}
	return args, env
}

// DockerSandbox is a running container.
type DockerSandbox struct {
	name     string
	provider *DockerProvider
	env      map[string]string
	mu       sync.Mutex
	closed   bool
}

// ID implements Sandbox (the container name).
func (s *DockerSandbox) ID() string { return s.name }

func (s *DockerSandbox) check() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	return nil
}

func (s *DockerSandbox) inside(p string) (string, error) {
	name, err := cleanName(strings.TrimPrefix(path.Clean("/"+p), "/"))
	if err != nil {
		return "", err
	}
	if name == "" {
		return s.provider.Workdir, nil
	}
	return path.Join(s.provider.Workdir, name), nil
}

// Exec implements Sandbox. On timeout the docker client is killed; the
// process inside the container may outlive it unless the image's command
// handles signals (Close destroys everything).
func (s *DockerSandbox) Exec(ctx context.Context, argv []string, opts ExecOptions) (*ExecResult, error) {
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
	dir := s.provider.Workdir
	if opts.Dir != "" {
		var err error
		if dir, err = s.inside(opts.Dir); err != nil {
			return nil, err
		}
	}
	merged := map[string]string{}
	for k, v := range s.env {
		merged[k] = v
	}
	for k, v := range opts.Env {
		merged[k] = v
	}
	keys, env := envArgs(merged)
	args := append([]string{"exec", "-i", "-w", dir}, keys...)
	args = append(args, s.name)
	args = append(args, argv...)

	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, s.provider.Binary, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Stdin = bytes.NewReader(opts.Stdin)
	stdout := &limitedBuffer{max: opts.MaxOutputBytes}
	stderr := &limitedBuffer{max: opts.MaxOutputBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	res := &ExecResult{Stdout: stdout.buf.String(), Stderr: stderr.buf.String(), OutputLimited: stdout.limited || stderr.limited}
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
		// 125-127 from `docker exec` itself mean the command never ran.
		if res.ExitCode == 125 {
			return res, fmt.Errorf("docker exec failed: %s", strings.TrimSpace(res.Stderr))
		}
	default:
		return res, err
	}
	return res, nil
}

// PutFile implements Sandbox via `docker cp -` (no shell needed in the image).
func (s *DockerSandbox) PutFile(ctx context.Context, p string, data []byte, mode os.FileMode) error {
	if err := s.check(); err != nil {
		return err
	}
	full, err := s.inside(p)
	if err != nil {
		return err
	}
	if full == s.provider.Workdir {
		return errors.New("path must name a file")
	}
	if mode == 0 {
		mode = 0o644
	}
	rel := strings.TrimPrefix(full, s.provider.Workdir+"/")
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: strings.Join(parts[:i], "/") + "/", Mode: 0o755, ModTime: time.Now()}); err != nil {
			return err
		}
	}
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: rel, Mode: int64(mode.Perm()), Size: int64(len(data)), ModTime: time.Now()}); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return s.provider.docker(ctx, &buf, io.Discard, nil, "cp", "-", s.name+":"+s.provider.Workdir)
}

// GetFile implements Sandbox via `docker cp container:path -`.
func (s *DockerSandbox) GetFile(ctx context.Context, p string) ([]byte, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	full, err := s.inside(p)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := s.provider.docker(ctx, nil, &buf, nil, "cp", s.name+":"+full, "-"); err != nil {
		return nil, err
	}
	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s is not a regular file", p)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(io.LimitReader(tr, hdr.Size))
		}
	}
}

// Checkpoint implements Sandbox: the workspace is copied out with
// `docker cp` and repacked into the canonical archive format.
func (s *DockerSandbox) Checkpoint(ctx context.Context, store CheckpointStore) (CheckpointRef, error) {
	if err := s.check(); err != nil {
		return CheckpointRef{}, err
	}
	raw, rawW := io.Pipe()
	packed, packedW := io.Pipe()
	go func() {
		rawW.CloseWithError(s.provider.docker(ctx, nil, rawW, nil, "cp", s.name+":"+s.provider.Workdir, "-"))
	}()
	go func() { packedW.CloseWithError(Repack(raw, packedW, 1, s.provider.Limits)) }()
	ref, err := store.Put(ctx, packed)
	packed.CloseWithError(err)
	raw.CloseWithError(err)
	return ref, err
}

// Restore implements Sandbox. Clearing the workspace needs `sh` and `rm` in the image.
func (s *DockerSandbox) Restore(ctx context.Context, store CheckpointStore, ref CheckpointRef) error {
	if err := s.check(); err != nil {
		return err
	}
	rc, err := store.Open(ctx, ref)
	if err != nil {
		return err
	}
	defer rc.Close()
	var plain bytes.Buffer
	if err := Unpack(rc, &plain, s.provider.Limits); err != nil {
		return err
	}
	w := s.provider.Workdir
	clear := fmt.Sprintf("rm -rf %[1]s/..?* %[1]s/.[!.]* %[1]s/*", w)
	if err := s.provider.docker(ctx, nil, io.Discard, nil, "exec", s.name, "sh", "-c", clear); err != nil {
		return err
	}
	return s.provider.docker(ctx, &plain, io.Discard, nil, "cp", "-", s.name+":"+w)
}

// Close implements Sandbox by force-removing the container.
func (s *DockerSandbox) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	return s.provider.docker(ctx, nil, io.Discard, nil, "rm", "-f", "-v", s.name)
}
