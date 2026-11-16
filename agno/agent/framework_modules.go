package agent

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/devalexandre/agno-golang/agno/compression"
	"github.com/devalexandre/agno-golang/agno/fs"
	"github.com/devalexandre/agno-golang/agno/metrics"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/offload"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// frameworkModules holds the agent's wiring of agno/fs, agno/offload,
// agno/compression and typed run metrics.
type frameworkModules struct {
	fileSystems    []*fs.FileSystem
	fsTools        []*fs.Tools
	resultStore    *offload.ResultStore
	compression    *compression.Manager
	metricsEnabled bool
}

// applyFrameworkModules wires the AgentConfig framework-module fields into
// the agent: tools, instructions, tool wrappers and the metrics model wrapper.
func (a *Agent) applyFrameworkModules(config AgentConfig) {
	fm := &frameworkModules{
		fileSystems:    config.FileSystems,
		resultStore:    config.OffloadToolResults,
		compression:    config.CompressionManager,
		metricsEnabled: !config.DisableRunMetrics,
	}
	a.framework = fm

	var extraInstructions []string

	// Filesystems: one toolkit per store, names qualified when several.
	usedNames := map[string]int{}
	for i, store := range fm.fileSystems {
		if store == nil {
			continue
		}
		name := store.ToolName()
		if name == "" {
			name = "filesystem"
		}
		if n := usedNames[name]; n > 0 || (i > 0 && store.ToolName() == "") {
			name = fmt.Sprintf("%s_%s", name, toolSafe(store.Namespace()))
			if usedNames[name] > 0 {
				name = fmt.Sprintf("%s_%d", name, i)
			}
		}
		usedNames[name]++
		tk := store.Tools(fs.WithToolkitName(name), fs.WithScopeFunc(a.fsRunScope))
		fm.fsTools = append(fm.fsTools, tk)
		a.tools = append(a.tools, tk)
		extraInstructions = append(extraInstructions, store.Instructions())
	}

	// Compression wraps tools first: whatever is still large is then offloaded.
	if fm.compression != nil {
		for i, t := range a.tools {
			a.tools[i] = fm.compression.Wrap(a.ctxOrBackground(), t, a.currentRunMetrics)
		}
	}

	if fm.resultStore != nil {
		a.tools = offload.WrapAll(a.tools, fm.resultStore, a.offloadScope)
		a.tools = append(a.tools, offload.NewTools(fm.resultStore, a.offloadScope))
		extraInstructions = append(extraInstructions, offload.OffloadInstruction)
	}

	if len(extraInstructions) > 0 {
		parts := append([]string{}, extraInstructions...)
		if strings.TrimSpace(a.instructions) != "" {
			parts = append([]string{a.instructions}, parts...)
		}
		a.instructions = strings.Join(parts, "\n\n")
	}

	if fm.metricsEnabled && a.model != nil {
		if _, already := a.model.(*metricsModel); !already {
			a.model = &metricsModel{AgnoModelInterface: a.model, agent: a, modelType: metrics.ModelTypeModel}
		}
	}
}

func toolSafe(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func (a *Agent) ctxOrBackground() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *Agent) fsRunScope() fs.RunScope {
	return fs.RunScope{UserID: a.userID, AgentID: a.name}
}

func (a *Agent) offloadScope() offload.Scope {
	sc := offload.Scope{SessionID: a.sessionID, UserID: a.userID}
	if a.currentRun != nil {
		sc.RunID = a.currentRun.id
	}
	return sc
}

func (a *Agent) currentRunMetrics() *metrics.RunMetrics {
	if a.currentRun == nil {
		return nil
	}
	return a.currentRun.metrics
}

// newRunMetrics starts a collector for a run (nil when disabled).
func (a *Agent) newRunMetrics() *metrics.RunMetrics {
	if a.framework == nil || !a.framework.metricsEnabled {
		return nil
	}
	m := metrics.NewRunMetrics()
	m.StartTimer()
	return m
}

// attachRunMetrics stops the timer and populates both the typed RunMetrics
// and the Metrics map of the response.
func attachRunMetrics(resp *models.RunResponse, m *metrics.RunMetrics) {
	if m == nil || resp.RunMetrics != nil {
		return
	}
	m.StopTimer()
	resp.RunMetrics = m
	if resp.Metrics == nil {
		resp.Metrics = map[string]interface{}{}
	}
	for k, v := range m.ToMap() {
		if _, exists := resp.Metrics[k]; !exists {
			resp.Metrics[k] = v
		}
	}
}

// RunMetrics returns the typed metrics of the run in progress (nil when idle).
func (a *Agent) RunMetrics() *metrics.RunMetrics { return a.currentRunMetrics() }

// FileSystemTools returns the toolkits built for AgentConfig.FileSystems.
func (a *Agent) FileSystemTools() []*fs.Tools {
	if a.framework == nil {
		return nil
	}
	return a.framework.fsTools
}

// ResultStore returns the offload store (nil when offloading is off).
func (a *Agent) ResultStore() *offload.ResultStore {
	if a.framework == nil {
		return nil
	}
	return a.framework.resultStore
}

// metricsModel accumulates provider-reported usage into the current run.
type metricsModel struct {
	models.AgnoModelInterface
	agent     *Agent
	modelType metrics.ModelType
}

// Unwrap returns the wrapped model.
func (m *metricsModel) Unwrap() models.AgnoModelInterface { return m.AgnoModelInterface }

// GetClientOptions forwards to the wrapped model when it exposes options.
func (m *metricsModel) GetClientOptions() *models.ClientOptions {
	if o, ok := m.AgnoModelInterface.(interface {
		GetClientOptions() *models.ClientOptions
	}); ok {
		return o.GetClientOptions()
	}
	return nil
}

// Invoke implements AgnoModelInterface and records usage.
func (m *metricsModel) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	resp, err := m.AgnoModelInterface.Invoke(ctx, messages, options...)
	if err == nil && resp != nil && resp.Metrics != nil {
		if rm := m.agent.currentRunMetrics(); rm != nil {
			rm.AccumulateModel(resp.Metrics, m.GetID(), ProviderOf(m.AgnoModelInterface), m.modelType)
		}
	}
	return resp, err
}

// ProviderOf derives a provider name from a model's Go package
// (agno/models/openai/chat -> "openai").
func ProviderOf(model interface{}) string {
	t := reflect.TypeOf(model)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil {
		return ""
	}
	pkg := t.PkgPath()
	if pkg == "main" {
		return ""
	}
	if i := strings.Index(pkg, "agno/models/"); i >= 0 {
		rest := pkg[i+len("agno/models/"):]
		return strings.SplitN(rest, "/", 2)[0]
	}
	if i := strings.LastIndex(pkg, "/"); i >= 0 {
		return pkg[i+1:]
	}
	return pkg
}

var _ toolkit.Tool = (*fs.Tools)(nil)
