package tools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"ls -la", []string{"ls", "-la"}},
		{`git commit -m "A & B; C | D"`, []string{"git", "commit", "-m", "A & B; C | D"}},
		{`echo 'it''s' "x\"y"`, []string{"echo", "its", `x"y`}},
		{`echo a\ b \;`, []string{"echo", "a b", ";"}},
		{`echo "$(not run)"`, []string{"echo", "$(not run)"}},
		{`echo $HOME`, []string{"echo", "$HOME"}},
		{"   ", nil},
	}
	for _, c := range cases {
		got, err := SplitCommandLine(c.in)
		if err != nil {
			t.Fatalf("%q: unexpected error %v", c.in, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %#v want %#v", c.in, got, c.want)
		}
	}
}

func TestSplitCommandLineRejectsOperators(t *testing.T) {
	for _, in := range []string{
		"ls; rm -rf /", "cat x | sh", "a && b", "a || b", "sleep 1 &",
		"echo hi > f", "cat < f", "echo `id`", "echo $(id)", "ls\nid",
	} {
		_, err := SplitCommandLine(in)
		if !errors.Is(err, ErrShellOperator) {
			t.Errorf("%q: expected ErrShellOperator, got %v", in, err)
		}
	}
	if _, err := SplitCommandLine(`echo "unterminated`); err == nil {
		t.Error("expected unterminated quote error")
	}
}

func runShell(t *testing.T, st *ShellTool, p ExecuteParams) (ShellResult, error) {
	t.Helper()
	raw, _ := json.Marshal(p)
	res, err := st.Execute("ShellTool_Execute", raw)
	if err != nil {
		return ShellResult{}, err
	}
	return res.(ShellResult), nil
}

func TestShellToolRunsWithoutShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix only")
	}
	dir := t.TempDir()
	st := NewShellTool()

	// Quoted operators reach the program literally and nothing is chained.
	res, err := runShell(t, st, ExecuteParams{Command: `echo "a; touch ` + filepath.Join(dir, "pwned") + `"`})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || !strings.Contains(res.Stdout, "a; touch") {
		t.Fatalf("unexpected result: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Fatal("operator inside quotes was executed")
	}

	// Unquoted operators are rejected before execution.
	if _, err := runShell(t, st, ExecuteParams{Command: "echo hi; touch " + filepath.Join(dir, "pwned")}); !errors.Is(err, ErrShellOperator) {
		t.Fatalf("expected ErrShellOperator, got %v", err)
	}

	// Explicit args are appended as literal argv.
	res, err = runShell(t, st, ExecuteParams{Command: "echo", Args: []string{"$(id)", "|"}})
	if err != nil || strings.TrimSpace(res.Stdout) != "$(id) |" {
		t.Fatalf("args not literal: %+v %v", res, err)
	}
}

func TestShellToolShellModeRequiresOptIn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix only")
	}
	if _, err := runShell(t, NewShellTool(), ExecuteParams{Command: "echo a | tr a b", Shell: true}); err == nil {
		t.Fatal("expected shell mode to be rejected by default")
	}
	res, err := runShell(t, NewShellTool(WithShellMode()), ExecuteParams{Command: "echo a | tr a b", Shell: true})
	if err != nil || strings.TrimSpace(res.Stdout) != "b" {
		t.Fatalf("shell mode opt-in failed: %+v %v", res, err)
	}
	if _, err := runShell(t, NewShellTool(WithShellMode(), WithAllowedCommands("echo")), ExecuteParams{Command: "echo", Shell: true}); err == nil {
		t.Fatal("shell mode with allowlist must be rejected")
	}
}

func TestShellToolAllowlist(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix only")
	}
	st := NewShellTool(WithAllowedCommands("echo"))
	if _, err := runShell(t, st, ExecuteParams{Command: "echo ok"}); err != nil {
		t.Fatalf("allowed command rejected: %v", err)
	}
	if _, err := runShell(t, st, ExecuteParams{Command: "/bin/echo ok"}); err != nil {
		t.Fatalf("allowed command by path rejected: %v", err)
	}
	if _, err := runShell(t, st, ExecuteParams{Command: "ls"}); err == nil || !strings.Contains(err.Error(), "not in the allowed") {
		t.Fatalf("expected allowlist rejection, got %v", err)
	}
}
