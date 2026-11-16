// Docker sandbox: one hardened container per session, driven through the
// docker CLI (no SDK). No network, dropped capabilities, memory/CPU/pids
// limits; files move with `docker cp`; checkpoints are content-addressed
// archives in a filesystem store (swap in SQLCheckpointStore or
// S3CheckpointStore for shared durability).
//
// Requirements: Docker daemon + CLI. Env: SANDBOX_IMAGE (default alpine:3).
// Run: go run ./cookbook/sandbox/docker
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/sandbox"
)

func main() {
	ctx := context.Background()
	image := os.Getenv("SANDBOX_IMAGE")
	if image == "" {
		image = "alpine:3"
	}
	provider := &sandbox.DockerProvider{Image: image, MemoryMB: 256, CPUs: 1}
	sb, err := provider.Create(ctx, "demo-session", sandbox.CreateOptions{Env: map[string]string{"API_TOKEN": "not-in-ps-output"}})
	if err != nil {
		log.Fatal(err)
	}
	defer sb.Close(ctx) // removes the container
	fmt.Println("container:", sb.ID())

	if err := sb.PutFile(ctx, "job/input.txt", []byte("3\n1\n2\n"), 0o644); err != nil {
		log.Fatal(err)
	}
	res, err := sb.Exec(ctx, []string{"sh", "-c", "sort -n job/input.txt > job/sorted.txt && cat job/sorted.txt"}, sandbox.ExecOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("exit=%d stdout=%q\n", res.ExitCode, res.Stdout)

	res, _ = sb.Exec(ctx, []string{"sh", "-c", "wget -q -T 2 -O- http://example.com || echo 'no network (expected)'"}, sandbox.ExecOptions{})
	fmt.Print("network check: ", res.Stdout)

	res, _ = sb.Exec(ctx, []string{"sleep", "30"}, sandbox.ExecOptions{Timeout: time.Second})
	fmt.Printf("timeout honoured: %v\n", res.TimedOut)

	dir, _ := os.MkdirTemp("", "agno-docker-checkpoints-")
	defer os.RemoveAll(dir)
	store, _ := sandbox.NewFileCheckpointStore(dir)
	ref, err := sb.Checkpoint(ctx, store)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("checkpoint %s… (%d bytes)\n", ref.SHA256[:12], ref.Size)

	_ = sb.PutFile(ctx, "job/sorted.txt", []byte("clobbered"), 0o644)
	if err := sb.Restore(ctx, store, ref); err != nil {
		log.Fatal(err)
	}
	b, _ := sb.GetFile(ctx, "job/sorted.txt")
	fmt.Printf("restored sorted.txt: %q\n", b)
}
