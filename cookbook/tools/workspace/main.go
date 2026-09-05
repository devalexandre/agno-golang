// Workspace toolkit example.
//
// The Workspace toolkit gives an agent read/write/search/shell access to a single
// directory tree, with the guard-rails that make that safe: paths cannot escape the
// root, an existing file cannot be overwritten before it has been read, and the noisy
// directories (.git, node_modules, vendor, bin) are skipped by listings and searches.
//
// Run it with a directory to work on:
//
//	go run ./cookbook/tools/workspace .
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models/factory"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	// Read-only surface: the agent can look around but cannot change anything.
	// Drop Allowed (or list the write ops) to hand it write/edit/move/delete/shell.
	workspace, err := tools.NewWorkspaceWithConfig(tools.WorkspaceConfig{
		Root:    root,
		Allowed: []string{tools.OpRead, tools.OpList, tools.OpSearch},
		Confirm: []string{},
	})
	if err != nil {
		log.Fatalf("Failed to create the workspace: %v", err)
	}
	fmt.Printf("Workspace root: %s\n\n", workspace.Root())

	// The guard-rails hold whether the toolkit is driven by an agent or by hand.
	demoGuardRails(workspace)

	// The model is resolved from a "provider:model-id" string: no provider import needed.
	model, err := factory.GetModel("openai:gpt-4o")
	if err != nil {
		log.Fatalf("Failed to create the model: %v (set OPENAI_API_KEY)", err)
	}

	assistant, err := agent.NewAgent(agent.AgentConfig{
		Context:     context.Background(),
		Name:        "Workspace Explorer",
		Role:        "Codebase Explorer",
		Description: "Explores a local directory and answers questions about its contents",
		Instructions: "You explore the workspace with the provided tools. " +
			"List and search before reading, and quote the file paths you used.",
		Model:         model,
		Tools:         []toolkit.Tool{workspace},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create the agent: %v", err)
	}

	assistant.PrintResponse("What is this project about? Look at the README and the top-level layout.", true, true)
}

// demoGuardRails exercises the toolkit directly to show what it refuses to do.
func demoGuardRails(workspace *tools.Workspace) {
	// A directory listing, with the excluded directories already filtered out.
	listing, err := workspace.ListFiles(tools.WorkspaceListFilesParams{})
	if err != nil {
		log.Fatalf("list_files: %v", err)
	}
	pretty, _ := json.MarshalIndent(listing, "", "  ")
	fmt.Printf("list_files:\n%s\n\n", pretty)

	// Path traversal is rejected: nothing outside the root is reachable.
	if _, err := workspace.ReadFile(tools.WorkspaceReadFileParams{Path: "../../etc/passwd"}); err != nil {
		fmt.Printf("path sandbox: %v\n\n", err)
	}

	// Read-before-write: an unread file cannot be overwritten.
	if _, err := workspace.WriteFile(tools.WorkspaceWriteFileParams{
		Path:      "go.mod",
		Content:   "module something-else\n",
		Overwrite: true,
	}); err != nil {
		fmt.Printf("read-before-write: %v\n\n", err)
	}
}
