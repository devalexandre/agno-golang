// Evaluation runs (/eval-runs).
//
// Shows: POST /eval-runs executes an accuracy, performance or reliability
// evaluation (agno/eval) against a registered agent and stores the result;
// GET/PATCH/DELETE manage stored runs. Accuracy uses the agent's model as the
// judge.
//
// Needs: Ollama at OLLAMA_HOST (default http://localhost:11434) with
// OLLAMA_MODEL (default llama3.2:latest).
//
//	go run ./cookbook/os/eval_runs
//	curl -X POST localhost:7777/eval-runs -H 'Content-Type: application/json' \
//	  -d '{"agent_id":"math","eval_type":"accuracy","input":"What is 2+2?","expected_output":"4","name":"sum"}'
//	curl -X POST localhost:7777/eval-runs -H 'Content-Type: application/json' \
//	  -d '{"agent_id":"math","eval_type":"performance","input":"Say hi","num_iterations":3}'
//	curl "localhost:7777/eval-runs?agent_id=math&eval_types=accuracy,performance"
//	curl -X PATCH localhost:7777/eval-runs/<id> -H 'Content-Type: application/json' -d '{"name":"renamed"}'
//	curl -X DELETE localhost:7777/eval-runs -H 'Content-Type: application/json' -d '{"eval_run_ids":["<id>"]}'
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
	math, err := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Model: model, Name: "math"})
	if err != nil {
		log.Fatal(err)
	}
	app, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{
		OSID:     "evals-os",
		Agents:   []*agent.Agent{math},
		Settings: &agnoos.AgentOSSettings{Host: "0.0.0.0", Port: 7777, EnableCORS: true, Timeout: 5 * time.Minute},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(app.Serve())
}
