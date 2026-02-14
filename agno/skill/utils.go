package skill

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// IsSafePath checks if the requested path stays within the base directory.
// This prevents path traversal attacks where a malicious path like
// '../../../etc/passwd' could be used to access files outside the
// intended directory.
func IsSafePath(baseDir string, requestedPath string) bool {
	base, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	full, err := filepath.Abs(filepath.Join(baseDir, requestedPath))
	if err != nil {
		return false
	}
	// Evaluate symlinks to prevent symlink-based traversal
	baseResolved, err := filepath.EvalSymlinks(base)
	if err != nil {
		baseResolved = base
	}
	fullResolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		// If the file doesn't exist yet, just check the path prefix
		fullResolved = full
	}
	return strings.HasPrefix(fullResolved, baseResolved+string(filepath.Separator)) ||
		fullResolved == baseResolved
}

// EnsureExecutable sets the executable bit for the owner on Unix systems.
func EnsureExecutable(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	mode := info.Mode()
	if mode&0100 == 0 {
		return os.Chmod(filePath, mode|0100)
	}
	return nil
}

// ParseShebang extracts the interpreter from a script's shebang line.
// Returns empty string if no valid shebang is found.
func ParseShebang(scriptPath string) string {
	f, err := os.Open(scriptPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return ""
	}
	firstLine := strings.TrimSpace(scanner.Text())

	if !strings.HasPrefix(firstLine, "#!") {
		return ""
	}

	shebang := strings.TrimSpace(firstLine[2:])
	if shebang == "" {
		return ""
	}

	parts := strings.Fields(shebang)

	// Handle /usr/bin/env style shebangs
	if filepath.Base(parts[0]) == "env" {
		for _, part := range parts[1:] {
			if !strings.HasPrefix(part, "-") {
				return part
			}
		}
		return ""
	}

	// Handle direct path shebangs like #!/bin/bash
	return filepath.Base(parts[0])
}

// GetInterpreterCommand maps an interpreter name to a command slice.
func GetInterpreterCommand(interpreter string) []string {
	lower := strings.ToLower(interpreter)
	switch lower {
	case "python", "python3", "python2":
		// Try to find python3 first, then python
		if path, err := exec.LookPath("python3"); err == nil {
			return []string{path}
		}
		if path, err := exec.LookPath("python"); err == nil {
			return []string{path}
		}
		return []string{interpreter}
	default:
		return []string{interpreter}
	}
}

// ScriptResult holds the result of script execution.
type ScriptResult struct {
	Stdout     string
	Stderr     string
	ReturnCode int
}

// RunScript executes a script file with optional arguments and timeout.
func RunScript(scriptPath string, args []string, timeout time.Duration, cwd string) (*ScriptResult, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmdArgs := buildWindowsCommand(scriptPath, args)
		cmd = exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	} else {
		if err := EnsureExecutable(scriptPath); err != nil {
			return nil, fmt.Errorf("failed to make script executable: %w", err)
		}
		allArgs := append([]string{}, args...)
		cmd = exec.CommandContext(ctx, scriptPath, allArgs...)
	}

	if cwd != "" {
		cmd.Dir = cwd
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &ScriptResult{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ReturnCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ReturnCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("script execution timed out after %v", timeout)
		} else {
			return nil, err
		}
	}

	return result, nil
}

// buildWindowsCommand builds the command for executing a script on Windows.
func buildWindowsCommand(scriptPath string, args []string) []string {
	interpreter := ParseShebang(scriptPath)
	if interpreter != "" {
		cmdPrefix := GetInterpreterCommand(interpreter)
		return append(append(cmdPrefix, scriptPath), args...)
	}
	return append([]string{scriptPath}, args...)
}

// ReadFileSafe reads a file's contents safely.
func ReadFileSafe(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
