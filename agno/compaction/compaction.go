package compaction

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/google/uuid"
)

// maxRenderedToolResultChars clips tool results rendered for the summarizer.
const maxRenderedToolResultChars = 20_000

// Compactor folds older history into a summary according to a Config.
// It is safe for concurrent use.
type Compactor struct {
	cfg Config

	mu    sync.Mutex
	stats Stats
}

// New validates cfg and returns a Compactor.
func New(cfg Config) (*Compactor, error) {
	n, err := cfg.Normalized()
	if err != nil {
		return nil, err
	}
	return &Compactor{cfg: n}, nil
}

// Config returns the normalized configuration.
func (c *Compactor) Config() Config { return c.cfg }

// SetDefaultModel sets the summarizer model when none was configured.
func (c *Compactor) SetDefaultModel(m models.AgnoModelInterface) {
	if c.cfg.Model == nil {
		c.cfg.Model = m
	}
}

// Stats returns running totals of the folds this Compactor performed.
func (c *Compactor) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// CountTokens counts a request with the configured counter or the local estimate.
func (c *Compactor) CountTokens(messages []models.Message) int {
	return c.cfg.countRequest(messages)
}

// ShouldCompact reports whether the request has reached the threshold.
func (c *Compactor) ShouldCompact(request []models.Message) bool {
	if !c.cfg.HasThreshold() {
		return false
	}
	return c.cfg.countRequest(request) >= c.cfg.CompactAtTokens
}

// ---------------------------------------------------------------------------
// Boundary planning
// ---------------------------------------------------------------------------

// keepFromIndex is the index the kept tail starts at for keepRuns runs.
// Returns -1 when the tail already covers everything (nothing to fold).
func keepFromIndex(messages []models.Message, keepRuns int) int {
	if keepRuns <= 0 {
		return len(messages)
	}
	var users []int
	for i, m := range messages {
		if m.Role == models.TypeUserRole && !IsInjectedSummary(m) {
			users = append(users, i)
		}
	}
	if len(users) <= keepRuns {
		return -1
	}
	return users[len(users)-keepRuns]
}

func boundaryFor(cfg Config, messages []models.Message, minIndex, overhead int) int {
	if cfg.UncompactedTokens > 0 {
		budget := cfg.UncompactedTokens
		if limit := cfg.tailLimitAfter(overhead); limit >= 0 && limit < budget {
			budget = limit
		}
		cut := ChooseBoundary(messages, budget, -1, minIndex)
		return outsideNewestExchange(messages, cut, minIndex)
	}
	keepFrom := keepFromIndex(messages, cfg.UncompactedRuns)
	boundary := -1
	if keepFrom >= 0 {
		boundary = ChooseBoundary(messages, 0, keepFrom, minIndex)
	}
	limit := cfg.tailLimitAfter(overhead)
	if limit < 0 {
		return boundary
	}
	tailStart := minIndex
	if boundary >= 0 {
		tailStart = boundary
	}
	if tailStart > len(messages) {
		tailStart = len(messages)
	}
	if EstimateTokens(messages[tailStart:]) <= limit {
		return boundary
	}
	cut := outsideNewestExchange(messages, ChooseBoundary(messages, limit, -1, minIndex), minIndex)
	if cut < 0 || cut <= tailStart {
		return boundary
	}
	return cut
}

// ResolveBoundary is where previous cut, as an index into history; 0 when it
// does not resolve (fail open).
func ResolveBoundary(history []models.Message, previous *Record) int {
	if previous == nil || previous.FirstKeptMessageID == "" {
		return 0
	}
	if idx := FindMessage(history, previous.FirstKeptMessageID, 0); idx >= 0 {
		return idx
	}
	return 0
}

func (c *Compactor) summaryAllowance(previous *Record) int {
	if c.cfg.CompactedTokenBudget > 0 {
		return c.cfg.CompactedTokenBudget
	}
	if previous != nil {
		return EstimateTextTokens(previous.Summary)
	}
	return 0
}

func tailSetting(cfg Config) string {
	if cfg.UncompactedTokens > 0 {
		return fmt.Sprintf("UncompactedTokens=%d", cfg.UncompactedTokens)
	}
	return fmt.Sprintf("UncompactedRuns=%d", cfg.UncompactedRuns)
}

// Plan returns the boundary a fold of history would use, or -1 with the
// status and reason it declined.
func (c *Compactor) Plan(history []models.Message, previous *Record, overheadTokens int) (int, Status, string) {
	return c.plan(c.cfg, history, previous, overheadTokens, true)
}

func (c *Compactor) plan(cfg Config, history []models.Message, previous *Record, overhead int, checkRatio bool) (int, Status, string) {
	if len(history) == 0 {
		return -1, StatusNoHistory, "There is no history to compact yet."
	}
	already := ResolveBoundary(history, previous)
	overhead += c.summaryAllowance(previous)
	boundary := boundaryFor(cfg, history, already, overhead)
	if boundary < 0 || boundary <= already {
		if previous == nil {
			return -1, StatusNothingToFold, fmt.Sprintf("Nothing to fold yet - %s covers the whole conversation, "+
				"so there is no history before the kept tail. Lower it to fold sooner.", tailSetting(cfg))
		}
		return -1, StatusAlreadyCompacted, "No safe cut past the previous fold yet - the conversation has not grown enough since then."
	}
	if ratio := cfg.foldRatio(); checkRatio && ratio > 0 {
		fold := EstimateTokens(history[already:boundary])
		keep := EstimateTokens(history[boundary:])
		if keep < 1 {
			keep = 1
		}
		if got := float64(fold) / float64(keep); got < ratio {
			return -1, StatusNotWorthIt, fmt.Sprintf("This fold would replace %d tokens against a %d-token tail "+
				"(ratio %.2f, needs %v), so the context would not shrink. Continue the conversation, lower "+
				"MinFoldRatio, or set EnforceMinFoldRatio=false, to fold sooner.", fold, keep, got, cfg.MinFoldRatio)
		}
	}
	return boundary, StatusCompacted, ""
}

// ---------------------------------------------------------------------------
// Folding
// ---------------------------------------------------------------------------

// Options tune a single Compact call.
type Options struct {
	// Previous is the record currently in force; its summary is folded into the new one.
	Previous *Record
	// Prefix is the context sent in front of the history (system prompt), used
	// to size the overhead and measure tokens before/after.
	Prefix []models.Message
	// RunID stamps the record.
	RunID string
	// Model is used when the Config has none.
	Model models.AgnoModelInterface
	// ForOverflow plans a fold after the provider rejected a request as too
	// long: the min-fold ratio is ignored and a run tail is shrunk one run at a
	// time until a fold is possible.
	ForOverflow bool
}

// Compact folds the head of history into a summary. A decline is reported in
// the Result status with a nil error; the error is non-nil only when ctx is done.
func (c *Compactor) Compact(ctx context.Context, history []models.Message, opts Options) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	overhead := EstimateTokens(opts.Prefix)
	cfg := c.cfg
	var boundary int
	var status Status
	var reason string
	if opts.ForOverflow {
		boundary, cfg, status, reason = c.planOverflow(history, opts.Previous, overhead)
	} else {
		boundary, status, reason = c.plan(cfg, history, opts.Previous, overhead, true)
	}
	if boundary < 0 {
		return &Result{Status: status, Message: reason}, nil
	}

	already := ResolveBoundary(history, opts.Previous)
	toCompact := history[already:boundary]

	model := cfg.Model
	if model == nil {
		model = opts.Model
	}
	if model == nil {
		return &Result{Status: StatusSummaryFailed, Message: "No compaction model available."}, nil
	}
	previousSummary := ""
	if opts.Previous != nil {
		previousSummary = opts.Previous.Summary
	}
	resp, err := model.Invoke(ctx, c.summaryMessages(toCompact, previousSummary))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return &Result{Status: StatusSummaryFailed, Message: fmt.Sprintf("Error compacting conversation: %v", err)}, nil
	}
	summary := ""
	if resp != nil {
		summary = strings.TrimSpace(resp.Content)
	}
	if summary == "" {
		return &Result{Status: StatusSummaryFailed, Message: "The summarizer returned an empty summary."}, nil
	}
	summary = c.fitToBudget(summary)

	record := &Record{
		ID:                 strings.ReplaceAll(uuid.New().String(), "-", ""),
		RunID:              opts.RunID,
		MessagesCompacted:  len(toCompact),
		Summary:            summary,
		FirstKeptMessageID: MessageFingerprint(history[boundary]),
		CreatedAt:          time.Now().Unix(),
	}
	kept := append([]models.Message(nil), history[boundary:]...)
	before := append(append([]models.Message(nil), opts.Prefix...), history[already:]...)
	after := append(append([]models.Message(nil), opts.Prefix...), SummaryMessage(record))
	after = append(after, kept...)
	record.TokensBefore = EstimateTokens(before)
	record.TokensAfter = EstimateTokens(after)
	if cfg.HasThreshold() && record.TokensAfter >= cfg.CompactAtTokens {
		log.Printf("Compaction: after folding, the context is still %d tokens, at or over CompactAtTokens=%d, "+
			"so the next run will fold again.", record.TokensAfter, cfg.CompactAtTokens)
	}

	c.mu.Lock()
	c.stats.Add(record)
	c.mu.Unlock()

	return &Result{
		Status:  StatusCompacted,
		Message: fmt.Sprintf("Compacted %d messages (%d -> %d tokens).", record.MessagesCompacted, record.TokensBefore, record.TokensAfter),
		Record:  record,
		Kept:    kept,
	}, nil
}

// planOverflow keeps the configured tail when it works; only when folding in
// front of it reclaims too little is the run tail given up, one run at a time.
func (c *Compactor) planOverflow(history []models.Message, previous *Record, overhead int) (int, Config, Status, string) {
	if len(history) == 0 {
		return -1, c.cfg, StatusNoHistory, "There is no history to compact yet."
	}
	var tails []int
	if c.cfg.UncompactedTokens > 0 {
		tails = []int{c.cfg.UncompactedRuns}
	} else {
		for k := c.cfg.UncompactedRuns; k >= 0; k-- {
			tails = append(tails, k)
		}
	}
	chosen, chosenCfg := -1, c.cfg
	status, reason := StatusNothingToFold, "No safe cut left to make - the most recent turn alone is too large to send."
	already := ResolveBoundary(history, previous)
	for _, keep := range tails {
		cand := c.cfg
		if c.cfg.UncompactedTokens == 0 {
			cand.UncompactedRuns = keep
		}
		b, st, r := c.plan(cand, history, previous, overhead, false)
		if b < 0 {
			if keep == c.cfg.UncompactedRuns {
				status, reason = st, r
			}
			continue
		}
		chosen, chosenCfg = b, cand
		if EstimateTokens(history[already:b]) >= EstimateTokens(history[b:]) {
			break
		}
	}
	if chosen < 0 {
		return -1, c.cfg, status, reason
	}
	return chosen, chosenCfg, StatusCompacted, ""
}

// SummaryMessage is the message that stands in for everything compacted away.
func SummaryMessage(r *Record) models.Message {
	return models.Message{Role: models.TypeUserRole, Content: SummaryPrefix + r.Summary}
}

// ApplyRecord derives the model-bound history for one call: the summary,
// then the kept tail. When the record's anchor does not resolve, the view
// fails open to the history as given. Leading system messages pass verbatim.
func ApplyRecord(history []models.Message, r *Record) []models.Message {
	lead := LeadingSystemCount(history)
	boundary := -1
	if r != nil && r.Summary != "" && r.FirstKeptMessageID != "" {
		boundary = FindMessage(history, r.FirstKeptMessageID, lead)
	}
	view := append([]models.Message(nil), history[:lead]...)
	start := lead
	if boundary >= 0 {
		view = append(view, SummaryMessage(r))
		start = boundary
	}
	for _, m := range history[start:] {
		if boundary >= 0 && IsInjectedSummary(m) {
			continue
		}
		view = append(view, m)
	}
	return view
}

// ---------------------------------------------------------------------------
// Summarizer prompt
// ---------------------------------------------------------------------------

// RenderMessage renders one message as a markdown block for the summarizer.
func RenderMessage(m models.Message) string {
	parts := []string{"## " + string(m.Role)}
	if m.Content != "" {
		content := m.Content
		if m.Role == models.TypeToolRole && len(content) > maxRenderedToolResultChars {
			content = content[:maxRenderedToolResultChars] +
				fmt.Sprintf("\n... [clipped, %d more characters]", len(m.Content)-maxRenderedToolResultChars)
		}
		parts = append(parts, content)
	}
	for _, tc := range m.ToolCalls {
		parts = append(parts, fmt.Sprintf("**calls** `%s`: %s", tc.Function.Name, tc.Function.Arguments))
	}
	return strings.Join(parts, "\n\n")
}

// RenderMessages renders messages as readable markdown.
func RenderMessages(messages []models.Message) string {
	out := make([]string, 0, len(messages))
	for _, m := range messages {
		out = append(out, RenderMessage(m))
	}
	return strings.Join(out, "\n\n")
}

// trimForSummary drops the oldest messages that do not fit the summarizer budget.
func trimForSummary(messages []models.Message) []models.Message {
	budget := DefaultSummarizeCharBudget
	start := len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		size := len(RenderMessage(messages[i])) + 2
		if budget-size < 0 && start < len(messages) {
			break
		}
		budget -= size
		start = i
	}
	return messages[start:]
}

func (c *Compactor) summaryMessages(messages []models.Message, previous string) []models.Message {
	transcript := RenderMessages(trimForSummary(messages))
	if previous != "" {
		transcript = "Summary of the conversation before this point:\n" + previous +
			"\n\nConversation since then:\n" + transcript
	}
	lengthRule := LengthRuleCompact
	if b := c.cfg.CompactedTokenBudget; b > 0 {
		lengthRule = strings.NewReplacer(
			"{budget_tokens}", strconv.Itoa(b),
			"{budget_words}", strconv.Itoa(int(float64(b)*0.75+0.5)),
			"{budget_characters}", strconv.Itoa(b*4),
		).Replace(LengthRuleBudget)
	}
	prompt := strings.Replace(DefaultCompactionPrompt, "{length_rule}", lengthRule, 1)
	if c.cfg.Instructions != "" {
		prompt += "\n\nAdditional instructions:\n" + c.cfg.Instructions
	}
	return []models.Message{
		{Role: models.TypeSystemRole, Content: prompt},
		{Role: models.TypeUserRole, Content: transcript},
	}
}

// fitToBudget cuts the summary from the end to CompactedTokenBudget when one
// is set and enforced, keeping whole lines and adding a note for what was cut.
func (c *Compactor) fitToBudget(summary string) string {
	budget := c.cfg.CompactedTokenBudget
	if budget <= 0 || !c.cfg.enforceBudget() {
		return summary
	}
	fits := func(s string) bool { return EstimateTextTokens(s) <= budget }
	if fits(summary) {
		return summary
	}
	withNote := func(kept []string) string {
		for len(kept) > 0 {
			last := strings.TrimSpace(kept[len(kept)-1])
			if last == "" || strings.HasPrefix(last, "#") {
				kept = kept[:len(kept)-1]
				continue
			}
			break
		}
		if len(kept) == 0 {
			return SummaryCutNote
		}
		return strings.Join(append(append([]string(nil), kept...), "", SummaryCutNote), "\n")
	}
	longest := func(count int, ok func(int) bool) int {
		lo, hi := 0, count
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if ok(mid) {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		return lo
	}
	lines := strings.Split(summary, "\n")
	kept := longest(len(lines), func(n int) bool { return fits(withNote(lines[:n])) })
	fitted := withNote(lines[:kept])
	if kept < len(lines) && strings.TrimSpace(lines[kept]) != "" && !strings.HasPrefix(strings.TrimSpace(lines[kept]), "#") {
		words := strings.Split(lines[kept], " ")
		build := func(n int) string {
			return withNote(append(append([]string(nil), lines[:kept]...), strings.Join(words[:n], " ")+" ..."))
		}
		if taken := longest(len(words), func(n int) bool { return fits(build(n)) }); taken > 0 {
			fitted = build(taken)
		}
	}
	if !fits(fitted) {
		words := strings.Fields(summary)
		n := longest(len(words), func(n int) bool { return fits(strings.Join(words[:n], " ")) })
		fitted = strings.Join(words[:n], " ")
	}
	return fitted
}
