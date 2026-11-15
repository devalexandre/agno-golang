// AgentOS run endpoint: POST /agents/:id/runs actually runs the agent, either
// streaming Server-Sent Events (the default, as in the Python AgentOS) or
// returning one JSON response when stream=false.
//
// The agent id in the path is the agent's name (or its generated id from
// GET /agents). The body may be JSON or form data.
//
// Runs offline with an in-file stub model (swap in ollama.NewOllamaChat or any
// provider for real answers).
//
//	go run ./cookbook/os/run_endpoint        # serve on http://localhost:7777
//	go run ./cookbook/os/run_endpoint demo   # call both endpoints in-process and exit
//
// Then, from another terminal:
//
//	# streaming (SSE: RunStarted, RunContent..., RunContentCompleted, RunCompleted)
//	curl -N -X POST http://localhost:7777/agents/assistant/runs \
//	     -F message="Tell me a joke" -F session_id=demo
//
//	# non-streaming (single JSON RunResponse)
//	curl -X POST http://localhost:7777/agents/assistant/runs \
//	     -H 'Content-Type: application/json' \
//	     -d '{"message": "Tell me a joke", "stream": false, "session_id": "demo"}'
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	agentOS "github.com/devalexandre/agno-golang/agno/os"
)

func main() {
	assistant, err := agent.NewAgent(agent.AgentConfig{
		Context:      context.Background(),
		Name:         "assistant",
		Instructions: "You are a helpful assistant.",
		Model:        stubModel{},
	})
	if err != nil {
		log.Fatal(err)
	}

	osInstance, err := agentOS.NewAgentOS(agentOS.AgentOSOptions{
		OSID:   "run-endpoint-demo",
		Agents: []*agent.Agent{assistant},
		Settings: &agentOS.AgentOSSettings{
			Port:       7777,
			Host:       "localhost",
			EnableCORS: true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 && os.Args[1] == "demo" {
		demo(osInstance)
		return
	}
	if err := osInstance.Serve(); err != nil {
		log.Fatal(err)
	}
}

// demo performs both calls against the AgentOS router without opening a port.
func demo(osInstance *agentOS.AgentOS) {
	srv := httptest.NewServer(osInstance.GetApp())
	defer srv.Close()
	endpoint := srv.URL + "/agents/assistant/runs"

	fmt.Println("== streaming (form data, stream defaults to true)")
	resp, err := http.PostForm(endpoint, url.Values{"message": {"Tell me a joke"}, "session_id": {"demo"}})
	if err != nil {
		log.Fatal(err)
	}
	printBody(resp)

	fmt.Println("\n== non-streaming (JSON, stream=false)")
	resp, err = http.Post(endpoint, "application/json",
		strings.NewReader(`{"message": "Tell me a joke", "stream": false, "session_id": "demo"}`))
	if err != nil {
		log.Fatal(err)
	}
	printBody(resp)
}

func printBody(resp *http.Response) {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("HTTP %d %s\n%s\n", resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// stubModel answers without a provider and streams its answer word by word.
type stubModel struct{}

const answer = "Why do programmers prefer dark mode? Because light attracts bugs."

func (stubModel) GetID() string { return "stub-model" }

func (stubModel) Invoke(context.Context, []models.Message, ...models.Option) (*models.MessageResponse, error) {
	return &models.MessageResponse{Role: "assistant", Content: answer}, nil
}

func (m stubModel) AInvoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch, errCh := make(chan *models.MessageResponse, 1), make(chan error, 1)
	r, _ := m.Invoke(ctx, msgs, opts...)
	ch <- r
	close(ch)
	close(errCh)
	return ch, errCh
}

func (stubModel) InvokeStream(ctx context.Context, _ []models.Message, opts ...models.Option) error {
	call := &models.CallOptions{}
	for _, o := range opts {
		o(call)
	}
	if call.StreamingFunc == nil {
		return nil
	}
	words := strings.SplitAfter(answer, " ")
	for _, w := range words {
		if err := call.StreamingFunc(ctx, []byte(w)); err != nil {
			return err
		}
	}
	return nil
}

func (m stubModel) AInvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return m.AInvoke(ctx, msgs, opts...)
}
