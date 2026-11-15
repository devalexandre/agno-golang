// Package registry ports agno/registry: a component registry that keeps the
// non-serializable objects (models, tools, functions, databases, vector
// databases, knowledge, schemas, agents, teams, workflows) a stored component
// config refers to by id or name, plus the framework-reserved component ids.
//
// Python additionally rehydrates serialized agent/team configs through the
// registry; the Go port has no component serialization yet, so this package
// provides the catalog, deduplication, ambiguity tracking and reserved-id
// enforcement that such rehydration (and AgentOS) builds on.
package registry

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// Framework-owned component ids (Python registry/_reserved.py).
const (
	ManagerTeamID           = "superagent"
	BuilderID               = "platform-builder"
	MiniAgentID             = "miniagent"
	LegacyProductManagerID  = "product-manager-internal"
	FormerRoutineExecutorID = "company-manager-internal"
)

// FormerManagerIDs maps pre-3.2 manager ids to the id that now does their work.
var FormerManagerIDs = map[string]string{
	"os-manager-internal":       ManagerTeamID,
	"platform-builder-internal": BuilderID,
	"platform-manager-internal": ManagerTeamID,
	"company-manager-internal":  ManagerTeamID,
}

// ReservedMemberIDs are built-in member ids.
var ReservedMemberIDs = map[string]bool{BuilderID: true, "platform-builder-internal": true, "platform-manager-internal": true}

// ReservedManagerIDs are every id a user component may not claim.
var ReservedManagerIDs = func() map[string]bool {
	m := map[string]bool{ManagerTeamID: true, MiniAgentID: true, LegacyProductManagerID: true}
	for k := range ReservedMemberIDs {
		m[k] = true
	}
	for k := range FormerManagerIDs {
		m[k] = true
	}
	return m
}()

// IsReservedID reports whether id is framework-owned.
func IsReservedID(id string) bool { return ReservedManagerIDs[id] }

// ReservedIDError is returned when a user component claims a reserved id.
type ReservedIDError struct{ ID string }

func (e *ReservedIDError) Error() string {
	if current, ok := FormerManagerIDs[e.ID]; ok {
		also := ""
		if e.ID == FormerRoutineExecutorID {
			also = fmt.Sprintf(", or '%s' for a Routine executor", MiniAgentID)
		}
		return fmt.Sprintf("Manager ID '%s' was replaced by '%s' in Agno 3.2. Use the new ID%s.", e.ID, current, also)
	}
	return fmt.Sprintf("Component ID '%s' is reserved. Please choose a different ID for your component.", e.ID)
}

// Component is anything with an id (agents, teams, workflows).
type Component interface{ GetID() string }

// Named is anything with a name (knowledge, learning machines, memory managers).
type Named interface{ GetName() string }

// ModelLike is the identity a model exposes (models.AgnoModelInterface satisfies it).
type ModelLike interface{ GetID() string }

// ToolSource says how a tool arrived in the registry.
type ToolSource string

const (
	ToolDeclared   ToolSource = "declared"
	ToolDiscovered ToolSource = "discovered"
)

// ToolLike is the minimal tool surface (toolkit.Tool satisfies it).
type ToolLike interface {
	GetName() string
}

// Registry is the component catalog. Safe for concurrent use.
type Registry struct {
	ID          string
	Name        string
	Description string

	mu              sync.RWMutex
	models          []ModelLike
	tools           []ToolLike
	undeclaredTools map[string]bool
	functions       map[string]interface{}
	dbs             map[string]interface{}
	vectorDBs       map[string]interface{}
	schemas         map[string]reflect.Type
	knowledge       map[string]interface{}
	ambiguousKnow   map[string]bool
	learning        map[string]interface{}
	ambiguousLearn  map[string]bool
	memoryManagers  map[string]interface{}
	agents          map[string]Component
	teams           map[string]Component
	workflows       map[string]Component
	excludedIDs     map[string]bool
}

// New creates an empty registry with the reserved manager ids excluded.
func New(name string) *Registry {
	r := &Registry{ID: uuid.NewString(), Name: name,
		undeclaredTools: map[string]bool{}, functions: map[string]interface{}{}, dbs: map[string]interface{}{},
		vectorDBs: map[string]interface{}{}, schemas: map[string]reflect.Type{}, knowledge: map[string]interface{}{},
		ambiguousKnow: map[string]bool{}, learning: map[string]interface{}{}, ambiguousLearn: map[string]bool{},
		memoryManagers: map[string]interface{}{}, agents: map[string]Component{}, teams: map[string]Component{},
		workflows: map[string]Component{}, excludedIDs: map[string]bool{}}
	for id := range ReservedManagerIDs {
		r.excludedIDs[id] = true
	}
	return r
}

// ReserveComponentIDs excludes ids before any component carries them.
func (r *Registry) ReserveComponentIDs(ids ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if id != "" {
			r.excludedIDs[id] = true
		}
	}
}

// ExcludedComponentIDs returns ids that are served but never building blocks.
func (r *Registry) ExcludedComponentIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return sortedKeys(r.excludedIDs)
}

// IsComponentExcluded reports whether id is excluded.
func (r *Registry) IsComponentExcluded(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.excludedIDs[id]
}

func (r *Registry) addComponent(dst map[string]Component, c Component, kind string) error {
	id := c.GetID()
	if id == "" {
		return fmt.Errorf("registry: %s has no id", kind)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.excludedIDs[id] {
		return &ReservedIDError{ID: id}
	}
	if existing, ok := dst[id]; ok && existing != c {
		return fmt.Errorf("registry: duplicate %s id %q", kind, id)
	}
	dst[id] = c
	return nil
}

// AddAgent registers an agent; reserved ids are refused.
func (r *Registry) AddAgent(c Component) error { return r.addComponent(r.agents, c, "agent") }

// AddTeam registers a team; reserved ids are refused.
func (r *Registry) AddTeam(c Component) error { return r.addComponent(r.teams, c, "team") }

// AddWorkflow registers a workflow; reserved ids are refused.
func (r *Registry) AddWorkflow(c Component) error { return r.addComponent(r.workflows, c, "workflow") }

// GetAgent returns an agent by id.
func (r *Registry) GetAgent(id string) Component { return r.get(r.agents, id) }

// GetTeam returns a team by id.
func (r *Registry) GetTeam(id string) Component { return r.get(r.teams, id) }

// GetWorkflow returns a workflow by id.
func (r *Registry) GetWorkflow(id string) Component { return r.get(r.workflows, id) }

func (r *Registry) get(m map[string]Component, id string) Component {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return m[id]
}

// AgentIDs, TeamIDs, WorkflowIDs list registered ids.
func (r *Registry) AgentIDs() []string    { return r.ids(r.agents) }
func (r *Registry) TeamIDs() []string     { return r.ids(r.teams) }
func (r *Registry) WorkflowIDs() []string { return r.ids(r.workflows) }

func (r *Registry) ids(m map[string]Component) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// AllComponentIDs is the union of agent, team and workflow ids.
func (r *Registry) AllComponentIDs() []string {
	seen := map[string]bool{}
	for _, list := range [][]string{r.AgentIDs(), r.TeamIDs(), r.WorkflowIDs()} {
		for _, id := range list {
			seen[id] = true
		}
	}
	return sortedKeys(seen)
}

func modelIdentity(m ModelLike) string {
	return fmt.Sprintf("%T|%s", m, m.GetID())
}

// AddModel adds a model unless one of the same Go type and id is present.
func (r *Registry) AddModel(m ModelLike) {
	if m == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := modelIdentity(m)
	for _, e := range r.models {
		if e == m || modelIdentity(e) == key {
			return
		}
	}
	r.models = append(r.models, m)
}

// GetModel returns the first model with id (and Go type name, when provider != "").
func (r *Registry) GetModel(id, provider string) ModelLike {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.models {
		if m.GetID() != id {
			continue
		}
		if provider == "" || containsFold(fmt.Sprintf("%T", m), provider) {
			return m
		}
	}
	return nil
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// Models returns the registered models.
func (r *Registry) Models() []ModelLike {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]ModelLike(nil), r.models...)
}

// AddTool adds a tool unless one with the same name is present. A
// declaration always wins over a discovery, whatever the order.
func (r *Registry) AddTool(t ToolLike, source ToolSource) {
	if t == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	name := t.GetName()
	exists := false
	for _, e := range r.tools {
		if e == t || e.GetName() == name {
			exists = true
			break
		}
	}
	if source == ToolDeclared || source == "" {
		delete(r.undeclaredTools, name)
	} else if !exists {
		r.undeclaredTools[name] = true
	}
	if !exists {
		r.tools = append(r.tools, t)
	}
}

// ToolIsDeclared reports whether the tool name is buildable (declared).
func (r *Registry) ToolIsDeclared(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.tools {
		if t.GetName() == name {
			return !r.undeclaredTools[name]
		}
	}
	return false
}

// GetTool returns a tool by name.
func (r *Registry) GetTool(name string) ToolLike {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.tools {
		if t.GetName() == name {
			return t
		}
	}
	return nil
}

// AddFunction registers a plain function under name.
func (r *Registry) AddFunction(name string, fn interface{}) { r.put(r.functions, name, fn) }

// GetFunction returns a function by name.
func (r *Registry) GetFunction(name string) interface{} { return r.lookup(r.functions, name) }

// AddDB registers a database under id.
func (r *Registry) AddDB(id string, db interface{}) { r.put(r.dbs, id, db) }

// GetDB returns a database by id.
func (r *Registry) GetDB(id string) interface{} { return r.lookup(r.dbs, id) }

// AddVectorDB registers a vector database under id.
func (r *Registry) AddVectorDB(id string, v interface{}) { r.put(r.vectorDBs, id, v) }

// GetVectorDB returns a vector database by id.
func (r *Registry) GetVectorDB(id string) interface{} { return r.lookup(r.vectorDBs, id) }

// AddMemoryManager registers a memory manager under id.
func (r *Registry) AddMemoryManager(id string, m interface{}) { r.put(r.memoryManagers, id, m) }

// GetMemoryManager returns a memory manager by id.
func (r *Registry) GetMemoryManager(id string) interface{} { return r.lookup(r.memoryManagers, id) }

// AddSchema registers a struct type (by its Go name) for output/input schemas.
func (r *Registry) AddSchema(example interface{}) {
	t := reflect.TypeOf(example)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Name() == "" {
		return
	}
	r.mu.Lock()
	r.schemas[t.Name()] = t
	r.mu.Unlock()
}

// GetSchema returns a new pointer to a zero value of the named schema.
func (r *Registry) GetSchema(name string) interface{} {
	r.mu.RLock()
	t, ok := r.schemas[name]
	r.mu.RUnlock()
	if !ok {
		return nil
	}
	return reflect.New(t).Interface()
}

func (r *Registry) put(m map[string]interface{}, k string, v interface{}) {
	if k == "" || v == nil {
		return
	}
	r.mu.Lock()
	if _, ok := m[k]; !ok {
		m[k] = v
	}
	r.mu.Unlock()
}

func (r *Registry) lookup(m map[string]interface{}, k string) interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return m[k]
}

func (r *Registry) addNamed(m map[string]interface{}, ambiguous map[string]bool, name string, v interface{}) {
	if name == "" || v == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := m[name]; ok {
		if existing != v {
			ambiguous[name] = true // lenient lookups keep the first
		}
		return
	}
	m[name] = v
}

// AddKnowledge registers a knowledge base by name; a second distinct base with
// the same name marks the name ambiguous.
func (r *Registry) AddKnowledge(name string, k interface{}) {
	r.addNamed(r.knowledge, r.ambiguousKnow, name, k)
}

// GetKnowledge returns the first knowledge registered under name.
func (r *Registry) GetKnowledge(name string) interface{} { return r.lookup(r.knowledge, name) }

// GetKnowledgeStrict refuses an ambiguous name.
func (r *Registry) GetKnowledgeStrict(name string) (interface{}, error) {
	if r.KnowledgeNameIsAmbiguous(name) {
		return nil, fmt.Errorf("registry: knowledge name %q is ambiguous", name)
	}
	return r.GetKnowledge(name), nil
}

// KnowledgeNameIsAmbiguous reports whether two distinct bases claim name.
func (r *Registry) KnowledgeNameIsAmbiguous(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ambiguousKnow[name]
}

// KnowledgeNames lists registered knowledge names.
func (r *Registry) KnowledgeNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return sortedAnyKeys(r.knowledge)
}

// AddLearning registers a learning machine by name.
func (r *Registry) AddLearning(name string, l interface{}) {
	r.addNamed(r.learning, r.ambiguousLearn, name, l)
}

// GetLearning returns the learning machine registered under name.
func (r *Registry) GetLearning(name string) interface{} { return r.lookup(r.learning, name) }

// LearningNameIsAmbiguous reports whether two distinct machines claim name.
func (r *Registry) LearningNameIsAmbiguous(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ambiguousLearn[name]
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedAnyKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
