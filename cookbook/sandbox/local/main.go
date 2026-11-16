// Local sandbox: a per-session workspace with exec, files, checkpoints and
// the agent toolkit.
//
// The local backend runs host processes in a private directory. It is a
// development convenience, NOT an isolation boundary (use the Docker
// backend for untrusted code). The example drives sandbox.Tools directly,
// exactly as an agent's tool calls would; to give it to an agent, pass
// sandbox.NewTools(sb, store) in the agent's Tools list.
//
// Requirements: a POSIX shell (sh). Checkpoints go to a temp directory.
// Run: go run ./cookbook/sandbox/local
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/sandbox"
)

func main() {
	ctx := context.Background()
	root, err := os.MkdirTemp("", "agno-sandbox-demo-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(root)

	provider := &sandbox.LocalProvider{Root: filepath.Join(root, "workspaces")}
	sb, err := provider.Create(ctx, "session-42", sandbox.CreateOptions{AgentID: "coder", Env: map[string]string{"APP_ENV": "demo"}})
	if err != nil {
		log.Fatal(err)
	}
	defer sb.Close(ctx)

	store, err := sandbox.NewFileCheckpointStore(filepath.Join(root, "checkpoints"))
	if err != nil {
		log.Fatal(err)
	}
	tools := sandbox.NewTools(sb, store)

	call := func(method, args string) any {
		out, err := tools.Execute(method, json.RawMessage(args))
		if err != nil {
			log.Fatalf("%s: %v", method, err)
		}
		return out
	}
	call("WriteFile", `{"path":"app/greet.sh","content":"echo \"hello from $APP_ENV, 6*7=$((6*7))\""}`)
	res := call("RunShell", `{"command":"sh app/greet.sh"}`).(*sandbox.ExecResult)
	fmt.Printf("exit=%d stdout=%q\n", res.ExitCode, res.Stdout)

	ref := call("Checkpoint", `{}`).(sandbox.CheckpointRef)
	fmt.Printf("checkpoint %s… (%d bytes)\n", ref.SHA256[:12], ref.Size)

	call("WriteFile", `{"path":"app/greet.sh","content":"echo broken; exit 1"}`)
	res = call("RunCommand", `{"argv":["sh","app/greet.sh"]}`).(*sandbox.ExecResult)
	fmt.Printf("after bad edit: exit=%d stdout=%q\n", res.ExitCode, res.Stdout)

	if err := sb.Restore(ctx, store, ref); err != nil {
		log.Fatal(err)
	}
	res, _ = sb.Exec(ctx, []string{"sh", "app/greet.sh"}, sandbox.ExecOptions{})
	fmt.Printf("after restore: exit=%d stdout=%q\n", res.ExitCode, res.Stdout)
}
