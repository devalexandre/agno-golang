// Hardened ShellTool: argv execution by default, an optional command
// allowlist, and an explicit opt-in for running through the system shell.
//
//   - NewShellTool(): the command line is split into argv (POSIX-like quoting)
//     and executed directly, with no shell. Unquoted operators
//     (; | & < > ` $( and newlines) are rejected, so model-generated input
//     cannot chain or substitute commands. shell=true is refused.
//   - WithAllowedCommands("ls", "echo"): only these executables (basename of
//     argv[0]) may run.
//   - WithShellMode(): honours shell=true (sh -c / cmd /C) for trusted,
//     supervised use. It cannot be combined with an allowlist.
//
// The tool methods are called directly (the same entry point an agent's tool
// call uses), so no model is needed. Unix-like OS assumed for the commands.
//
//	go run ./cookbook/tools/shell_safe
package main

import (
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/tools"
)

func main() {
	fmt.Println("== default (argv, no shell)")
	def := tools.NewShellTool()
	run(def, tools.ExecuteParams{Command: "echo 'hello world'"})
	run(def, tools.ExecuteParams{Command: "echo", Args: []string{"literal; not an operator"}})
	run(def, tools.ExecuteParams{Command: "echo hi; id"}) // rejected
	run(def, tools.ExecuteParams{Command: "cat /etc/hostname | wc -c"})
	run(def, tools.ExecuteParams{Command: "echo $(whoami)"})
	run(def, tools.ExecuteParams{Command: "echo hi", Shell: true}) // shell mode not enabled

	fmt.Println("\n== WithAllowedCommands(\"echo\", \"ls\")")
	allow := tools.NewShellTool(tools.WithAllowedCommands("echo", "ls"))
	run(allow, tools.ExecuteParams{Command: "/bin/echo allowed by basename"})
	run(allow, tools.ExecuteParams{Command: "uname -a"}) // not in the list

	fmt.Println("\n== WithShellMode() (trusted use only)")
	sh := tools.NewShellTool(tools.WithShellMode())
	run(sh, tools.ExecuteParams{Command: "echo one && echo two | tr a-z A-Z", Shell: true})

	// The word splitter is exported too.
	argv, err := tools.SplitCommandLine(`grep -n "two words" file.txt`)
	fmt.Printf("\nSplitCommandLine -> %q %v\n", argv, err)
}

func run(st *tools.ShellTool, p tools.ExecuteParams) {
	raw, _ := json.Marshal(p)
	out, err := st.Execute("Execute", raw)
	label := p.Command
	if len(p.Args) > 0 {
		label = fmt.Sprintf("%s %q", p.Command, p.Args)
	}
	if p.Shell {
		label += " [shell=true]"
	}
	if err != nil {
		fmt.Printf("%-40s -> rejected: %v\n", label, err)
		return
	}
	res := out.(tools.ShellResult)
	fmt.Printf("%-40s -> exit=%d stdout=%q\n", label, res.ExitCode, res.Stdout)
}
