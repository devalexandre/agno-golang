package querytransformer

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
)

// DefaultHyDEPrompt is the prompt used to write the hypothetical answer. It
// must contain the {query} placeholder.
const DefaultHyDEPrompt = "Write a short passage that answers the question below, as it might appear in a " +
	"reference document. Do not hedge, ask for clarification, or mention that you are " +
	"uncertain: an invented but plausible answer is what is wanted. Reply with the " +
	"passage only.\n\nQuestion: {query}"

// DefaultHyDEMaxCharacters caps the generated passage.
const DefaultHyDEMaxCharacters = 2000

// ErrInvalidPrompt is returned when a HyDE prompt lacks the {query} placeholder.
var ErrInvalidPrompt = errors.New("hyde: prompt must contain the {query} placeholder")

// ErrNoModel is returned when HyDE has no model configured.
var ErrNoModel = errors.New("hyde: a model is required")

// HyDE (Hypothetical Document Embeddings) searches with a hypothetical answer
// instead of the question: an LLM invents a plausible answer, which embeds
// closer to real answers than the question does. Port of agno-python
// knowledge/query_transformer/hyde.py.
//
// Costs one model call per search. If that call fails the original query is
// returned (with a nil error), so a provider outage degrades results rather
// than breaking search.
type HyDE struct {
	// Model writes the hypothetical answer. Required (Python defaults to an
	// OpenAI model; Go asks the caller to choose).
	Model models.AgnoModelInterface
	// Prompt replaces DefaultHyDEPrompt; it must contain {query}.
	Prompt string
	// IncludeQuery searches with the question and the hypothetical answer
	// together ("query\n\nanswer"). Keep it on for keyword/hybrid stores,
	// whose lexical half only matches the words it is given.
	IncludeQuery bool
	// MaxCharacters caps the generated passage (default 2000). It is trimmed
	// on a sentence break, else a word break.
	MaxCharacters int
}

// NewHyDE creates a HyDE transformer with the default prompt and limits.
func NewHyDE(model models.AgnoModelInterface) *HyDE {
	return &HyDE{Model: model, Prompt: DefaultHyDEPrompt, MaxCharacters: DefaultHyDEMaxCharacters}
}

// Transform implements QueryTransformer.
func (h *HyDE) Transform(ctx context.Context, query string) (string, error) {
	if h.Model == nil {
		return query, ErrNoModel
	}
	prompt := h.Prompt
	if prompt == "" {
		prompt = DefaultHyDEPrompt
	}
	if !strings.Contains(prompt, "{query}") {
		return query, ErrInvalidPrompt
	}
	messages := []models.Message{
		// ReplaceAll, not a template: prompts may carry other braces.
		{Role: models.TypeUserRole, Content: strings.ReplaceAll(prompt, "{query}", query)},
	}
	resp, err := h.Model.Invoke(ctx, messages)
	if err != nil {
		log.Printf("[HYDE] could not generate a hypothetical answer, using the query as asked: %v", err)
		return query, nil
	}
	content := ""
	if resp != nil {
		content = resp.Content
	}
	return h.combine(query, content), nil
}

func (h *HyDE) combine(query, hypothetical string) string {
	passage := h.truncate(strings.TrimSpace(hypothetical))
	if passage == "" {
		return query
	}
	if h.IncludeQuery {
		return query + "\n\n" + passage
	}
	return passage
}

// truncate trims to MaxCharacters on a sentence break, else a word break, so
// no word fragment ends up in the text being embedded.
func (h *HyDE) truncate(passage string) string {
	max := h.MaxCharacters
	if max <= 0 {
		max = DefaultHyDEMaxCharacters
	}
	runes := []rune(passage)
	if len(runes) <= max {
		return passage
	}
	window := string(runes[:max])
	end := -1
	for _, sep := range []string{". ", "! ", "? "} {
		if i := strings.LastIndex(window, sep); i > end {
			end = i
		}
	}
	if end > 0 {
		return window[:end+1]
	}
	if i := strings.LastIndex(window, " "); i > 0 {
		return window[:i]
	}
	return window
}

var _ QueryTransformer = (*HyDE)(nil)
