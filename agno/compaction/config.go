// Package compaction keeps a long conversation inside the model's context
// window: when history grows past a threshold, older runs are folded into a
// generated summary and only the most recent runs are kept verbatim.
//
// It is a port of Python agno.compaction. Only the message list sent to the
// model is rewritten; stored runs are never modified.
package compaction

import (
	"errors"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/models"
)

const (
	// DefaultCompactAtTokens is the default automatic threshold (Python: 150_000).
	DefaultCompactAtTokens = 150_000
	// DefaultUncompactedRuns is the default number of recent runs kept verbatim.
	DefaultUncompactedRuns = 5
	// DefaultMinFoldRatio is the default ratio enforced by EnforceMinFoldRatio.
	DefaultMinFoldRatio = 2.0
	// DefaultSummarizeCharBudget bounds what the summarizer reads in one call
	// (about 100k tokens), trimming the oldest messages first.
	DefaultSummarizeCharBudget = 400_000

	// NoThreshold disables the automatic size trigger when used as CompactAtTokens:
	// compaction then only happens on Agent.Compact or (with OnContextOverflow)
	// when the provider rejects a request as too long.
	NoThreshold = -1
)

// TokenCounter counts the tokens of a request. When it returns a negative
// number the local estimate is used instead.
type TokenCounter func(messages []models.Message) int

// Config configures conversation compaction. Field names follow Python's
// agno.compaction.Compaction. Zero values mean "use the default".
type Config struct {
	// Model writes the summary. Nil uses the agent's model.
	Model models.AgnoModelInterface
	// Instructions are extra guidance appended to the default summarizer prompt.
	Instructions string
	// CompactedTokenBudget is a length budget for the summary in tokens. Zero
	// asks for a compact summary without naming a size and never cuts it.
	CompactedTokenBudget int
	// EnforceTokenBudget cuts a summary that runs over CompactedTokenBudget
	// (at a line break). Nil means true. Has no effect without a budget.
	EnforceTokenBudget *bool

	// CompactAtTokens folds when the request reaches this many tokens.
	// Zero uses DefaultCompactAtTokens, except when OnContextOverflow is set,
	// where zero disables the threshold (as in Python). NoThreshold (-1)
	// disables the automatic trigger explicitly.
	CompactAtTokens int
	// TokenCounter replaces the local estimate (len/4 heuristic) when set.
	TokenCounter TokenCounter

	// UncompactedRuns is the number of recent runs kept verbatim. Zero uses
	// DefaultUncompactedRuns. Mutually exclusive with UncompactedTokens.
	UncompactedRuns int
	// UncompactedTokens keeps the recent history by size instead of by runs.
	UncompactedTokens int

	// OnContextOverflow also folds and retries once when the provider rejects
	// a request as exceeding the context window.
	OnContextOverflow bool
	// EnforceMinFoldRatio skips a fold that would not shrink the context:
	// the folded span must be at least MinFoldRatio times the kept tail.
	EnforceMinFoldRatio bool
	// MinFoldRatio defaults to DefaultMinFoldRatio.
	MinFoldRatio float64
}

// OverflowOnly is the equivalent of Python's compaction=True: no proactive
// threshold, fold and retry once when the provider reports a context overflow.
func OverflowOnly() *Config {
	return &Config{OnContextOverflow: true}
}

// Normalized validates the config and returns a copy with defaults applied.
func (c Config) Normalized() (Config, error) {
	if c.CompactAtTokens == 0 {
		if c.OnContextOverflow {
			c.CompactAtTokens = NoThreshold
		} else {
			c.CompactAtTokens = DefaultCompactAtTokens
		}
	}
	if c.CompactAtTokens < NoThreshold {
		return c, fmt.Errorf("compaction: CompactAtTokens must be positive or NoThreshold, got %d", c.CompactAtTokens)
	}
	if c.UncompactedRuns < 0 {
		return c, fmt.Errorf("compaction: UncompactedRuns must be zero or positive, got %d", c.UncompactedRuns)
	}
	if c.UncompactedTokens < 0 {
		return c, fmt.Errorf("compaction: UncompactedTokens must be zero or positive, got %d", c.UncompactedTokens)
	}
	if c.CompactedTokenBudget < 0 {
		return c, fmt.Errorf("compaction: CompactedTokenBudget must be zero or positive, got %d", c.CompactedTokenBudget)
	}
	if c.MinFoldRatio < 0 {
		return c, fmt.Errorf("compaction: MinFoldRatio must be zero or positive, got %v", c.MinFoldRatio)
	}
	if c.UncompactedTokens > 0 && c.UncompactedRuns > 0 {
		return c, errors.New("compaction: UncompactedRuns and UncompactedTokens cannot both be set - " +
			"they describe the same kept tail in different units")
	}
	if c.UncompactedTokens == 0 && c.UncompactedRuns == 0 {
		c.UncompactedRuns = DefaultUncompactedRuns
	}
	if c.MinFoldRatio == 0 && c.EnforceMinFoldRatio {
		c.MinFoldRatio = DefaultMinFoldRatio
	}
	if c.UncompactedTokens > 0 && c.HasThreshold() && c.UncompactedTokens >= c.CompactAtTokens {
		return c, fmt.Errorf("compaction: UncompactedTokens=%d must be smaller than CompactAtTokens=%d: "+
			"a tail that large holds the whole context, so nothing could fold", c.UncompactedTokens, c.CompactAtTokens)
	}
	return c, nil
}

// HasThreshold reports whether the automatic size trigger is enabled.
func (c Config) HasThreshold() bool { return c.CompactAtTokens > 0 }

func (c Config) enforceBudget() bool {
	return c.EnforceTokenBudget == nil || *c.EnforceTokenBudget
}

func (c Config) foldRatio() float64 {
	if c.EnforceMinFoldRatio {
		return c.MinFoldRatio
	}
	return 0
}

// tailLimit is the largest tail a fold can keep at the threshold (0 = none).
func (c Config) tailLimit() int {
	if !c.HasThreshold() {
		return 0
	}
	return int(float64(c.CompactAtTokens) / (1 + c.foldRatio()))
}

// tailLimitAfter is the tail limit once the overhead sharing the threshold is
// counted: the tail gets half of what is left. Returns -1 when unlimited.
func (c Config) tailLimitAfter(overhead int) int {
	limit := c.tailLimit()
	if limit == 0 {
		return -1
	}
	room := (c.CompactAtTokens - overhead) / 2
	if room < limit {
		limit = room
	}
	if limit < 0 {
		limit = 0
	}
	return limit
}
