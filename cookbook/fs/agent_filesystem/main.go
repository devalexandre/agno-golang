// Agent filesystem (agno/fs).
//
// Shows a durable, private filesystem for an agent: a writable notes store on
// local disk, a read-only view of a shared namespace, a per-user templated
// namespace, quotas, and the tools/instructions the agent receives through
// AgentConfig.FileSystems.
//
// Runs fully offline with a stub model. Requires no env vars or services.
// Files are written under ./agent-files (delete it to start over).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/fs"
	"github.com/devalexandre/agno-golang/agno/models"
)

// stubModel answers without calling any provider.
type stubModel struct{}

func (stubModel) Invoke(context.Context, []models.Message, ...models.Option) (*models.MessageResponse, error) {
	return &models.MessageResponse{Role: "assistant", Content: "Noted."}, nil
}
func (stubModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (stubModel) InvokeStream(context.Context, []models.Message, ...models.Option) error { return nil }
func (stubModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (stubModel) GetID() string { return "stub" }

func main() {
	backend := fs.NewLocalBackend("./agent-files")

	// 1) A writable store for this agent's notes, with small quotas.
	notes := fs.MustNew(backend,
		fs.WithNamespace("research/notes"),
		fs.WithMaxFileBytes(64_000),
		fs.WithMaxNamespaceBytes(1_000_000),
		fs.WithAllowDelete(),
	)

	// Programmatic API: write, append with dedupe, check records, search.
	must(notes.Write("decisions.md", "# Decisions\n2026-11-14: use Go for the port\n"))
	must(notes.Append("seen/urls.txt", "https://a.example\nhttps://b.example", true))
	must(notes.Append("seen/urls.txt", "https://b.example\nhttps://c.example", true)) // b is skipped
	seen, _ := notes.Contains([]string{"https://b.example", "https://z.example"}, "seen")
	fmt.Printf("check_lines -> found=%v missing=%v\n", seen.Found, seen.Missing)
	hits, _ := notes.Search("go for", "", 5)
	for _, h := range hits {
		fmt.Printf("search hit: %s line %d: %q\n", h.Path, h.Line, h.Snippet)
	}

	// 2) A read-only view of a namespace another agent maintains.
	shared := fs.MustNew(backend, fs.WithNamespace("research/shared"), fs.WithReadOnly())

	// 3) A per-user store: {user_id} is resolved from the run, never from the model.
	perUser := fs.MustNew(backend, fs.WithNamespace("profiles/{user_id}"), fs.WithToolName("profile"))

	// Tools are what the model sees. Execute one directly to show the output format.
	tools := notes.Tools()
	out, _ := tools.Execute("filesystem_read_file", json.RawMessage(`{"path":"decisions.md"}`))
	fmt.Printf("read_file output:\n%s\n", out)

	// Attach all stores to an agent: each contributes tools and instructions.
	a, err := agent.NewAgent(agent.AgentConfig{
		Context:     context.Background(),
		Model:       stubModel{},
		UserID:      "alice",
		FileSystems: []*fs.FileSystem{notes, shared, perUser},
	})
	if err != nil {
		log.Fatal(err)
	}
	var names []string
	for _, t := range a.GetTools() {
		for m := range t.GetMethods() {
			names = append(names, m)
		}
	}
	sort.Strings(names)
	fmt.Println("agent tools:", names)

	resp, err := a.Run("Remember that we chose Go.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("agent:", resp.TextContent)
}

func must(_ fs.FileMeta, err error) {
	if err != nil {
		log.Fatal(err)
	}
}
