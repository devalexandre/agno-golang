package os

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Detached run execution and resumable SSE, ported from Python
// agno/run/background.py and agno/os/event_streams.
//
// Every run started through the HTTP API publishes its events into a
// runEventStream. Each event gets a 0-based event_index. A client that loses
// the connection to a background run reconnects to
// POST /{agents|teams|workflows}/{id}/runs/{run_id}/resume with
// last_event_index (or ?from=N, the first index it wants) and receives the
// missed events followed by the live tail.

// eventStreamRetention is how long a finished run's events stay buffered.
const eventStreamRetention = 30 * time.Minute

// sseKeepaliveInterval matches Python SSE_KEEPALIVE_INTERVAL_SECONDS.
var sseKeepaliveInterval = 15 * time.Second

type runEventStream struct {
	mu            sync.Mutex
	runID         string
	sessionID     string
	componentType string // agents, teams, workflows
	componentID   string
	background    bool
	status        models.RunStatus
	stage         models.CancellationStage
	events        []map[string]interface{}
	changed       chan struct{} // closed and replaced on every change
	done          bool
	finishedAt    time.Time
	cancel        context.CancelFunc
	cancelled     bool
	started       bool
}

func newRunEventStream(runID, sessionID, componentType, componentID string, background bool) *runEventStream {
	return &runEventStream{
		runID: runID, sessionID: sessionID, componentType: componentType, componentID: componentID,
		background: background, status: models.RunStatusPending, changed: make(chan struct{}),
	}
}

func (s *runEventStream) notifyLocked() {
	close(s.changed)
	s.changed = make(chan struct{})
}

// publish appends an event, injecting event_index and run_id.
func (s *runEventStream) publish(event map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return
	}
	event["event_index"] = len(s.events)
	if _, ok := event["run_id"]; !ok {
		event["run_id"] = s.runID
	}
	s.events = append(s.events, event)
	s.notifyLocked()
}

func (s *runEventStream) setStatus(status models.RunStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	s.notifyLocked()
}

func (s *runEventStream) complete(status models.RunStatus, stage models.CancellationStage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return
	}
	s.status, s.stage, s.done, s.finishedAt = status, stage, true, time.Now()
	s.notifyLocked()
}

// snapshot returns events with index > lastIndex, the done flag and the
// channel that signals the next change.
func (s *runEventStream) snapshot(lastIndex int) ([]map[string]interface{}, bool, models.RunStatus, chan struct{}, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	start := lastIndex + 1
	if start < 0 {
		start = 0
	}
	var out []map[string]interface{}
	if start < len(s.events) {
		out = append(out, s.events[start:]...)
	}
	return out, s.done, s.status, s.changed, len(s.events)
}

// requestCancel cancels the run; it reports whether the run had started.
func (s *runEventStream) requestCancel() {
	s.mu.Lock()
	s.cancelled = true
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// runRegistry tracks live and recently finished runs.
type runRegistry struct {
	mu      sync.Mutex
	runs    map[string]*runEventStream
	intents map[string]time.Time // cancel requests for runs not registered yet
}

func newRunRegistry() *runRegistry {
	return &runRegistry{runs: map[string]*runEventStream{}, intents: map[string]time.Time{}}
}

func (r *runRegistry) register(s *runEventStream) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for id, old := range r.runs {
		old.mu.Lock()
		expired := old.done && now.Sub(old.finishedAt) > eventStreamRetention
		old.mu.Unlock()
		if expired {
			delete(r.runs, id)
		}
	}
	for id, at := range r.intents {
		if now.Sub(at) > eventStreamRetention {
			delete(r.intents, id)
		}
	}
	if _, ok := r.intents[s.runID]; ok {
		s.cancelled = true
		delete(r.intents, s.runID)
	}
	r.runs[s.runID] = s
}

func (r *runRegistry) get(runID string) *runEventStream {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runs[runID]
}

// cancel cancels a registered run or records the intent for a run that has
// not been registered yet (cancel-before-start).
func (r *runRegistry) cancel(runID string) *runEventStream {
	r.mu.Lock()
	s := r.runs[runID]
	if s == nil {
		r.intents[runID] = time.Now()
	}
	r.mu.Unlock()
	if s != nil {
		s.requestCancel()
	}
	return s
}

// formatSSE renders one event as an SSE frame.
func formatSSE(event map[string]interface{}) []byte {
	name, _ := event["event"].(string)
	if name == "" {
		name = "message"
	}
	data, err := json.Marshal(event)
	if err != nil {
		data = []byte(fmt.Sprintf(`{"event":"error","error":%q}`, err.Error()))
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", name, data))
}
