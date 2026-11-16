// Package knowledgetools ports Agno's KnowledgeTools: think / search_knowledge /
// analyze over an agent's knowledge base.
//
// Pass the same knowledge.Knowledge you give the agent (agent.GetKnowledge()).
package knowledgetools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/tools/internal/scratchpad"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Searcher is the part of knowledge.Knowledge the toolkit needs.
type Searcher interface {
	Search(ctx context.Context, query string, numDocuments int) ([]*knowledge.SearchResult, error)
}

// KnowledgeTools provides Think, SearchKnowledge and Analyze.
type KnowledgeTools struct {
	toolkit.Toolkit
	// Instructions are the toolkit instructions (Python default add_instructions=true).
	Instructions    string
	AddInstructions bool

	knowledge     Searcher
	numDocuments  int
	enableThink   bool
	enableSearch  bool
	enableAnalyze bool
	addFewShot    bool
	fewShot       string
	custom        *string
	state         *scratchpad.State
}

// Option configures KnowledgeTools.
type Option func(*KnowledgeTools)

// WithThink enables or disables Think (default true).
func WithThink(b bool) Option { return func(k *KnowledgeTools) { k.enableThink = b } }

// WithSearch enables or disables SearchKnowledge (default true).
func WithSearch(b bool) Option { return func(k *KnowledgeTools) { k.enableSearch = b } }

// WithAnalyze enables or disables Analyze (default true).
func WithAnalyze(b bool) Option { return func(k *KnowledgeTools) { k.enableAnalyze = b } }

// WithAll enables every tool.
func WithAll() Option {
	return func(k *KnowledgeTools) { k.enableThink, k.enableSearch, k.enableAnalyze = true, true, true }
}

// WithInstructions replaces the default instructions.
func WithInstructions(s string) Option { return func(k *KnowledgeTools) { k.custom = &s } }

// WithAddInstructions controls whether instructions are added to the prompt.
func WithAddInstructions(b bool) Option { return func(k *KnowledgeTools) { k.AddInstructions = b } }

// WithFewShot appends few-shot examples ("" uses the defaults).
func WithFewShot(examples string) Option {
	return func(k *KnowledgeTools) { k.addFewShot, k.fewShot = true, examples }
}

// WithNumDocuments sets how many documents a search returns (default 10).
func WithNumDocuments(n int) Option { return func(k *KnowledgeTools) { k.numDocuments = n } }

// WithSessionState backs the thoughts/analysis logs with the given map.
func WithSessionState(m map[string]interface{}) Option {
	return func(k *KnowledgeTools) { k.state = scratchpad.New(m) }
}

// ThinkParams are the parameters of Think.
type ThinkParams struct {
	Thought string `json:"thought" description:"Your thought process and reasoning." required:"true"`
}

// SearchParams are the parameters of SearchKnowledge.
type SearchParams struct {
	Query string `json:"query" description:"The query to search the knowledge base for." required:"true"`
}

// AnalyzeParams are the parameters of Analyze.
type AnalyzeParams struct {
	Analysis string `json:"analysis" description:"A thought to think about and log." required:"true"`
}

// NewKnowledgeTools creates the toolkit. It returns an error when kb is nil
// (Python raises ValueError).
func NewKnowledgeTools(kb Searcher, opts ...Option) (*KnowledgeTools, error) {
	if kb == nil {
		return nil, fmt.Errorf("knowledge must be provided when using KnowledgeTools")
	}
	k := &KnowledgeTools{
		knowledge: kb, numDocuments: 10, AddInstructions: true,
		enableThink: true, enableSearch: true, enableAnalyze: true,
	}
	for _, o := range opts {
		o(k)
	}
	if k.state == nil {
		k.state = scratchpad.New(nil)
	}
	if k.custom != nil {
		k.Instructions = *k.custom
	} else {
		k.Instructions = DefaultInstructions
		if k.addFewShot {
			ex := k.fewShot
			if ex == "" {
				ex = FewShotExamples
			}
			k.Instructions += "\n" + ex
		}
	}

	k.Toolkit = toolkit.NewToolkit()
	k.Toolkit.Name = "knowledge_tools"
	k.Toolkit.Description = "Think, search the knowledge base and analyze the results."
	if k.enableThink {
		k.Toolkit.Register("Think", "Use this tool as a scratchpad to reason about the question, refine your approach, brainstorm search terms, or revise your plan. Call Think whenever you need to figure out what to do next, analyze the user's question, or plan your approach. Returns the full log of reasoning and the new thought.", k, k.Think, ThinkParams{})
	}
	if k.enableSearch {
		k.Toolkit.Register("SearchKnowledge", "Use this tool to search the knowledge base for relevant information. After thinking through the question, use this tool as many times as needed to search for relevant information.", k, k.SearchKnowledge, SearchParams{})
	}
	if k.enableAnalyze {
		k.Toolkit.Register("Analyze", "Use this tool to evaluate whether the returned documents are correct and sufficient. If not, go back to Think or Search with refined queries. Returns the full log of thoughts and the new thought.", k, k.Analyze, AnalyzeParams{})
	}
	return k, nil
}

// Think logs a thought and returns every thought so far.
func (k *KnowledgeTools) Think(p ThinkParams) (interface{}, error) {
	return k.state.AppendLog("thoughts", "Thoughts", p.Thought), nil
}

// SearchKnowledge searches the knowledge base and returns the documents as JSON.
func (k *KnowledgeTools) SearchKnowledge(p SearchParams) (interface{}, error) {
	timeout := k.Toolkit.Timeout
	if timeout <= 0 {
		timeout = toolkit.DefaultHTTPTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	results, err := k.knowledge.Search(ctx, p.Query, k.numDocuments)
	if err != nil {
		return fmt.Sprintf("Error searching knowledge base: %v", err), nil
	}
	docs := make([]interface{}, 0, len(results))
	for _, r := range results {
		if r == nil || r.Document == nil {
			continue
		}
		docs = append(docs, r.Document)
	}
	if len(docs) == 0 {
		return "No documents found", nil
	}
	b, err := json.Marshal(docs)
	if err != nil {
		return fmt.Sprintf("Error searching knowledge base: %v", err), nil
	}
	return string(b), nil
}

// Analyze logs an analysis and returns every analysis so far.
func (k *KnowledgeTools) Analyze(p AnalyzeParams) (interface{}, error) {
	return k.state.AppendLog("analysis", "Analysis", p.Analysis), nil
}

// SessionState returns the thoughts/analysis logs.
func (k *KnowledgeTools) SessionState() map[string]interface{} { return k.state.Snapshot() }

// Reset clears the logs.
func (k *KnowledgeTools) Reset() { k.state.Reset() }
