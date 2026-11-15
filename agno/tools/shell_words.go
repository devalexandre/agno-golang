package tools

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrShellOperator is returned when a command line contains an unquoted shell
// operator while running in safe (shell-less) mode.
var ErrShellOperator = errors.New("shell operator not supported")

// SplitCommandLine tokenizes a command line using POSIX-like quoting rules
// (single quotes, double quotes, backslash escapes) without performing any
// expansion. Unquoted control/substitution operators (; | & < > ` $( and
// newlines) are rejected with ErrShellOperator, since without a shell they
// would be passed as literal arguments; quoted occurrences (e.g. -m "A & B")
// are kept as ordinary text.
func SplitCommandLine(line string) ([]string, error) {
	var (
		args    []string
		cur     strings.Builder
		inWord  bool
		inS     bool // inside '...'
		inD     bool // inside "..."
		escaped bool
	)
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
			inWord = true
		case inS:
			if r == '\'' {
				inS = false
			} else {
				cur.WriteRune(r)
			}
		case inD:
			switch r {
			case '"':
				inD = false
			case '\\':
				// Inside double quotes a backslash only escapes these characters.
				if i+1 < len(runes) && strings.ContainsRune("\"\\$`\n", runes[i+1]) {
					i++
					cur.WriteRune(runes[i])
				} else {
					cur.WriteRune(r)
				}
			default:
				cur.WriteRune(r)
			}
		default:
			switch r {
			case '\\':
				escaped = true
				inWord = true
			case '\'':
				inS = true
				inWord = true
			case '"':
				inD = true
				inWord = true
			case ' ', '\t', '\r':
				if inWord {
					args = append(args, cur.String())
					cur.Reset()
					inWord = false
				}
			case ';', '|', '&', '<', '>', '`', '\n':
				return nil, fmt.Errorf("%w: %q (run separate commands, or enable shell mode for trusted use)", ErrShellOperator, string(r))
			case '$':
				if i+1 < len(runes) && runes[i+1] == '(' {
					return nil, fmt.Errorf("%w: %q (command substitution)", ErrShellOperator, "$(")
				}
				cur.WriteRune(r)
				inWord = true
			default:
				cur.WriteRune(r)
				inWord = true
			}
		}
	}
	if inS || inD {
		return nil, errors.New("unterminated quote in command")
	}
	if escaped {
		return nil, errors.New("trailing backslash in command")
	}
	if inWord {
		args = append(args, cur.String())
	}
	return args, nil
}

// buildCommand validates params against the tool's policy and returns the argv
// to execute directly, or useShell=true when legacy shell mode applies.
func (st *ShellTool) buildCommand(params ExecuteParams) (argv []string, useShell bool, err error) {
	if params.Shell {
		if !st.AllowShell {
			return nil, false, errors.New("shell mode is disabled for this tool: commands run without a shell. " +
				"Pass the command and arguments directly (operators like |, ;, && and redirects are not supported), " +
				"or create the tool with WithShellMode() for trusted, supervised use")
		}
		if st.AllowedCommands != nil {
			return nil, false, errors.New("shell mode cannot be used together with an allowed-commands list")
		}
		return nil, true, nil
	}

	argv, err = SplitCommandLine(params.Command)
	if err != nil {
		return nil, false, err
	}
	if len(argv) == 0 {
		return nil, false, errors.New("command is required")
	}
	argv = append(argv, params.Args...)

	if st.AllowedCommands != nil {
		base := filepath.Base(argv[0])
		if runtime.GOOS == "windows" {
			base = strings.TrimSuffix(strings.ToLower(base), ".exe")
		}
		allowed := false
		for _, c := range st.AllowedCommands {
			if c == base {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, false, fmt.Errorf("command %q is not in the allowed commands list", base)
		}
	}
	return argv, false, nil
}
