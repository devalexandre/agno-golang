// Background runs with resumable SSE and cancellation.
//
// Shows: POST /agents/{id}/runs with background=true. With stream=true the
// run keeps executing when the client disconnects; every SSE event carries an
// event_index, and POST /agents/{id}/runs/{run_id}/resume replays what was
// missed (last_event_index=N, or ?from=N for "start at N") and then tails the
// live run. With stream=false the request answers 202 and the run is polled.
// POST .../cancel records the cancellation; the stored run gets status
// CANCELLED and cancellation_stage PENDING (not started) or EXECUTING.
//
// Needs: Ollama at OLLAMA_HOST (default http://localhost:11434) with
// OLLAMA_MODEL (default llama3.2:latest).
//
//	go run ./cookbook/os/background_runs
//	curl -N -F message="Write a long story" -F stream=true -F background=true -F session_id=s1 localhost:7777/agents/writer/runs
//	# Ctrl+C after a few events, note run_id, then:
//	curl -N -F last_event_index=3 -F session_id=s1 localhost:7777/agents/writer/runs/<run_id>/resume
//	curl -N -X POST "localhost:7777/agents/writer/runs/<run_id>/resume?from=0"
//	curl -F message="hi" -F stream=false -F background=true localhost:7777/agents/writer/runs   # 202
//	curl -X POST localhost:7777/agents/writer/runs/<run_id>/cancel
//	curl localhost:7777/agents/writer/runs/<run_id>
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	agnoos "github.com/devalexandre/agno-golang/agno/os"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	model, err := ollama.NewOllamaChat(
		models.WithID(env("OLLAMA_MODEL", "llama3.2:latest")),
		models.WithBaseURL(env("OLLAMA_HOST", "http://localhost:11434")),
	)
	if err != nil {
		log.Fatal(err)
	}
	writer, err := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Model: model, Name: "writer"})
	if err != nil {
		log.Fatal(err)
	}
	app, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{
		OSID:   "background-os",
		Agents: []*agent.Agent{writer},
		// Runs and their events are stored here; resume falls back to it
		// (with session_id) once a finished run leaves the in-process buffer.
		DB:       agnoos.NewInMemoryDB("runs-db"),
		Settings: &agnoos.AgentOSSettings{Host: "0.0.0.0", Port: 7777, EnableCORS: true, Timeout: 10 * time.Minute},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(app.Serve())
}
