// Package airflow ports Agno's AirflowTools: saves and reads Apache Airflow
// DAG files inside a DAGs directory. Paths are joined safely (relative only,
// no control characters, symlink-aware containment), like Agno's
// safe_join_relative_path.
package airflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// AirflowTools writes and reads DAG files.
type AirflowTools struct {
	toolkit.Toolkit
	dagsDir    string
	enableSave bool
	enableRead bool
	all        bool
}

// Option configures AirflowTools.
type Option func(*AirflowTools)

// WithDagsDir sets the DAGs directory (default: current working directory).
func WithDagsDir(dir string) Option { return func(a *AirflowTools) { a.dagsDir = dir } }

// WithEnableSaveDagFile toggles SaveDagFile (default true).
func WithEnableSaveDagFile(b bool) Option { return func(a *AirflowTools) { a.enableSave = b } }

// WithEnableReadDagFile toggles ReadDagFile (default true).
func WithEnableReadDagFile(b bool) Option { return func(a *AirflowTools) { a.enableRead = b } }

// WithAll enables all methods.
func WithAll() Option { return func(a *AirflowTools) { a.all = true } }

// SaveDagFileParams are the parameters of SaveDagFile.
type SaveDagFileParams struct {
	Contents string `json:"contents" description:"The contents of the DAG." required:"true"`
	DagFile  string `json:"dag_file" description:"The name of the file to save to." required:"true"`
}

// ReadDagFileParams are the parameters of ReadDagFile.
type ReadDagFileParams struct {
	DagFile string `json:"dag_file" description:"The name of the file to read." required:"true"`
}

// New creates the Airflow toolkit.
func New(opts ...Option) *AirflowTools {
	a := &AirflowTools{enableSave: true, enableRead: true}
	a.Toolkit = toolkit.NewToolkit()
	a.Toolkit.Name = "AirflowTools"
	a.Toolkit.Description = "Save and read Apache Airflow DAG files."
	for _, o := range opts {
		o(a)
	}
	if a.dagsDir == "" {
		a.dagsDir, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(a.dagsDir); err == nil {
		a.dagsDir = abs
	}
	if a.all || a.enableSave {
		a.Toolkit.Register("SaveDagFile", "Saves python code for an Airflow DAG to a file called `dag_file` and returns the file path if successful.", a, a.SaveDagFile, SaveDagFileParams{})
	}
	if a.all || a.enableRead {
		a.Toolkit.Register("ReadDagFile", "Reads an Airflow DAG file `dag_file` and returns the contents if successful.", a, a.ReadDagFile, ReadDagFileParams{})
	}
	return a
}

// DagsDir returns the resolved DAGs directory.
func (a *AirflowTools) DagsDir() string { return a.dagsDir }

var reservedNames = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[0-9¹²³]|LPT[0-9¹²³])(\..*)?$`)

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// resolveExisting resolves symlinks of the longest existing prefix of p.
func resolveExisting(p string) string {
	rest := ""
	cur := p
	for {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// safeJoin mirrors agno.utils.path_safety.safe_join_relative_path.
func safeJoin(base, sub string) (string, error) {
	if strings.TrimSpace(sub) == "" {
		return "", fmt.Errorf("Invalid subpath: '%s'", sub)
	}
	if hasControl(sub) {
		return "", fmt.Errorf("Invalid subpath: %q", sub)
	}
	sub = strings.ReplaceAll(sub, `\`, "/")
	if strings.HasPrefix(sub, "/") || (len(sub) >= 2 && sub[1] == ':') {
		return "", fmt.Errorf("Subpath must be relative: '%s'", sub)
	}
	parts := strings.Split(sub, "/")
	for i, seg := range parts {
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		stripped := strings.TrimRight(seg, ". ")
		if stripped == "" || reservedNames.MatchString(stripped) {
			return "", fmt.Errorf("Invalid path segment: '%s'", seg)
		}
		parts[i] = stripped
	}
	resolvedBase := resolveExisting(filepath.Clean(base))
	target := resolveExisting(filepath.Join(base, filepath.FromSlash(strings.Join(parts, "/"))))
	rel, err := filepath.Rel(resolvedBase, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("'%s' resolves outside %s", sub, base)
	}
	return target, nil
}

// SaveDagFile writes contents to dag_file inside the DAGs directory and returns its path.
func (a *AirflowTools) SaveDagFile(p SaveDagFileParams) (interface{}, error) {
	path, err := safeJoin(a.dagsDir, p.DagFile)
	if err != nil {
		return fmt.Sprintf("Error saving to file: %v", err), nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Sprintf("Error saving to file: %v", err), nil
	}
	if err := os.WriteFile(path, []byte(p.Contents), 0o644); err != nil {
		return fmt.Sprintf("Error saving to file: %v", err), nil
	}
	return path, nil
}

// ReadDagFile returns the contents of dag_file inside the DAGs directory.
func (a *AirflowTools) ReadDagFile(p ReadDagFileParams) (interface{}, error) {
	path, err := safeJoin(a.dagsDir, p.DagFile)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err), nil
	}
	return string(data), nil
}

// Execute implements toolkit.Tool.
func (a *AirflowTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return a.Toolkit.Execute(methodName, input)
}
