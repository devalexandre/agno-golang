// Package scratchpad stores the think/analyze logs used by Agno's
// agent-internal toolkits (knowledge, memory, workflow tools).
//
// Python keeps these lists in run_context.session_state; Go tool methods get
// no run context, so each toolkit owns a State (optionally backed by a map the
// caller supplies, e.g. an agent's session state).
package scratchpad

import (
	"strings"
	"sync"
)

// State is a concurrency-safe session-state map.
type State struct {
	mu   sync.Mutex
	data map[string]interface{}
}

// New returns a State backed by data (a new map when nil).
func New(data map[string]interface{}) *State {
	if data == nil {
		data = map[string]interface{}{}
	}
	return &State{data: data}
}

// AppendLog appends entry to the string list stored under key and returns the
// formatted log: "<header>:\n- a\n- b".
func (s *State) AppendLog(key, header, entry string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, _ := s.data[key].([]string)
	list = append(list, entry)
	s.data[key] = list
	var sb strings.Builder
	sb.WriteString(header + ":")
	for _, e := range list {
		sb.WriteString("\n- " + e)
	}
	return sb.String()
}

// AppendValue appends v to the generic list stored under key.
func (s *State) AppendValue(key string, v interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, _ := s.data[key].([]interface{})
	s.data[key] = append(list, v)
}

// Snapshot returns a shallow copy of the state.
func (s *State) Snapshot() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]interface{}, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

// Reset clears every key.
func (s *State) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.data {
		delete(s.data, k)
	}
}
