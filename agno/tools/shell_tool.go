package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// ShellTool provides command execution capabilities
type ShellTool struct {
	toolkit.Toolkit
}

// ShellResult represents the result of command execution
type ShellResult struct {
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	Duration   string `json:"duration"`
	WorkingDir string `json:"working_dir"`
	Operation  string `json:"operation"`
}

// ExecuteParams represents parameters for command execution
type ExecuteParams struct {
	Command    string   `json:"command" description:"Command to execute" required:"true"`
	Args       []string `json:"args,omitempty" description:"Command arguments"`
	WorkingDir string   `json:"working_dir,omitempty" description:"Working directory for command execution"`
	Timeout    int      `json:"timeout,omitempty" description:"Timeout in seconds. Default: 30"`
	Shell      bool     `json:"shell,omitempty" description:"Execute in shell environment. Default: false"`
}

// SystemInfoParams represents parameters for system information
type SystemInfoParams struct {
	InfoType string `json:"info_type" description:"Type of system info: os, env, path, user, disk, memory" required:"true"`
}

// NewShellTool creates a new ShellTool instance
func NewShellTool() *ShellTool {
	tk := toolkit.NewToolkit()
	tk.Name = "ShellTool"
	tk.Description = "A powerful shell tool for executing system commands, managing processes, and retrieving system information. Supports cross-platform command execution with timeout and error handling."

	st := &ShellTool{tk}

	// Register methods
	st.Toolkit.Register("Execute", "Execute shell commands", st, st.Execute, ExecuteParams{})
	st.Toolkit.Register("SystemInfo", "Get system information", st, st.GetSystemInfo, SystemInfoParams{})
	st.Toolkit.Register("ListFiles", "List files in current directory", st, st.ListFiles, struct{}{})
	st.Toolkit.Register("GetCurrentDirectory", "Get current working directory", st, st.GetCurrentDirectory, struct{}{})
	st.Toolkit.Register("ChangeDirectory", "Change current directory", st, st.ChangeDirectory, struct {
		Path string `json:"path" description:"Directory path to change to" required:"true"`
	}{})

	return st
}

// Execute implements the toolkit.Tool interface
func (st *ShellTool) Execute(action string, params json.RawMessage) (interface{}, error) {
	switch action {
	case "Execute":
		var executeParams ExecuteParams
		if err := json.Unmarshal(params, &executeParams); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		return st.executeCommand(executeParams)
	case "GetSystemInfo":
		var sysParams SystemInfoParams
		if err := json.Unmarshal(params, &sysParams); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		return st.GetSystemInfo(sysParams)
	case "ListProcesses":
		return st.ListProcesses(struct{}{})
	case "GetCurrentDirectory":
		return st.GetCurrentDirectory(struct{}{})
	case "ChangeDirectory":
		var dirParams struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(params, &dirParams); err != nil {
			return nil, fmt.Errorf("failed to parse directory parameter: %w", err)
		}
		return st.ChangeDirectory(struct {
			Path string `json:"path" description:"Directory path to change to" required:"true"`
		}{Path: dirParams.Path})
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// ExecuteCommand runs a command in the system shell
func (st *ShellTool) executeCommand(params ExecuteParams) (interface{}, error) {
	if params.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Set default timeout
	if params.Timeout <= 0 {
		params.Timeout = 30
	}

	// Set working directory
	workingDir := params.WorkingDir
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			workingDir = "unknown"
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(params.Timeout)*time.Second)
	defer cancel()

	start := time.Now()
	var cmd *exec.Cmd

	// Prepare command based on shell flag and platform
	if params.Shell {
		// Execute in shell
		switch runtime.GOOS {
		case "windows":
			fullCommand := params.Command
			if len(params.Args) > 0 {
				fullCommand += " " + strings.Join(params.Args, " ")
			}
			cmd = exec.CommandContext(ctx, "cmd", "/C", fullCommand)
		default:
			fullCommand := params.Command
			if len(params.Args) > 0 {
				fullCommand += " " + strings.Join(params.Args, " ")
			}
			cmd = exec.CommandContext(ctx, "sh", "-c", fullCommand)
		}
	} else {
		// Execute directly
		if len(params.Args) > 0 {
			cmd = exec.CommandContext(ctx, params.Command, params.Args...)
		} else {
			cmd = exec.CommandContext(ctx, params.Command)
		}
	}

	// Set working directory
	if params.WorkingDir != "" {
		cmd.Dir = params.WorkingDir
	}

	// Execute command
	stdout, err := cmd.Output()
	duration := time.Since(start)

	result := ShellResult{
		Command:    params.Command,
		WorkingDir: workingDir,
		Duration:   duration.String(),
		Operation:  "Execute",
	}

	if err != nil {
		// Handle different types of errors
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Stderr = string(exitError.Stderr)
			result.Success = false
		} else {
			result.Error = fmt.Sprintf("execution failed: %v", err)
			result.Success = false
			result.ExitCode = -1
		}
	} else {
		result.ExitCode = 0
		result.Success = true
	}

	result.Stdout = string(stdout)

	// Truncate output if too long to avoid token overflow
	if len(result.Stdout) > 5000 {
		result.Stdout = result.Stdout[:5000] + "\n[... output truncated ...]"
	}
	if len(result.Stderr) > 2000 {
		result.Stderr = result.Stderr[:2000] + "\n[... error output truncated ...]"
	}

	return result, nil
}

// GetSystemInfo retrieves various system information
func (st *ShellTool) GetSystemInfo(params SystemInfoParams) (interface{}, error) {
	if params.InfoType == "" {
		return nil, fmt.Errorf("info_type is required")
	}

	infoType := strings.ToLower(params.InfoType)

	switch infoType {
	case "os":
		return map[string]interface{}{
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"num_cpu":    runtime.NumCPU(),
			"go_version": runtime.Version(),
			"operation":  "GetSystemInfo",
			"info_type":  "os",
		}, nil

	case "env":
		env := make(map[string]string)
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				env[parts[0]] = parts[1]
			}
		}
		return map[string]interface{}{
			"environment": env,
			"operation":   "GetSystemInfo",
			"info_type":   "env",
		}, nil

	case "path":
		pathVar := os.Getenv("PATH")
		var paths []string
		if runtime.GOOS == "windows" {
			paths = strings.Split(pathVar, ";")
		} else {
			paths = strings.Split(pathVar, ":")
		}
		return map[string]interface{}{
			"path_variable": pathVar,
			"paths":         paths,
			"operation":     "GetSystemInfo",
			"info_type":     "path",
		}, nil

	case "user":
		homeDir, _ := os.UserHomeDir()
		currentDir, _ := os.Getwd()
		return map[string]interface{}{
			"home_directory":    homeDir,
			"current_directory": currentDir,
			"user":              os.Getenv("USER"),
			"username":          os.Getenv("USERNAME"), // Windows
			"operation":         "GetSystemInfo",
			"info_type":         "user",
		}, nil

	case "disk":
		currentDir, _ := os.Getwd()
		// Get disk usage for current directory
		var diskInfo map[string]interface{}

		if runtime.GOOS == "windows" {
			// For Windows, we'd need to use syscalls for accurate disk info
			diskInfo = map[string]interface{}{
				"current_directory": currentDir,
				"note":              "Detailed disk information requires platform-specific implementation",
			}
		} else {
			// For Unix-like systems, we could use statvfs syscall
			diskInfo = map[string]interface{}{
				"current_directory": currentDir,
				"note":              "Detailed disk information requires platform-specific implementation",
			}
		}

		diskInfo["operation"] = "GetSystemInfo"
		diskInfo["info_type"] = "disk"
		return diskInfo, nil

	case "memory":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		return map[string]interface{}{
			"alloc_mb":       bToMb(m.Alloc),
			"total_alloc_mb": bToMb(m.TotalAlloc),
			"sys_mb":         bToMb(m.Sys),
			"num_gc":         m.NumGC,
			"goroutines":     runtime.NumGoroutine(),
			"operation":      "GetSystemInfo",
			"info_type":      "memory",
		}, nil

	default:
		return nil, fmt.Errorf("unsupported info_type: %s. Supported: os, env, path, user, disk, memory", infoType)
	}
}

// ListProcesses lists running processes (simplified version)
func (st *ShellTool) ListProcesses(params struct{}) (interface{}, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("tasklist", "/FO", "CSV")
	case "darwin", "linux":
		cmd = exec.Command("ps", "aux")
	default:
		return nil, fmt.Errorf("process listing not supported on %s", runtime.GOOS)
	}

	output, err := cmd.Output()
	if err != nil {
		return ShellResult{
			Command:   "ps/tasklist",
			Success:   false,
			Error:     fmt.Sprintf("failed to list processes: %v", err),
			Operation: "ListProcesses",
		}, nil
	}

	outputStr := string(output)
	// Truncate if too long
	if len(outputStr) > 8000 {
		outputStr = outputStr[:8000] + "\n[... output truncated ...]"
	}

	return map[string]interface{}{
		"processes": outputStr,
		"operation": "ListProcesses",
		"platform":  runtime.GOOS,
	}, nil
}

// ListFiles lists files in the current directory
func (st *ShellTool) ListFiles(params struct{}) (interface{}, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return ShellResult{
			Success:   false,
			Error:     fmt.Sprintf("failed to get current directory: %v", err),
			Operation: "ListFiles",
		}, nil
	}

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return ShellResult{
			Success:   false,
			Error:     fmt.Sprintf("failed to read directory: %v", err),
			Operation: "ListFiles",
		}, nil
	}

	var files []map[string]interface{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, map[string]interface{}{
			"name":    entry.Name(),
			"is_dir":  entry.IsDir(),
			"size":    info.Size(),
			"mode":    info.Mode().String(),
			"modtime": info.ModTime().Unix(),
		})
	}

	return map[string]interface{}{
		"files":       files,
		"count":       len(files),
		"current_dir": currentDir,
		"operation":   "ListFiles",
		"success":     true,
	}, nil
}

// GetCurrentDirectory gets the current working directory
func (st *ShellTool) GetCurrentDirectory(params struct{}) (interface{}, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return ShellResult{
			Success:   false,
			Error:     fmt.Sprintf("failed to get current directory: %v", err),
			Operation: "GetCurrentDirectory",
		}, nil
	}

	return map[string]interface{}{
		"current_directory": currentDir,
		"absolute_path":     filepath.IsAbs(currentDir),
		"operation":         "GetCurrentDirectory",
	}, nil
}

// ChangeDirectory changes the current working directory
func (st *ShellTool) ChangeDirectory(params struct {
	Path string `json:"path" description:"Directory path to change to" required:"true"`
}) (interface{}, error) {
	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Get current directory before change
	oldDir, _ := os.Getwd()

	err := os.Chdir(params.Path)
	if err != nil {
		return ShellResult{
			Success:   false,
			Error:     fmt.Sprintf("failed to change directory: %v", err),
			Operation: "ChangeDirectory",
		}, nil
	}

	// Get new directory after change
	newDir, _ := os.Getwd()

	return map[string]interface{}{
		"old_directory":  oldDir,
		"new_directory":  newDir,
		"requested_path": params.Path,
		"operation":      "ChangeDirectory",
		"success":        true,
	}, nil
}

// Helper function to convert bytes to megabytes
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
