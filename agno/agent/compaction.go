package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/compaction"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/storage"
)

// sessionDataCompactionKey is where the compaction record in force is kept in
// the session's SessionData.
const sessionDataCompactionKey = "compaction"

// requestParts splits a model request into the context in front of the
// history (system messages), the replayed history, and what follows it (the
// current input), so the history can be swapped after a compaction.
type requestParts struct {
	prefix  []models.Message
	history []models.Message
	suffix  []models.Message
}

// splitRequest splits messages built by prepareMessages: leading system
// messages, then history, then the final user input.
func splitRequest(messages []models.Message) requestParts {
	lead := compaction.LeadingSystemCount(messages)
	end := len(messages)
	if end > lead {
		end--
	}
	return requestParts{
		prefix:  append([]models.Message(nil), messages[:lead]...),
		history: append([]models.Message(nil), messages[lead:end]...),
		suffix:  append([]models.Message(nil), messages[end:]...),
	}
}

func (p requestParts) join() []models.Message {
	out := make([]models.Message, 0, len(p.prefix)+len(p.history)+len(p.suffix))
	out = append(out, p.prefix...)
	out = append(out, p.history...)
	return append(out, p.suffix...)
}

// historyView is the replayed history sent to the model: the compaction
// summary (when one is in force) followed by the kept messages.
func (a *Agent) historyView() []models.Message {
	if !a.addHistoryToMessages {
		return nil
	}
	var out []models.Message
	if a.compactionRecord != nil {
		out = append(out, compaction.SummaryMessage(a.compactionRecord))
	}
	if len(a.messages) > 0 {
		out = append(out, a.filterToolCallsFromHistory(a.messages)...)
	}
	return out
}

// historyRunLimit is the number of runs replayed. With a compaction threshold,
// compaction owns the replay window and NumHistoryRuns does not apply.
func (a *Agent) historyRunLimit() int {
	if a.compactor != nil && a.compactor.Config().HasThreshold() {
		return 0
	}
	return a.numHistoryRuns
}

// trimHistory keeps only the most recent runs in memory.
func (a *Agent) trimHistory() {
	if limit := a.historyRunLimit(); limit > 0 {
		maxMessages := limit * 2 // user + assistant per run
		if len(a.messages) > maxMessages {
			a.messages = a.messages[len(a.messages)-maxMessages:]
		}
	}
}

// maybeCompactBeforeRun folds older history when the request crosses the
// compaction threshold, rewriting parts.history in place.
func (a *Agent) maybeCompactBeforeRun(parts *requestParts) {
	if a.compactor == nil || !a.addHistoryToMessages || !a.compactor.Config().HasThreshold() {
		return
	}
	if len(a.messages) == 0 || !a.compactor.ShouldCompact(parts.join()) {
		return
	}
	res, err := a.compactor.Compact(a.ctx, a.messages, compaction.Options{
		Previous: a.compactionRecord,
		Prefix:   parts.prefix,
		RunID:    a.currentRunID(),
		Model:    a.model,
	})
	if err != nil || res == nil {
		return
	}
	if !res.Compacted() {
		if a.debug {
			log.Printf("Compaction: %s", res.Message)
		}
		return
	}
	a.applyCompaction(res)
	parts.history = a.historyView()
}

// recoverFromOverflow folds harder after the provider rejected a request as
// too long. It returns true when the request shrank and should be retried.
func (a *Agent) recoverFromOverflow(parts *requestParts, err error) bool {
	if a.compactor == nil || !a.compactor.Config().OnContextOverflow || !models.IsContextOverflowError(err) {
		return false
	}
	if len(a.messages) == 0 {
		log.Printf("Compaction: the request exceeded the model's context window and there is no history to fold.")
		return false
	}
	before := a.compactor.CountTokens(parts.join())
	res, cerr := a.compactor.Compact(a.ctx, a.messages, compaction.Options{
		Previous:    a.compactionRecord,
		Prefix:      parts.prefix,
		RunID:       a.currentRunID(),
		Model:       a.model,
		ForOverflow: true,
	})
	if cerr != nil || res == nil || !res.Compacted() {
		if res != nil {
			log.Printf("Compaction: context overflow, but no fold was possible: %s", res.Message)
		}
		return false
	}
	candidate := *parts
	candidate.history = append([]models.Message{compaction.SummaryMessage(res.Record)}, a.filterToolCallsFromHistory(res.Kept)...)
	after := a.compactor.CountTokens(candidate.join())
	if after >= before {
		log.Printf("Compaction: folding after a context-window rejection did not shrink the request (%d -> %d tokens), not retrying.", before, after)
		return false
	}
	a.applyCompaction(res)
	parts.history = a.historyView()
	log.Printf("Compaction: request exceeded the context window, folded %d messages (%d -> %d tokens) and retrying once.",
		res.Record.MessagesCompacted, before, after)
	return true
}

// applyCompaction makes a successful fold the record in force: the replayed
// history becomes the kept tail, the record is attached to the current run
// and persisted in the session.
func (a *Agent) applyCompaction(res *compaction.Result) {
	a.messages = res.Kept
	a.compactionRecord = res.Record
	if a.currentRun != nil {
		a.currentRun.compaction = res.Record
	}
	if err := a.persistCompaction(); err != nil && a.debug {
		log.Printf("Warning: failed to persist compaction: %v", err)
	}
}

func (a *Agent) currentRunID() string {
	if a.currentRun != nil {
		return a.currentRun.id
	}
	return ""
}

// persistCompaction stores the record in force in the session's SessionData.
func (a *Agent) persistCompaction() error {
	if a.db == nil || a.sessionID == "" || a.compactionRecord == nil {
		return nil
	}
	session, err := a.db.ReadSession(a.ctx, a.sessionID)
	if err != nil && err.Error() != "session not found" {
		return fmt.Errorf("failed to read session: %w", err)
	}
	if session == nil {
		session = storage.NewAgentSession(a.sessionID, a.userID, "default-agent")
		if cerr := a.db.CreateSession(a.ctx, session); cerr != nil {
			return fmt.Errorf("failed to create session: %w", cerr)
		}
	}
	if session.SessionData == nil {
		session.SessionData = make(map[string]interface{})
	}
	session.SessionData[sessionDataCompactionKey] = compaction.RecordToMap(a.compactionRecord)
	session.UpdatedAt = time.Now().Unix()
	return a.db.UpdateSession(a.ctx, session)
}

// loadCompaction restores the record in force from a stored session.
func (a *Agent) loadCompaction(session *storage.AgentSession) {
	if a.compactor == nil || session == nil || session.SessionData == nil {
		return
	}
	a.compactionRecord = compaction.RecordFromValue(session.SessionData[sessionDataCompactionKey])
}

// locateRunMessage maps an index into the history built by
// buildMessageHistoryFromRuns to (run index, message offset within that run).
func locateRunMessage(runs []*storage.AgentRun, idx int) (int, int) {
	pos := 0
	for i, run := range runs {
		n := 0
		if run.UserMessage != "" {
			n++
		}
		if run.AgentMessage != "" {
			n++
		}
		if idx < pos+n {
			return i, idx - pos
		}
		pos += n
	}
	return len(runs), 0
}

// Compact folds this session's history now, without waiting for the size
// trigger (port of Python Agent.compact). A decline is reported in the
// result's Status/Message with a nil error.
func (a *Agent) Compact(ctx context.Context) (*compaction.Result, error) {
	if a.compactor == nil {
		return &compaction.Result{Status: compaction.StatusNotEnabled, Message: "Compaction is not enabled on this agent."}, nil
	}
	if ctx == nil {
		ctx = a.ctx
	}
	if len(a.messages) == 0 {
		return &compaction.Result{Status: compaction.StatusNoHistory, Message: "There is no history to compact yet."}, nil
	}
	prev := a.ctx
	a.ctx = ctx
	defer func() { a.ctx = prev }()
	res, err := a.compactor.Compact(ctx, a.messages, compaction.Options{
		Previous: a.compactionRecord,
		RunID:    a.currentRunID(),
		Model:    a.model,
	})
	if err != nil {
		return nil, err
	}
	if res.Compacted() {
		a.messages = res.Kept
		a.compactionRecord = res.Record
		if perr := a.persistCompaction(); perr != nil {
			return res, perr
		}
	}
	return res, nil
}

// GetCompactionRecord returns the compaction record in force, or nil.
func (a *Agent) GetCompactionRecord() *compaction.Record { return a.compactionRecord }

// GetCompactionStats returns the running compaction totals for this agent.
func (a *Agent) GetCompactionStats() compaction.Stats {
	if a.compactor == nil {
		return compaction.Stats{}
	}
	return a.compactor.Stats()
}
