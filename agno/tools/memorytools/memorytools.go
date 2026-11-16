// Package memorytools ports Agno's MemoryTools: think / get / add / update /
// delete / analyze over a user's memories stored in a memory database.
//
// Differences from Python:
//   - Tool methods get no run context, so the user ID is fixed on the toolkit
//     (WithUserID; default "default").
//   - The Go memory.UserMemory has no topics field: topics passed to AddMemory /
//     UpdateMemory are echoed in the result but not persisted.
package memorytools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/devalexandre/agno-golang/agno/memory"
	"github.com/devalexandre/agno-golang/agno/tools/internal/scratchpad"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DB is the subset of memory.MemoryDatabase used by the toolkit.
type DB interface {
	CreateUserMemory(ctx context.Context, m *memory.UserMemory) error
	GetUserMemories(ctx context.Context, userID string) ([]*memory.UserMemory, error)
	UpdateUserMemory(ctx context.Context, m *memory.UserMemory) error
	DeleteUserMemory(ctx context.Context, memoryID string) error
}

// MemoryTools provides memory management tools.
type MemoryTools struct {
	toolkit.Toolkit
	Instructions    string
	AddInstructions bool

	db                                        DB
	userID                                    string
	enGet, enAdd, enUpdate, enDelete, enThink bool
	enAnalyze, addFewShot                     bool
	fewShot                                   string
	custom                                    *string
	state                                     *scratchpad.State
}

// Option configures MemoryTools.
type Option func(*MemoryTools)

// WithUserID sets the user whose memories are managed (default "default").
func WithUserID(id string) Option { return func(m *MemoryTools) { m.userID = id } }

// WithGetMemories enables or disables GetMemories (default true).
func WithGetMemories(b bool) Option { return func(m *MemoryTools) { m.enGet = b } }

// WithAddMemory enables or disables AddMemory (default true).
func WithAddMemory(b bool) Option { return func(m *MemoryTools) { m.enAdd = b } }

// WithUpdateMemory enables or disables UpdateMemory (default true).
func WithUpdateMemory(b bool) Option { return func(m *MemoryTools) { m.enUpdate = b } }

// WithDeleteMemory enables or disables DeleteMemory (default true).
func WithDeleteMemory(b bool) Option { return func(m *MemoryTools) { m.enDelete = b } }

// WithThink enables or disables Think (default true).
func WithThink(b bool) Option { return func(m *MemoryTools) { m.enThink = b } }

// WithAnalyze enables or disables Analyze (default true).
func WithAnalyze(b bool) Option { return func(m *MemoryTools) { m.enAnalyze = b } }

// WithAll enables every tool.
func WithAll() Option {
	return func(m *MemoryTools) {
		m.enGet, m.enAdd, m.enUpdate, m.enDelete, m.enThink, m.enAnalyze = true, true, true, true, true, true
	}
}

// WithInstructions replaces the default instructions.
func WithInstructions(s string) Option { return func(m *MemoryTools) { m.custom = &s } }

// WithAddInstructions controls whether instructions are added to the prompt (default true).
func WithAddInstructions(b bool) Option { return func(m *MemoryTools) { m.AddInstructions = b } }

// WithFewShot controls the few-shot examples (Python default add_few_shot=true;
// examples "" uses the defaults).
func WithFewShot(add bool, examples string) Option {
	return func(m *MemoryTools) { m.addFewShot, m.fewShot = add, examples }
}

// WithSessionState backs the logs/operations with the given map.
func WithSessionState(s map[string]interface{}) Option {
	return func(m *MemoryTools) { m.state = scratchpad.New(s) }
}

// ThinkParams are the parameters of Think.
type ThinkParams struct {
	Thought string `json:"thought" description:"Your thought process and reasoning about memory operations." required:"true"`
}

// GetMemoriesParams are the parameters of GetMemories (none).
type GetMemoriesParams struct{}

// AddMemoryParams are the parameters of AddMemory.
type AddMemoryParams struct {
	Memory string   `json:"memory" description:"The memory content to store" required:"true"`
	Topics []string `json:"topics,omitempty" description:"Optional list of topics associated with this memory"`
}

// UpdateMemoryParams are the parameters of UpdateMemory.
type UpdateMemoryParams struct {
	MemoryID string   `json:"memory_id" description:"The ID of the memory to update" required:"true"`
	Memory   string   `json:"memory,omitempty" description:"Updated memory content (if provided)"`
	Topics   []string `json:"topics,omitempty" description:"Updated list of topics (if provided)"`
}

// DeleteMemoryParams are the parameters of DeleteMemory.
type DeleteMemoryParams struct {
	MemoryID string `json:"memory_id" description:"The ID of the memory to delete" required:"true"`
}

// AnalyzeParams are the parameters of Analyze.
type AnalyzeParams struct {
	Analysis string `json:"analysis" description:"Your analysis of the memory operations results." required:"true"`
}

// NewMemoryTools creates the toolkit over db (e.g. a memory/sqlite database).
func NewMemoryTools(db DB, opts ...Option) (*MemoryTools, error) {
	if db == nil {
		return nil, fmt.Errorf("db must be provided when using MemoryTools")
	}
	m := &MemoryTools{
		db: db, userID: "default", AddInstructions: true, addFewShot: true,
		enGet: true, enAdd: true, enUpdate: true, enDelete: true, enThink: true, enAnalyze: true,
	}
	for _, o := range opts {
		o(m)
	}
	if m.state == nil {
		m.state = scratchpad.New(nil)
	}
	if m.custom != nil {
		m.Instructions = *m.custom
	} else {
		m.Instructions = DefaultInstructions
		if m.addFewShot {
			ex := m.fewShot
			if ex == "" {
				ex = FewShotExamples
			}
			m.Instructions += "\n" + ex
		}
	}

	m.Toolkit = toolkit.NewToolkit()
	m.Toolkit.Name = "memory_tools"
	m.Toolkit.Description = "Manage memories about the current user."
	if m.enThink {
		m.Toolkit.Register("Think", "Use this tool as a scratchpad to reason about memory operations, refine your approach, brainstorm memory content, or revise your plan. Call Think whenever you need to figure out what to do next, analyze the user's requirements, plan memory operations, or decide on execution strategy.", m, m.Think, ThinkParams{})
	}
	if m.enGet {
		m.Toolkit.Register("GetMemories", "Use this tool to get a list of memories for the current user from the database.", m, m.GetMemories, GetMemoriesParams{})
	}
	if m.enAdd {
		m.Toolkit.Register("AddMemory", "Use this tool to add a new memory to the database. Returns a JSON string containing the created memory information.", m, m.AddMemory, AddMemoryParams{})
	}
	if m.enUpdate {
		m.Toolkit.Register("UpdateMemory", "Use this tool to update an existing memory in the database. Returns a JSON string containing the updated memory information.", m, m.UpdateMemory, UpdateMemoryParams{})
	}
	if m.enDelete {
		m.Toolkit.Register("DeleteMemory", "Use this tool to delete a memory from the database. Returns a JSON string containing the deletion result.", m, m.DeleteMemory, DeleteMemoryParams{})
	}
	if m.enAnalyze {
		m.Toolkit.Register("Analyze", "Use this tool to evaluate whether the memory operations results are correct and sufficient. If not, go back to Think or use memory operations with refined parameters.", m, m.Analyze, AnalyzeParams{})
	}
	return m, nil
}

func (m *MemoryTools) ctx() (context.Context, context.CancelFunc) {
	t := m.Toolkit.Timeout
	if t <= 0 {
		t = toolkit.DefaultHTTPTimeout
	}
	return context.WithTimeout(context.Background(), t)
}

func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func memoryDict(um *memory.UserMemory, topics []string) map[string]interface{} {
	d := map[string]interface{}{
		"memory_id":  um.ID,
		"memory":     um.Memory,
		"user_id":    um.UserID,
		"updated_at": um.UpdatedAt,
	}
	if topics != nil {
		d["topics"] = topics
	}
	return d
}

func (m *MemoryTools) find(ctx context.Context, id string) (*memory.UserMemory, error) {
	mems, err := m.db.GetUserMemories(ctx, m.userID)
	if err != nil {
		return nil, err
	}
	for _, um := range mems {
		if um != nil && um.ID == id {
			return um, nil
		}
	}
	return nil, nil
}

// Think logs a thought and returns every thought so far.
func (m *MemoryTools) Think(p ThinkParams) (interface{}, error) {
	return m.state.AppendLog("memory_thoughts", "Memory Thoughts", p.Thought), nil
}

// GetMemories lists the user's memories as JSON.
func (m *MemoryTools) GetMemories(GetMemoriesParams) (interface{}, error) {
	ctx, cancel := m.ctx()
	defer cancel()
	mems, err := m.db.GetUserMemories(ctx, m.userID)
	if err != nil {
		return toJSON(map[string]string{"error": err.Error()}), nil
	}
	out := make([]map[string]interface{}, 0, len(mems))
	for _, um := range mems {
		if um != nil {
			out = append(out, memoryDict(um, nil))
		}
	}
	m.state.AppendValue("memory_operations", map[string]interface{}{
		"operation": "get_memories", "success": true, "memories": out, "error": nil,
	})
	return toJSON(out), nil
}

// AddMemory stores a new memory.
func (m *MemoryTools) AddMemory(p AddMemoryParams) (interface{}, error) {
	ctx, cancel := m.ctx()
	defer cancel()
	now := time.Now().UTC()
	um := &memory.UserMemory{ID: uuid.NewString(), UserID: m.userID, Memory: p.Memory, CreatedAt: now, UpdatedAt: now}
	if err := m.db.CreateUserMemory(ctx, um); err != nil {
		return toJSON(map[string]interface{}{"success": false, "operation": "add_memory", "error": err.Error()}), nil
	}
	d := memoryDict(um, p.Topics)
	m.state.AppendValue("memory_operations", map[string]interface{}{
		"operation": "add_memory", "success": true, "memory": d, "error": nil,
	})
	return toJSON(map[string]interface{}{"success": true, "operation": "add_memory", "memory": d}), nil
}

// UpdateMemory updates an existing memory.
func (m *MemoryTools) UpdateMemory(p UpdateMemoryParams) (interface{}, error) {
	ctx, cancel := m.ctx()
	defer cancel()
	existing, err := m.find(ctx, p.MemoryID)
	if err != nil {
		return toJSON(map[string]interface{}{"success": false, "operation": "update_memory", "error": err.Error()}), nil
	}
	if existing == nil {
		return toJSON(map[string]interface{}{"success": false, "operation": "update_memory",
			"error": fmt.Sprintf("Memory with ID %s not found", p.MemoryID)}), nil
	}
	updated := *existing
	if p.Memory != "" {
		updated.Memory = p.Memory
	}
	updated.UpdatedAt = time.Now().UTC()
	if err := m.db.UpdateUserMemory(ctx, &updated); err != nil {
		return toJSON(map[string]interface{}{"success": false, "operation": "update_memory", "error": err.Error()}), nil
	}
	d := memoryDict(&updated, p.Topics)
	m.state.AppendValue("memory_operations", map[string]interface{}{
		"operation": "update_memory", "success": true, "memory": d, "error": nil,
	})
	return toJSON(map[string]interface{}{"success": true, "operation": "update_memory", "memory": d}), nil
}

// DeleteMemory deletes a memory.
func (m *MemoryTools) DeleteMemory(p DeleteMemoryParams) (interface{}, error) {
	ctx, cancel := m.ctx()
	defer cancel()
	existing, err := m.find(ctx, p.MemoryID)
	if err != nil {
		return toJSON(map[string]interface{}{"success": false, "operation": "delete_memory", "error": err.Error()}), nil
	}
	if existing == nil {
		return toJSON(map[string]interface{}{"success": false, "operation": "delete_memory",
			"error": fmt.Sprintf("Memory with ID %s not found", p.MemoryID)}), nil
	}
	if err := m.db.DeleteUserMemory(ctx, p.MemoryID); err != nil {
		return toJSON(map[string]interface{}{"success": false, "operation": "delete_memory", "error": err.Error()}), nil
	}
	d := memoryDict(existing, nil)
	m.state.AppendValue("memory_operations", map[string]interface{}{
		"operation": "delete_memory", "success": true, "memory_id": p.MemoryID, "deleted_memory": d, "error": nil,
	})
	return toJSON(map[string]interface{}{"success": true, "operation": "delete_memory",
		"memory_id": p.MemoryID, "deleted_memory": d}), nil
}

// Analyze logs an analysis and returns every analysis so far.
func (m *MemoryTools) Analyze(p AnalyzeParams) (interface{}, error) {
	return m.state.AppendLog("memory_analysis", "Memory Analysis", p.Analysis), nil
}

// SessionState returns the logs and recorded operations.
func (m *MemoryTools) SessionState() map[string]interface{} { return m.state.Snapshot() }
