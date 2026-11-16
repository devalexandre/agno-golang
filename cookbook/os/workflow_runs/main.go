// Team and workflow runs that actually execute.
//
// Shows: POST /workflows/{id}/runs runs the workflow's steps and streams
// WorkflowStarted, Step* events and WorkflowCompleted (SSE), or returns the
// output as JSON with stream=false. POST /teams/{id}/runs runs the team and
// streams TeamRunStarted/TeamRunContent/TeamRunCompleted. Both support
// background=true, resume and cancel like agents.
//
// Needs: Ollama at OLLAMA_HOST (default http://localhost:11434) with
// OLLAMA_MODEL (default llama3.2:latest) for the team; the workflow step is
// plain Go.
//
//	go run ./cookbook/os/workflow_runs
//	curl -N -F message="hello world" -F stream=true localhost:7777/workflows/shout/runs
//	curl -F message="hello world" -F stream=false localhost:7777/workflows/shout/runs
//	curl -N -F message="Plan a trip to Lisbon" localhost:7777/teams/planners/runs
package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	agnoos "github.com/devalexandre/agno-golang/agno/os"
	"github.com/devalexandre/agno-golang/agno/team"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	step, err := v2.NewStep(v2.WithName("upper"), v2.WithExecutor(func(in *v2.StepInput) (*v2.StepOutput, error) {
		return &v2.StepOutput{Content: strings.ToUpper(in.GetMessageAsString())}, nil
	}))
	if err != nil {
		log.Fatal(err)
	}
	shout := v2.NewWorkflow(v2.WithWorkflowID("shout"), v2.WithWorkflowName("shout"), v2.WithWorkflowSteps([]interface{}{step}))

	model, err := ollama.NewOllamaChat(
		models.WithID(env("OLLAMA_MODEL", "llama3.2:latest")),
		models.WithBaseURL(env("OLLAMA_HOST", "http://localhost:11434")),
	)
	if err != nil {
		log.Fatal(err)
	}
	researcher, err := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Model: model, Name: "researcher"})
	if err != nil {
		log.Fatal(err)
	}
	planners := team.NewTeam(team.TeamConfig{Context: context.Background(), Name: "planners", Model: model, Members: []*agent.Agent{researcher}})

	app, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{
		OSID:      "workflow-os",
		Teams:     []*team.Team{planners},
		Workflows: []*v2.Workflow{shout},
		Settings:  &agnoos.AgentOSSettings{Host: "0.0.0.0", Port: 7777, EnableCORS: true, Timeout: 5 * time.Minute},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(app.Serve())
}
